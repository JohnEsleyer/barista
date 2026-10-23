package lang

import (
	"fmt"
	"strconv"
)

// Long is the boxed form of java.lang.Long. Kotlin data classes normally use
// the primitive J descriptor; this shim keeps boxed Long references usable.
type Long struct {
	Object
	Value int64
}

// NewLong constructs a boxed Long.
func NewLong(value int64) *Long {
	return &Long{Value: value}
}

// LongValue returns the primitive value.
func (l *Long) LongValue() int64 {
	if l == nil {
		return 0
	}
	return l.Value
}

// HashCode implements Long.hashCode(). It is also reachable through Kotlin's
// structural equality helper.
func (l *Long) HashCode() int32 {
	if l == nil {
		panic(NewNullPointerException(NewString("Long is null")))
	}
	return LongHashCode(l.Value)
}

// Equals compares boxed Long values.
func (l *Long) Equals(other any) bool {
	o, ok := other.(*Long)
	if !ok {
		return false
	}
	if l == nil {
		return o == nil
	}
	return o != nil && l.Value == o.Value
}

// ToString renders the boxed value.
func (l *Long) ToString() *String {
	if l == nil {
		return NewString("null")
	}
	return NewString(fmt.Sprintf("%d", l.Value))
}

// String lets Long satisfy fmt.Stringer.
func (l *Long) String() string {
	return l.ToString().GoString()
}

// MarshalJSON allows *Long to be serialized as a native JSON number.
func (l *Long) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("null"), nil
	}
	return []byte(strconv.FormatInt(l.Value, 10)), nil
}

// LongHashCode implements the static java.lang.Long.hashCode(J) intrinsic.
func LongHashCode(value int64) int32 {
	unsigned := uint64(value)
	return int32(unsigned ^ (unsigned >> 32))
}

// CompareLong implements lcmp, returning -1, 0, or 1.
func CompareLong(left, right int64) int32 {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

// CheckCast implements JVM checkcast. Null casts succeed for reference types.
func CheckCast[T any](value any) T {
	var zero T
	if value == nil {
		return zero
	}
	typed, ok := value.(T)
	if !ok {
		panic(NewClassCastException(NewString("value cannot be cast to requested type")))
	}
	return typed
}

// IsInstanceOf implements JVM instanceof.
func IsInstanceOf[T any](value any) bool {
	if value == nil {
		return false
	}
	_, ok := value.(T)
	return ok
}
