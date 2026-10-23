package jvm_test

import (
	"testing"

	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
	"github.com/JohnEsleyer/barista/pkg/runtime/kotlin/jvm"
)

func TestCheckNotNullParameter(t *testing.T) {
	jvm.CheckNotNullParameter(lang.NewString("ok"), lang.NewString("name"))

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected a NullPointerException panic")
		}
		if !lang.MatchesException(recovered, "java/lang/NullPointerException") {
			t.Fatalf("recovered %T, want NullPointerException", recovered)
		}
	}()
	jvm.CheckNotNullParameter(nil, lang.NewString("name"))
}

func TestAreEqual(t *testing.T) {
	if !jvm.AreEqual(nil, nil) {
		t.Error("AreEqual(nil, nil) = false")
	}
	if jvm.AreEqual(nil, lang.NewString("x")) {
		t.Error("AreEqual(nil, string) = true")
	}
	if !jvm.AreEqual(lang.NewString("Barista"), lang.NewString("Barista")) {
		t.Error("equal strings did not compare equal")
	}
	if jvm.AreEqual(lang.NewString("Barista"), lang.NewString("barista")) {
		t.Error("case-different strings compared equal")
	}
}

func TestStringPlus(t *testing.T) {
	result := jvm.StringPlus(lang.NewString("Hello, "), lang.NewString("Barista"))
	if got := result.GoString(); got != "Hello, Barista" {
		t.Errorf("StringPlus = %q, want \"Hello, Barista\"", got)
	}
	resultNum := jvm.StringPlus(lang.NewString("Count: "), int32(42))
	if got := resultNum.GoString(); got != "Count: 42" {
		t.Errorf("StringPlus with int = %q, want \"Count: 42\"", got)
	}
}
