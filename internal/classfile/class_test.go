package classfile_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/JohnEsleyer/barista/internal/classfile"
)

func writeU1(t *testing.T, buf *bytes.Buffer, v uint8) {
	t.Helper()
	if err := buf.WriteByte(v); err != nil {
		t.Fatalf("write u1: %v", err)
	}
}

func writeU2(t *testing.T, buf *bytes.Buffer, v uint16) {
	t.Helper()
	if err := binary.Write(buf, binary.BigEndian, v); err != nil {
		t.Fatalf("write u2: %v", err)
	}
}

func writeU4(t *testing.T, buf *bytes.Buffer, v uint32) {
	t.Helper()
	if err := binary.Write(buf, binary.BigEndian, v); err != nil {
		t.Fatalf("write u4: %v", err)
	}
}

func writeU8(t *testing.T, buf *bytes.Buffer, v uint64) {
	t.Helper()
	if err := binary.Write(buf, binary.BigEndian, v); err != nil {
		t.Fatalf("write u8: %v", err)
	}
}

func writeClassHeader(t *testing.T, buf *bytes.Buffer, major uint16, cpCount uint16) {
	t.Helper()
	writeU4(t, buf, classfile.ClassMagic)
	writeU2(t, buf, 0)
	writeU2(t, buf, major)
	writeU2(t, buf, cpCount)
}

func writeClassTail(t *testing.T, buf *bytes.Buffer, thisClass uint16) {
	t.Helper()
	writeU2(t, buf, classfile.AccPublic)
	writeU2(t, buf, thisClass)
	writeU2(t, buf, 0) // super_class
	writeU2(t, buf, 0) // interfaces
	writeU2(t, buf, 0) // fields
	writeU2(t, buf, 0) // methods
	writeU2(t, buf, 0) // attributes
}

func TestParseClassFileHeader(t *testing.T) {
	buf := new(bytes.Buffer)
	writeClassHeader(t, buf, 61, 3)

	writeU1(t, buf, classfile.TagUtf8)
	writeU2(t, buf, 4)
	buf.WriteString("Test")

	writeU1(t, buf, classfile.TagClass)
	writeU2(t, buf, 1)
	writeClassTail(t, buf, 2)

	cf, err := classfile.ParseClassFile(buf)
	if err != nil {
		t.Fatalf("expected successful parse, got: %v", err)
	}
	if cf.MajorVersion != 61 || cf.MinorVersion != 0 {
		t.Errorf("expected version 61.0, got %d.%d", cf.MajorVersion, cf.MinorVersion)
	}
	if cf.ThisClass != "Test" {
		t.Errorf("expected class name Test, got %q", cf.ThisClass)
	}
}

func TestParseClassFileRejectsBadMagic(t *testing.T) {
	buf := new(bytes.Buffer)
	writeU4(t, buf, 0xDEADBEEF)
	writeU2(t, buf, 0)
	writeU2(t, buf, 61)
	writeU2(t, buf, 1)
	writeClassTail(t, buf, 0)

	_, err := classfile.ParseClassFile(buf)
	if err == nil {
		t.Fatal("expected an error for bad magic")
	}
}

func TestParseClassFileRejectsUnsupportedVersion(t *testing.T) {
	buf := new(bytes.Buffer)
	writeClassHeader(t, buf, classfile.MaxSupportedMajorVersion+1, 1)
	writeClassTail(t, buf, 0)

	_, err := classfile.ParseClassFile(buf)
	if err == nil {
		t.Fatal("expected an error for an unsupported class file version")
	}
}

func TestParseConstantPoolLongAndDoubleReserveSlots(t *testing.T) {
	buf := new(bytes.Buffer)
	writeClassHeader(t, buf, 61, 7)

	writeU1(t, buf, classfile.TagLong)
	writeU8(t, buf, uint64(1<<40))

	writeU1(t, buf, classfile.TagDouble)
	writeU8(t, buf, math.Float64bits(1.5))

	writeU1(t, buf, classfile.TagUtf8)
	writeU2(t, buf, 4)
	buf.WriteString("Test")

	writeU1(t, buf, classfile.TagClass)
	writeU2(t, buf, 5)
	writeClassTail(t, buf, 6)

	cf, err := classfile.ParseClassFile(buf)
	if err != nil {
		t.Fatalf("parse class: %v", err)
	}
	long, ok := cf.ConstantPool[1].(*classfile.ConstantLong)
	if !ok {
		t.Fatalf("cp[1] = %T, want *ConstantLong", cf.ConstantPool[1])
	}
	if long.Value != 1<<40 {
		t.Errorf("long = %d, want %d", long.Value, 1<<40)
	}
	if cf.ConstantPool[2] != nil {
		t.Errorf("cp[2] should be reserved and nil, got %T", cf.ConstantPool[2])
	}
	double, ok := cf.ConstantPool[3].(*classfile.ConstantDouble)
	if !ok {
		t.Fatalf("cp[3] = %T, want *ConstantDouble", cf.ConstantPool[3])
	}
	if double.Value != 1.5 {
		t.Errorf("double = %v, want 1.5", double.Value)
	}
	if cf.ConstantPool[4] != nil {
		t.Errorf("cp[4] should be reserved and nil, got %T", cf.ConstantPool[4])
	}
	if cf.ThisClass != "Test" {
		t.Errorf("ThisClass = %q, want Test", cf.ThisClass)
	}
}

func TestParseConstantPoolDynamicModuleAndPackage(t *testing.T) {
	buf := new(bytes.Buffer)
	writeClassHeader(t, buf, 61, 9)

	writeU1(t, buf, classfile.TagUtf8)
	writeU2(t, buf, 3)
	buf.WriteString("foo")

	writeU1(t, buf, classfile.TagUtf8)
	writeU2(t, buf, 1)
	buf.WriteString("I")

	writeU1(t, buf, classfile.TagNameAndType)
	writeU2(t, buf, 1)
	writeU2(t, buf, 2)

	writeU1(t, buf, classfile.TagDynamic)
	writeU2(t, buf, 0)
	writeU2(t, buf, 3)

	writeU1(t, buf, classfile.TagModule)
	writeU2(t, buf, 1)

	writeU1(t, buf, classfile.TagPackage)
	writeU2(t, buf, 1)

	writeU1(t, buf, classfile.TagUtf8)
	writeU2(t, buf, 4)
	buf.WriteString("Test")

	writeU1(t, buf, classfile.TagClass)
	writeU2(t, buf, 7)
	writeClassTail(t, buf, 8)

	cf, err := classfile.ParseClassFile(buf)
	if err != nil {
		t.Fatalf("parse class: %v", err)
	}
	if _, ok := cf.ConstantPool[4].(*classfile.ConstantDynamic); !ok {
		t.Errorf("cp[4] = %T, want *ConstantDynamic", cf.ConstantPool[4])
	}
	if _, ok := cf.ConstantPool[5].(*classfile.ConstantModule); !ok {
		t.Errorf("cp[5] = %T, want *ConstantModule", cf.ConstantPool[5])
	}
	if _, ok := cf.ConstantPool[6].(*classfile.ConstantPackage); !ok {
		t.Errorf("cp[6] = %T, want *ConstantPackage", cf.ConstantPool[6])
	}
}
