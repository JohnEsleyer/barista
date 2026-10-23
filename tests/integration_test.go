package tests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JohnEsleyer/barista/internal/classfile"
	"github.com/JohnEsleyer/barista/internal/codegen"
	"github.com/JohnEsleyer/barista/internal/ir"
)

func compileCalculator(t *testing.T) string {
	t.Helper()
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac is not installed")
	}
	outDir := t.TempDir()
	cmd := exec.Command(javac, "-d", outDir, filepath.Join("fixtures", "java", "Calculator.java"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac failed: %v\n%s", err, output)
	}
	return filepath.Join(outDir, "Calculator.class")
}

func TestCalculatorFixtureCompiles(t *testing.T) {
	classPath := compileCalculator(t)
	classBytes, err := os.ReadFile(classPath)
	if err != nil {
		t.Fatalf("read Calculator.class: %v", err)
	}
	const magic = "\xca\xfe\xba\xbe"
	if len(classBytes) < len(magic) || !bytes.Equal(classBytes[:len(magic)], []byte(magic)) {
		t.Fatal("Calculator.class does not start with 0xCAFEBABE")
	}
}

func TestCalculatorPipeline(t *testing.T) {
	classPath := compileCalculator(t)
	cf, err := classfile.ParseClassFilePath(classPath)
	if err != nil {
		t.Fatalf("parse Calculator.class: %v", err)
	}
	if cf.ThisClass != "Calculator" {
		t.Fatalf("ThisClass = %q, want Calculator", cf.ThisClass)
	}
	if cf.SuperClass != "java/lang/Object" {
		t.Fatalf("SuperClass = %q, want java/lang/Object", cf.SuperClass)
	}

	add := cf.FindMethod("add", "(II)I")
	if add == nil {
		t.Fatal("add(II)I not found")
	}
	addCode := add.FindCode()
	if addCode == nil {
		t.Fatal("add(II)I has no Code attribute")
	}
	if addCode.MaxStack != 2 || addCode.MaxLocals != 2 {
		t.Errorf("add max_stack/max_locals = %d/%d, want 2/2", addCode.MaxStack, addCode.MaxLocals)
	}
	if addCode.FindAttribute("LineNumberTable") == nil {
		t.Error("add(II)I has no parsed LineNumberTable")
	}

	lowered := make([]*ir.MethodIR, 0, len(cf.Methods))
	methods := make(map[string]*ir.MethodIR)
	for _, m := range cf.Methods {
		code := m.FindCode()
		if code == nil || !m.IsStatic() || strings.HasPrefix(m.Name, "<") {
			continue
		}
		mir, err := ir.LowerBytecodeWithPool(m.Name, m.Descriptor, code.Code, m.IsStatic(), cf.ConstantPool)
		if err != nil {
			t.Fatalf("lower %s: %v", m.Name, err)
		}
		lowered = append(lowered, mir)
		methods[m.Name] = mir
	}

	for _, name := range []string{"add", "subtract", "multiply", "divide", "sumTo", "absolute", "max"} {
		if methods[name] == nil {
			t.Errorf("method %s was not lowered", name)
		}
	}
	if !hasOp(methods["sumTo"].Instructions, ir.OpGoto) {
		t.Error("sumTo should contain a loop back-edge (goto)")
	}
	if !hasOp(methods["absolute"].Instructions, ir.OpBranch) {
		t.Error("absolute should contain a branch")
	}

	goCode, err := codegen.GenerateGo(cf, lowered)
	if err != nil {
		t.Fatalf("generate Go: %v", err)
	}
	if !bytes.Contains(goCode, []byte("func SumTo(local_0 int32) int32")) {
		t.Errorf("generated Go does not contain SumTo:\n%s", goCode)
	}
	if !bytes.Contains(goCode, []byte("goto L")) {
		t.Errorf("generated Go does not contain a goto:\n%s", goCode)
	}

	runGeneratedGo(t, goCode)
}

func hasOp(insts []ir.Instruction, op ir.OpKind) bool {
	for _, inst := range insts {
		if inst.Op == op {
			return true
		}
	}
	return false
}

func runGeneratedGo(t *testing.T, goCode []byte) {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain is not available")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "calculator.go"), goCode, 0644); err != nil {
		t.Fatalf("write generated Go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module generated\n\ngo 1.24\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	testSource := `package calculator

import "testing"

func TestGeneratedCalculator(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Errorf("Add(2, 3) = %d, want 5", got)
	}
	if got := Subtract(9, 4); got != 5 {
		t.Errorf("Subtract(9, 4) = %d, want 5", got)
	}
	if got := Multiply(6, 7); got != 42 {
		t.Errorf("Multiply(6, 7) = %d, want 42", got)
	}
	if got := Divide(20, 5); got != 4 {
		t.Errorf("Divide(20, 5) = %d, want 4", got)
	}
	if got := SumTo(10); got != 55 {
		t.Errorf("SumTo(10) = %d, want 55", got)
	}
	if got := Absolute(-7); got != 7 {
		t.Errorf("Absolute(-7) = %d, want 7", got)
	}
	if got := Absolute(7); got != 7 {
		t.Errorf("Absolute(7) = %d, want 7", got)
	}
	if got := Max(3, 9); got != 9 {
		t.Errorf("Max(3, 9) = %d, want 9", got)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "calculator_test.go"), []byte(testSource), 0644); err != nil {
		t.Fatalf("write generated Go test: %v", err)
	}

	cmd := exec.Command(goTool, "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Go failed to compile or run: %v\n%s\n--- generated source ---\n%s", err, output, goCode)
	}
}
