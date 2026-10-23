package ir

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"github.com/JohnEsleyer/barista/internal/classfile"
)

// JVM opcodes supported in Phase 1.
const (
	BytecodeNop      = 0x00
	BytecodeIconstM1 = 0x02
	BytecodeIconst0  = 0x03
	BytecodeIconst1  = 0x04
	BytecodeIconst2  = 0x05
	BytecodeIconst3  = 0x06
	BytecodeIconst4  = 0x07
	BytecodeIconst5  = 0x08
	BytecodeBipush   = 0x10
	BytecodeSipush   = 0x11
	BytecodeLdc      = 0x12
	BytecodeLdcW     = 0x13
	BytecodeLdc2W    = 0x14
	BytecodeIload    = 0x15
	BytecodeIload0   = 0x1a
	BytecodeIload1   = 0x1b
	BytecodeIload2   = 0x1c
	BytecodeIload3   = 0x1d
	BytecodeIstore   = 0x36
	BytecodeIstore0  = 0x3b
	BytecodeIstore1  = 0x3c
	BytecodeIstore2  = 0x3d
	BytecodeIstore3  = 0x3e
	BytecodeIadd     = 0x60
	BytecodeIsub     = 0x64
	BytecodeImul     = 0x68
	BytecodeIdiv     = 0x6c
	BytecodeIneg     = 0x74
	BytecodeIinc     = 0x84
	BytecodeIfeq     = 0x99
	BytecodeIfne     = 0x9a
	BytecodeIflt     = 0x9b
	BytecodeIfge     = 0x9c
	BytecodeIfgt     = 0x9d
	BytecodeIfle     = 0x9e
	BytecodeIfIcmpeq = 0x9f
	BytecodeIfIcmpne = 0xa0
	BytecodeIfIcmplt = 0xa1
	BytecodeIfIcmpge = 0xa2
	BytecodeIfIcmpgt = 0xa3
	BytecodeIfIcmple = 0xa4
	BytecodeGoto     = 0xa7
	BytecodeIreturn  = 0xac
	BytecodeReturn   = 0xb1
	BytecodeGotoW    = 0xc8
)

// LowerBytecode translates JVM bytecode into normalized IR without resolving
// constant-pool operands.
func LowerBytecode(methodName, descriptor string, code []byte, isStatic bool) (*MethodIR, error) {
	return LowerBytecodeWithPool(methodName, descriptor, code, isStatic, nil)
}

// LowerBytecodeWithPool translates JVM bytecode into normalized IR. The pool is
// required for ldc/ldc_w instructions and may be nil otherwise.
func LowerBytecodeWithPool(methodName, descriptor string, code []byte, isStatic bool, cp classfile.ConstantPool) (*MethodIR, error) {
	targets, err := scanBytecode(code)
	if err != nil {
		return nil, fmt.Errorf("invalid bytecode in %s: %w", methodName, err)
	}
	labels := make(map[int]int, len(targets))
	for i, target := range targets {
		labels[target] = i
	}

	mir := &MethodIR{
		Name:         methodName,
		Descriptor:   descriptor,
		Instructions: make([]Instruction, 0),
		ParamCount:   countDescriptorSlots(descriptor),
		IsStatic:     isStatic,
	}

	stack := make([]Value, 0)
	varID := 0

	nextVal := func() Value {
		v := Value{ID: varID, Type: "int32"}
		varID++
		return v
	}
	push := func(v Value) { stack = append(stack, v) }
	pop := func() (Value, error) {
		if len(stack) == 0 {
			return Value{}, fmt.Errorf("stack underflow in method %s", methodName)
		}
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return top, nil
	}
	emit := func(inst Instruction) { mir.Instructions = append(mir.Instructions, inst) }
	pushConst := func(value int32) {
		result := nextVal()
		emit(Instruction{Op: OpConstInt, Result: &result, Constant: value})
		push(result)
	}
	loadLocal := func(index int) {
		result := nextVal()
		emit(Instruction{Op: OpLoadVar, Result: &result, LocalVar: index})
		push(result)
	}
	storeLocal := func(index int) error {
		value, err := pop()
		if err != nil {
			return err
		}
		emit(Instruction{Op: OpStoreVar, Args: []Value{value}, LocalVar: index})
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
			return fmt.Errorf("branch to pc %d carries %d operand-stack values; phi nodes are not supported in Phase 1", target, len(stack))
		}
		return nil
	}

	pc := 0
	reachable := true
	for pc < len(code) {
		if label, ok := labels[pc]; ok {
			if len(stack) != 0 {
				return nil, fmt.Errorf("branch target pc %d is reached with %d operand-stack values; phi nodes are not supported in Phase 1", pc, len(stack))
			}
			emit(Instruction{Op: OpLabel, Label: label})
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
			pushConst(int32(op) - int32(BytecodeIconst0))

		case BytecodeBipush:
			value, err := readI1(code, pc)
			if err != nil {
				return nil, err
			}
			pc++
			pushConst(int32(value))

		case BytecodeSipush:
			value, err := readI2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			pushConst(int32(value))

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
			pushConst(value)

		case BytecodeLdc2W:
			return nil, fmt.Errorf("ldc2_w is not supported in Phase 1 (method %s)", methodName)

		case BytecodeIload, BytecodeIstore:
			index, err := readU1(code, pc)
			if err != nil {
				return nil, err
			}
			pc++
			if op == BytecodeIload {
				loadLocal(int(index))
			} else if err := storeLocal(int(index)); err != nil {
				return nil, err
			}

		case BytecodeIload0, BytecodeIload1, BytecodeIload2, BytecodeIload3:
			loadLocal(int(op - BytecodeIload0))

		case BytecodeIstore0, BytecodeIstore1, BytecodeIstore2, BytecodeIstore3:
			if err := storeLocal(int(op - BytecodeIstore0)); err != nil {
				return nil, err
			}

		case BytecodeIadd, BytecodeIsub, BytecodeImul, BytecodeIdiv:
			right, err := pop()
			if err != nil {
				return nil, err
			}
			left, err := pop()
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
			result := nextVal()
			emit(Instruction{Op: kind, Result: &result, Args: []Value{left, right}})
			push(result)

		case BytecodeIneg:
			value, err := pop()
			if err != nil {
				return nil, err
			}
			result := nextVal()
			emit(Instruction{Op: OpNeg, Result: &result, Args: []Value{value}})
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
			loadLocal(int(index))
			pushConst(int32(increment))
			right, _ := pop()
			left, _ := pop()
			result := nextVal()
			emit(Instruction{Op: OpAdd, Result: &result, Args: []Value{left, right}})
			push(result)
			if err := storeLocal(int(index)); err != nil {
				return nil, err
			}

		case BytecodeIfeq, BytecodeIfne, BytecodeIflt, BytecodeIfge, BytecodeIfgt, BytecodeIfle:
			value, err := pop()
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
			emit(Instruction{
				Op:        OpBranch,
				Args:      []Value{value},
				Condition: conditionForZeroBranch(op),
				Target:    label,
			})

		case BytecodeIfIcmpeq, BytecodeIfIcmpne, BytecodeIfIcmplt,
			BytecodeIfIcmpge, BytecodeIfIcmpgt, BytecodeIfIcmple:
			right, err := pop()
			if err != nil {
				return nil, err
			}
			left, err := pop()
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
			emit(Instruction{
				Op:        OpBranch,
				Args:      []Value{left, right},
				Condition: conditionForCompareBranch(op),
				Target:    label,
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
			emit(Instruction{Op: OpGoto, Target: label})
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
			emit(Instruction{Op: OpGoto, Target: label})
			stack = nil
			reachable = false

		case BytecodeIreturn:
			value, err := pop()
			if err != nil {
				return nil, err
			}
			emit(Instruction{Op: OpReturn, Args: []Value{value}})
			mir.HasReturnVal = true
			stack = nil
			reachable = false

		case BytecodeReturn:
			if len(stack) != 0 {
				return nil, fmt.Errorf("return with %d values on the operand stack", len(stack))
			}
			emit(Instruction{Op: OpReturn})
			stack = nil
			reachable = false

		default:
			return nil, fmt.Errorf("unsupported opcode 0x%02X at pc %d in %s", op, pc-1, methodName)
		}
	}

	return mir, nil
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

func countDescriptorSlots(desc string) int {
	if !strings.HasPrefix(desc, "(") {
		return 0
	}
	end := strings.Index(desc, ")")
	if end < 0 {
		return 0
	}
	count := 0
	for i := 1; i < end; {
		switch desc[i] {
		case 'J', 'D':
			count += 2
			i++
		case 'L':
			next := strings.IndexByte(desc[i:], ';')
			if next < 0 {
				return count
			}
			i += next + 1
			count++
		case '[':
			for i < end && desc[i] == '[' {
				i++
			}
			if i < end {
				if desc[i] == 'L' {
					next := strings.IndexByte(desc[i:], ';')
					if next < 0 {
						return count
					}
					i += next + 1
				} else {
					i++
				}
			}
			count++
		default:
			count++
			i++
		}
	}
	return count
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
	case BytecodeNop,
		BytecodeIconstM1, BytecodeIconst0, BytecodeIconst1, BytecodeIconst2,
		BytecodeIconst3, BytecodeIconst4, BytecodeIconst5,
		BytecodeIload0, BytecodeIload1, BytecodeIload2, BytecodeIload3,
		BytecodeIstore0, BytecodeIstore1, BytecodeIstore2, BytecodeIstore3,
		BytecodeIadd, BytecodeIsub, BytecodeImul, BytecodeIdiv, BytecodeIneg,
		BytecodeIreturn, BytecodeReturn:
		return decodedInstruction{size: 1}, nil
	case BytecodeBipush, BytecodeLdc, BytecodeIload, BytecodeIstore:
		if pc+2 > len(code) {
			return decodedInstruction{}, fmt.Errorf("truncated opcode 0x%02X at pc %d", op, pc)
		}
		return decodedInstruction{size: 2}, nil
	case BytecodeSipush, BytecodeLdcW, BytecodeLdc2W, BytecodeIinc:
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
