package lang

import (
	"encoding/json"
	"unicode/utf16"
)

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

// MarshalJSON allows *String to be serialized as a native JSON string.
func (s *String) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("null"), nil
	}
	return json.Marshal(s.value)
}

// UnmarshalJSON allows native JSON strings to be deserialized into *String.
func (s *String) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	s.value = str
	return nil
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

// Init initializes or constructs a String instance from bytes or strings.
func (s *String) Init(args ...any) *String {
	if s == nil {
		return s
	}
	if len(args) == 0 {
		s.value = ""
		return s
	}
	switch v := args[0].(type) {
	case []int8:
		if len(args) >= 3 {
			off := args[1].(int32)
			length := args[2].(int32)
			if off >= 0 && length >= 0 && int(off+length) <= len(v) {
				buf := make([]byte, length)
				for i := 0; i < int(length); i++ {
					buf[i] = byte(v[int(off)+i])
				}
				s.value = string(buf)
				return s
			}
		}
		buf := make([]byte, len(v))
		for i, b := range v {
			buf[i] = byte(b)
		}
		s.value = string(buf)
	case []byte:
		s.value = string(v)
	case string:
		s.value = v
	case *String:
		if v != nil {
			s.value = v.value
		}
	}
	return s
}

// GetBytes encodes this String into a sequence of bytes using UTF-8, returning []int8.
func (s *String) GetBytes() []int8 {
	if s == nil {
		return nil
	}
	raw := []byte(s.value)
	res := make([]int8, len(raw))
	for i, b := range raw {
		res[i] = int8(b)
	}
	return res
}
