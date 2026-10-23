package lang

import "reflect"

// Object is the native shim for java.lang.Object. Generated classes do not
// embed Object yet, but runtime-backed values share this small surface.
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
