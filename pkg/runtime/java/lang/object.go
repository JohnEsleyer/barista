package lang

import (
	"fmt"
	"reflect"
)

// Object is the native shim for java.lang.Object.
type Object struct{}

// HashCode returns a stable identity-based hash for the object.
func (o *Object) HashCode() int32 {
	if o == nil {
		return 0
	}
	return int32(reflect.ValueOf(o).Pointer())
}

// Equals implements reference equality for Object values.
func (o *Object) Equals(other any) bool {
	if o == nil {
		return other == nil
	}
	return o == other
}

// ToString returns the default Object string representation.
func (o *Object) ToString() *String {
	return NewString("java.lang.Object")
}

// String lets Object satisfy fmt.Stringer.
func (o *Object) String() string {
	return o.ToString().GoString()
}

// ObjectEquals implements Java Object.equals(Object) dynamically across any types.
func ObjectEquals(left, right any) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	if eq, ok := left.(interface{ Equals(any) bool }); ok {
		return eq.Equals(right)
	}
	return left == right
}

// ObjectHashCode implements Java Object.hashCode() dynamically across any types.
func ObjectHashCode(value any) int32 {
	if value == nil {
		return 0
	}
	if h, ok := value.(interface{ HashCode() int32 }); ok {
		return h.HashCode()
	}
	return int32(reflect.ValueOf(value).Pointer())
}

// ObjectToString implements Java Object.toString() dynamically across any types.
func ObjectToString(value any) *String {
	if value == nil {
		return NewString("null")
	}
	if s, ok := value.(interface{ ToString() *String }); ok {
		return s.ToString()
	}
	if s, ok := value.(fmt.Stringer); ok {
		return NewString(s.String())
	}
	return NewString(fmt.Sprintf("%v", value))
}
