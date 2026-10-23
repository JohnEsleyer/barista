package codegen

import (
	"bytes"
	"go/format"
	"go/token"
	"testing"
)

func TestDescriptorToGoType(t *testing.T) {
	tests := map[string]string{
		"I":                  "int32",
		"J":                  "int64",
		"Z":                  "bool",
		"Ljava/lang/String;": "*JavaLangString",
		"LCounter;":          "*Counter",
		"[I":                 "[]int32",
		"[[LCounter;":        "[][]*Counter",
	}
	for desc, want := range tests {
		expr := DescriptorToGoType(desc)
		if expr == nil {
			t.Errorf("DescriptorToGoType(%q) returned nil", desc)
			continue
		}
		var buf bytes.Buffer
		if err := format.Node(&buf, token.NewFileSet(), expr); err != nil {
			t.Errorf("format %q: %v", desc, err)
			continue
		}
		if got := buf.String(); got != want {
			t.Errorf("DescriptorToGoType(%q) = %q, want %q", desc, got, want)
		}
	}
	if DescriptorToGoType("V") != nil {
		t.Error("DescriptorToGoType(V) should be nil")
	}
}

func TestCleanNames(t *testing.T) {
	if got := CleanClassName("com/example/User"); got != "ComExampleUser" {
		t.Errorf("CleanClassName = %q, want ComExampleUser", got)
	}
	if got := CleanClassName("Outer$Inner"); got != "Outer_Inner" {
		t.Errorf("CleanClassName = %q, want Outer_Inner", got)
	}
	if got := CleanMemberName("value"); got != "Value" {
		t.Errorf("CleanMemberName = %q, want Value", got)
	}
	if got := staticFieldName("Counter", "created"); got != "CounterCreated" {
		t.Errorf("staticFieldName = %q, want CounterCreated", got)
	}
}
