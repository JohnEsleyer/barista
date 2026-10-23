package classfile

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ClassHierarchy holds parsed class files indexed by their JVM internal name.
type ClassHierarchy struct {
	Classes   map[string]*ClassFile
	MainClass string
}

// NewClassHierarchy creates an empty class hierarchy.
func NewClassHierarchy() *ClassHierarchy {
	return &ClassHierarchy{Classes: make(map[string]*ClassFile)}
}

// Add registers a class file in the hierarchy.
func (h *ClassHierarchy) Add(cf *ClassFile) {
	if cf != nil && cf.ThisClass != "" {
		h.Classes[cf.ThisClass] = cf
	}
}

// Get returns the class with the given JVM internal name, or nil.
func (h *ClassHierarchy) Get(internalName string) *ClassFile {
	if h == nil {
		return nil
	}
	return h.Classes[internalName]
}

// Names returns the class names in deterministic order.
func (h *ClassHierarchy) Names() []string {
	names := make([]string, 0, len(h.Classes))
	for name := range h.Classes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ParseJar reads a .jar or .zip archive and parses every contained .class file.
func ParseJar(r io.ReaderAt, size int64) (*ClassHierarchy, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("open jar archive: %w", err)
	}

	hierarchy := NewClassHierarchy()
	for _, entry := range zr.File {
		if entry.FileInfo().IsDir() {
			continue
		}

		if strings.EqualFold(entry.Name, "META-INF/MANIFEST.MF") {
			rc, err := entry.Open()
			if err != nil {
				return nil, fmt.Errorf("open manifest: %w", err)
			}
			data, readErr := io.ReadAll(rc)
			closeErr := rc.Close()
			if readErr != nil {
				return nil, fmt.Errorf("read manifest: %w", readErr)
			}
			if closeErr != nil {
				return nil, fmt.Errorf("close manifest: %w", closeErr)
			}
			hierarchy.MainClass = parseManifestMainClass(string(data))
			continue
		}

		if !strings.HasSuffix(strings.ToLower(entry.Name), ".class") {
			continue
		}

		rc, err := entry.Open()
		if err != nil {
			return nil, fmt.Errorf("open class entry %s: %w", entry.Name, err)
		}
		cf, parseErr := ParseClassFile(rc)
		closeErr := rc.Close()
		if parseErr != nil {
			return nil, fmt.Errorf("parse class %s: %w", entry.Name, parseErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close class entry %s: %w", entry.Name, closeErr)
		}
		hierarchy.Add(cf)
	}

	if len(hierarchy.Classes) == 0 {
		return nil, fmt.Errorf("jar archive contains no .class files")
	}
	return hierarchy, nil
}

// ParseJarPath reads a .jar or .zip archive directly from disk.
func ParseJarPath(path string) (*ClassHierarchy, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return ParseJar(f, fi.Size())
}

// LoadClassDir loads all .class files in a directory tree into a ClassHierarchy.
func LoadClassDir(dir string) (*ClassHierarchy, error) {
	hierarchy := NewClassHierarchy()
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".class") {
			return nil
		}
		cf, err := ParseClassFilePath(p)
		if err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		hierarchy.Add(cf)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(hierarchy.Classes) == 0 {
		return nil, fmt.Errorf("directory %s contains no .class files", dir)
	}
	return hierarchy, nil
}

// LoadClassOrJar loads a single .class file, a directory of .class files, or a .jar/.zip archive.
func LoadClassOrJar(path string) (*ClassHierarchy, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return LoadClassDir(path)
	}

	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".jar") || strings.HasSuffix(lower, ".zip") {
		return ParseJarPath(path)
	}

	cf, err := ParseClassFilePath(path)
	if err != nil {
		return nil, err
	}
	hierarchy := NewClassHierarchy()
	hierarchy.Add(cf)
	hierarchy.MainClass = cf.ThisClass
	return hierarchy, nil
}

func parseManifestMainClass(manifest string) string {
	for _, line := range strings.Split(manifest, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Main-Class:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "Main-Class:"))
		return strings.ReplaceAll(value, ".", "/")
	}
	return ""
}
