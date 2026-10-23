package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/JohnEsleyer/barista/internal/classfile"
	"github.com/JohnEsleyer/barista/internal/codegen"
	"github.com/JohnEsleyer/barista/internal/ir"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to locate test directory")
	}
	return filepath.Dir(filepath.Dir(filename))
}

func compileFixtureJava(t *testing.T, javaPath string) string {
	t.Helper()
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not found on PATH; skipping Java integration test")
	}
	outDir := t.TempDir()
	cmd := exec.Command(javac, "-d", outDir, javaPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac failed: %v\n%s", err, string(out))
	}
	base := strings.TrimSuffix(filepath.Base(javaPath), ".java")
	return filepath.Join(outDir, base+".class")
}

func brewClass(t *testing.T, classPath string) []byte {
	t.Helper()
	cf, err := classfile.ParseClassFilePath(classPath)
	if err != nil {
		t.Fatalf("parse classfile %s: %v", classPath, err)
	}
	methods := make([]*ir.MethodIR, 0, len(cf.Methods))
	for _, m := range cf.Methods {
		code := m.FindCode()
		if code == nil {
			continue
		}
		mir, err := ir.LowerMethodWithExceptions(cf.ConstantPool, cf.ThisClass, m.Name, m.Descriptor, code.Code, m.IsStatic(), code.ExceptionTable)
		if err != nil {
			t.Fatalf("lower method %s: %v", m.Name, err)
		}
		methods = append(methods, mir)
	}
	goCode, err := codegen.GenerateGo(cf, methods)
	if err != nil {
		t.Fatalf("codegen for %s: %v", classPath, err)
	}
	return goCode
}

func runGoTest(t *testing.T, testFiles map[string][]byte) {
	t.Helper()
	root := repoRoot(t)
	workDir := t.TempDir()

	goMod := fmt.Sprintf(`module testpkg

go 1.24

require github.com/JohnEsleyer/barista v0.0.0
replace github.com/JohnEsleyer/barista => %s
`, root)

	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	for name, content := range testFiles {
		path := filepath.Join(workDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("create directory for %s: %v", name, err)
		}
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	cmd := exec.Command("go", "test", "-v", "./...")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test failed: %v\n%s", err, string(out))
	}
}

func TestIntegrationCalculator(t *testing.T) {
	root := repoRoot(t)
	classPath := compileFixtureJava(t, filepath.Join(root, "tests", "fixtures", "java", "Calculator.java"))
	goCode := brewClass(t, classPath)

	testCode := `package calculator

import "testing"

func TestCalculator(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Errorf("Add(2, 3) = %d, want 5", got)
	}
	if got := Subtract(10, 4); got != 6 {
		t.Errorf("Subtract(10, 4) = %d, want 6", got)
	}
	if got := Multiply(6, 7); got != 42 {
		t.Errorf("Multiply(6, 7) = %d, want 42", got)
	}
	if got := Divide(20, 4); got != 5 {
		t.Errorf("Divide(20, 4) = %d, want 5", got)
	}
	if got := SumTo(10); got != 55 {
		t.Errorf("SumTo(10) = %d, want 55", got)
	}
	if got := Absolute(-7); got != 7 {
		t.Errorf("Absolute(-7) = %d, want 7", got)
	}
	if got := Max(10, 20); got != 20 {
		t.Errorf("Max(10, 20) = %d, want 20", got)
	}
}
`
	runGoTest(t, map[string][]byte{
		"calculator.go":      goCode,
		"calculator_test.go": []byte(testCode),
	})
}

func TestIntegrationCounter(t *testing.T) {
	root := repoRoot(t)
	classPath := compileFixtureJava(t, filepath.Join(root, "tests", "fixtures", "java", "Counter.java"))
	goCode := brewClass(t, classPath)

	testCode := `package counter

import "testing"

func TestCounter(t *testing.T) {
	c := Create(10)
	if got := c.GetValue(); got != 10 {
		t.Errorf("c.GetValue() = %d, want 10", got)
	}
	if got := c.Increment(); got != 11 {
		t.Errorf("c.Increment() = %d, want 11", got)
	}
	if got := c.AddValue(4); got != 16 {
		t.Errorf("c.AddValue(4) = %d, want 16", got)
	}
	if got := CreateAndGet(99); got != 99 {
		t.Errorf("CreateAndGet(99) = %d, want 99", got)
	}
	if got := GetCreated(); got < 2 {
		t.Errorf("GetCreated() = %d, want >= 2", got)
	}
}
`
	runGoTest(t, map[string][]byte{
		"counter.go":      goCode,
		"counter_test.go": []byte(testCode),
	})
}

func TestIntegrationGreeter(t *testing.T) {
	root := repoRoot(t)
	classPath := compileFixtureJava(t, filepath.Join(root, "tests", "fixtures", "java", "Greeter.java"))
	goCode := brewClass(t, classPath)

	testCode := `package greeter

import "testing"

func TestGreeter(t *testing.T) {
	result := Run()
	if result <= 0 {
		t.Errorf("Run() = %d, want positive value", result)
	}
}
`
	runGoTest(t, map[string][]byte{
		"greeter.go":      goCode,
		"greeter_test.go": []byte(testCode),
	})
}

func TestIntegrationTryCatch(t *testing.T) {
	root := repoRoot(t)
	classPath := compileFixtureJava(t, filepath.Join(root, "tests", "fixtures", "java", "TryCatch.java"))
	goCode := brewClass(t, classPath)

	testCode := `package trycatch

import "testing"

func TestTryCatch(t *testing.T) {
	if got := Run(false); got != 1 {
		t.Errorf("Run(false) = %d, want 1", got)
	}
	if got := Run(true); got != 2 {
		t.Errorf("Run(true) = %d, want 2", got)
	}
}
`
	runGoTest(t, map[string][]byte{
		"trycatch.go":      goCode,
		"trycatch_test.go": []byte(testCode),
	})
}

func TestIntegrationSwitch(t *testing.T) {
	root := repoRoot(t)
	classPath := compileFixtureJava(t, filepath.Join(root, "tests", "fixtures", "java", "Switch.java"))
	goCode := brewClass(t, classPath)

	testCode := `package switch_

import "testing"

func TestSwitch(t *testing.T) {
	if got := Dense(0); got != 10 {
		t.Errorf("Dense(0) = %d, want 10", got)
	}
	if got := Dense(1); got != 11 {
		t.Errorf("Dense(1) = %d, want 11", got)
	}
	if got := Dense(2); got != 12 {
		t.Errorf("Dense(2) = %d, want 12", got)
	}
	if got := Dense(99); got != -1 {
		t.Errorf("Dense(99) = %d, want -1", got)
	}

	if got := Sparse(-100); got != 1 {
		t.Errorf("Sparse(-100) = %d, want 1", got)
	}
	if got := Sparse(7); got != 2 {
		t.Errorf("Sparse(7) = %d, want 2", got)
	}
	if got := Sparse(1000); got != 3 {
		t.Errorf("Sparse(1000) = %d, want 3", got)
	}
	if got := Sparse(0); got != 0 {
		t.Errorf("Sparse(0) = %d, want 0", got)
	}
}
`
	runGoTest(t, map[string][]byte{
		"switch.go":      goCode,
		"switch_test.go": []byte(testCode),
	})
}

func TestIntegrationReferenceOps(t *testing.T) {
	root := repoRoot(t)
	classPath := compileFixtureJava(t, filepath.Join(root, "tests", "fixtures", "java", "ReferenceOps.java"))
	goCode := brewClass(t, classPath)

	testCode := `package referenceops

import "testing"

func TestReferenceOps(t *testing.T) {
	r1 := &ReferenceOps{Id: 100}
	r2 := &ReferenceOps{Id: 200}

	if !r1.Same(r1) {
		t.Error("r1.Same(r1) = false, want true")
	}
	if r1.Same(r2) {
		t.Error("r1.Same(r2) = true, want false")
	}

	if got := Ternary(true, 10, 20); got != 10 {
		t.Errorf("Ternary(true, 10, 20) = %d, want 10", got)
	}
	if got := Ternary(false, 10, 20); got != 20 {
		t.Errorf("Ternary(false, 10, 20) = %d, want 20", got)
	}

	if !r1.IsReferenceOps(r2) {
		t.Error("r1.IsReferenceOps(r2) = false, want true")
	}
	if r1.IsReferenceOps("not a reference ops") {
		t.Error("r1.IsReferenceOps(string) = true, want false")
	}
	if got := r1.CastId(r2); got != 200 {
		t.Errorf("r1.CastId(r2) = %d, want 200", got)
	}

	if got := r1.Compare(1, 2); got != -1 {
		t.Errorf("Compare(1, 2) = %d, want -1", got)
	}
	if got := r1.Compare(5, 5); got != 0 {
		t.Errorf("Compare(5, 5) = %d, want 0", got)
	}
	if got := r1.Compare(10, 3); got != 1 {
		t.Errorf("Compare(10, 3) = %d, want 1", got)
	}

	if got := Mask(2); got != 7 {
		t.Errorf("Mask(2) = %d, want 7", got)
	}
	if got := Mask(1); got != 9 {
		t.Errorf("Mask(1) = %d, want 9", got)
	}

	longs := MakeLongs(10, 20)
	if got := SumLongs(longs); got != 30 {
		t.Errorf("SumLongs = %d, want 30", got)
	}
}
`
	runGoTest(t, map[string][]byte{
		"referenceops.go":      goCode,
		"referenceops_test.go": []byte(testCode),
	})
}

func TestIntegrationKotlinGreeting(t *testing.T) {
	kotlinc := os.Getenv("KOTLINC")
	if kotlinc == "" {
		path, err := exec.LookPath("kotlinc")
		if err != nil {
			t.Skip("kotlinc not available; set KOTLINC or add kotlinc to PATH to run Kotlin integration test")
		}
		kotlinc = path
	}

	root := repoRoot(t)
	outDir := t.TempDir()
	ktSource := filepath.Join(root, "tests", "fixtures", "kotlin", "Greeting.kt")
	cmd := exec.Command(kotlinc, ktSource, "-d", outDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("kotlinc failed: %v\n%s", err, string(out))
	}

	userGo := brewClass(t, filepath.Join(outDir, "User.class"))
	greetingGo := brewClass(t, filepath.Join(outDir, "GreetingKt.class"))

	userTest := `package user

import (
	"testing"
	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
)

func TestKotlinUser(t *testing.T) {
	name := lang.NewString("Barista")
	user := &User{Id: 101, Name: name}
	if user.GetId() != 101 {
		t.Errorf("user.GetId() = %d, want 101", user.GetId())
	}
	if user.GetName().GoString() != "Barista" {
		t.Errorf("user.GetName() = %q, want \"Barista\"", user.GetName().GoString())
	}

	user2 := &User{Id: 101, Name: name}
	if !user.Equals(user2) {
		t.Error("data class equals returned false for identical instances")
	}
}
`
	runGoTest(t, map[string][]byte{
		"user/user.go":      userGo,
		"user/user_test.go": []byte(userTest),
	})

	greetingTest := `package greetingkt

import (
	"testing"
	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
)

func TestKotlinGreeting(t *testing.T) {
	name := lang.NewString("Barista")
	msg := Greet(name)
	if got := msg.GoString(); got != "Hello, Barista!" {
		t.Errorf("Greet() = %q, want \"Hello, Barista!\"", got)
	}

	fw := Farewell_default(lang.NewString("Barista"), nil, 2, nil)
	if got := fw.GoString(); got != "Goodbye, Barista!" {
		t.Errorf("Farewell_default() = %q, want \"Goodbye, Barista!\"", got)
	}

	Main()
}
`
	runGoTest(t, map[string][]byte{
		"greetingkt/greetingkt.go":    greetingGo,
		"greetingkt/greeting_test.go": []byte(greetingTest),
	})
}
