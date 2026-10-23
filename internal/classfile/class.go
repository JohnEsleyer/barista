package classfile

import (
	"fmt"
	"io"
	"os"
)

const ClassMagic = 0xCAFEBABE

const (
	MinSupportedMajorVersion = 45
	MaxSupportedMajorVersion = 70
)

// Common access flags. Some bits have different meanings for classes, fields,
// and methods, so aliases are provided below.
const (
	AccPublic       = 0x0001
	AccPrivate      = 0x0002
	AccProtected    = 0x0004
	AccStatic       = 0x0008
	AccFinal        = 0x0010
	AccSuper        = 0x0020
	AccSynchronized = 0x0020
	AccVolatile     = 0x0040
	AccBridge       = 0x0040
	AccTransient    = 0x0080
	AccVarargs      = 0x0080
	AccNative       = 0x0100
	AccInterface    = 0x0200
	AccAbstract     = 0x0400
	AccStrict       = 0x0800
	AccSynthetic    = 0x1000
	AccAnnotation   = 0x2000
	AccEnum         = 0x4000
	AccModule       = 0x8000
)

// MemberInfo describes a JVM field or method.
type MemberInfo struct {
	AccessFlags uint16
	Name        string
	Descriptor  string
	Attributes  []AttributeInfo
}

// FindCode returns the CodeAttribute for a method, if present.
func (m *MemberInfo) FindCode() *CodeAttribute {
	for _, attr := range m.Attributes {
		if code, ok := attr.(*CodeAttribute); ok {
			return code
		}
	}
	return nil
}

// IsStatic reports whether the member has ACC_STATIC set.
func (m *MemberInfo) IsStatic() bool { return m.AccessFlags&AccStatic != 0 }

// ClassFile models an in-memory parsed JVM class.
type ClassFile struct {
	MinorVersion uint16
	MajorVersion uint16
	ConstantPool ConstantPool
	AccessFlags  uint16
	ThisClass    string
	SuperClass   string
	Interfaces   []string
	Fields       []MemberInfo
	Methods      []MemberInfo
	Attributes   []AttributeInfo
}

// FindMethod returns the first method matching name and descriptor.
func (cf *ClassFile) FindMethod(name, descriptor string) *MemberInfo {
	for i := range cf.Methods {
		if cf.Methods[i].Name == name && cf.Methods[i].Descriptor == descriptor {
			return &cf.Methods[i]
		}
	}
	return nil
}

// ParseClassFile reads a .class from a byte stream.
func ParseClassFile(r io.Reader) (*ClassFile, error) {
	reader, err := ReadAll(r)
	if err != nil {
		return nil, err
	}

	magic, err := reader.ReadU4()
	if err != nil {
		return nil, err
	}
	if magic != ClassMagic {
		return nil, fmt.Errorf("%w: found 0x%08X", ErrBadMagic, magic)
	}

	minor, err := reader.ReadU2()
	if err != nil {
		return nil, err
	}
	major, err := reader.ReadU2()
	if err != nil {
		return nil, err
	}
	if major < MinSupportedMajorVersion || major > MaxSupportedMajorVersion {
		return nil, fmt.Errorf("unsupported class file version %d.%d (supported major versions %d-%d)", major, minor, MinSupportedMajorVersion, MaxSupportedMajorVersion)
	}

	cpCount, err := reader.ReadU2()
	if err != nil {
		return nil, err
	}
	if cpCount == 0 {
		return nil, fmt.Errorf("invalid constant pool count 0")
	}
	cp, err := parseConstantPool(reader, cpCount)
	if err != nil {
		return nil, fmt.Errorf("constant pool parsing error: %w", err)
	}

	accessFlags, err := reader.ReadU2()
	if err != nil {
		return nil, err
	}

	thisClassIdx, err := reader.ReadU2()
	if err != nil {
		return nil, err
	}
	thisClass, err := cp.ResolveClassName(thisClassIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve this_class: %w", err)
	}

	superClassIdx, err := reader.ReadU2()
	if err != nil {
		return nil, err
	}
	var superClass string
	if superClassIdx > 0 {
		superClass, err = cp.ResolveClassName(superClassIdx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve super_class: %w", err)
		}
	}

	interfacesCount, err := reader.ReadU2()
	if err != nil {
		return nil, err
	}
	interfaces := make([]string, interfacesCount)
	for i := 0; i < int(interfacesCount); i++ {
		idx, err := reader.ReadU2()
		if err != nil {
			return nil, err
		}
		iface, err := cp.ResolveClassName(idx)
		if err != nil {
			return nil, err
		}
		interfaces[i] = iface
	}

	fields, err := parseMembers(reader, cp)
	if err != nil {
		return nil, fmt.Errorf("field parsing error: %w", err)
	}

	methods, err := parseMembers(reader, cp)
	if err != nil {
		return nil, fmt.Errorf("method parsing error: %w", err)
	}

	attrsCount, err := reader.ReadU2()
	if err != nil {
		return nil, err
	}
	attrs, err := parseAttributes(reader, cp, attrsCount)
	if err != nil {
		return nil, fmt.Errorf("class attributes parsing error: %w", err)
	}
	if reader.Remaining() != 0 {
		return nil, fmt.Errorf("class file has %d trailing bytes", reader.Remaining())
	}

	return &ClassFile{
		MinorVersion: minor,
		MajorVersion: major,
		ConstantPool: cp,
		AccessFlags:  accessFlags,
		ThisClass:    thisClass,
		SuperClass:   superClass,
		Interfaces:   interfaces,
		Fields:       fields,
		Methods:      methods,
		Attributes:   attrs,
	}, nil
}

// ParseClassFilePath parses a .class file directly from disk.
func ParseClassFilePath(path string) (*ClassFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseClassFile(f)
}

func parseMembers(r *Reader, cp ConstantPool) ([]MemberInfo, error) {
	count, err := r.ReadU2()
	if err != nil {
		return nil, err
	}
	members := make([]MemberInfo, count)
	for i := 0; i < int(count); i++ {
		flags, err := r.ReadU2()
		if err != nil {
			return nil, err
		}
		nameIdx, err := r.ReadU2()
		if err != nil {
			return nil, err
		}
		name, err := cp.ResolveUtf8(nameIdx)
		if err != nil {
			return nil, err
		}
		descIdx, err := r.ReadU2()
		if err != nil {
			return nil, err
		}
		desc, err := cp.ResolveUtf8(descIdx)
		if err != nil {
			return nil, err
		}
		attrCount, err := r.ReadU2()
		if err != nil {
			return nil, err
		}
		attrs, err := parseAttributes(r, cp, attrCount)
		if err != nil {
			return nil, err
		}
		members[i] = MemberInfo{
			AccessFlags: flags,
			Name:        name,
			Descriptor:  desc,
			Attributes:  attrs,
		}
	}
	return members, nil
}
