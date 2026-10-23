package runtime

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ShimsFS embeds all Java and Kotlin runtime shims into the compiler binary.
//
//go:embed java/lang/*.go java/net/*.go java/util/*.go kotlin/jvm/*.go
var ShimsFS embed.FS

// ExtractShims writes the embedded runtime source files into destDir, creating a
// self-contained Go module that matches "github.com/JohnEsleyer/barista".
func ExtractShims(destDir string) error {
	runtimeGoMod := `module github.com/JohnEsleyer/barista

go 1.24
`
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(destDir, "go.mod"), []byte(runtimeGoMod), 0644); err != nil {
		return err
	}

	dirs := []string{"java/lang", "java/net", "java/util", "kotlin/jvm"}
	for _, dir := range dirs {
		entries, err := ShimsFS.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("read embedded dir %s: %w", dir, err)
		}
		targetDir := filepath.Join(destDir, "pkg", "runtime", dir)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			srcPath := dir + "/" + entry.Name()
			data, err := ShimsFS.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("read embedded file %s: %w", srcPath, err)
			}
			dstPath := filepath.Join(targetDir, entry.Name())
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return fmt.Errorf("write extracted file %s: %w", dstPath, err)
			}
		}
	}
	return nil
}
