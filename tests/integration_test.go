package tests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCalculatorFixtureCompiles verifies that the Phase 1 fixture can be
// compiled by the host JDK and produces a well-formed class file.
//
// Once the class-file parser lands, this test should be extended to parse the
// generated class and assert the recovered bytecode for Calculator.add.
func TestCalculatorFixtureCompiles(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac is not installed")
	}

	outDir := t.TempDir()
	cmd := exec.Command(javac, "-d", outDir, filepath.Join("fixtures", "java", "Calculator.java"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac failed: %v\n%s", err, output)
	}

	classBytes, err := os.ReadFile(filepath.Join(outDir, "Calculator.class"))
	if err != nil {
		t.Fatalf("read Calculator.class: %v", err)
	}

	const magic = "\xca\xfe\xba\xbe"
	if len(classBytes) < len(magic) || !bytes.Equal(classBytes[:len(magic)], []byte(magic)) {
		t.Fatalf("Calculator.class does not start with 0xCAFEBABE")
	}
}
