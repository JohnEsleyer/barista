package lang_test

import (
	"testing"

	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
)

func TestStringBasics(t *testing.T) {
	s := lang.NewString("Barista")
	if got := s.Length(); got != 7 {
		t.Errorf("Length() = %d, want 7", got)
	}
	if !s.Equals(lang.NewString("Barista")) {
		t.Error("equal strings did not compare equal")
	}
	if s.Equals(lang.NewString("barista")) {
		t.Error("case-different strings compared equal")
	}
	if got := s.Concat(lang.NewString("!")).GoString(); got != "Barista!" {
		t.Errorf("Concat() = %q, want %q", got, "Barista!")
	}
	if got := s.Substring(0, 3).GoString(); got != "Bar" {
		t.Errorf("Substring(0,3) = %q, want %q", got, "Bar")
	}
}

func TestStringBuilder(t *testing.T) {
	b := lang.NewStringBuilder()
	b.Init()
	b.AppendString(lang.NewString("Barista #"))
	b.AppendInt(7)
	if got := b.ToString().GoString(); got != "Barista #7" {
		t.Errorf("builder = %q, want %q", got, "Barista #7")
	}
}

func TestExceptionMatching(t *testing.T) {
	npe := lang.NewNullPointerException(lang.NewString("null"))
	if !lang.MatchesException(npe, "java/lang/RuntimeException") {
		t.Error("NullPointerException should match RuntimeException")
	}
	if !lang.MatchesException(npe, "java/lang/Exception") {
		t.Error("NullPointerException should match Exception")
	}
	if lang.MatchesException(npe, "java/lang/IndexOutOfBoundsException") {
		t.Error("NullPointerException should not match IndexOutOfBoundsException")
	}
	if got := lang.AsThrowable(npe); got == nil || got.GetMessage().GoString() != "null" {
		t.Errorf("AsThrowable(npe) = %#v", got)
	}
}

func TestArrayHelpersTranslateBoundsChecks(t *testing.T) {
	values := []int32{1, 2, 3}
	if got := lang.ArrayLoad(values, 1); got != 2 {
		t.Errorf("ArrayLoad(values,1) = %d, want 2", got)
	}
	lang.ArrayStore(values, 2, 9)
	if values[2] != 9 {
		t.Errorf("ArrayStore did not update slice: %v", values)
	}
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected an IndexOutOfBoundsException panic")
		}
		if !lang.MatchesException(recovered, "java/lang/IndexOutOfBoundsException") {
			t.Fatalf("recovered %T, want IndexOutOfBoundsException", recovered)
		}
	}()
	lang.ArrayLoad(values, 99)
}

func TestCurrentTimeMillis(t *testing.T) {
	if got := lang.CurrentTimeMillis(); got <= 0 {
		t.Errorf("CurrentTimeMillis() = %d, want a positive Unix timestamp", got)
	}
}

func TestClassNameUsesBinaryNotation(t *testing.T) {
	class := lang.NewClass("java/lang/String")
	if got := class.GetName().GoString(); got != "java.lang.String" {
		t.Errorf("GetName() = %q, want %q", got, "java.lang.String")
	}
}
