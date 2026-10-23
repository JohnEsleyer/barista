package builder_test

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JohnEsleyer/barista/internal/builder"
	"github.com/JohnEsleyer/barista/internal/classfile"
)

func TestParseTarget(t *testing.T) {
	osVal, archVal, err := builder.ParseTarget("linux/amd64")
	if err != nil || osVal != "linux" || archVal != "amd64" {
		t.Errorf("ParseTarget(linux/amd64) = (%q, %q, %v), want (linux, amd64, nil)", osVal, archVal, err)
	}

	osVal, archVal, err = builder.ParseTarget("darwin/arm64")
	if err != nil || osVal != "darwin" || archVal != "arm64" {
		t.Errorf("ParseTarget(darwin/arm64) = (%q, %q, %v), want (darwin, arm64, nil)", osVal, archVal, err)
	}

	_, _, err = builder.ParseTarget("invalid")
	if err == nil {
		t.Error("expected error for invalid target format")
	}

	osVal, archVal, err = builder.ParseTarget("")
	if err != nil || osVal == "" || archVal == "" {
		t.Errorf("ParseTarget(\"\") failed to return host target: (%q, %q, %v)", osVal, archVal, err)
	}
}

func TestBuildAndServeHelloBarista(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not found on PATH; skipping native binary build test")
	}

	tempDir := t.TempDir()
	javaSrc := filepath.Join("..", "..", "tests", "fixtures", "java", "HelloBarista.java")
	compileCmd := exec.Command(javac, "-d", tempDir, javaSrc)
	if out, err := compileCmd.CombinedOutput(); err != nil {
		t.Fatalf("javac compilation failed: %v\n%s", err, string(out))
	}

	classPath := filepath.Join(tempDir, "HelloBarista.class")
	cf, err := classfile.ParseClassFilePath(classPath)
	if err != nil {
		t.Fatalf("failed to parse compiled classfile: %v", err)
	}

	outBin := filepath.Join(tempDir, "hello_barista_bin")
	res, err := builder.Build(builder.BuildOptions{
		ClassFile:  cf,
		OutputPath: outBin,
		Static:     true,
	})
	if err != nil {
		t.Fatalf("builder.Build failed: %v", err)
	}

	if res.Size <= 0 {
		t.Errorf("expected positive binary size, got %d", res.Size)
	}

	// 1. Execute the standalone compiled binary
	runCmd := exec.Command(res.BinaryPath, "Gopher")
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("executing compiled binary failed: %v\n%s", err, string(out))
	}
	want := "Hello from Barista, Gopher!"
	if !strings.Contains(string(out), want) {
		t.Errorf("binary stdout = %q, want containing %q", string(out), want)
	}

	// 2. Test ServeWithIO
	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)
	err = builder.ServeWithIO(
		context.Background(),
		builder.BuildOptions{ClassFile: cf},
		[]string{"Developer"},
		stdoutBuf,
		stderrBuf,
		strings.NewReader(""),
	)
	if err != nil {
		t.Fatalf("builder.ServeWithIO failed: %v", err)
	}
	wantServe := "Hello from Barista, Developer!"
	if !strings.Contains(stdoutBuf.String(), wantServe) {
		t.Errorf("serve stdout = %q, want containing %q", stdoutBuf.String(), wantServe)
	}
}

func TestBuildWithCustomEntrypoint(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not found on PATH; skipping test")
	}

	tempDir := t.TempDir()
	javaSrc := filepath.Join("..", "..", "tests", "fixtures", "java", "Greeter.java")
	compileCmd := exec.Command(javac, "-d", tempDir, javaSrc)
	if out, err := compileCmd.CombinedOutput(); err != nil {
		t.Fatalf("javac compilation failed: %v\n%s", err, string(out))
	}

	classPath := filepath.Join(tempDir, "Greeter.class")
	cf, err := classfile.ParseClassFilePath(classPath)
	if err != nil {
		t.Fatalf("failed to parse compiled classfile: %v", err)
	}

	outBin := filepath.Join(tempDir, "greeter_bin")
	res, err := builder.Build(builder.BuildOptions{
		ClassFile:  cf,
		OutputPath: outBin,
		Entrypoint: "run",
		Static:     true,
	})
	if err != nil {
		t.Fatalf("builder.Build with -entry run failed: %v", err)
	}

	runCmd := exec.Command(res.BinaryPath)
	out, err := runCmd.CombinedOutput()
	// Greeter.run() returns an integer status code (non-zero), so exec.Command returns an ExitError.
	if !strings.Contains(string(out), "Freshly brewed with Barista!") {
		t.Errorf("greeter output missing expected string: %s (err: %v)", string(out), err)
	}
}
