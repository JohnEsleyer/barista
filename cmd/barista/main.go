// Command barista is the command-line interface for the Barista AOT compiler.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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
		return runBrew(args[1:], stdout)
	case "serve":
		return runServe(args[1:], stdout)
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
  grind <input>   Disassemble a .class or .jar into Barista IR
  brew  <input>   Compile a .class or .jar into a Go module/binary
  serve <input>   Compile and run an application immediately
  version         Print the Barista version
  help            Show this help text

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
		return errors.New("grind requires exactly one input file")
	}
	return errors.New("grind is not implemented yet")
}

func runBrew(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("brew", flag.ContinueOnError)
	fs.SetOutput(stdout)
	out := fs.String("o", "", "output directory for the generated Go module")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: barista brew [-o output-dir] <input.class|input.jar>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("brew requires exactly one input file")
	}
	_ = *out
	return errors.New("brew is not implemented yet")
}

func runServe(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stdout)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: barista serve <input.class|input.jar>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("serve requires exactly one input file")
	}
	return errors.New("serve is not implemented yet")
}
