package lang

import (
	"fmt"
	"strings"
)

// StringBuilder is the native shim for java.lang.StringBuilder.
type StringBuilder struct {
	Object
	builder strings.Builder
}

// NewStringBuilder constructs an empty builder.
func NewStringBuilder() *StringBuilder {
	return &StringBuilder{}
}

// Init implements the JVM <init> call. Extra arguments are ignored for now.
func (b *StringBuilder) Init(args ...any) *StringBuilder {
	if b == nil {
		return b
	}
	b.builder.Reset()
	return b
}

// AppendString appends a java.lang.String value.
func (b *StringBuilder) AppendString(value *String) *StringBuilder {
	if b == nil {
		panic(NewNullPointerException(NewString("StringBuilder is null")))
	}
	if value == nil {
		b.builder.WriteString("null")
	} else {
		b.builder.WriteString(value.value)
	}
	return b
}

// AppendInt appends an int.
func (b *StringBuilder) AppendInt(value int32) *StringBuilder {
	if b == nil {
		panic(NewNullPointerException(NewString("StringBuilder is null")))
	}
	fmt.Fprint(&b.builder, value)
	return b
}

// AppendAny appends arbitrary values, using Java-like null and String handling.
func (b *StringBuilder) AppendAny(value any) *StringBuilder {
	if b == nil {
		panic(NewNullPointerException(NewString("StringBuilder is null")))
	}
	switch v := value.(type) {
	case nil:
		b.builder.WriteString("null")
	case *String:
		if v == nil {
			b.builder.WriteString("null")
		} else {
			b.builder.WriteString(v.value)
		}
	default:
		fmt.Fprint(&b.builder, v)
	}
	return b
}

// Length returns the number of bytes currently buffered.
func (b *StringBuilder) Length() int32 {
	if b == nil {
		return 0
	}
	return int32(b.builder.Len())
}

// ToString returns the buffered contents as a String.
func (b *StringBuilder) ToString() *String {
	if b == nil {
		return NewString("")
	}
	return NewString(b.builder.String())
}

// String lets StringBuilder satisfy fmt.Stringer.
func (b *StringBuilder) String() string {
	if b == nil {
		return ""
	}
	return b.builder.String()
}
