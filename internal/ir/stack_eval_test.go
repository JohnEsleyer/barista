package ir_test

import (
	"strings"
	"testing"

	"github.com/JohnEsleyer/barista/internal/classfile"
	"github.com/JohnEsleyer/barista/internal/ir"
)

func TestLowerBytecodeArithmetic(t *testing.T) {
	code := []byte{ir.BytecodeIload0, ir.BytecodeIload1, ir.BytecodeIadd, ir.BytecodeIreturn}
	mir, err := ir.LowerBytecode("add", "(II)I", code, true)
	if err != nil {
		t.Fatalf("lower bytecode: %v", err)
	}
	if !mir.HasReturnVal || !mir.IsStatic {
		t.Fatalf("unexpected method flags: %+v", mir)
	}
	if len(mir.Instructions) != 4 {
		t.Fatalf("got %d instructions, want 4", len(mir.Instructions))
	}
	wantOps := []ir.OpKind{ir.OpLoadVar, ir.OpLoadVar, ir.OpAdd, ir.OpReturn}
	for i, want := range wantOps {
		if mir.Instructions[i].Op != want {
			t.Errorf("instruction %d = %s, want %s", i, mir.Instructions[i].Op, want)
		}
	}
}

func TestLowerBytecodeConditional(t *testing.T) {
	// if (value < 0) return -value; return value;
	code := []byte{
		ir.BytecodeIload0,
		ir.BytecodeIfge, 0x00, 0x06,
		ir.BytecodeIload0,
		ir.BytecodeIneg,
		ir.BytecodeIreturn,
		ir.BytecodeIload0,
		ir.BytecodeIreturn,
	}
	mir, err := ir.LowerBytecode("absolute", "(I)I", code, true)
	if err != nil {
		t.Fatalf("lower bytecode: %v", err)
	}

	var sawLabel, sawBranch bool
	for _, inst := range mir.Instructions {
		switch inst.Op {
		case ir.OpLabel:
			sawLabel = true
		case ir.OpBranch:
			sawBranch = true
			if inst.Condition != ir.CondGE {
				t.Errorf("branch condition = %s, want >=", inst.Condition)
			}
		}
	}
	if !sawLabel || !sawBranch {
		t.Fatalf("expected label and branch instructions, got %+v", mir.Instructions)
	}

	if mir.CFG == nil {
		t.Fatal("expected a control flow graph")
	}
	if mir.CFG.Entry == nil || len(mir.CFG.Blocks) < 2 {
		t.Fatalf("unexpected CFG: %+v", mir.CFG)
	}
	if block := mir.CFG.FindBlockByPC(5); block == nil {
		t.Fatal("FindBlockByPC(5) returned nil")
	}
	if len(mir.CFG.Entry.Successors) != 2 {
		t.Fatalf("entry has %d successors, want 2 (branch and fallthrough)", len(mir.CFG.Entry.Successors))
	}
	for _, block := range mir.CFG.Blocks {
		for _, successor := range block.Successors {
			found := false
			for _, predecessor := range successor.Predecessors {
				if predecessor == block {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s -> %s but reverse edge is missing", block.Label(), successor.Label())
			}
		}
	}
}

func TestParseMethodDescriptor(t *testing.T) {
	md, err := ir.ParseMethodDescriptor("(IJLjava/lang/String;[I)LCounter;")
	if err != nil {
		t.Fatalf("parse descriptor: %v", err)
	}
	wantParams := []string{"I", "J", "Ljava/lang/String;", "[I"}
	if len(md.Params) != len(wantParams) {
		t.Fatalf("got %d params, want %d", len(md.Params), len(wantParams))
	}
	for i, want := range wantParams {
		if md.Params[i] != want {
			t.Errorf("param %d = %q, want %q", i, md.Params[i], want)
		}
	}
	if md.ParamSlots != 5 {
		t.Errorf("ParamSlots = %d, want 5", md.ParamSlots)
	}
	if md.Return != "LCounter;" {
		t.Errorf("Return = %q, want LCounter;", md.Return)
	}
}

func TestLowerBytecodeLdc(t *testing.T) {
	cp := classfile.ConstantPool{
		nil,
		&classfile.ConstantInteger{Value: 42},
	}
	mir, err := ir.LowerBytecodeWithPool("answer", "()I", []byte{ir.BytecodeLdc, 0x01, ir.BytecodeIreturn}, true, cp)
	if err != nil {
		t.Fatalf("lower bytecode: %v", err)
	}
	if len(mir.Instructions) != 2 || mir.Instructions[0].Constant != 42 {
		t.Fatalf("unexpected instructions: %+v", mir.Instructions)
	}
}

func TestLowerBytecodeLdcStringAndClass(t *testing.T) {
	cp := classfile.ConstantPool{
		nil,
		&classfile.ConstantUtf8{Value: "Freshly brewed"},
		&classfile.ConstantString{StringIndex: 1},
		&classfile.ConstantClass{NameIndex: 1},
	}
	code := []byte{
		ir.BytecodeLdc, 0x02,
		ir.BytecodePop,
		ir.BytecodeLdc, 0x03,
		ir.BytecodePop,
		ir.BytecodeReturn,
	}
	mir, err := ir.LowerBytecodeWithPool("constants", "()V", code, true, cp)
	if err != nil {
		t.Fatalf("lower bytecode: %v", err)
	}
	if len(mir.Instructions) != 3 {
		t.Fatalf("got %d instructions, want 3: %+v", len(mir.Instructions), mir.Instructions)
	}
	if mir.Instructions[0].Op != ir.OpConstString || mir.Instructions[0].Descriptor != "Freshly brewed" {
		t.Errorf("first instruction = %+v, want CONST_STRING", mir.Instructions[0])
	}
	if mir.Instructions[1].Op != ir.OpConstClass || mir.Instructions[1].Descriptor != "Freshly brewed" {
		t.Errorf("second instruction = %+v, want CONST_CLASS", mir.Instructions[1])
	}
}

func TestLowerBytecodeArrays(t *testing.T) {
	code := []byte{
		ir.BytecodeIconst2,
		ir.BytecodeNewarray, 10,
		ir.BytecodeAstore0,
		ir.BytecodeAload0,
		ir.BytecodeIconst0,
		ir.BytecodeBipush, 42,
		ir.BytecodeIastore,
		ir.BytecodeAload0,
		ir.BytecodeIconst0,
		ir.BytecodeIaload,
		ir.BytecodeIreturn,
	}
	mir, err := ir.LowerBytecode("first", "()I", code, true)
	if err != nil {
		t.Fatalf("lower bytecode: %v", err)
	}
	for _, op := range []ir.OpKind{ir.OpNewArray, ir.OpArrayStore, ir.OpArrayLoad, ir.OpReturn} {
		found := false
		for _, inst := range mir.Instructions {
			if inst.Op == op {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("instructions do not contain %s: %+v", op, mir.Instructions)
		}
	}
}

func TestLowerBytecodeAconstNull(t *testing.T) {
	mir, err := ir.LowerBytecode("nullValue", "()V", []byte{ir.BytecodeAconstNull, ir.BytecodePop, ir.BytecodeReturn}, true)
	if err != nil {
		t.Fatalf("lower aconst_null: %v", err)
	}
	if len(mir.Instructions) != 2 || mir.Instructions[0].Op != ir.OpConstNull {
		t.Fatalf("unexpected instructions: %+v", mir.Instructions)
	}
}

func TestLowerBytecodeRejectsValuesAcrossBranches(t *testing.T) {
	// The branch leaves v0 on the operand stack at the target.
	code := []byte{
		ir.BytecodeIconst0,
		ir.BytecodeIconst1,
		ir.BytecodeIfeq, 0x00, 0x03,
		ir.BytecodeIreturn,
	}
	_, err := ir.LowerBytecode("bad", "()I", code, true)
	if err == nil || !strings.Contains(err.Error(), "phi nodes") {
		t.Fatalf("expected phi-node error, got %v", err)
	}
}

func TestLowerBytecodeRejectsUnsupportedOpcode(t *testing.T) {
	// 0xC0 is checkcast, which Phase 3 does not lower yet.
	_, err := ir.LowerBytecode("bad", "()V", []byte{0xC0, 0x00, 0x01, ir.BytecodeReturn}, true)
	if err == nil {
		t.Fatal("expected an error for checkcast")
	}
}
