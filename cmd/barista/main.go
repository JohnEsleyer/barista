// Command barista is the command-line interface for the Barista AOT compiler.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JohnEsleyer/barista/internal/builder"
	"github.com/JohnEsleyer/barista/internal/classfile"
	"github.com/JohnEsleyer/barista/internal/codegen"
	"github.com/JohnEsleyer/barista/internal/ir"
	"github.com/JohnEsleyer/barista/internal/ui"
)

const version = "0.1.0-dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "barista: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return errors.New("no command specified")
	}

	switch args[0] {
	case "grind":
		return runGrind(args[1:], stdout)
	case "brew":
		return runBrew(args[1:], stdout, stderr)
	case "build":
		return runBuild(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "barista %s\n", version)
		return nil
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Barista - compile JVM bytecode to static Go binaries

Usage:
  barista <command> [options]

Commands:
  grind <input>                  Disassemble a class or JAR and print Barista IR
  brew  [-o output.go|dir] <input> Compile a class or JAR to idiomatic Go source
  build [options] <input>        Compile a class or JAR to a stripped static binary
  serve [options] <input> [args] Compile in memory and execute immediately
  version                        Print the Barista version
  help                           Show this help text

Run "barista <command> -h" for command-specific options.
`)
}

func runGrind(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("grind", flag.ContinueOnError)
	fs.SetOutput(stdout)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: barista grind <input.class|input.jar>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("grind requires a path to a .class or .jar file")
	}

	classPath := fs.Arg(0)
	hierarchy, err := classfile.LoadClassOrJar(classPath)
	if err != nil {
		return fmt.Errorf("grind failed to parse input: %w", err)
	}

	for _, className := range hierarchy.Names() {
		cf := hierarchy.Classes[className]
		fmt.Fprintf(stdout, "Class: %s (version %d.%d)\n", cf.ThisClass, cf.MajorVersion, cf.MinorVersion)
		fmt.Fprintf(stdout, "Super: %s\n\n", cf.SuperClass)

		for _, m := range cf.Methods {
			code := m.FindCode()
			if code == nil {
				continue
			}
			mir, err := ir.LowerMethodWithExceptions(cf.ConstantPool, cf.ThisClass, m.Name, m.Descriptor, code.Code, m.IsStatic(), code.ExceptionTable)
			if err != nil {
				fmt.Fprintf(stdout, "Method %s%s: [Lowering Error: %v]\n\n", m.Name, m.Descriptor, err)
				continue
			}
			fmt.Fprintf(stdout, "Method %s%s:\n", m.Name, m.Descriptor)
			for _, inst := range mir.Instructions {
				fmt.Fprintf(stdout, "    %s\n", inst)
			}
			if len(mir.ExceptionTraps) > 0 {
				fmt.Fprintln(stdout, "    Exception traps:")
				for _, trap := range mir.ExceptionTraps {
					fmt.Fprintf(stdout, "        [%d,%d) -> pc %d catch %s\n", trap.StartPC, trap.EndPC, trap.HandlerPC, trap.CatchClass)
				}
			}
			if mir.CFG != nil {
				fmt.Fprintf(stdout, "    CFG: %d blocks, entry %s\n", len(mir.CFG.Blocks), mir.CFG.Entry.Label())
				for _, block := range mir.CFG.Blocks {
					fmt.Fprintf(stdout, "        %s [pc %d..%d)", block.Label(), block.StartPC, block.EndPC)
					if len(block.Successors) > 0 {
						successors := make([]string, 0, len(block.Successors))
						for _, successor := range block.Successors {
							successors = append(successors, successor.Label())
						}
						fmt.Fprintf(stdout, " -> %s", strings.Join(successors, ", "))
					}
					fmt.Fprintln(stdout)
				}
			}
			fmt.Fprintln(stdout)
		}
	}
	return nil
}

func runBrew(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("brew", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outPath := fs.String("o", "", "output .go file path (defaults to stdout)")
	quiet := fs.Bool("q", false, "suppress progress indicators")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: barista brew [-o output.go|output-dir] [-q] <input.class|input.jar>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("brew requires a path to a .class or .jar file")
	}

	classPath := fs.Arg(0)
	var pb *ui.ProgressBar
	if *outPath != "" && !*quiet {
		pb = ui.NewProgressBar(stderr, 3)
		pb.SetStep(1, "Grinding classes and constant pools...")
	}

	hierarchy, err := classfile.LoadClassOrJar(classPath)
	if err != nil {
		return fmt.Errorf("brew failed to parse input: %w", err)
	}

	if pb != nil {
		pb.SetStep(2, "Brewing SSA intermediate representation...")
	}

	modules, err := lowerModules(hierarchy)
	if err != nil {
		return err
	}

	if pb != nil {
		pb.SetStep(3, "Synthesizing Go source...")
	}

	generated := make(map[string][]byte, len(modules))
	if len(modules) == 1 {
		goCode, err := codegen.GenerateGo(modules[0].ClassFile, modules[0].Methods)
		if err != nil {
			return fmt.Errorf("code generation failed: %w", err)
		}
		generated[strings.ToLower(codegen.CleanClassName(modules[0].ClassFile.ThisClass))+".go"] = goCode
	} else {
		generated, err = codegen.GenerateModule(modules, "app")
		if err != nil {
			return fmt.Errorf("code generation failed: %w", err)
		}
	}

	if *outPath != "" {
		if len(generated) == 1 && strings.HasSuffix(strings.ToLower(*outPath), ".go") {
			for _, content := range generated {
				if dir := filepath.Dir(*outPath); dir != "." {
					if err := os.MkdirAll(dir, 0755); err != nil {
						return err
					}
				}
				if err := os.WriteFile(*outPath, content, 0644); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
				if pb != nil {
					pb.Complete(fmt.Sprintf("Brewed %s -> %s (%s)", classPath, *outPath, ui.FormatBytes(int64(len(content)))))
				}
			}
			return nil
		}

		dir := *outPath
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		names := sortedKeys(generated)
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(dir, name), generated[name], 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", name, err)
			}
		}
		if pb != nil {
			pb.Complete(fmt.Sprintf("Brewed %s -> %s (%d files)", classPath, dir, len(generated)))
		}
		return nil
	}

	for _, name := range sortedKeys(generated) {
		fmt.Fprintf(stdout, "// %s\n%s\n", name, strings.TrimSpace(string(generated[name])))
	}
	return nil
}

func lowerModules(hierarchy *classfile.ClassHierarchy) ([]codegen.ClassModule, error) {
	modules := make([]codegen.ClassModule, 0, len(hierarchy.Classes))
	for _, name := range hierarchy.Names() {
		cf := hierarchy.Classes[name]
		loweredMethods := make([]*ir.MethodIR, 0, len(cf.Methods))
		for _, m := range cf.Methods {
			code := m.FindCode()
			if code == nil {
				continue
			}
			mir, err := ir.LowerMethodWithExceptions(cf.ConstantPool, cf.ThisClass, m.Name, m.Descriptor, code.Code, m.IsStatic(), code.ExceptionTable)
			if err != nil {
				return nil, fmt.Errorf("failed compiling method %s.%s: %w", cf.ThisClass, m.Name, err)
			}
			loweredMethods = append(loweredMethods, mir)
		}
		modules = append(modules, codegen.ClassModule{ClassFile: cf, Methods: loweredMethods})
	}
	return modules, nil
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func runBuild(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outPath := fs.String("o", "", "output binary path (defaults to lowercase class name)")
	target := fs.String("target", "", "cross-compilation target (e.g. linux/amd64, darwin/arm64, windows/amd64)")
	entry := fs.String("entry", "", "custom entrypoint method name (default: auto-detect main)")
	quiet := fs.Bool("q", false, "quiet mode (suppress progress bar)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: barista build [-o binary] [-target os/arch] [-entry method] [-q] <input.class|input.jar>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("build requires a path to a .class or .jar file")
	}

	classPath := fs.Arg(0)
	hierarchy, err := classfile.LoadClassOrJar(classPath)
	if err != nil {
		return fmt.Errorf("failed to parse input: %w", err)
	}

	var pb *ui.ProgressBar
	var onProgress func(step int, total int, message string)
	if !*quiet {
		pb = ui.NewProgressBar(stderr, 4)
		onProgress = func(step int, total int, message string) {
			pb.SetStep(step, message)
		}
	}

	res, err := builder.Build(builder.BuildOptions{
		Hierarchy:  hierarchy,
		OutputPath: *outPath,
		TargetOS:   *target,
		Static:     true,
		Entrypoint: *entry,
		OnProgress: onProgress,
	})
	if err != nil {
		return err
	}

	if pb != nil {
		pb.Complete(fmt.Sprintf("Brewed static binary in %s", ui.FormatDuration(res.Duration)))
		className := res.Class
		if len(hierarchy.Classes) > 1 {
			className = fmt.Sprintf("%s (%d classes)", res.Class, len(hierarchy.Classes))
		}
		ui.PrintCard(stdout, ui.CardDetails{
			Title:    "Freshly Brewed Native Binary",
			Class:    className,
			Output:   res.BinaryPath,
			Size:     fmt.Sprintf("%s (stripped -s -w static executable)", ui.FormatBytes(res.Size)),
			Target:   fmt.Sprintf("%s/%s (CGO_ENABLED=0)", res.TargetOS, res.TargetArch),
			Duration: ui.FormatDuration(res.Duration),
		})
	}
	return nil
}

func runServe(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	entry := fs.String("entry", "", "custom entrypoint method name (default: auto-detect main)")
	quiet := fs.Bool("q", false, "quiet mode (suppress progress bar)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: barista serve [-entry method] [-q] <input.class|input.jar> [args...]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		fs.Usage()
		return errors.New("serve requires a path to a .class or .jar file")
	}

	classPath := fs.Arg(0)
	appArgs := fs.Args()[1:]

	hierarchy, err := classfile.LoadClassOrJar(classPath)
	if err != nil {
		return fmt.Errorf("failed to parse input: %w", err)
	}

	var pb *ui.ProgressBar
	var onProgress func(step int, total int, message string)
	if !*quiet {
		pb = ui.NewProgressBar(stderr, 4)
		onProgress = func(step int, total int, message string) {
			pb.SetStep(step, message)
		}
	}

	opts := builder.BuildOptions{
		Hierarchy:  hierarchy,
		Static:     true,
		Entrypoint: *entry,
		OnProgress: onProgress,
	}

	if pb != nil {
		defer func() {
			pb.Complete("Serving application...")
		}()
	}

	return builder.ServeWithIO(context.Background(), opts, appArgs, stdout, stderr, os.Stdin)
}
