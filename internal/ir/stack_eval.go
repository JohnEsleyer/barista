package ir

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/JohnEsleyer/barista/internal/classfile"
)

// JVM opcodes supported by the Phase 2 lowering.
const (
	BytecodeNop           = 0x00
	BytecodeAconstNull    = 0x01
	BytecodeIconstM1      = 0x02
	BytecodeIconst0       = 0x03
	BytecodeIconst1       = 0x04
	BytecodeIconst2       = 0x05
	BytecodeIconst3       = 0x06
	BytecodeIconst4       = 0x07
	BytecodeIconst5       = 0x08
	BytecodeBipush        = 0x10
	BytecodeSipush        = 0x11
	BytecodeLdc           = 0x12
	BytecodeLdcW          = 0x13
	BytecodeLdc2W         = 0x14
	BytecodeIload         = 0x15
	BytecodeAload         = 0x19
	BytecodeIload0        = 0x1a
	BytecodeIload1        = 0x1b
	BytecodeIload2        = 0x1c
	BytecodeIload3        = 0x1d
	BytecodeAload0        = 0x2a
	BytecodeAload1        = 0x2b
	BytecodeAload2        = 0x2c
	BytecodeAload3        = 0x2d
	BytecodeIstore        = 0x36
	BytecodeAstore        = 0x3a
	BytecodeIstore0       = 0x3b
	BytecodeIstore1       = 0x3c
	BytecodeIstore2       = 0x3d
	BytecodeIstore3       = 0x3e
	BytecodeAstore0       = 0x4b
	BytecodeAstore1       = 0x4c
	BytecodeAstore2       = 0x4d
	BytecodeAstore3       = 0x4e
	BytecodeDup           = 0x59
	BytecodeIadd          = 0x60
	BytecodeIsub          = 0x64
	BytecodeImul          = 0x68
	BytecodeIdiv          = 0x6c
	BytecodeIneg          = 0x74
	BytecodeIinc          = 0x84
	BytecodeIfeq          = 0x99
	BytecodeIfne          = 0x9a
	BytecodeIflt          = 0x9b
	BytecodeIfge          = 0x9c
	BytecodeIfgt          = 0x9d
	BytecodeIfle          = 0x9e
	BytecodeIfIcmpeq      = 0x9f
	BytecodeIfIcmpne      = 0xa0
	BytecodeIfIcmplt      = 0xa1
	BytecodeIfIcmpge      = 0xa2
	BytecodeIfIcmpgt      = 0xa3
	BytecodeIfIcmple      = 0xa4
	BytecodeGoto          = 0xa7
	BytecodeIreturn       = 0xac
	BytecodeAreturn       = 0xb0
	BytecodeReturn        = 0xb1
	BytecodeGetstatic     = 0xb2
	BytecodePutstatic     = 0xb3
	BytecodeGetfield      = 0xb4
	BytecodePutfield      = 0xb5
	BytecodeInvokevirtual = 0xb6
	BytecodeInvokespecial = 0xb7
	BytecodeInvokestatic  = 0xb8
	BytecodeNew           = 0xbb
	BytecodeGotoW         = 0xc8
)

// LowerBytecode translates JVM bytecode into normalized IR without resolving
// constant-pool operands. It is kept as the simple Phase 1 entry point.
func LowerBytecode(methodName, descriptor string, code []byte, isStatic bool) (*MethodIR, error) {
	return LowerBytecodeWithPool(methodName, descriptor, code, isStatic, nil)
}

// LowerBytecodeWithPool translates JVM bytecode into normalized IR. The pool is
// required for operations with constant-pool operands.
func LowerBytecodeWithPool(methodName, descriptor string, code []byte, isStatic bool, cp classfile.ConstantPool) (*MethodIR, error) {
	return LowerMethod(cp, "", methodName, descriptor, code, isStatic)
}

// LowerMethod lowers one method owned by ownerClass. ownerClass is used to seed
// local 0 for instance methods and to disambiguate generated object types.
func LowerMethod(cp classfile.ConstantPool, ownerClass, methodName, descriptor string, code []byte, isStatic bool) (*MethodIR, error) {
	targets, err := scanBytecode(code)
	if err != nil {
		return nil, fmt.Errorf("invalid bytecode in %s: %w", methodName, err)
	}
	labels := make(map[int]int, len(targets))
	for i, target := range targets {
		labels[target] = i
	}

	md, err := ParseMethodDescriptor(descriptor)
	if err != nil {
		return nil, fmt.Errorf("method %s: %w", methodName, err)
	}

	mir := &MethodIR{
		OwnerClass:   ownerClass,
		Name:         methodName,
		Descriptor:   descriptor,
		Instructions: make([]Instruction, 0),
		ParamCount:   md.ParamSlots,
		IsStatic:     isStatic,
	}

	localTypes := make(map[int]string)
	startSlot := 0
	if !isStatic {
		startSlot = 1
		if ownerClass != "" {
			localTypes[0] = "L" + ownerClass + ";"
		}
	}
	slot := startSlot
	for _, param := range md.Params {
		localTypes[slot] = param
		slot += descriptorSlots(param)
	}

	stack := make([]Value, 0)
	varID := 0

	nextVal := func(typ string) Value {
		if typ == "" {
			typ = "I"
		}
		v := Value{ID: varID, Type: typ}
		varID++
		return v
	}
	push := func(v Value) { stack = append(stack, v) }
	pop := func(pc int) (Value, error) {
		if len(stack) == 0 {
			return Value{}, fmt.Errorf("stack underflow at pc %d in method %s", pc, methodName)
		}
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return top, nil
	}
	emit := func(pc int, inst Instruction) {
		inst.PC = pc
		mir.Instructions = append(mir.Instructions, inst)
	}
	pushConstAt := func(pc int, value int32) {
		result := nextVal("I")
		emit(pc, Instruction{Op: OpConstInt, Result: &result, Constant: value})
		push(result)
	}
	localType := func(index int) string {
		if typ, ok := localTypes[index]; ok && typ != "" {
			return typ
		}
		return "I"
	}
	loadLocalAt := func(pc, index int, typ string) {
		if typ == "" {
			typ = localType(index)
		}
		result := nextVal(typ)
		emit(pc, Instruction{Op: OpLoadVar, Result: &result, LocalVar: index})
		push(result)
	}
	storeLocalAt := func(pc, index int) error {
		value, err := pop(pc)
		if err != nil {
			return err
		}
		localTypes[index] = value.Type
		emit(pc, Instruction{Op: OpStoreVar, Args: []Value{value}, LocalVar: index})
		return nil
	}
	branchLabel := func(target int) (int, error) {
		label, ok := labels[target]
		if !ok {
			return 0, fmt.Errorf("branch target pc %d has no label", target)
		}
		return label, nil
	}
	requireEmptyStackAtBranch := func(target int) error {
		if len(stack) != 0 {
			return fmt.Errorf("branch to pc %d carries %d operand-stack values; phi nodes are not supported yet", target, len(stack))
		}
		return nil
	}

	pc := 0
	reachable := true
	for pc < len(code) {
		if label, ok := labels[pc]; ok {
			if len(stack) != 0 {
				return nil, fmt.Errorf("branch target pc %d is reached with %d operand-stack values; phi nodes are not supported yet", pc, len(stack))
			}
			emit(pc, Instruction{Op: OpLabel, Label: label})
			stack = nil
			reachable = true
		}
		if !reachable {
			decoded, err := decodeInstruction(code, pc)
			if err != nil {
				return nil, err
			}
			pc += decoded.size
			continue
		}

		op := code[pc]
		pc++
		opPC := pc - 1

		switch op {
		case BytecodeNop:

		case BytecodeIconstM1, BytecodeIconst0, BytecodeIconst1, BytecodeIconst2,
			BytecodeIconst3, BytecodeIconst4, BytecodeIconst5:
			pushConstAt(opPC, int32(op)-int32(BytecodeIconst0))

		case BytecodeBipush:
			value, err := readI1(code, pc)
			if err != nil {
				return nil, err
			}
			pc++
			pushConstAt(opPC, int32(value))

		case BytecodeSipush:
			value, err := readI2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			pushConstAt(opPC, int32(value))

		case BytecodeLdc, BytecodeLdcW:
			var index uint16
			if op == BytecodeLdc {
				idx, err := readU1(code, pc)
				if err != nil {
					return nil, err
				}
				index = uint16(idx)
				pc++
			} else {
				idx, err := readU2(code, pc)
				if err != nil {
					return nil, err
				}
				index = idx
				pc += 2
			}
			value, err := resolveIntConstant(cp, index)
			if err != nil {
				return nil, err
			}
			pushConstAt(opPC, value)

		case BytecodeLdc2W:
			return nil, fmt.Errorf("ldc2_w is not supported yet (method %s)", methodName)

		case BytecodeIload, BytecodeIstore, BytecodeAload, BytecodeAstore:
			index, err := readU1(code, pc)
			if err != nil {
				return nil, err
			}
			pc++
			switch op {
			case BytecodeIload:
				loadLocalAt(opPC, int(index), "I")
			case BytecodeAload:
				loadLocalAt(opPC, int(index), "")
			case BytecodeIstore:
				localTypes[int(index)] = "I"
				if err := storeLocalAt(opPC, int(index)); err != nil {
					return nil, err
				}
			case BytecodeAstore:
				if err := storeLocalAt(opPC, int(index)); err != nil {
					return nil, err
				}
			}

		case BytecodeIload0, BytecodeIload1, BytecodeIload2, BytecodeIload3:
			loadLocalAt(opPC, int(op-BytecodeIload0), "I")

		case BytecodeAload0, BytecodeAload1, BytecodeAload2, BytecodeAload3:
			loadLocalAt(opPC, int(op-BytecodeAload0), "")

		case BytecodeIstore0, BytecodeIstore1, BytecodeIstore2, BytecodeIstore3:
			index := int(op - BytecodeIstore0)
			localTypes[index] = "I"
			if err := storeLocalAt(opPC, index); err != nil {
				return nil, err
			}

		case BytecodeAstore0, BytecodeAstore1, BytecodeAstore2, BytecodeAstore3:
			if err := storeLocalAt(opPC, int(op-BytecodeAstore0)); err != nil {
				return nil, err
			}

		case BytecodeDup:
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			push(value)
			push(value)

		case BytecodeIadd, BytecodeIsub, BytecodeImul, BytecodeIdiv:
			right, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			left, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			var kind OpKind
			switch op {
			case BytecodeIadd:
				kind = OpAdd
			case BytecodeIsub:
				kind = OpSub
			case BytecodeImul:
				kind = OpMul
			case BytecodeIdiv:
				kind = OpDiv
			}
			result := nextVal("I")
			emit(opPC, Instruction{Op: kind, Result: &result, Args: []Value{left, right}})
			push(result)

		case BytecodeIneg:
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			result := nextVal("I")
			emit(opPC, Instruction{Op: OpNeg, Result: &result, Args: []Value{value}})
			push(result)

		case BytecodeIinc:
			index, err := readU1(code, pc)
			if err != nil {
				return nil, err
			}
			increment, err := readI1(code, pc+1)
			if err != nil {
				return nil, err
			}
			pc += 2
			loadLocalAt(opPC, int(index), "I")
			pushConstAt(opPC, int32(increment))
			right, _ := pop(opPC)
			left, _ := pop(opPC)
			result := nextVal("I")
			emit(opPC, Instruction{Op: OpAdd, Result: &result, Args: []Value{left, right}})
			push(result)
			if err := storeLocalAt(opPC, int(index)); err != nil {
				return nil, err
			}

		case BytecodeIfeq, BytecodeIfne, BytecodeIflt, BytecodeIfge, BytecodeIfgt, BytecodeIfle:
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			offset, err := readI2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			target := opPC + int(offset)
			if err := requireEmptyStackAtBranch(target); err != nil {
				return nil, err
			}
			label, err := branchLabel(target)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{
				Op:        OpBranch,
				Args:      []Value{value},
				Condition: conditionForZeroBranch(op),
				Target:    label,
				TargetPC:  target,
			})

		case BytecodeIfIcmpeq, BytecodeIfIcmpne, BytecodeIfIcmplt,
			BytecodeIfIcmpge, BytecodeIfIcmpgt, BytecodeIfIcmple:
			right, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			left, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			offset, err := readI2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			target := opPC + int(offset)
			if err := requireEmptyStackAtBranch(target); err != nil {
				return nil, err
			}
			label, err := branchLabel(target)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{
				Op:        OpBranch,
				Args:      []Value{left, right},
				Condition: conditionForCompareBranch(op),
				Target:    label,
				TargetPC:  target,
			})

		case BytecodeGoto:
			offset, err := readI2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			target := opPC + int(offset)
			if err := requireEmptyStackAtBranch(target); err != nil {
				return nil, err
			}
			label, err := branchLabel(target)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{Op: OpGoto, Target: label, TargetPC: target})
			stack = nil
			reachable = false

		case BytecodeGotoW:
			offset, err := readI4(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 4
			target := opPC + int(offset)
			if err := requireEmptyStackAtBranch(target); err != nil {
				return nil, err
			}
			label, err := branchLabel(target)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{Op: OpGoto, Target: label, TargetPC: target})
			stack = nil
			reachable = false

		case BytecodeNew:
			index, err := readU2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			className, err := cp.ResolveClassName(index)
			if err != nil {
				return nil, err
			}
			result := nextVal("L" + className + ";")
			emit(opPC, Instruction{Op: OpNew, Result: &result, OwnerClass: className})
			push(result)

		case BytecodeGetfield:
			index, err := readU2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			owner, name, desc, err := cp.ResolveFieldRef(index)
			if err != nil {
				return nil, err
			}
			object, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			result := nextVal(desc)
			emit(opPC, Instruction{
				Op:         OpGetField,
				Result:     &result,
				Args:       []Value{object},
				OwnerClass: owner,
				MemberName: name,
				Descriptor: desc,
			})
			push(result)

		case BytecodePutfield:
			index, err := readU2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			owner, name, desc, err := cp.ResolveFieldRef(index)
			if err != nil {
				return nil, err
			}
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			object, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{
				Op:         OpPutField,
				Args:       []Value{object, value},
				OwnerClass: owner,
				MemberName: name,
				Descriptor: desc,
			})

		case BytecodeGetstatic:
			index, err := readU2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			owner, name, desc, err := cp.ResolveFieldRef(index)
			if err != nil {
				return nil, err
			}
			result := nextVal(desc)
			emit(opPC, Instruction{
				Op:         OpGetStatic,
				Result:     &result,
				OwnerClass: owner,
				MemberName: name,
				Descriptor: desc,
			})
			push(result)

		case BytecodePutstatic:
			index, err := readU2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			owner, name, desc, err := cp.ResolveFieldRef(index)
			if err != nil {
				return nil, err
			}
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{
				Op:         OpPutStatic,
				Args:       []Value{value},
				OwnerClass: owner,
				MemberName: name,
				Descriptor: desc,
			})

		case BytecodeInvokevirtual, BytecodeInvokespecial, BytecodeInvokestatic:
			index, err := readU2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			owner, name, desc, err := cp.ResolveMethodRef(index)
			if err != nil {
				return nil, err
			}
			methodDesc, err := ParseMethodDescriptor(desc)
			if err != nil {
				return nil, fmt.Errorf("method %s: %w", name, err)
			}

			args := make([]Value, len(methodDesc.Params))
			for i := len(methodDesc.Params) - 1; i >= 0; i-- {
				arg, err := pop(opPC)
				if err != nil {
					return nil, err
				}
				args[i] = arg
			}

			var opKind OpKind
			switch op {
			case BytecodeInvokevirtual:
				opKind = OpInvokeVirtual
			case BytecodeInvokestatic:
				opKind = OpInvokeStatic
			default:
				opKind = OpInvokeSpecial
			}

			if opKind != OpInvokeStatic {
				receiver, err := pop(opPC)
				if err != nil {
					return nil, err
				}
				args = append([]Value{receiver}, args...)
			}

			var result *Value
			if methodDesc.Return != "V" {
				value := nextVal(methodDesc.Return)
				result = &value
			}
			emit(opPC, Instruction{
				Op:         opKind,
				Result:     result,
				Args:       args,
				OwnerClass: owner,
				MemberName: name,
				Descriptor: desc,
			})
			if result != nil {
				push(*result)
			}

		case BytecodeIreturn, BytecodeAreturn:
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{Op: OpReturn, Args: []Value{value}})
			mir.HasReturnVal = true
			stack = nil
			reachable = false

		case BytecodeReturn:
			if len(stack) != 0 {
				return nil, fmt.Errorf("return with %d values on the operand stack", len(stack))
			}
			emit(opPC, Instruction{Op: OpReturn})
			stack = nil
			reachable = false

		case BytecodeAconstNull:
			return nil, fmt.Errorf("aconst_null is not supported yet (method %s)", methodName)

		default:
			return nil, fmt.Errorf("unsupported opcode 0x%02X at pc %d in %s", op, opPC, methodName)
		}
	}

	cfg, err := buildCFG(code, mir.Instructions)
	if err != nil {
		return nil, fmt.Errorf("method %s: %w", methodName, err)
	}
	mir.CFG = cfg

	return mir, nil
}

func descriptorSlots(desc string) int {
	if desc == "J" || desc == "D" {
		return 2
	}
	return 1
}

func conditionForZeroBranch(op byte) BranchCondition {
	switch op {
	case BytecodeIfeq:
		return CondEQ
	case BytecodeIfne:
		return CondNE
	case BytecodeIflt:
		return CondLT
	case BytecodeIfge:
		return CondGE
	case BytecodeIfgt:
		return CondGT
	default:
		return CondLE
	}
}

func conditionForCompareBranch(op byte) BranchCondition {
	switch op {
	case BytecodeIfIcmpeq:
		return CondEQ
	case BytecodeIfIcmpne:
		return CondNE
	case BytecodeIfIcmplt:
		return CondLT
	case BytecodeIfIcmpge:
		return CondGE
	case BytecodeIfIcmpgt:
		return CondGT
	default:
		return CondLE
	}
}

func resolveIntConstant(cp classfile.ConstantPool, index uint16) (int32, error) {
	if cp == nil {
		return 0, fmt.Errorf("ldc requires a constant pool")
	}
	if index == 0 || int(index) >= len(cp) || cp[index] == nil {
		return 0, fmt.Errorf("invalid constant pool index %d for ldc", index)
	}
	switch entry := cp[index].(type) {
	case *classfile.ConstantInteger:
		return entry.Value, nil
	default:
		return 0, fmt.Errorf("ldc constant at index %d has unsupported type %T", index, entry)
	}
}

// buildCFG partitions a method's IR into basic blocks and connects the blocks
// with directed edges. Branch targets are recorded by the lowering step.
func buildCFG(code []byte, instructions []Instruction) (*ControlFlowGraph, error) {
	leaders, err := collectLeaders(code)
	if err != nil {
		return nil, err
	}
	if len(leaders) == 0 {
		return nil, nil
	}

	cfg := &ControlFlowGraph{Blocks: make([]*BasicBlock, 0, len(leaders))}
	for i, start := range leaders {
		end := len(code)
		if i+1 < len(leaders) {
			end = leaders[i+1]
		}
		cfg.Blocks = append(cfg.Blocks, &BasicBlock{
			ID:      i,
			StartPC: start,
			EndPC:   end,
		})
	}
	cfg.Entry = cfg.Blocks[0]

	for _, inst := range instructions {
		block := cfg.FindBlockByPC(inst.PC)
		if block == nil {
			continue
		}
		block.Instructions = append(block.Instructions, inst)
	}

	for _, block := range cfg.Blocks {
		var last *Instruction
		for i := len(block.Instructions) - 1; i >= 0; i-- {
			if block.Instructions[i].Op != OpLabel {
				last = &block.Instructions[i]
				break
			}
		}
		if last == nil {
			addFallthroughEdge(cfg, block)
			continue
		}

		switch last.Op {
		case OpGoto:
			target := cfg.FindBlockByPC(last.TargetPC)
			if target == nil {
				return nil, fmt.Errorf("goto target pc %d has no basic block", last.TargetPC)
			}
			addEdge(block, target)
		case OpBranch:
			target := cfg.FindBlockByPC(last.TargetPC)
			if target == nil {
				return nil, fmt.Errorf("branch target pc %d has no basic block", last.TargetPC)
			}
			addEdge(block, target)
			addFallthroughEdge(cfg, block)
		case OpReturn:
			// No successors.
		default:
			addFallthroughEdge(cfg, block)
		}
	}

	return cfg, nil
}

func collectLeaders(code []byte) ([]int, error) {
	leaderSet := map[int]bool{0: true}
	for pc := 0; pc < len(code); {
		decoded, err := decodeInstruction(code, pc)
		if err != nil {
			return nil, err
		}
		if decoded.hasTarget && decoded.target >= 0 && decoded.target < len(code) {
			leaderSet[decoded.target] = true
		}
		if isControlTransfer(code[pc]) {
			next := pc + decoded.size
			if next < len(code) {
				leaderSet[next] = true
			}
		}
		pc += decoded.size
	}

	leaders := make([]int, 0, len(leaderSet))
	for leader := range leaderSet {
		if leader >= 0 && leader < len(code) {
			leaders = append(leaders, leader)
		}
	}
	sort.Ints(leaders)
	return leaders, nil
}

func isControlTransfer(op byte) bool {
	switch op {
	case BytecodeIfeq, BytecodeIfne, BytecodeIflt, BytecodeIfge, BytecodeIfgt, BytecodeIfle,
		BytecodeIfIcmpeq, BytecodeIfIcmpne, BytecodeIfIcmplt, BytecodeIfIcmpge,
		BytecodeIfIcmpgt, BytecodeIfIcmple, BytecodeGoto, BytecodeGotoW,
		BytecodeIreturn, BytecodeAreturn, BytecodeReturn:
		return true
	default:
		return false
	}
}

func addFallthroughEdge(cfg *ControlFlowGraph, block *BasicBlock) {
	if block.ID+1 < len(cfg.Blocks) {
		addEdge(block, cfg.Blocks[block.ID+1])
	}
}

func addEdge(from, to *BasicBlock) {
	for _, existing := range from.Successors {
		if existing == to {
			return
		}
	}
	from.Successors = append(from.Successors, to)
	to.Predecessors = append(to.Predecessors, from)
}

type decodedInstruction struct {
	size      int
	target    int
	hasTarget bool
}

func scanBytecode(code []byte) ([]int, error) {
	boundaries := make(map[int]bool)
	targetSet := make(map[int]bool)
	for pc := 0; pc < len(code); {
		boundaries[pc] = true
		decoded, err := decodeInstruction(code, pc)
		if err != nil {
			return nil, err
		}
		if decoded.hasTarget {
			targetSet[decoded.target] = true
		}
		pc += decoded.size
	}
	targets := make([]int, 0, len(targetSet))
	for target := range targetSet {
		targets = append(targets, target)
	}
	sort.Ints(targets)
	for _, target := range targets {
		if target < 0 || target >= len(code) || !boundaries[target] {
			return nil, fmt.Errorf("branch target pc %d is not an instruction boundary", target)
		}
	}
	return targets, nil
}

func decodeInstruction(code []byte, pc int) (decodedInstruction, error) {
	if pc < 0 || pc >= len(code) {
		return decodedInstruction{}, fmt.Errorf("pc %d is outside bytecode", pc)
	}
	op := code[pc]
	switch op {
	case BytecodeNop, BytecodeAconstNull,
		BytecodeIconstM1, BytecodeIconst0, BytecodeIconst1, BytecodeIconst2,
		BytecodeIconst3, BytecodeIconst4, BytecodeIconst5,
		BytecodeIload0, BytecodeIload1, BytecodeIload2, BytecodeIload3,
		BytecodeAload0, BytecodeAload1, BytecodeAload2, BytecodeAload3,
		BytecodeIstore0, BytecodeIstore1, BytecodeIstore2, BytecodeIstore3,
		BytecodeAstore0, BytecodeAstore1, BytecodeAstore2, BytecodeAstore3,
		BytecodeDup,
		BytecodeIadd, BytecodeIsub, BytecodeImul, BytecodeIdiv, BytecodeIneg,
		BytecodeIreturn, BytecodeAreturn, BytecodeReturn:
		return decodedInstruction{size: 1}, nil
	case BytecodeBipush, BytecodeLdc, BytecodeIload, BytecodeAload, BytecodeIstore, BytecodeAstore:
		if pc+2 > len(code) {
			return decodedInstruction{}, fmt.Errorf("truncated opcode 0x%02X at pc %d", op, pc)
		}
		return decodedInstruction{size: 2}, nil
	case BytecodeSipush, BytecodeLdcW, BytecodeLdc2W, BytecodeIinc,
		BytecodeNew, BytecodeGetfield, BytecodePutfield, BytecodeGetstatic, BytecodePutstatic,
		BytecodeInvokevirtual, BytecodeInvokespecial, BytecodeInvokestatic:
		if pc+3 > len(code) {
			return decodedInstruction{}, fmt.Errorf("truncated opcode 0x%02X at pc %d", op, pc)
		}
		return decodedInstruction{size: 3}, nil
	case BytecodeIfeq, BytecodeIfne, BytecodeIflt, BytecodeIfge, BytecodeIfgt, BytecodeIfle,
		BytecodeIfIcmpeq, BytecodeIfIcmpne, BytecodeIfIcmplt, BytecodeIfIcmpge,
		BytecodeIfIcmpgt, BytecodeIfIcmple, BytecodeGoto:
		if pc+3 > len(code) {
			return decodedInstruction{}, fmt.Errorf("truncated branch opcode 0x%02X at pc %d", op, pc)
		}
		offset := int(int16(binary.BigEndian.Uint16(code[pc+1 : pc+3])))
		return decodedInstruction{size: 3, target: pc + offset, hasTarget: true}, nil
	case BytecodeGotoW:
		if pc+5 > len(code) {
			return decodedInstruction{}, fmt.Errorf("truncated goto_w at pc %d", pc)
		}
		offset := int(int32(binary.BigEndian.Uint32(code[pc+1 : pc+5])))
		return decodedInstruction{size: 5, target: pc + offset, hasTarget: true}, nil
	default:
		return decodedInstruction{}, fmt.Errorf("unsupported opcode 0x%02X at pc %d", op, pc)
	}
}

func readU1(code []byte, pc int) (uint8, error) {
	if pc < 0 || pc+1 > len(code) {
		return 0, fmt.Errorf("truncated bytecode at pc %d: %w", pc, classfile.ErrUnexpectedEOF)
	}
	return code[pc], nil
}

func readI1(code []byte, pc int) (int8, error) {
	v, err := readU1(code, pc)
	return int8(v), err
}

func readU2(code []byte, pc int) (uint16, error) {
	if pc < 0 || pc+2 > len(code) {
		return 0, fmt.Errorf("truncated bytecode at pc %d: %w", pc, classfile.ErrUnexpectedEOF)
	}
	return binary.BigEndian.Uint16(code[pc : pc+2]), nil
}

func readI2(code []byte, pc int) (int16, error) {
	v, err := readU2(code, pc)
	return int16(v), err
}

func readI4(code []byte, pc int) (int32, error) {
	if pc < 0 || pc+4 > len(code) {
		return 0, fmt.Errorf("truncated bytecode at pc %d: %w", pc, classfile.ErrUnexpectedEOF)
	}
	return int32(binary.BigEndian.Uint32(code[pc : pc+4])), nil
}
