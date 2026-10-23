package ir_test

import (
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

func TestLowerBytecodeStackMergePhi(t *testing.T) {
	// 0: iconst_0
	// 1: iconst_1
	// 2: ifeq 5
	// 5: ireturn
	code := []byte{
		ir.BytecodeIconst0,
		ir.BytecodeIconst1,
		ir.BytecodeIfeq, 0x00, 0x03,
		ir.BytecodeIreturn,
	}
	mir, err := ir.LowerBytecode("ternaryMerge", "()I", code, true)
	if err != nil {
		t.Fatalf("expected successful lowering with phi nodes, got error: %v", err)
	}
	var sawBranch, sawAssign bool
	for _, inst := range mir.Instructions {
		if inst.Op == ir.OpBranch && len(inst.PhiAssignments) > 0 {
			sawBranch = true
		}
		if inst.Op == ir.OpAssign {
			sawAssign = true
		}
	}
	if !sawBranch || !sawAssign {
		t.Errorf("expected branch with phi assignments and fallthrough assign: %+v", mir.Instructions)
	}
}

func TestLowerBytecodeTableSwitch(t *testing.T) {
	code := []byte{
		ir.BytecodeIload0,      // 0
		ir.BytecodeTableswitch, // 1
		0x00, 0x00,             // alignment padding
		0x00, 0x00, 0x00, 0x1b, // default target pc 28
		0x00, 0x00, 0x00, 0x00, // low 0
		0x00, 0x00, 0x00, 0x01, // high 1
		0x00, 0x00, 0x00, 0x17, // case 0 -> pc 24
		0x00, 0x00, 0x00, 0x19, // case 1 -> pc 26
		ir.BytecodeIconst0, ir.BytecodeIreturn, // 24, 25
		ir.BytecodeIconst1, ir.BytecodeIreturn, // 26, 27
		ir.BytecodeIconstM1, ir.BytecodeIreturn, // 28, 29
	}
	mir, err := ir.LowerBytecode("choose", "(I)I", code, true)
	if err != nil {
		t.Fatalf("lower bytecode: %v", err)
	}

	var sw *ir.Instruction
	for i := range mir.Instructions {
		if mir.Instructions[i].Op == ir.OpTableSwitch {
			sw = &mir.Instructions[i]
			break
		}
	}
	if sw == nil {
		t.Fatalf("expected a tableswitch instruction, got %+v", mir.Instructions)
	}
	if sw.DefaultPC != 28 {
		t.Errorf("default pc = %d, want 28", sw.DefaultPC)
	}
	if len(sw.Cases) != 2 {
		t.Fatalf("got %d switch cases, want 2", len(sw.Cases))
	}
	if sw.Cases[0].Match != 0 || sw.Cases[0].TargetPC != 24 {
		t.Errorf("case 0 = %+v, want match 0 target 24", sw.Cases[0])
	}
	if sw.Cases[1].Match != 1 || sw.Cases[1].TargetPC != 26 {
		t.Errorf("case 1 = %+v, want match 1 target 26", sw.Cases[1])
	}
	if mir.CFG == nil || mir.CFG.Entry == nil {
		t.Fatal("expected a CFG entry block")
	}
	if got := len(mir.CFG.Entry.Successors); got != 3 {
		t.Errorf("switch entry has %d successors, want 3", got)
	}
}

func TestLowerBytecodeLongArithmetic(t *testing.T) {
	code := []byte{
		ir.BytecodeLconst0,
		ir.BytecodeLconst1,
		ir.BytecodeLadd,
		ir.BytecodeL2I,
		ir.BytecodeIreturn,
	}
	mir, err := ir.LowerBytecode("longAdd", "()I", code, true)
	if err != nil {
		t.Fatalf("lower bytecode: %v", err)
	}
	add := mir.Instructions[2]
	if add.Op != ir.OpAdd || add.Result == nil || add.Result.Type != "J" {
		t.Fatalf("long add result = %+v, want type J", add.Result)
	}
	convert := mir.Instructions[3]
	if convert.Op != ir.OpL2I || convert.Result == nil || convert.Result.Type != "I" {
		t.Fatalf("l2i = %+v, want I result", convert)
	}
}

func TestLowerBytecodeSwap(t *testing.T) {
	code := []byte{
		ir.BytecodeIconst1,
		ir.BytecodeIconst2,
		ir.BytecodeSwap,
		ir.BytecodeIsub,
		ir.BytecodeIreturn,
	}
	mir, err := ir.LowerBytecode("swapped", "()I", code, true)
	if err != nil {
		t.Fatalf("lower bytecode: %v", err)
	}
	sub := mir.Instructions[2]
	if sub.Op != ir.OpSub {
		t.Fatalf("instruction 2 = %s, want SUB", sub.Op)
	}
	if sub.Args[0].String() != mir.Instructions[1].Result.String() ||
		sub.Args[1].String() != mir.Instructions[0].Result.String() {
		t.Fatalf("swap operands = %v, want constant-2 - constant-1", sub.Args)
	}
}
