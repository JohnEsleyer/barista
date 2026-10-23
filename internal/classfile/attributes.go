package classfile

import "fmt"

// AttributeInfo represents a parsed or unrecognised JVM attribute.
type AttributeInfo interface {
	Name() string
}

// RawAttribute captures attributes that are not specialised yet.
type RawAttribute struct {
	AttrName string
	Info     []byte
}

func (r *RawAttribute) Name() string { return r.AttrName }

// ExceptionTableEntry describes a protected try-catch range.
type ExceptionTableEntry struct {
	StartPC   uint16
	EndPC     uint16
	HandlerPC uint16
	CatchType uint16
}

// CodeAttribute contains a method's bytecode and related metadata.
type CodeAttribute struct {
	MaxStack       uint16
	MaxLocals      uint16
	Code           []byte
	ExceptionTable []ExceptionTableEntry
	Attributes     []AttributeInfo
}

func (c *CodeAttribute) Name() string { return "Code" }

// FindAttribute returns the first nested attribute with the given name.
func (c *CodeAttribute) FindAttribute(name string) AttributeInfo {
	for _, attr := range c.Attributes {
		if attr.Name() == name {
			return attr
		}
	}
	return nil
}

// LineNumberEntry maps a bytecode offset to a source line.
type LineNumberEntry struct {
	StartPC    uint16
	LineNumber uint16
}

// LineNumberTableAttribute is the parsed form of the LineNumberTable attribute.
type LineNumberTableAttribute struct {
	Entries []LineNumberEntry
}

func (l *LineNumberTableAttribute) Name() string { return "LineNumberTable" }

func parseAttributes(r *Reader, cp ConstantPool, count uint16) ([]AttributeInfo, error) {
	attrs := make([]AttributeInfo, 0, count)
	for i := 0; i < int(count); i++ {
		nameIdx, err := r.ReadU2()
		if err != nil {
			return nil, err
		}
		name, err := cp.ResolveUtf8(nameIdx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve attribute name at index %d: %w", nameIdx, err)
		}
		length, err := r.ReadU4()
		if err != nil {
			return nil, err
		}
		if uint64(length) > uint64(r.Remaining()) {
			return nil, fmt.Errorf("attribute %q declares %d bytes but only %d remain: %w", name, length, r.Remaining(), ErrUnexpectedEOF)
		}
		info, err := r.ReadBytes(int(length))
		if err != nil {
			return nil, err
		}

		switch name {
		case "Code":
			sub := NewReader(info)
			code, err := parseCodeAttribute(sub, cp)
			if err != nil {
				return nil, fmt.Errorf("failed to parse Code attribute: %w", err)
			}
			if sub.Remaining() != 0 {
				return nil, fmt.Errorf("Code attribute has %d trailing bytes", sub.Remaining())
			}
			attrs = append(attrs, code)
		case "LineNumberTable":
			lineTable, err := parseLineNumberTable(info)
			if err != nil {
				return nil, fmt.Errorf("failed to parse LineNumberTable attribute: %w", err)
			}
			attrs = append(attrs, lineTable)
		default:
			attrs = append(attrs, &RawAttribute{AttrName: name, Info: info})
		}
	}
	return attrs, nil
}

func parseCodeAttribute(r *Reader, cp ConstantPool) (*CodeAttribute, error) {
	maxStack, err := r.ReadU2()
	if err != nil {
		return nil, err
	}
	maxLocals, err := r.ReadU2()
	if err != nil {
		return nil, err
	}
	codeLength, err := r.ReadU4()
	if err != nil {
		return nil, err
	}
	if uint64(codeLength) > uint64(r.Remaining()) {
		return nil, fmt.Errorf("code length %d exceeds remaining attribute bytes: %w", codeLength, ErrUnexpectedEOF)
	}
	code, err := r.ReadBytes(int(codeLength))
	if err != nil {
		return nil, err
	}

	excTableLength, err := r.ReadU2()
	if err != nil {
		return nil, err
	}
	excTable := make([]ExceptionTableEntry, excTableLength)
	for i := 0; i < int(excTableLength); i++ {
		startPC, err := r.ReadU2()
		if err != nil {
			return nil, err
		}
		endPC, err := r.ReadU2()
		if err != nil {
			return nil, err
		}
		handlerPC, err := r.ReadU2()
		if err != nil {
			return nil, err
		}
		catchType, err := r.ReadU2()
		if err != nil {
			return nil, err
		}
		excTable[i] = ExceptionTableEntry{
			StartPC:   startPC,
			EndPC:     endPC,
			HandlerPC: handlerPC,
			CatchType: catchType,
		}
	}

	subAttrsCount, err := r.ReadU2()
	if err != nil {
		return nil, err
	}
	subAttrs, err := parseAttributes(r, cp, subAttrsCount)
	if err != nil {
		return nil, err
	}

	return &CodeAttribute{
		MaxStack:       maxStack,
		MaxLocals:      maxLocals,
		Code:           code,
		ExceptionTable: excTable,
		Attributes:     subAttrs,
	}, nil
}

func parseLineNumberTable(info []byte) (*LineNumberTableAttribute, error) {
	r := NewReader(info)
	count, err := r.ReadU2()
	if err != nil {
		return nil, err
	}
	entries := make([]LineNumberEntry, count)
	for i := 0; i < int(count); i++ {
		startPC, err := r.ReadU2()
		if err != nil {
			return nil, err
		}
		lineNumber, err := r.ReadU2()
		if err != nil {
			return nil, err
		}
		entries[i] = LineNumberEntry{StartPC: startPC, LineNumber: lineNumber}
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("LineNumberTable has %d trailing bytes", r.Remaining())
	}
	return &LineNumberTableAttribute{Entries: entries}, nil
}
