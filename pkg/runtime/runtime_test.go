package runtime_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JohnEsleyer/barista/pkg/runtime"
)

func TestExtractShims(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "barista-runtime-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if err := runtime.ExtractShims(tempDir); err != nil {
		t.Fatalf("ExtractShims failed: %v", err)
	}

	expectedFiles := []string{
		filepath.Join(tempDir, "go.mod"),
		filepath.Join(tempDir, "pkg", "runtime", "java", "lang", "string.go"),
		filepath.Join(tempDir, "pkg", "runtime", "java", "net", "http_server.go"),
		filepath.Join(tempDir, "pkg", "runtime", "kotlin", "jvm", "intrinsics.go"),
	}

	for _, path := range expectedFiles {
		fi, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected extracted file %s not found: %v", path, err)
			continue
		}
		if fi.Size() == 0 {
			t.Errorf("extracted file %s is empty", path)
		}
	}
}
