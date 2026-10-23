package ir

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"github.com/JohnEsleyer/barista/internal/classfile"
)

// JVM opcodes supported by the lowering engine.
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
	BytecodeLconst0       = 0x09
	BytecodeLconst1       = 0x0a
	BytecodeBipush        = 0x10
	BytecodeSipush        = 0x11
	BytecodeLdc           = 0x12
	BytecodeLdcW          = 0x13
	BytecodeLdc2W         = 0x14
	BytecodeIload         = 0x15
	BytecodeLload         = 0x16
	BytecodeAload         = 0x19
	BytecodeIload0        = 0x1a
	BytecodeIload1        = 0x1b
	BytecodeIload2        = 0x1c
	BytecodeIload3        = 0x1d
	BytecodeLload0        = 0x1e
	BytecodeLload1        = 0x1f
	BytecodeLload2        = 0x20
	BytecodeLload3        = 0x21
	BytecodeAload0        = 0x2a
	BytecodeAload1        = 0x2b
	BytecodeAload2        = 0x2c
	BytecodeAload3        = 0x2d
	BytecodeIaload        = 0x2e
	BytecodeLaload        = 0x2f
	BytecodeAaload        = 0x32
	BytecodeIstore        = 0x36
	BytecodeLstore        = 0x37
	BytecodeAstore        = 0x3a
	BytecodeIstore0       = 0x3b
	BytecodeIstore1       = 0x3c
	BytecodeIstore2       = 0x3d
	BytecodeIstore3       = 0x3e
	BytecodeLstore0       = 0x3f
	BytecodeLstore1       = 0x40
	BytecodeLstore2       = 0x41
	BytecodeLstore3       = 0x42
	BytecodeAstore0       = 0x4b
	BytecodeAstore1       = 0x4c
	BytecodeAstore2       = 0x4d
	BytecodeAstore3       = 0x4e
	BytecodeIastore       = 0x4f
	BytecodeLastore       = 0x50
	BytecodeAastore       = 0x53
	BytecodePop           = 0x57
	BytecodeDup           = 0x59
	BytecodeSwap          = 0x5f
	BytecodeIadd          = 0x60
	BytecodeLadd          = 0x61
	BytecodeIsub          = 0x64
	BytecodeLsub          = 0x65
	BytecodeImul          = 0x68
	BytecodeLmul          = 0x69
	BytecodeIdiv          = 0x6c
	BytecodeLdiv          = 0x6d
	BytecodeIneg          = 0x74
	BytecodeLneg          = 0x75
	BytecodeIand          = 0x7e
	BytecodeI2L           = 0x85
	BytecodeL2I           = 0x88
	BytecodeIinc          = 0x84
	BytecodeLcmp          = 0x94
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
	BytecodeIfAcmpeq      = 0xa5
	BytecodeIfAcmpne      = 0xa6
	BytecodeGoto          = 0xa7
	BytecodeTableswitch   = 0xaa
	BytecodeLookupswitch  = 0xab
	BytecodeIreturn       = 0xac
	BytecodeLreturn       = 0xad
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
	BytecodeNewarray      = 0xbc
	BytecodeAnewarray     = 0xbd
	BytecodeArraylength   = 0xbe
	BytecodeAthrow        = 0xbf
	BytecodeCheckcast     = 0xc0
	BytecodeInstanceof    = 0xc1
	BytecodeIfnull        = 0xc6
	BytecodeIfnonnull     = 0xc7
	BytecodeGotoW         = 0xc8
	BytecodeInvokeinterface = 0xb9
)

// LowerBytecode translates JVM bytecode into normalized IR without resolving
// constant-pool operands.
func LowerBytecode(methodName, descriptor string, code []byte, isStatic bool) (*MethodIR, error) {
	return LowerBytecodeWithPool(methodName, descriptor, code, isStatic, nil)
}

// LowerBytecodeWithPool translates JVM bytecode into normalized IR.
func LowerBytecodeWithPool(methodName, descriptor string, code []byte, isStatic bool, cp classfile.ConstantPool) (*MethodIR, error) {
	return LowerMethod(cp, "", methodName, descriptor, code, isStatic)
}

// LowerMethod lowers one method owned by ownerClass.
func LowerMethod(cp classfile.ConstantPool, ownerClass, methodName, descriptor string, code []byte, isStatic bool) (*MethodIR, error) {
	return LowerMethodWithExceptions(cp, ownerClass, methodName, descriptor, code, isStatic, nil)
}

// LowerMethodWithExceptions lowers a method and carries its JVM exception table
// into the IR. Catch types are resolved against cp.
func LowerMethodWithExceptions(cp classfile.ConstantPool, ownerClass, methodName, descriptor string, code []byte, isStatic bool, excTable []classfile.ExceptionTableEntry) (*MethodIR, error) {
	traps := make([]ExceptionTrap, 0, len(excTable))
	handlerPCs := make(map[int]bool)
	extraTargets := make([]int, 0, len(excTable))
	for _, entry := range excTable {
		catchClass := "java/lang/Throwable"
		if entry.CatchType != 0 {
			resolved, err := cp.ResolveClassName(entry.CatchType)
			if err != nil {
				return nil, fmt.Errorf("method %s: resolve catch type: %w", methodName, err)
			}
			catchClass = resolved
		}
		traps = append(traps, ExceptionTrap{
			StartPC:    int(entry.StartPC),
			EndPC:      int(entry.EndPC),
			HandlerPC:  int(entry.HandlerPC),
			CatchClass: catchClass,
		})
		handlerPCs[int(entry.HandlerPC)] = true
		extraTargets = append(extraTargets, int(entry.HandlerPC))
	}

	targets, err := scanBytecodeWithTargets(code, extraTargets)
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
		OwnerClass:     ownerClass,
		Name:           methodName,
		Descriptor:     descriptor,
		Instructions:   make([]Instruction, 0),
		ExceptionTraps: traps,
		ParamCount:     md.ParamSlots,
		IsStatic:       isStatic,
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
	pushLongConstAt := func(pc int, value int64) {
		result := nextVal("J")
		emit(pc, Instruction{Op: OpConstLong, Result: &result, Constant64: value})
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

	targetPhi := make(map[int][]Value)
	getOrCreateTargetPhi := func(targetPC int, currentStack []Value) ([]PhiAssignment, error) {
		if len(currentStack) == 0 {
			if existing, ok := targetPhi[targetPC]; ok && len(existing) != 0 {
				return nil, fmt.Errorf("branch to pc %d carries 0 stack values, but target expects %d", targetPC, len(existing))
			}
			return nil, nil
		}
		existing, ok := targetPhi[targetPC]
		if ok {
			if len(existing) != len(currentStack) {
				return nil, fmt.Errorf("branch to pc %d carries %d stack values, but target expects %d", targetPC, len(currentStack), len(existing))
			}
			assigns := make([]PhiAssignment, len(currentStack))
			for i := range currentStack {
				assigns[i] = PhiAssignment{Dest: existing[i], Src: currentStack[i]}
			}
			return assigns, nil
		}
		phis := make([]Value, len(currentStack))
		assigns := make([]PhiAssignment, len(currentStack))
		for i := range currentStack {
			phis[i] = nextVal(currentStack[i].Type)
			assigns[i] = PhiAssignment{Dest: phis[i], Src: currentStack[i]}
		}
		targetPhi[targetPC] = phis
		return assigns, nil
	}

	pc := 0
	reachable := true
	for pc < len(code) {
		if label, ok := labels[pc]; ok {
			if handlerPCs[pc] {
				emit(pc, Instruction{Op: OpLabel, Label: label})
				stack = nil
				reachable = true
				caught := nextVal("Ljava/lang/Throwable;")
				emit(pc, Instruction{Op: OpLoadException, Result: &caught})
				push(caught)
			} else {
				if reachable {
					phis, ok := targetPhi[pc]
					if ok && len(phis) > 0 {
						if len(stack) != len(phis) {
							return nil, fmt.Errorf("branch target pc %d is reached with %d stack values, expected %d", pc, len(stack), len(phis))
						}
						for i := range phis {
							dest := phis[i]
							emit(pc, Instruction{Op: OpAssign, Result: &dest, Args: []Value{stack[i]}})
						}
						stack = append([]Value(nil), phis...)
					} else if len(stack) > 0 {
						phis := make([]Value, len(stack))
						for i := range stack {
							phis[i] = nextVal(stack[i].Type)
							dest := phis[i]
							emit(pc, Instruction{Op: OpAssign, Result: &dest, Args: []Value{stack[i]}})
						}
						targetPhi[pc] = phis
						stack = append([]Value(nil), phis...)
					} else {
						stack = nil
					}
				} else {
					if phis, ok := targetPhi[pc]; ok && len(phis) > 0 {
						stack = append([]Value(nil), phis...)
					} else {
						stack = nil
					}
					reachable = true
				}
				emit(pc, Instruction{Op: OpLabel, Label: label})
			}
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

		case BytecodeLconst0, BytecodeLconst1:
			pushLongConstAt(opPC, int64(op-BytecodeLconst0))

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
			kind, intValue, text, className, err := resolveLdcConstant(cp, index)
			if err != nil {
				return nil, err
			}
			switch kind {
			case ldcInteger:
				pushConstAt(opPC, intValue)
			case ldcString:
				result := nextVal("Ljava/lang/String;")
				emit(opPC, Instruction{Op: OpConstString, Result: &result, Descriptor: text})
				push(result)
			case ldcClass:
				result := nextVal("Ljava/lang/Class;")
				emit(opPC, Instruction{Op: OpConstClass, Result: &result, Descriptor: className})
				push(result)
			}

		case BytecodeLdc2W:
			index, err := readU2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			value, err := resolveLongConstant(cp, index)
			if err != nil {
				return nil, fmt.Errorf("method %s: %w", methodName, err)
			}
			pushLongConstAt(opPC, value)

		case BytecodeIload, BytecodeLload, BytecodeIstore, BytecodeLstore, BytecodeAload, BytecodeAstore:
			index, err := readU1(code, pc)
			if err != nil {
				return nil, err
			}
			pc++
			switch op {
			case BytecodeIload, BytecodeLload:
				loadLocalAt(opPC, int(index), "")
			case BytecodeAload:
				loadLocalAt(opPC, int(index), "")
			case BytecodeIstore:
				localTypes[int(index)] = "I"
				if err := storeLocalAt(opPC, int(index)); err != nil {
					return nil, err
				}
			case BytecodeLstore:
				localTypes[int(index)] = "J"
				if err := storeLocalAt(opPC, int(index)); err != nil {
					return nil, err
				}
			case BytecodeAstore:
				if err := storeLocalAt(opPC, int(index)); err != nil {
					return nil, err
				}
			}

		case BytecodeIload0, BytecodeIload1, BytecodeIload2, BytecodeIload3:
			loadLocalAt(opPC, int(op-BytecodeIload0), "")

		case BytecodeLload0, BytecodeLload1, BytecodeLload2, BytecodeLload3:
			loadLocalAt(opPC, int(op-BytecodeLload0), "J")

		case BytecodeAload0, BytecodeAload1, BytecodeAload2, BytecodeAload3:
			loadLocalAt(opPC, int(op-BytecodeAload0), "")

		case BytecodeIstore0, BytecodeIstore1, BytecodeIstore2, BytecodeIstore3:
			index := int(op - BytecodeIstore0)
			localTypes[index] = "I"
			if err := storeLocalAt(opPC, index); err != nil {
				return nil, err
			}

		case BytecodeLstore0, BytecodeLstore1, BytecodeLstore2, BytecodeLstore3:
			index := int(op - BytecodeLstore0)
			localTypes[index] = "J"
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

		case BytecodeSwap:
			top, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			below, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			push(top)
			push(below)

		case BytecodePop:
			if _, err := pop(opPC); err != nil {
				return nil, err
			}

		case BytecodeIadd, BytecodeIsub, BytecodeImul, BytecodeIdiv, BytecodeIand,
			BytecodeLadd, BytecodeLsub, BytecodeLmul, BytecodeLdiv:
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
			case BytecodeIadd, BytecodeLadd:
				kind = OpAdd
			case BytecodeIsub, BytecodeLsub:
				kind = OpSub
			case BytecodeImul, BytecodeLmul:
				kind = OpMul
			case BytecodeIdiv, BytecodeLdiv:
				kind = OpDiv
			case BytecodeIand:
				kind = OpAnd
			}
			resultType := "I"
			if op == BytecodeLadd || op == BytecodeLsub || op == BytecodeLmul || op == BytecodeLdiv {
				resultType = "J"
			}
			result := nextVal(resultType)
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

		case BytecodeLneg:
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			result := nextVal("J")
			emit(opPC, Instruction{Op: OpNeg, Result: &result, Args: []Value{value}})
			push(result)

		case BytecodeLcmp:
			right, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			left, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			result := nextVal("I")
			emit(opPC, Instruction{Op: OpCmpLong, Result: &result, Args: []Value{left, right}})
			push(result)

		case BytecodeI2L:
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			result := nextVal("J")
			emit(opPC, Instruction{Op: OpI2L, Result: &result, Args: []Value{value}})
			push(result)

		case BytecodeL2I:
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			result := nextVal("I")
			emit(opPC, Instruction{Op: OpL2I, Result: &result, Args: []Value{value}})
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
			phiAssigns, err := getOrCreateTargetPhi(target, stack)
			if err != nil {
				return nil, fmt.Errorf("method %s at pc %d: %w", methodName, opPC, err)
			}
			label, err := branchLabel(target)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{
				Op:             OpBranch,
				Args:           []Value{value},
				Condition:      conditionForZeroBranch(op),
				Target:         label,
				TargetPC:       target,
				PhiAssignments: phiAssigns,
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
			phiAssigns, err := getOrCreateTargetPhi(target, stack)
			if err != nil {
				return nil, fmt.Errorf("method %s at pc %d: %w", methodName, opPC, err)
			}
			label, err := branchLabel(target)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{
				Op:             OpBranch,
				Args:           []Value{left, right},
				Condition:      conditionForCompareBranch(op),
				Target:         label,
				TargetPC:       target,
				PhiAssignments: phiAssigns,
			})

		case BytecodeIfAcmpeq, BytecodeIfAcmpne:
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
			phiAssigns, err := getOrCreateTargetPhi(target, stack)
			if err != nil {
				return nil, fmt.Errorf("method %s at pc %d: %w", methodName, opPC, err)
			}
			label, err := branchLabel(target)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{
				Op:             OpBranch,
				Args:           []Value{left, right},
				Condition:      conditionForCompareBranch(op),
				Target:         label,
				TargetPC:       target,
				PhiAssignments: phiAssigns,
			})

		case BytecodeIfnull, BytecodeIfnonnull:
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
			phiAssigns, err := getOrCreateTargetPhi(target, stack)
			if err != nil {
				return nil, fmt.Errorf("method %s at pc %d: %w", methodName, opPC, err)
			}
			label, err := branchLabel(target)
			if err != nil {
				return nil, err
			}
			condition := CondEQ
			if op == BytecodeIfnonnull {
				condition = CondNE
			}
			emit(opPC, Instruction{
				Op:             OpBranch,
				Args:           []Value{value},
				Condition:      condition,
				Target:         label,
				TargetPC:       target,
				PhiAssignments: phiAssigns,
			})

		case BytecodeGoto:
			offset, err := readI2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			target := opPC + int(offset)
			label, err := branchLabel(target)
			if err != nil {
				return nil, err
			}
			phiAssigns, err := getOrCreateTargetPhi(target, stack)
			if err != nil {
				return nil, fmt.Errorf("method %s at pc %d: %w", methodName, opPC, err)
			}
			for _, pa := range phiAssigns {
				dest := pa.Dest
				emit(opPC, Instruction{Op: OpAssign, Result: &dest, Args: []Value{pa.Src}})
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
			label, err := branchLabel(target)
			if err != nil {
				return nil, err
			}
			phiAssigns, err := getOrCreateTargetPhi(target, stack)
			if err != nil {
				return nil, fmt.Errorf("method %s at pc %d: %w", methodName, opPC, err)
			}
			for _, pa := range phiAssigns {
				dest := pa.Dest
				emit(opPC, Instruction{Op: OpAssign, Result: &dest, Args: []Value{pa.Src}})
			}
			emit(opPC, Instruction{Op: OpGoto, Target: label, TargetPC: target})
			stack = nil
			reachable = false

		case BytecodeTableswitch, BytecodeLookupswitch:
			key, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			defaultPC, cases, nextPC, err := parseSwitch(code, opPC)
			if err != nil {
				return nil, err
			}
			pc = nextPC
			defaultTarget, err := branchLabel(defaultPC)
			if err != nil {
				return nil, err
			}
			defPhi, err := getOrCreateTargetPhi(defaultPC, stack)
			if err != nil {
				return nil, fmt.Errorf("method %s at pc %d: %w", methodName, opPC, err)
			}
			loweredCases := make([]SwitchCase, len(cases))
			for i, c := range cases {
				label, err := branchLabel(c.TargetPC)
				if err != nil {
					return nil, err
				}
				casePhi, err := getOrCreateTargetPhi(c.TargetPC, stack)
				if err != nil {
					return nil, fmt.Errorf("method %s at pc %d: %w", methodName, opPC, err)
				}
				loweredCases[i] = SwitchCase{
					Match:          c.Match,
					Target:         label,
					TargetPC:       c.TargetPC,
					PhiAssignments: casePhi,
				}
			}
			opKind := OpTableSwitch
			if op == BytecodeLookupswitch {
				opKind = OpLookupSwitch
			}
			emit(opPC, Instruction{
				Op:                    opKind,
				Args:                  []Value{key},
				DefaultTarget:         defaultTarget,
				DefaultPC:             defaultPC,
				DefaultPhiAssignments: defPhi,
				Cases:                 loweredCases,
			})
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

		case BytecodeCheckcast:
			index, err := readU2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			className, err := cp.ResolveClassName(index)
			if err != nil {
				return nil, err
			}
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			result := nextVal("L" + className + ";")
			emit(opPC, Instruction{Op: OpCheckCast, Result: &result, Args: []Value{value}, OwnerClass: className})
			push(result)

		case BytecodeInstanceof:
			index, err := readU2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			className, err := cp.ResolveClassName(index)
			if err != nil {
				return nil, err
			}
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			result := nextVal("Z")
			emit(opPC, Instruction{Op: OpInstanceOf, Result: &result, Args: []Value{value}, OwnerClass: className})
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

		case BytecodeInvokevirtual, BytecodeInvokespecial, BytecodeInvokestatic, BytecodeInvokeinterface:
			index, err := readU2(code, pc)
			if err != nil {
				return nil, err
			}
			if op == BytecodeInvokeinterface {
				pc += 4 // 2 bytes index, 1 byte count, 1 byte reserved zero
			} else {
				pc += 2
			}
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
			case BytecodeInvokevirtual, BytecodeInvokeinterface:
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

		case BytecodeIreturn, BytecodeLreturn, BytecodeAreturn:
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

		case BytecodeNewarray:
			atype, err := readU1(code, pc)
			if err != nil {
				return nil, err
			}
			pc++
			arrayDesc, ok := newArrayDescriptor(atype)
			if !ok {
				return nil, fmt.Errorf("unsupported newarray atype %d at pc %d", atype, opPC)
			}
			size, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			result := nextVal(arrayDesc)
			emit(opPC, Instruction{Op: OpNewArray, Result: &result, Args: []Value{size}, Descriptor: arrayDesc})
			push(result)

		case BytecodeAnewarray:
			index, err := readU2(code, pc)
			if err != nil {
				return nil, err
			}
			pc += 2
			element, err := cp.ResolveClassName(index)
			if err != nil {
				return nil, err
			}
			size, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			arrayDesc := "[L" + element + ";"
			if strings.HasPrefix(element, "[") {
				arrayDesc = "[" + element
			}
			result := nextVal(arrayDesc)
			emit(opPC, Instruction{Op: OpANewArray, Result: &result, Args: []Value{size}, Descriptor: arrayDesc})
			push(result)

		case BytecodeArraylength:
			array, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			result := nextVal("I")
			emit(opPC, Instruction{Op: OpArrayLength, Result: &result, Args: []Value{array}})
			push(result)

		case BytecodeIaload, BytecodeLaload, BytecodeAaload:
			index, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			array, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			elementType := "I"
			if op == BytecodeLaload {
				elementType = "J"
			}
			if op == BytecodeAaload {
				elementType, err = arrayElementDescriptor(array.Type)
				if err != nil {
					return nil, fmt.Errorf("aaload at pc %d: %w", opPC, err)
				}
			}
			result := nextVal(elementType)
			emit(opPC, Instruction{Op: OpArrayLoad, Result: &result, Args: []Value{array, index}, Descriptor: array.Type})
			push(result)

		case BytecodeIastore, BytecodeLastore, BytecodeAastore:
			value, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			index, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			array, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{Op: OpArrayStore, Args: []Value{array, index, value}, Descriptor: array.Type})

		case BytecodeAthrow:
			exception, err := pop(opPC)
			if err != nil {
				return nil, err
			}
			emit(opPC, Instruction{Op: OpThrow, Args: []Value{exception}})
			stack = nil
			reachable = false

		case BytecodeAconstNull:
			result := nextVal("Ljava/lang/Object;")
			emit(opPC, Instruction{Op: OpConstNull, Result: &result})
			push(result)

		default:
			return nil, fmt.Errorf("unsupported opcode 0x%02X at pc %d in %s", op, opPC, methodName)
		}
	}

	cfg, err := buildCFG(code, mir.Instructions, extraTargets)
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
	case BytecodeIfAcmpeq:
		return CondEQ
	case BytecodeIfAcmpne:
		return CondNE
	default:
		return CondLE
	}
}

type ldcKind int

const (
	ldcInteger ldcKind = iota
	ldcString
	ldcClass
)

func resolveLdcConstant(cp classfile.ConstantPool, index uint16) (ldcKind, int32, string, string, error) {
	if cp == nil {
		return ldcInteger, 0, "", "", fmt.Errorf("ldc requires a constant pool")
	}
	if index == 0 || int(index) >= len(cp) || cp[index] == nil {
		return ldcInteger, 0, "", "", fmt.Errorf("invalid constant pool index %d for ldc", index)
	}
	switch entry := cp[index].(type) {
	case *classfile.ConstantInteger:
		return ldcInteger, entry.Value, "", "", nil
	case *classfile.ConstantString:
		text, err := cp.ResolveString(index)
		if err != nil {
			return ldcString, 0, "", "", err
		}
		return ldcString, 0, text, "", nil
	case *classfile.ConstantClass:
		name, err := cp.ResolveClassName(index)
		if err != nil {
			return ldcClass, 0, "", "", err
		}
		return ldcClass, 0, "", name, nil
	default:
		return ldcInteger, 0, "", "", fmt.Errorf("ldc constant at index %d has unsupported type %T", index, entry)
	}
}

func resolveLongConstant(cp classfile.ConstantPool, index uint16) (int64, error) {
	if cp == nil {
		return 0, fmt.Errorf("ldc2_w requires a constant pool")
	}
	if index == 0 || int(index) >= len(cp) || cp[index] == nil {
		return 0, fmt.Errorf("invalid constant pool index %d for ldc2_w", index)
	}
	entry, ok := cp[index].(*classfile.ConstantLong)
	if !ok {
		return 0, fmt.Errorf("ldc2_w constant at index %d has unsupported type %T", index, cp[index])
	}
	return entry.Value, nil
}

func parseSwitch(code []byte, opPC int) (int, []SwitchCase, int, error) {
	if opPC < 0 || opPC >= len(code) {
		return 0, nil, 0, fmt.Errorf("switch pc %d is outside bytecode", opPC)
	}
	op := code[opPC]
	if op != BytecodeTableswitch && op != BytecodeLookupswitch {
		return 0, nil, 0, fmt.Errorf("opcode 0x%02X at pc %d is not a switch", op, opPC)
	}

	cursor := opPC + 1
	padding := (4 - (cursor % 4)) % 4
	cursor += padding
	if cursor+8 > len(code) {
		return 0, nil, 0, fmt.Errorf("truncated switch opcode 0x%02X at pc %d", op, opPC)
	}
	defaultOffset := int32(binary.BigEndian.Uint32(code[cursor : cursor+4]))
	cursor += 4

	if op == BytecodeTableswitch {
		if cursor+8 > len(code) {
			return 0, nil, 0, fmt.Errorf("truncated tableswitch at pc %d", opPC)
		}
		low := int32(binary.BigEndian.Uint32(code[cursor : cursor+4]))
		high := int32(binary.BigEndian.Uint32(code[cursor+4 : cursor+8]))
		cursor += 8
		if high < low {
			return 0, nil, 0, fmt.Errorf("tableswitch at pc %d has high %d below low %d", opPC, high, low)
		}
		count64 := int64(high) - int64(low) + 1
		remaining := len(code) - cursor
		if count64 > int64(remaining/4) {
			return 0, nil, 0, fmt.Errorf("tableswitch at pc %d claims %d entries beyond bytecode end", opPC, count64)
		}
		cases := make([]SwitchCase, int(count64))
		for i := range cases {
			offset := int32(binary.BigEndian.Uint32(code[cursor : cursor+4]))
			cursor += 4
			cases[i] = SwitchCase{Match: low + int32(i), TargetPC: opPC + int(offset)}
		}
		return opPC + int(defaultOffset), cases, cursor, nil
	}

	if cursor+4 > len(code) {
		return 0, nil, 0, fmt.Errorf("truncated lookupswitch at pc %d", opPC)
	}
	npairs := int32(binary.BigEndian.Uint32(code[cursor : cursor+4]))
	cursor += 4
	if npairs < 0 {
		return 0, nil, 0, fmt.Errorf("lookupswitch at pc %d has negative pair count %d", opPC, npairs)
	}
	remaining := len(code) - cursor
	if int64(npairs) > int64(remaining/8) {
		return 0, nil, 0, fmt.Errorf("lookupswitch at pc %d claims %d pairs beyond bytecode end", opPC, npairs)
	}
	cases := make([]SwitchCase, int(npairs))
	for i := range cases {
		match := int32(binary.BigEndian.Uint32(code[cursor : cursor+4]))
		offset := int32(binary.BigEndian.Uint32(code[cursor+4 : cursor+8]))
		cursor += 8
		cases[i] = SwitchCase{Match: match, TargetPC: opPC + int(offset)}
	}
	return opPC + int(defaultOffset), cases, cursor, nil
}

func newArrayDescriptor(atype uint8) (string, bool) {
	switch atype {
	case 4:
		return "[Z", true
	case 5:
		return "[C", true
	case 6:
		return "[F", true
	case 7:
		return "[D", true
	case 8:
		return "[B", true
	case 9:
		return "[S", true
	case 10:
		return "[I", true
	case 11:
		return "[J", true
	default:
		return "", false
	}
}

func arrayElementDescriptor(arrayDesc string) (string, error) {
	if len(arrayDesc) < 2 || arrayDesc[0] != '[' {
		return "", fmt.Errorf("expected array descriptor, got %q", arrayDesc)
	}
	element := arrayDesc[1:]
	if element == "" {
		return "", fmt.Errorf("array descriptor %q has no element type", arrayDesc)
	}
	return element, nil
}

func buildCFG(code []byte, instructions []Instruction, extraLeaders []int) (*ControlFlowGraph, error) {
	leaders, err := collectLeaders(code, extraLeaders)
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
		case OpTableSwitch, OpLookupSwitch:
			defaultTarget := cfg.FindBlockByPC(last.DefaultPC)
			if defaultTarget == nil {
				return nil, fmt.Errorf("switch default target pc %d has no basic block", last.DefaultPC)
			}
			addEdge(block, defaultTarget)
			for _, c := range last.Cases {
				target := cfg.FindBlockByPC(c.TargetPC)
				if target == nil {
					return nil, fmt.Errorf("switch target pc %d has no basic block", c.TargetPC)
				}
				addEdge(block, target)
			}
		case OpReturn, OpThrow:
		default:
			addFallthroughEdge(cfg, block)
		}
	}

	return cfg, nil
}

func collectLeaders(code []byte, extraLeaders []int) ([]int, error) {
	leaderSet := map[int]bool{0: true}
	for _, extra := range extraLeaders {
		leaderSet[extra] = true
	}
	for pc := 0; pc < len(code); {
		decoded, err := decodeInstruction(code, pc)
		if err != nil {
			return nil, err
		}
		if decoded.hasTarget && decoded.target >= 0 && decoded.target < len(code) {
			leaderSet[decoded.target] = true
		}
		for _, target := range decoded.targets {
			if target >= 0 && target < len(code) {
				leaderSet[target] = true
			}
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
		BytecodeIfIcmpgt, BytecodeIfIcmple, BytecodeIfAcmpeq, BytecodeIfAcmpne,
		BytecodeIfnull, BytecodeIfnonnull, BytecodeGoto, BytecodeGotoW,
		BytecodeTableswitch, BytecodeLookupswitch,
		BytecodeIreturn, BytecodeLreturn, BytecodeAreturn, BytecodeReturn, BytecodeAthrow:
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
	targets   []int
}

func scanBytecode(code []byte) ([]int, error) {
	return scanBytecodeWithTargets(code, nil)
}

func scanBytecodeWithTargets(code []byte, extraTargets []int) ([]int, error) {
	boundaries := make(map[int]bool)
	targetSet := make(map[int]bool)
	for _, target := range extraTargets {
		targetSet[target] = true
	}
	for pc := 0; pc < len(code); {
		boundaries[pc] = true
		decoded, err := decodeInstruction(code, pc)
		if err != nil {
			return nil, err
		}
		if decoded.hasTarget {
			targetSet[decoded.target] = true
		}
		for _, target := range decoded.targets {
			targetSet[target] = true
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
		BytecodeLconst0, BytecodeLconst1,
		BytecodeIload0, BytecodeIload1, BytecodeIload2, BytecodeIload3,
		BytecodeLload0, BytecodeLload1, BytecodeLload2, BytecodeLload3,
		BytecodeAload0, BytecodeAload1, BytecodeAload2, BytecodeAload3,
		BytecodeIstore0, BytecodeIstore1, BytecodeIstore2, BytecodeIstore3,
		BytecodeLstore0, BytecodeLstore1, BytecodeLstore2, BytecodeLstore3,
		BytecodeAstore0, BytecodeAstore1, BytecodeAstore2, BytecodeAstore3,
		BytecodeIaload, BytecodeLaload, BytecodeAaload, BytecodeIastore, BytecodeLastore, BytecodeAastore,
		BytecodeArraylength, BytecodeAthrow,
		BytecodePop, BytecodeDup, BytecodeSwap,
		BytecodeIadd, BytecodeIsub, BytecodeImul, BytecodeIdiv, BytecodeIneg, BytecodeIand,
		BytecodeLadd, BytecodeLsub, BytecodeLmul, BytecodeLdiv, BytecodeLneg,
		BytecodeI2L, BytecodeL2I,
		BytecodeLcmp,
		BytecodeIreturn, BytecodeLreturn, BytecodeAreturn, BytecodeReturn:
		return decodedInstruction{size: 1}, nil
	case BytecodeBipush, BytecodeLdc, BytecodeIload, BytecodeLload, BytecodeAload,
		BytecodeIstore, BytecodeLstore, BytecodeAstore, BytecodeNewarray:
		if pc+2 > len(code) {
			return decodedInstruction{}, fmt.Errorf("truncated opcode 0x%02X at pc %d", op, pc)
		}
		return decodedInstruction{size: 2}, nil
	case BytecodeSipush, BytecodeLdcW, BytecodeLdc2W, BytecodeIinc,
		BytecodeNew, BytecodeAnewarray, BytecodeCheckcast, BytecodeInstanceof,
		BytecodeGetfield, BytecodePutfield, BytecodeGetstatic, BytecodePutstatic,
		BytecodeInvokevirtual, BytecodeInvokespecial, BytecodeInvokestatic:
		if pc+3 > len(code) {
			return decodedInstruction{}, fmt.Errorf("truncated opcode 0x%02X at pc %d", op, pc)
		}
		return decodedInstruction{size: 3}, nil
	case BytecodeIfeq, BytecodeIfne, BytecodeIflt, BytecodeIfge, BytecodeIfgt, BytecodeIfle,
		BytecodeIfIcmpeq, BytecodeIfIcmpne, BytecodeIfIcmplt, BytecodeIfIcmpge,
		BytecodeIfIcmpgt, BytecodeIfIcmple, BytecodeIfAcmpeq, BytecodeIfAcmpne,
		BytecodeIfnull, BytecodeIfnonnull, BytecodeGoto:
		if pc+3 > len(code) {
			return decodedInstruction{}, fmt.Errorf("truncated branch opcode 0x%02X at pc %d", op, pc)
		}
		offset := int(int16(binary.BigEndian.Uint16(code[pc+1 : pc+3])))
		return decodedInstruction{size: 3, target: pc + offset, hasTarget: true}, nil
	case BytecodeTableswitch, BytecodeLookupswitch:
		defaultPC, cases, nextPC, err := parseSwitch(code, pc)
		if err != nil {
			return decodedInstruction{}, err
		}
		targets := make([]int, 0, len(cases)+1)
		targets = append(targets, defaultPC)
		for _, c := range cases {
			targets = append(targets, c.TargetPC)
		}
		return decodedInstruction{size: nextPC - pc, targets: targets}, nil
	case BytecodeInvokeinterface:
		if pc+5 > len(code) {
			return decodedInstruction{}, fmt.Errorf("truncated opcode 0x%02X at pc %d", op, pc)
		}
		return decodedInstruction{size: 5}, nil
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
