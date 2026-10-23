package classfile

import (
	"encoding/binary"
	"fmt"
	"math"
)

// JVMS §4.4 constant pool tags.
const (
	TagUtf8               = 1
	TagInteger            = 3
	TagFloat              = 4
	TagLong               = 5
	TagDouble             = 6
	TagClass              = 7
	TagString             = 8
	TagFieldref           = 9
	TagMethodref          = 10
	TagInterfaceMethodref = 11
	TagNameAndType        = 12
	TagMethodHandle       = 15
	TagMethodType         = 16
	TagDynamic            = 17
	TagInvokeDynamic      = 18
	TagModule             = 19
	TagPackage            = 20
)

// CPInfo represents an entry in the constant pool.
type CPInfo interface {
	Tag() uint8
}

// ConstantPool is the 1-indexed constant pool. Index 0 is always nil, and the
// slot following a Long or Double entry is nil, as required by the JVMS.
type ConstantPool []CPInfo

func (cp ConstantPool) entry(index uint16) (CPInfo, error) {
	if index == 0 || int(index) >= len(cp) || cp[index] == nil {
		return nil, fmt.Errorf("invalid constant pool index %d", index)
	}
	return cp[index], nil
}

// ResolveUtf8 returns the string value of a Utf8 constant pool entry.
func (cp ConstantPool) ResolveUtf8(index uint16) (string, error) {
	entry, err := cp.entry(index)
	if err != nil {
		return "", err
	}
	utf8, ok := entry.(*ConstantUtf8)
	if !ok {
		return "", fmt.Errorf("constant pool entry at %d is not UTF-8 (tag %d)", index, entry.Tag())
	}
	return utf8.Value, nil
}

// ResolveClassName returns the decoded internal class name (e.g. "java/lang/Object").
func (cp ConstantPool) ResolveClassName(index uint16) (string, error) {
	entry, err := cp.entry(index)
	if err != nil {
		return "", err
	}
	classEntry, ok := entry.(*ConstantClass)
	if !ok {
		return "", fmt.Errorf("constant pool entry at %d is not CONSTANT_Class", index)
	}
	return cp.ResolveUtf8(classEntry.NameIndex)
}

// ResolveString returns the UTF-8 value of a CONSTANT_String entry.
func (cp ConstantPool) ResolveString(index uint16) (string, error) {
	entry, err := cp.entry(index)
	if err != nil {
		return "", err
	}
	str, ok := entry.(*ConstantString)
	if !ok {
		return "", fmt.Errorf("constant pool entry at %d is not CONSTANT_String", index)
	}
	return cp.ResolveUtf8(str.StringIndex)
}

// ResolveNameAndType resolves the name and descriptor pair.
func (cp ConstantPool) ResolveNameAndType(index uint16) (string, string, error) {
	entry, err := cp.entry(index)
	if err != nil {
		return "", "", err
	}
	nat, ok := entry.(*ConstantNameAndType)
	if !ok {
		return "", "", fmt.Errorf("constant pool entry at %d is not CONSTANT_NameAndType", index)
	}
	name, err := cp.ResolveUtf8(nat.NameIndex)
	if err != nil {
		return "", "", err
	}
	desc, err := cp.ResolveUtf8(nat.DescriptorIndex)
	if err != nil {
		return "", "", err
	}
	return name, desc, nil
}

// ResolveMemberRef resolves a field, method, or interface-method reference.
func (cp ConstantPool) ResolveMemberRef(index uint16) (className, name, descriptor string, err error) {
	entry, err := cp.entry(index)
	if err != nil {
		return "", "", "", err
	}
	ref, ok := entry.(*ConstantRef)
	if !ok {
		return "", "", "", fmt.Errorf("constant pool entry at %d is not a member reference", index)
	}
	className, err = cp.ResolveClassName(ref.ClassIndex)
	if err != nil {
		return "", "", "", err
	}
	name, descriptor, err = cp.ResolveNameAndType(ref.NameAndTypeIndex)
	return className, name, descriptor, err
}

// ResolveFieldRef resolves the owning class name, field name, and descriptor.
func (cp ConstantPool) ResolveFieldRef(index uint16) (string, string, string, error) {
	entry, err := cp.entry(index)
	if err != nil {
		return "", "", "", err
	}
	ref, ok := entry.(*ConstantRef)
	if !ok || ref.Tag() != TagFieldref {
		return "", "", "", fmt.Errorf("constant pool entry at %d is not CONSTANT_Fieldref", index)
	}
	return cp.ResolveMemberRef(index)
}

// ResolveMethodRef resolves the owning class name, method name, and descriptor.
func (cp ConstantPool) ResolveMethodRef(index uint16) (string, string, string, error) {
	entry, err := cp.entry(index)
	if err != nil {
		return "", "", "", err
	}
	ref, ok := entry.(*ConstantRef)
	if !ok || (ref.Tag() != TagMethodref && ref.Tag() != TagInterfaceMethodref) {
		return "", "", "", fmt.Errorf("constant pool entry at %d is not a method reference", index)
	}
	return cp.ResolveMemberRef(index)
}

// Concrete constant pool entries.

type ConstantUtf8 struct{ Value string }

func (c *ConstantUtf8) Tag() uint8 { return TagUtf8 }

type ConstantInteger struct{ Value int32 }

func (c *ConstantInteger) Tag() uint8 { return TagInteger }

type ConstantFloat struct{ Value float32 }

func (c *ConstantFloat) Tag() uint8 { return TagFloat }

type ConstantLong struct{ Value int64 }

func (c *ConstantLong) Tag() uint8 { return TagLong }

type ConstantDouble struct{ Value float64 }

func (c *ConstantDouble) Tag() uint8 { return TagDouble }

type ConstantClass struct{ NameIndex uint16 }

func (c *ConstantClass) Tag() uint8 { return TagClass }

type ConstantString struct{ StringIndex uint16 }

func (c *ConstantString) Tag() uint8 { return TagString }

// ConstantRef represents CONSTANT_Fieldref, CONSTANT_Methodref, or
// CONSTANT_InterfaceMethodref.
type ConstantRef struct {
	tag              uint8
	ClassIndex       uint16
	NameAndTypeIndex uint16
}

func (c *ConstantRef) Tag() uint8 { return c.tag }

type ConstantNameAndType struct {
	NameIndex       uint16
	DescriptorIndex uint16
}

func (c *ConstantNameAndType) Tag() uint8 { return TagNameAndType }

type ConstantMethodHandle struct {
	ReferenceKind  uint8
	ReferenceIndex uint16
}

func (c *ConstantMethodHandle) Tag() uint8 { return TagMethodHandle }

type ConstantMethodType struct{ DescriptorIndex uint16 }

func (c *ConstantMethodType) Tag() uint8 { return TagMethodType }

// ConstantDynamic represents a CONSTANT_Dynamic entry (JVMS §4.4.10).
type ConstantDynamic struct {
	BootstrapMethodAttrIndex uint16
	NameAndTypeIndex         uint16
}

func (c *ConstantDynamic) Tag() uint8 { return TagDynamic }

type ConstantInvokeDynamic struct {
	BootstrapMethodAttrIndex uint16
	NameAndTypeIndex         uint16
}

func (c *ConstantInvokeDynamic) Tag() uint8 { return TagInvokeDynamic }

type ConstantModule struct{ NameIndex uint16 }

func (c *ConstantModule) Tag() uint8 { return TagModule }

type ConstantPackage struct{ NameIndex uint16 }

func (c *ConstantPackage) Tag() uint8 { return TagPackage }

// parseConstantPool parses the 1-indexed constant pool table.
func parseConstantPool(r *Reader, count uint16) (ConstantPool, error) {
	cp := make(ConstantPool, count)
	for i := 1; i < int(count); i++ {
		tag, err := r.ReadU1()
		if err != nil {
			return nil, err
		}

		switch tag {
		case TagUtf8:
			length, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			data, err := r.ReadBytes(int(length))
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantUtf8{Value: string(data)}

		case TagInteger:
			v, err := r.ReadU4()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantInteger{Value: int32(v)}

		case TagFloat:
			v, err := r.ReadU4()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantFloat{Value: math.Float32frombits(v)}

		case TagLong:
			if i+1 >= int(count) {
				return nil, fmt.Errorf("CONSTANT_Long at index %d has no reserved slot", i)
			}
			data, err := r.ReadBytes(8)
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantLong{Value: int64(binary.BigEndian.Uint64(data))}
			i++ // Long takes two constant-pool slots; the second is unusable.

		case TagDouble:
			if i+1 >= int(count) {
				return nil, fmt.Errorf("CONSTANT_Double at index %d has no reserved slot", i)
			}
			data, err := r.ReadBytes(8)
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantDouble{Value: math.Float64frombits(binary.BigEndian.Uint64(data))}
			i++ // Double takes two constant-pool slots; the second is unusable.

		case TagClass:
			idx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantClass{NameIndex: idx}

		case TagString:
			idx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantString{StringIndex: idx}

		case TagFieldref, TagMethodref, TagInterfaceMethodref:
			classIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			natIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantRef{tag: tag, ClassIndex: classIdx, NameAndTypeIndex: natIdx}

		case TagNameAndType:
			nameIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			descIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantNameAndType{NameIndex: nameIdx, DescriptorIndex: descIdx}

		case TagMethodHandle:
			refKind, err := r.ReadU1()
			if err != nil {
				return nil, err
			}
			refIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantMethodHandle{ReferenceKind: refKind, ReferenceIndex: refIdx}

		case TagMethodType:
			descIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantMethodType{DescriptorIndex: descIdx}

		case TagDynamic:
			bsIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			natIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantDynamic{BootstrapMethodAttrIndex: bsIdx, NameAndTypeIndex: natIdx}

		case TagInvokeDynamic:
			bsIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			natIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantInvokeDynamic{BootstrapMethodAttrIndex: bsIdx, NameAndTypeIndex: natIdx}

		case TagModule:
			nameIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantModule{NameIndex: nameIdx}

		case TagPackage:
			nameIdx, err := r.ReadU2()
			if err != nil {
				return nil, err
			}
			cp[i] = &ConstantPackage{NameIndex: nameIdx}

		default:
			return nil, fmt.Errorf("unsupported constant pool tag %d at index %d", tag, i)
		}
	}
	return cp, nil
}
