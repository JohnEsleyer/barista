package builder_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestSpringJSONSerialization(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not found on PATH; skipping test")
	}

	tempDir := t.TempDir()
	sdkSrc := filepath.Join("..", "..", "sdk", "spring", "src", "main", "java", "barista", "spring", "annotation", "*.java")
	userSrc := filepath.Join("..", "..", "tests", "fixtures", "java", "UserController.java")

	// Compile annotations and fixture together
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s -d %s %s %s", javac, tempDir, sdkSrc, userSrc))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac compilation failed: %v\n%s", err, string(out))
	}

	hierarchy, err := classfile.LoadClassOrJar(tempDir)
	if err != nil {
		t.Fatalf("load classes: %v", err)
	}

	outBin := filepath.Join(tempDir, "user_service")
	res, err := builder.Build(builder.BuildOptions{
		Hierarchy:  hierarchy,
		OutputPath: outBin,
		Static:     true,
	})
	if err != nil {
		t.Fatalf("builder.Build failed: %v", err)
	}

	// Boot native server on port 19080
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runCmd := exec.CommandContext(ctx, res.BinaryPath)
	runCmd.Env = append(os.Environ(), "PORT=19080")
	if err := runCmd.Start(); err != nil {
		t.Fatalf("start binary: %v", err)
	}
	defer func() { _ = runCmd.Process.Kill() }()

	time.Sleep(100 * time.Millisecond) // Server boots in <5ms

	// 1. Test GET /api/users/current -> asserts automatic JSON serialization
	resp, err := http.Get("http://127.0.0.1:19080/api/users/current")
	if err != nil {
		t.Fatalf("GET /current: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	wantJSON := `{"id":101,"name":"Alice Developer","email":"alice@example.com"}`
	if strings.TrimSpace(string(body)) != wantJSON {
		t.Errorf("got %q, want %q", string(body), wantJSON)
	}

	// 2. Test POST /api/users -> asserts automatic JSON request deserialization
	payload := `{"name":"Bob Gopher","email":"bob@golang.org"}`
	postResp, err := http.Post("http://127.0.0.1:19080/api/users", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /users: %v", err)
	}
	defer postResp.Body.Close()

	postBody, _ := io.ReadAll(postResp.Body)
	wantCreatedJSON := `{"id":102,"name":"Bob Gopher","email":"bob@golang.org"}`
	if strings.TrimSpace(string(postBody)) != wantCreatedJSON {
		t.Errorf("POST response = %q, want %q", string(postBody), wantCreatedJSON)
	}
}

func TestSpringOrderController(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac not found on PATH; skipping test")
	}

	tempDir := t.TempDir()
	sdkSrc := filepath.Join("..", "..", "sdk", "spring", "src", "main", "java", "barista", "spring", "annotation", "*.java")
	orderSrc := filepath.Join("..", "..", "tests", "fixtures", "java", "OrderController.java")

	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s -d %s %s %s", javac, tempDir, sdkSrc, orderSrc))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("javac compilation failed: %v\n%s", err, string(out))
	}

	hierarchy, err := classfile.LoadClassOrJar(tempDir)
	if err != nil {
		t.Fatalf("load classes: %v", err)
	}

	outBin := filepath.Join(tempDir, "orders_service")
	res, err := builder.Build(builder.BuildOptions{
		Hierarchy:  hierarchy,
		OutputPath: outBin,
		Static:     true,
	})
	if err != nil {
		t.Fatalf("builder.Build failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runCmd := exec.CommandContext(ctx, res.BinaryPath)
	runCmd.Env = append(os.Environ(), "PORT=19081")
	if err := runCmd.Start(); err != nil {
		t.Fatalf("start binary: %v", err)
	}
	defer func() { _ = runCmd.Process.Kill() }()

	time.Sleep(100 * time.Millisecond)

	// GET /api/status -> "Orders service healthy"
	resp, err := http.Get("http://127.0.0.1:19081/api/status")
	if err != nil {
		t.Fatalf("GET /api/status: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	wantStatus := "Orders service healthy"
	if string(body) != wantStatus {
		t.Errorf("GET /status got %q, want %q", string(body), wantStatus)
	}

	// POST /api/submit -> "Order received: {"item": "Espresso"}"
	postPayload := `{"item": "Espresso"}`
	postResp, err := http.Post("http://127.0.0.1:19081/api/submit", "text/plain", strings.NewReader(postPayload))
	if err != nil {
		t.Fatalf("POST /api/submit: %v", err)
	}
	defer postResp.Body.Close()

	postBody, _ := io.ReadAll(postResp.Body)
	wantSubmit := `Order received: {"item": "Espresso"}`
	if string(postBody) != wantSubmit {
		t.Errorf("POST /submit got %q, want %q", string(postBody), wantSubmit)
	}
}
