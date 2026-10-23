package lang

import "strings"

// Class is a minimal native shim for java.lang.Class.
type Class struct {
	Object
	Name string
}

// NewClass constructs a Class for the given JVM internal name.
func NewClass(internalName string) *Class {
	return &Class{Name: internalName}
}

// GetName returns the binary name of the class.
func (c *Class) GetName() *String {
	if c == nil {
		return nil
	}
	return NewString(strings.ReplaceAll(c.Name, "/", "."))
}

// ToString returns the class name.
func (c *Class) ToString() *String {
	return c.GetName()
}

// String lets Class satisfy fmt.Stringer.
func (c *Class) String() string {
	if c == nil {
		return ""
	}
	return c.Name
}
