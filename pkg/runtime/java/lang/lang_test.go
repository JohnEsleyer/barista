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
	b.AppendChar('!')
	if got := b.ToString().GoString(); got != "Barista #7!" {
		t.Errorf("builder = %q, want %q", got, "Barista #7!")
	}
}

func TestStringHashCodeMatchesJava(t *testing.T) {
	if got := lang.NewString("Barista").HashCode(); got != 1331252106 {
		t.Errorf("HashCode(\"Barista\") = %d, want 1331252106", got)
	}
}

func TestLongAndCastHelpers(t *testing.T) {
	if got := lang.LongHashCode(42); got != 42 {
		t.Errorf("LongHashCode(42) = %d, want 42", got)
	}
	if got := lang.LongHashCode(0x1_0000_0001); got != 0 {
		t.Errorf("LongHashCode(0x100000001) = %d, want 0", got)
	}
	if got := lang.CompareLong(1, 2); got != -1 {
		t.Errorf("CompareLong(1,2) = %d, want -1", got)
	}
	if got := lang.CompareLong(2, 2); got != 0 {
		t.Errorf("CompareLong(2,2) = %d, want 0", got)
	}
	if got := lang.CompareLong(3, 2); got != 1 {
		t.Errorf("CompareLong(3,2) = %d, want 1", got)
	}

	s := lang.NewString("Barista")
	if got := lang.CheckCast[*lang.String](s); got != s {
		t.Error("CheckCast did not return the concrete value")
	}
	if !lang.IsInstanceOf[*lang.String](s) {
		t.Error("IsInstanceOf[*String] = false")
	}
	if lang.IsInstanceOf[*lang.IndexOutOfBoundsException](s) {
		t.Error("IsInstanceOf[*IndexOutOfBoundsException] = true")
	}
	if got := lang.CheckCast[*lang.String](nil); got != nil {
		t.Error("CheckCast(nil) did not return nil")
	}
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected a ClassCastException panic")
		}
		if !lang.MatchesException(recovered, "java/lang/ClassCastException") {
			t.Fatalf("recovered %T, want ClassCastException", recovered)
		}
	}()
	lang.CheckCast[*lang.IndexOutOfBoundsException](s)
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
