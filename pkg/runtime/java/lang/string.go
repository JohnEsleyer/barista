package lang

import "unicode/utf16"

// String is the native shim for java.lang.String.
type String struct {
	Object
	value string
}

// NewString constructs a Java String from a Go string.
func NewString(value string) *String {
	return &String{value: value}
}

// GoString exposes the underlying Go string to generated code and tests.
func (s *String) GoString() string {
	if s == nil {
		return ""
	}
	return s.value
}

// String lets String satisfy fmt.Stringer while keeping GoString explicit.
func (s *String) String() string {
	return s.GoString()
}

// Length returns the number of UTF-16 code units, matching Java semantics for
// ordinary strings.
func (s *String) Length() int32 {
	if s == nil {
		return 0
	}
	return int32(len(utf16.Encode([]rune(s.value))))
}

// IsEmpty reports whether the string has zero length.
func (s *String) IsEmpty() bool {
	return s == nil || s.value == ""
}

// Equals compares two Strings by value. Java's String.equals requires a String
// argument, so other Object types compare false.
func (s *String) Equals(other any) bool {
	o, ok := other.(*String)
	if !ok {
		return false
	}
	if s == nil {
		return o == nil
	}
	return o != nil && s.value == o.value
}

// CharAt returns one UTF-16 code unit.
func (s *String) CharAt(index int32) uint16 {
	if s == nil {
		panic(NewNullPointerException(NewString("String is null")))
	}
	units := utf16.Encode([]rune(s.value))
	if index < 0 || int(index) >= len(units) {
		panic(NewIndexOutOfBoundsException(NewString("string index out of range")))
	}
	return units[index]
}

// Substring returns the substring in [start, end).
func (s *String) Substring(start, end int32) *String {
	if s == nil {
		panic(NewNullPointerException(NewString("String is null")))
	}
	runes := []rune(s.value)
	if start < 0 || end < start || int(end) > len(runes) {
		panic(NewIndexOutOfBoundsException(NewString("substring index out of range")))
	}
	return NewString(string(runes[start:end]))
}

// Concat appends another String.
func (s *String) Concat(other *String) *String {
	if s == nil {
		panic(NewNullPointerException(NewString("String is null")))
	}
	if other == nil {
		return NewString(s.value + "null")
	}
	return NewString(s.value + other.value)
}

// HashCode returns the Java String hash over UTF-16 code units.
func (s *String) HashCode() int32 {
	if s == nil {
		panic(NewNullPointerException(NewString("String is null")))
	}
	var hash int32
	for _, unit := range utf16.Encode([]rune(s.value)) {
		hash = 31*hash + int32(unit)
	}
	return hash
}

// ToString returns the receiver.
func (s *String) ToString() *String {
	return s
}
