package builder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goRuntime "runtime"
	"strings"
	"time"

	"github.com/JohnEsleyer/barista/internal/classfile"
	"github.com/JohnEsleyer/barista/internal/codegen"
	"github.com/JohnEsleyer/barista/internal/ir"
	baristaRuntime "github.com/JohnEsleyer/barista/pkg/runtime"
)

// BuildOptions configures the ahead-of-time compilation pipeline.
type BuildOptions struct {
	ClassFile   *classfile.ClassFile
	Methods     []*ir.MethodIR
	OutputPath  string
	TargetOS    string
	TargetArch  string
	Static      bool
	Entrypoint  string
	OnProgress  func(step int, total int, message string)
}

// BuildResult captures metrics from a successful binary compilation.
type BuildResult struct {
	BinaryPath string
	Size       int64
	Duration   time.Duration
	TargetOS   string
	TargetArch string
	Class      string
}

// ParseTarget parses a "os/arch" target string (e.g. "linux/amd64").
func ParseTarget(target string) (string, string, error) {
	if target == "" {
		return goRuntime.GOOS, goRuntime.GOARCH, nil
	}
	parts := strings.Split(target, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid target %q (must be os/arch, e.g. linux/amd64)", target)
	}
	return parts[0], parts[1], nil
}

// Build compiles a JVM class into a single static binary.
func Build(opts BuildOptions) (*BuildResult, error) {
	start := time.Now()

	if opts.ClassFile == nil {
		return nil, errors.New("classfile cannot be nil")
	}

	targetOS, targetArch, err := ParseTarget(opts.TargetOS)
	if err != nil {
		return nil, err
	}

	outPath := opts.OutputPath
	if outPath == "" {
		outPath = strings.ToLower(codegen.CleanClassName(opts.ClassFile.ThisClass))
	}
	if targetOS == "windows" && !strings.HasSuffix(strings.ToLower(outPath), ".exe") {
		outPath += ".exe"
	}
	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return nil, fmt.Errorf("resolve output path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absOut), 0755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	if opts.OnProgress != nil {
		opts.OnProgress(1, 4, "Grinding bytecode and constant pool...")
	}

	loweredMethods := opts.Methods
	if len(loweredMethods) == 0 {
		for _, m := range opts.ClassFile.Methods {
			code := m.FindCode()
			if code == nil {
				continue
			}
			mir, err := ir.LowerMethodWithExceptions(
				opts.ClassFile.ConstantPool,
				opts.ClassFile.ThisClass,
				m.Name,
				m.Descriptor,
				code.Code,
				m.IsStatic(),
				code.ExceptionTable,
			)
			if err != nil {
				return nil, fmt.Errorf("failed compiling method %s: %w", m.Name, err)
			}
			loweredMethods = append(loweredMethods, mir)
		}
	}

	if opts.OnProgress != nil {
		opts.OnProgress(2, 4, "Brewing SSA IR and control flow graph...")
	}

	workDir, err := os.MkdirTemp("", "barista-build-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary build workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	if opts.OnProgress != nil {
		opts.OnProgress(3, 4, "Synthesizing Go AST and native shims...")
	}

	runtimeDest := filepath.Join(workDir, "barista_runtime")
	if err := baristaRuntime.ExtractShims(runtimeDest); err != nil {
		return nil, fmt.Errorf("extract runtime shims: %w", err)
	}

	rootGoMod := `module barista_app

go 1.24

require github.com/JohnEsleyer/barista v0.0.0
replace github.com/JohnEsleyer/barista => ./barista_runtime
`
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(rootGoMod), 0644); err != nil {
		return nil, fmt.Errorf("write app go.mod: %w", err)
	}

	appDir := filepath.Join(workDir, "app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, fmt.Errorf("create app package dir: %w", err)
	}

	goSrc, err := codegen.GenerateGoWithPackage(opts.ClassFile, loweredMethods, "app")
	if err != nil {
		return nil, fmt.Errorf("generate Go source: %w", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "class.go"), goSrc, 0644); err != nil {
		return nil, fmt.Errorf("write app class.go: %w", err)
	}

	entry, err := findEntrypoint(opts.ClassFile, loweredMethods, opts.Entrypoint)
	if err != nil {
		return nil, err
	}

	mainSrc := generateMainGo(entry)
	if err := os.WriteFile(filepath.Join(workDir, "main.go"), mainSrc, 0644); err != nil {
		return nil, fmt.Errorf("write main.go: %w", err)
	}

	if opts.OnProgress != nil {
		opts.OnProgress(4, 4, "Linking native static binary (-s -w)...")
	}

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", absOut, ".")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if targetOS != "" {
		cmd.Env = append(cmd.Env, "GOOS="+targetOS)
	}
	if targetArch != "" {
		cmd.Env = append(cmd.Env, "GOARCH="+targetArch)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go build toolchain error: %w\n%s", err, string(out))
	}

	fi, err := os.Stat(absOut)
	if err != nil {
		return nil, fmt.Errorf("stat output binary: %w", err)
	}

	return &BuildResult{
		BinaryPath: absOut,
		Size:       fi.Size(),
		Duration:   time.Since(start),
		TargetOS:   targetOS,
		TargetArch: targetArch,
		Class:      opts.ClassFile.ThisClass,
	}, nil
}

// Serve compiles the class file in memory and executes it immediately.
func Serve(ctx context.Context, opts BuildOptions, appArgs []string, stdout, stderr os.FileInfo, stdin *os.File) error {
	return ServeWithIO(ctx, opts, appArgs, os.Stdout, os.Stderr, os.Stdin)
}

// ServeWithIO compiles the class file and executes it with customized standard I/O streams.
func ServeWithIO(ctx context.Context, opts BuildOptions, appArgs []string, stdout, stderr any, stdin any) error {
	binDir, err := os.MkdirTemp("", "barista-serve-bin-*")
	if err != nil {
		return fmt.Errorf("create serve workspace: %w", err)
	}
	defer os.RemoveAll(binDir)

	binName := "app"
	if goRuntime.GOOS == "windows" {
		binName = "app.exe"
	}
	opts.OutputPath = filepath.Join(binDir, binName)

	res, err := Build(opts)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, res.BinaryPath, appArgs...)
	if out, ok := stdout.(interface{ Write([]byte) (int, error) }); ok {
		cmd.Stdout = out
	}
	if errOut, ok := stderr.(interface{ Write([]byte) (int, error) }); ok {
		cmd.Stderr = errOut
	}
	if in, ok := stdin.(interface{ Read([]byte) (int, error) }); ok {
		cmd.Stdin = in
	}

	return cmd.Run()
}

type entrypoint struct {
	goName     string
	isStatic   bool
	hasArgs    bool
	returnsInt bool
	className  string
}

func findEntrypoint(cf *classfile.ClassFile, methods []*ir.MethodIR, custom string) (*entrypoint, error) {
	methodNames := codegen.ResolveMethodNames(cf.ThisClass, methods)

	if custom != "" {
		for _, m := range cf.Methods {
			if strings.EqualFold(m.Name, custom) || strings.EqualFold(codegen.CleanMemberName(m.Name), custom) {
				key := codegen.MethodKey(cf.ThisClass, m.Name, m.Descriptor)
				goName := methodNames[key]
				if goName == "" {
					goName = codegen.CleanMemberName(m.Name)
				}
				md, _ := ir.ParseMethodDescriptor(m.Descriptor)
				hasArgs := len(md.Params) > 0 && strings.HasPrefix(md.Params[0], "[")
				returnsInt := md.Return == "I" || md.Return == "Z"
				return &entrypoint{
					goName:     goName,
					isStatic:   m.IsStatic(),
					hasArgs:    hasArgs,
					returnsInt: returnsInt,
					className:  codegen.CleanClassName(cf.ThisClass),
				}, nil
			}
		}
		return nil, fmt.Errorf("custom entrypoint %q not found in class %s", custom, cf.ThisClass)
	}

	// Priority 1: main([Ljava/lang/String;)V or ([Ljava/lang/String;)I
	for _, m := range cf.Methods {
		if m.Name == "main" && (m.Descriptor == "([Ljava/lang/String;)V" || m.Descriptor == "([Ljava/lang/String;)I") {
			key := codegen.MethodKey(cf.ThisClass, m.Name, m.Descriptor)
			return &entrypoint{
				goName:     methodNames[key],
				isStatic:   m.IsStatic(),
				hasArgs:    true,
				returnsInt: strings.HasSuffix(m.Descriptor, ")I"),
				className:  codegen.CleanClassName(cf.ThisClass),
			}, nil
		}
	}

	// Priority 2: main()V or main()I
	for _, m := range cf.Methods {
		if m.Name == "main" && (m.Descriptor == "()V" || m.Descriptor == "()I") {
			key := codegen.MethodKey(cf.ThisClass, m.Name, m.Descriptor)
			return &entrypoint{
				goName:     methodNames[key],
				isStatic:   m.IsStatic(),
				hasArgs:    false,
				returnsInt: strings.HasSuffix(m.Descriptor, ")I"),
				className:  codegen.CleanClassName(cf.ThisClass),
			}, nil
		}
	}

	// Priority 3: any other method named "main"
	for _, m := range cf.Methods {
		if m.Name == "main" {
			key := codegen.MethodKey(cf.ThisClass, m.Name, m.Descriptor)
			md, _ := ir.ParseMethodDescriptor(m.Descriptor)
			hasArgs := len(md.Params) > 0 && strings.HasPrefix(md.Params[0], "[")
			return &entrypoint{
				goName:     methodNames[key],
				isStatic:   m.IsStatic(),
				hasArgs:    hasArgs,
				returnsInt: md.Return == "I" || md.Return == "Z",
				className:  codegen.CleanClassName(cf.ThisClass),
			}, nil
		}
	}

	// Priority 4: parameterless run or start method
	for _, m := range cf.Methods {
		if (m.Name == "run" || m.Name == "start" || m.Name == "Run" || m.Name == "Start") && (m.Descriptor == "()V" || m.Descriptor == "()I") {
			key := codegen.MethodKey(cf.ThisClass, m.Name, m.Descriptor)
			return &entrypoint{
				goName:     methodNames[key],
				isStatic:   m.IsStatic(),
				hasArgs:    false,
				returnsInt: strings.HasSuffix(m.Descriptor, ")I"),
				className:  codegen.CleanClassName(cf.ThisClass),
			}, nil
		}
	}

	return nil, fmt.Errorf("class %s does not declare a 'public static void main' entrypoint (use -entry to specify a custom method)", cf.ThisClass)
}

func generateMainGo(entry *entrypoint) []byte {
	var b strings.Builder
	b.WriteString(`package main

import (
	"fmt"
	"os"

	app "barista_app/app"
	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			if t := lang.AsThrowable(r); t != nil {
				fmt.Fprintf(os.Stderr, "Exception in thread \"main\" %s\n", t.ToString().GoString())
				os.Exit(1)
			}
			panic(r)
		}
	}()

`)

	if entry.hasArgs {
		b.WriteString(`	args := make([]*lang.String, len(os.Args)-1)
	for i, a := range os.Args[1:] {
		args[i] = lang.NewString(a)
	}
`)
	}

	callTarget := "app." + entry.goName
	if !entry.isStatic {
		b.WriteString(fmt.Sprintf("\treceiver := &app.%s{}\n", entry.className))
		callTarget = "receiver." + entry.goName
	}

	callArgs := ""
	if entry.hasArgs {
		callArgs = "args"
	}

	if entry.returnsInt {
		b.WriteString(fmt.Sprintf("\tcode := %s(%s)\n\tos.Exit(int(code))\n", callTarget, callArgs))
	} else {
		b.WriteString(fmt.Sprintf("\t%s(%s)\n", callTarget, callArgs))
	}
	b.WriteString("}\n")
	return []byte(b.String())
}
