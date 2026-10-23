package classfile

import (
	"testing"
)

func TestClassHierarchy(t *testing.T) {
	h := NewClassHierarchy()
	if len(h.Classes) != 0 {
		t.Fatalf("expected 0 classes, got %d", len(h.Classes))
	}

	cf := &ClassFile{ThisClass: "com/example/Test"}
	h.Add(cf)

	if got := h.Get("com/example/Test"); got != cf {
		t.Fatalf("expected %v, got %v", cf, got)
	}

	names := h.Names()
	if len(names) != 1 || names[0] != "com/example/Test" {
		t.Fatalf("expected [com/example/Test], got %v", names)
	}
}
