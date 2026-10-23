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

func compileJavaFixture(t *testing.T, source string) string {
	t.Helper()
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac is not installed")
	}
	outDir := t.TempDir()
	cmd := exec.Command(javac, "-d", outDir, filepath.Join("fixtures", "java", source))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac failed: %v\n%s", err, output)
	}
	className := strings.TrimSuffix(source, ".java")
	return filepath.Join(outDir, className+".class")
}

func compileCalculator(t *testing.T) string {
	t.Helper()
	return compileJavaFixture(t, "Calculator.java")
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
		if code == nil {
			continue
		}
		mir, err := ir.LowerMethod(cf.ConstantPool, cf.ThisClass, m.Name, m.Descriptor, code.Code, m.IsStatic())
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

func TestCounterPipeline(t *testing.T) {
	classPath := compileJavaFixture(t, "Counter.java")
	cf, err := classfile.ParseClassFilePath(classPath)
	if err != nil {
		t.Fatalf("parse Counter.class: %v", err)
	}
	if cf.ThisClass != "Counter" {
		t.Fatalf("ThisClass = %q, want Counter", cf.ThisClass)
	}

	lowered := make([]*ir.MethodIR, 0, len(cf.Methods))
	methods := make(map[string]*ir.MethodIR)
	for _, m := range cf.Methods {
		code := m.FindCode()
		if code == nil {
			continue
		}
		mir, err := ir.LowerMethod(cf.ConstantPool, cf.ThisClass, m.Name, m.Descriptor, code.Code, m.IsStatic())
		if err != nil {
			t.Fatalf("lower %s: %v", m.Name, err)
		}
		lowered = append(lowered, mir)
		methods[m.Name] = mir
	}

	for _, name := range []string{"<clinit>", "<init>", "increment", "getValue", "addValue", "create", "createAndGet", "getCreated"} {
		if methods[name] == nil {
			t.Errorf("method %s was not lowered", name)
		}
	}
	if !hasOp(methods["<clinit>"].Instructions, ir.OpPutStatic) {
		t.Error("static initializer should write the created field")
	}
	if !hasOp(methods["<init>"].Instructions, ir.OpPutField) {
		t.Error("constructor should write an instance field")
	}
	if !hasOp(methods["<init>"].Instructions, ir.OpGetStatic) ||
		!hasOp(methods["<init>"].Instructions, ir.OpPutStatic) {
		t.Error("constructor should update static state")
	}
	if !hasOp(methods["increment"].Instructions, ir.OpGetField) ||
		!hasOp(methods["increment"].Instructions, ir.OpPutField) {
		t.Error("increment should read and write the value field")
	}
	if !hasOp(methods["create"].Instructions, ir.OpNew) ||
		!hasOp(methods["create"].Instructions, ir.OpInvokeSpecial) {
		t.Error("create should allocate and construct a Counter")
	}
	if !hasOp(methods["addValue"].Instructions, ir.OpInvokeVirtual) {
		t.Error("addValue should dispatch increment virtually")
	}
	if !hasOp(methods["createAndGet"].Instructions, ir.OpInvokeStatic) ||
		!hasOp(methods["createAndGet"].Instructions, ir.OpInvokeVirtual) {
		t.Error("createAndGet should use static and virtual dispatch")
	}
	if methods["increment"].CFG == nil || len(methods["increment"].CFG.Blocks) == 0 {
		t.Error("increment should have a CFG")
	}

	goCode, err := codegen.GenerateGo(cf, lowered)
	if err != nil {
		t.Fatalf("generate Go: %v", err)
	}
	for _, want := range [][]byte{
		[]byte("type Counter struct"),
		[]byte("Value int32"),
		[]byte("var CounterCreated int32"),
		[]byte("func init()"),
		[]byte("func (this *Counter) Init(local_1 int32)"),
		[]byte("func Create(local_0 int32) *Counter"),
	} {
		if !bytes.Contains(goCode, want) {
			t.Errorf("generated Go does not contain %q:\n%s", want, goCode)
		}
	}

	runGeneratedCounter(t, goCode)
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

func runGeneratedCounter(t *testing.T, goCode []byte) {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain is not available")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "counter.go"), goCode, 0644); err != nil {
		t.Fatalf("write generated Go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module generated\n\ngo 1.24\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	testSource := `package counter

import "testing"

func TestGeneratedCounter(t *testing.T) {
	c := &Counter{}
	c.Init(10)
	if got := c.GetValue(); got != 10 {
		t.Errorf("GetValue() = %d, want 10", got)
	}
	if got := c.Increment(); got != 11 {
		t.Errorf("Increment() = %d, want 11", got)
	}
	if got := c.AddValue(5); got != 17 {
		t.Errorf("AddValue(5) = %d, want 17", got)
	}
	if got := GetCreated(); got != 3 {
		t.Errorf("GetCreated() = %d, want 3", got)
	}

	other := Create(5)
	if got := other.GetValue(); got != 5 {
		t.Errorf("created GetValue() = %d, want 5", got)
	}
	if got := CreateAndGet(7); got != 7 {
		t.Errorf("CreateAndGet(7) = %d, want 7", got)
	}
	if got := GetCreated(); got != 5 {
		t.Errorf("GetCreated() = %d, want 5", got)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "counter_test.go"), []byte(testSource), 0644); err != nil {
		t.Fatalf("write generated Go test: %v", err)
	}

	cmd := exec.Command(goTool, "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated object code failed to compile or run: %v\n%s\n--- generated source ---\n%s", err, output, goCode)
	}
}
