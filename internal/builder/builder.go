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
	ClassFile  *classfile.ClassFile
	Hierarchy  *classfile.ClassHierarchy
	Methods    []*ir.MethodIR
	OutputPath string
	TargetOS   string
	TargetArch string
	Static     bool
	Entrypoint string
	OnProgress func(step int, total int, message string)
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

// Build compiles a JVM class or class hierarchy into a single static binary.
func Build(opts BuildOptions) (*BuildResult, error) {
	start := time.Now()

	hierarchy := opts.Hierarchy
	if hierarchy == nil {
		if opts.ClassFile == nil {
			return nil, errors.New("classfile or hierarchy must be provided")
		}
		hierarchy = classfile.NewClassHierarchy()
		hierarchy.Add(opts.ClassFile)
		hierarchy.MainClass = opts.ClassFile.ThisClass
	}
	if len(hierarchy.Classes) == 0 {
		return nil, errors.New("no classes to compile")
	}

	targetOS, targetArch, err := ParseTarget(opts.TargetOS)
	if err != nil {
		return nil, err
	}

	primaryClass := selectPrimaryClass(hierarchy, opts.Entrypoint)
	if primaryClass == nil {
		return nil, errors.New("no classes to compile")
	}

	outPath := opts.OutputPath
	if outPath == "" {
		outPath = strings.ToLower(codegen.CleanClassName(primaryClass.ThisClass))
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
		opts.OnProgress(1, 4, fmt.Sprintf("Grinding %d classes and constant pools...", len(hierarchy.Classes)))
	}

	modules, err := lowerHierarchy(hierarchy)
	if err != nil {
		return nil, err
	}

	if opts.OnProgress != nil {
		opts.OnProgress(2, 4, "Brewing SSA IR and cross-class call graph...")
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

	generatedFiles, err := codegen.GenerateModule(modules, "app")
	if err != nil {
		return nil, fmt.Errorf("generate Go module: %w", err)
	}
	for name, content := range generatedFiles {
		if err := os.WriteFile(filepath.Join(appDir, name), content, 0644); err != nil {
			return nil, fmt.Errorf("write app/%s: %w", name, err)
		}
	}

	methodsByClass := make(map[string][]*ir.MethodIR, len(modules))
	for _, mod := range modules {
		methodsByClass[mod.ClassFile.ThisClass] = mod.Methods
	}

	springControllers := codegen.DetectSpringControllers(hierarchy, methodsByClass)
	var mainSrc []byte
	if len(springControllers) > 0 {
		mainSrc = codegen.GenerateSpringMainGo(springControllers)
	} else {
		entry, err := findEntrypointInHierarchy(hierarchy, modules, opts.Entrypoint)
		if err != nil {
			return nil, err
		}
		mainSrc = generateMainGo(entry)
	}

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
		Class:      primaryClass.ThisClass,
	}, nil
}

func lowerHierarchy(hierarchy *classfile.ClassHierarchy) ([]codegen.ClassModule, error) {
	modules := make([]codegen.ClassModule, 0, len(hierarchy.Classes))
	for _, name := range hierarchy.Names() {
		cf := hierarchy.Classes[name]
		methods := make([]*ir.MethodIR, 0, len(cf.Methods))
		for _, m := range cf.Methods {
			code := m.FindCode()
			if code == nil {
				continue
			}
			mir, err := ir.LowerMethodWithExceptions(
				cf.ConstantPool,
				cf.ThisClass,
				m.Name,
				m.Descriptor,
				code.Code,
				m.IsStatic(),
				code.ExceptionTable,
			)
			if err != nil {
				return nil, fmt.Errorf("failed compiling method %s.%s: %w", cf.ThisClass, m.Name, err)
			}
			methods = append(methods, mir)
		}
		modules = append(modules, codegen.ClassModule{ClassFile: cf, Methods: methods})
	}
	return modules, nil
}

func selectPrimaryClass(hierarchy *classfile.ClassHierarchy, customEntry string) *classfile.ClassFile {
	if cf := hierarchy.Get(hierarchy.MainClass); cf != nil {
		return cf
	}
	names := hierarchy.Names()
	if customEntry != "" {
		for _, name := range names {
			cf := hierarchy.Classes[name]
			for _, m := range cf.Methods {
				if strings.EqualFold(m.Name, customEntry) || strings.EqualFold(codegen.CleanMemberName(m.Name), customEntry) {
					return cf
				}
			}
		}
	}
	for _, name := range names {
		cf := hierarchy.Classes[name]
		for _, m := range cf.Methods {
			if m.Name == "main" {
				return cf
			}
		}
	}
	if len(names) > 0 {
		return hierarchy.Classes[names[0]]
	}
	return nil
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

func findEntrypointInHierarchy(hierarchy *classfile.ClassHierarchy, modules []codegen.ClassModule, custom string) (*entrypoint, error) {
	methodNames := codegen.ResolveModuleMethodNames(modules)
	classes := hierarchyClassOrder(hierarchy)

	if custom != "" {
		for _, cf := range classes {
			for i := range cf.Methods {
				m := &cf.Methods[i]
				if strings.EqualFold(m.Name, custom) || strings.EqualFold(codegen.CleanMemberName(m.Name), custom) {
					return makeEntrypoint(cf, m, methodNames), nil
				}
			}
		}
		return nil, fmt.Errorf("custom entrypoint %q not found in module", custom)
	}

	// Priority 1: main([Ljava/lang/String;)V or ([Ljava/lang/String;)I
	for _, cf := range classes {
		for i := range cf.Methods {
			m := &cf.Methods[i]
			if m.Name == "main" && (m.Descriptor == "([Ljava/lang/String;)V" || m.Descriptor == "([Ljava/lang/String;)I") {
				return makeEntrypoint(cf, m, methodNames), nil
			}
		}
	}

	// Priority 2: main()V or main()I
	for _, cf := range classes {
		for i := range cf.Methods {
			m := &cf.Methods[i]
			if m.Name == "main" && (m.Descriptor == "()V" || m.Descriptor == "()I") {
				return makeEntrypoint(cf, m, methodNames), nil
			}
		}
	}

	// Priority 3: any other method named "main"
	for _, cf := range classes {
		for i := range cf.Methods {
			m := &cf.Methods[i]
			if m.Name == "main" {
				return makeEntrypoint(cf, m, methodNames), nil
			}
		}
	}

	// Priority 4: parameterless run or start method
	for _, cf := range classes {
		for i := range cf.Methods {
			m := &cf.Methods[i]
			if (m.Name == "run" || m.Name == "start" || m.Name == "Run" || m.Name == "Start") && (m.Descriptor == "()V" || m.Descriptor == "()I") {
				return makeEntrypoint(cf, m, methodNames), nil
			}
		}
	}

	return nil, fmt.Errorf("module does not declare a 'public static void main' entrypoint (use -entry to specify a custom method)")
}

func hierarchyClassOrder(hierarchy *classfile.ClassHierarchy) []*classfile.ClassFile {
	seen := make(map[string]bool)
	classes := make([]*classfile.ClassFile, 0, len(hierarchy.Classes))
	if cf := hierarchy.Get(hierarchy.MainClass); cf != nil {
		classes = append(classes, cf)
		seen[cf.ThisClass] = true
	}
	for _, name := range hierarchy.Names() {
		if !seen[name] {
			classes = append(classes, hierarchy.Classes[name])
		}
	}
	return classes
}

func makeEntrypoint(cf *classfile.ClassFile, m *classfile.MemberInfo, methodNames map[string]string) *entrypoint {
	md, _ := ir.ParseMethodDescriptor(m.Descriptor)
	goName := methodNames[codegen.MethodKey(cf.ThisClass, m.Name, m.Descriptor)]
	if goName == "" {
		goName = codegen.CleanMemberName(m.Name)
	}
	hasArgs := len(md.Params) > 0 && strings.HasPrefix(md.Params[0], "[")
	returnsInt := md.Return == "I" || md.Return == "Z"
	return &entrypoint{
		goName:     goName,
		isStatic:   m.IsStatic(),
		hasArgs:    hasArgs,
		returnsInt: returnsInt,
		className:  codegen.CleanClassName(cf.ThisClass),
	}
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
