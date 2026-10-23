package ir

import "fmt"

// OpKind identifies intermediate representation operation types.
type OpKind int

const (
	OpConstInt OpKind = iota
	OpLoadVar
	OpStoreVar
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpNeg
	OpLabel
	OpGoto
	OpBranch
	OpReturn

	// Phase 2: objects, fields, and member dispatch.
	OpNew
	OpGetField
	OpPutField
	OpGetStatic
	OpPutStatic
	OpInvokeVirtual
	OpInvokeStatic
	OpInvokeSpecial

	// Phase 3: strings, arrays, and exceptions.
	OpConstString
	OpConstClass
	OpConstNull
	OpNewArray
	OpANewArray
	OpArrayLength
	OpArrayLoad
	OpArrayStore
	OpThrow
	OpLoadException

	// Phase 4: Kotlin engine (longs, bitwise masks, switches, casts, assignments).
	OpAnd
	OpCmpLong
	OpConstLong
	OpI2L
	OpL2I
	OpTableSwitch
	OpLookupSwitch
	OpCheckCast
	OpInstanceOf
	OpAssign
)

func (k OpKind) String() string {
	switch k {
	case OpConstInt:
		return "CONST_INT"
	case OpLoadVar:
		return "LOAD_VAR"
	case OpStoreVar:
		return "STORE_VAR"
	case OpAdd:
		return "ADD"
	case OpSub:
		return "SUB"
	case OpMul:
		return "MUL"
	case OpDiv:
		return "DIV"
	case OpNeg:
		return "NEG"
	case OpLabel:
		return "LABEL"
	case OpGoto:
		return "GOTO"
	case OpBranch:
		return "BRANCH"
	case OpReturn:
		return "RETURN"
	case OpNew:
		return "NEW"
	case OpGetField:
		return "GET_FIELD"
	case OpPutField:
		return "PUT_FIELD"
	case OpGetStatic:
		return "GET_STATIC"
	case OpPutStatic:
		return "PUT_STATIC"
	case OpInvokeVirtual:
		return "INVOKE_VIRTUAL"
	case OpInvokeStatic:
		return "INVOKE_STATIC"
	case OpInvokeSpecial:
		return "INVOKE_SPECIAL"
	case OpConstString:
		return "CONST_STRING"
	case OpConstClass:
		return "CONST_CLASS"
	case OpConstNull:
		return "CONST_NULL"
	case OpNewArray:
		return "NEW_ARRAY"
	case OpANewArray:
		return "ANEW_ARRAY"
	case OpArrayLength:
		return "ARRAY_LENGTH"
	case OpArrayLoad:
		return "ARRAY_LOAD"
	case OpArrayStore:
		return "ARRAY_STORE"
	case OpThrow:
		return "THROW"
	case OpLoadException:
		return "LOAD_EXCEPTION"
	case OpAnd:
		return "AND"
	case OpCmpLong:
		return "CMP_LONG"
	case OpConstLong:
		return "CONST_LONG"
	case OpI2L:
		return "I2L"
	case OpL2I:
		return "L2I"
	case OpTableSwitch:
		return "TABLE_SWITCH"
	case OpLookupSwitch:
		return "LOOKUP_SWITCH"
	case OpCheckCast:
		return "CHECK_CAST"
	case OpInstanceOf:
		return "INSTANCE_OF"
	case OpAssign:
		return "ASSIGN"
	default:
		return fmt.Sprintf("UNKNOWN_OP(%d)", k)
	}
}

// BranchCondition is the comparison used by OpBranch.
type BranchCondition uint8

const (
	CondEQ BranchCondition = iota
	CondNE
	CondLT
	CondGE
	CondGT
	CondLE
)

func (c BranchCondition) String() string {
	switch c {
	case CondEQ:
		return "=="
	case CondNE:
		return "!="
	case CondLT:
		return "<"
	case CondGE:
		return ">="
	case CondGT:
		return ">"
	case CondLE:
		return "<="
	default:
		return "?"
	}
}

// Value represents a typed symbolic value in the SSA-style IR. Type stores a
// JVM descriptor such as "I" or "Lcom/example/User;".
type Value struct {
	ID   int
	Type string
}

func (v Value) String() string { return fmt.Sprintf("v%d", v.ID) }

// PhiAssignment represents an assignment of an operand-stack value to a join
// temporary (phi variable) before a branch or switch target.
type PhiAssignment struct {
	Dest Value
	Src  Value
}

// SwitchCase is one arm of a tableswitch or lookupswitch.
type SwitchCase struct {
	Match          int32
	Target         int
	TargetPC       int
	PhiAssignments []PhiAssignment
}

// Instruction represents a single normalized intermediate instruction.
type Instruction struct {
	Op         OpKind
	Result     *Value
	Args       []Value
	Constant   int32
	Constant64 int64
	LocalVar   int
	Label      int
	Target     int
	Condition  BranchCondition

	// PC is the bytecode offset of the JVM instruction that produced this IR
	// instruction. Synthetic instructions may share a PC.
	PC int
	// TargetPC is the absolute bytecode offset of a branch target.
	TargetPC int

	// Switch metadata. DefaultTarget is the IR label for DefaultPC.
	DefaultTarget int
	DefaultPC     int
	Cases         []SwitchCase

	// PhiAssignments contains assignments to join variables executed when this
	// branch or switch arm is taken.
	PhiAssignments        []PhiAssignment
	DefaultPhiAssignments []PhiAssignment

	// Object/member metadata resolved from the constant pool.
	OwnerClass string
	MemberName string
	Descriptor string
}

func (inst Instruction) String() string {
	switch inst.Op {
	case OpConstInt:
		return fmt.Sprintf("%s = %d", inst.Result, inst.Constant)
	case OpLoadVar:
		return fmt.Sprintf("%s = local_%d", inst.Result, inst.LocalVar)
	case OpStoreVar:
		return fmt.Sprintf("local_%d = %s", inst.LocalVar, inst.Args[0])
	case OpAssign:
		return fmt.Sprintf("%s = %s", inst.Result, inst.Args[0])
	case OpAdd, OpSub, OpMul, OpDiv, OpAnd:
		return fmt.Sprintf("%s = %s %s %s", inst.Result, inst.Args[0], binarySymbol(inst.Op), inst.Args[1])
	case OpConstLong:
		return fmt.Sprintf("%s = %d", inst.Result, inst.Constant64)
	case OpCmpLong:
		return fmt.Sprintf("%s = cmp(%s, %s)", inst.Result, inst.Args[0], inst.Args[1])
	case OpI2L:
		return fmt.Sprintf("%s = int64(%s)", inst.Result, inst.Args[0])
	case OpL2I:
		return fmt.Sprintf("%s = int32(%s)", inst.Result, inst.Args[0])
	case OpNeg:
		return fmt.Sprintf("%s = -%s", inst.Result, inst.Args[0])
	case OpLabel:
		if inst.PC != 0 {
			return fmt.Sprintf("L%d: (pc %d)", inst.Label, inst.PC)
		}
		return fmt.Sprintf("L%d:", inst.Label)
	case OpGoto:
		return fmt.Sprintf("goto L%d (pc %d)", inst.Target, inst.TargetPC)
	case OpTableSwitch, OpLookupSwitch:
		return fmt.Sprintf("%s = switch %s default L%d cases=%d", inst.Op, inst.Args[0], inst.DefaultTarget, len(inst.Cases))
	case OpCheckCast:
		return fmt.Sprintf("%s = (%s) %s", inst.Result, inst.OwnerClass, inst.Args[0])
	case OpInstanceOf:
		return fmt.Sprintf("%s = %s instanceof %s", inst.Result, inst.Args[0], inst.OwnerClass)
	case OpBranch:
		if len(inst.Args) == 1 {
			return fmt.Sprintf("if %s %s 0 goto L%d (pc %d)", inst.Args[0], inst.Condition, inst.Target, inst.TargetPC)
		}
		if len(inst.Args) == 2 {
			return fmt.Sprintf("if %s %s %s goto L%d (pc %d)", inst.Args[0], inst.Condition, inst.Args[1], inst.Target, inst.TargetPC)
		}
		return fmt.Sprintf("if <invalid> goto L%d", inst.Target)
	case OpReturn:
		if len(inst.Args) > 0 {
			return fmt.Sprintf("return %s", inst.Args[0])
		}
		return "return"
	case OpNew:
		return fmt.Sprintf("%s = new %s", inst.Result, inst.OwnerClass)
	case OpGetField:
		return fmt.Sprintf("%s = %s.%s", inst.Result, inst.Args[0], inst.MemberName)
	case OpPutField:
		return fmt.Sprintf("%s.%s = %s", inst.Args[0], inst.MemberName, inst.Args[1])
	case OpGetStatic:
		return fmt.Sprintf("%s = %s::%s", inst.Result, inst.OwnerClass, inst.MemberName)
	case OpPutStatic:
		return fmt.Sprintf("%s::%s = %s", inst.OwnerClass, inst.MemberName, inst.Args[0])
	case OpInvokeVirtual, OpInvokeStatic, OpInvokeSpecial:
		recv := ""
		if inst.Op != OpInvokeStatic && len(inst.Args) > 0 {
			recv = inst.Args[0].String() + "."
		}
		callArgs := inst.Args
		if inst.Op != OpInvokeStatic && len(callArgs) > 0 {
			callArgs = callArgs[1:]
		}
		if inst.Result != nil {
			return fmt.Sprintf("%s = %s%s%s(%v) [%s]", inst.Result, recv, inst.MemberName, inst.Descriptor, callArgs, inst.OwnerClass)
		}
		return fmt.Sprintf("%s%s%s(%v) [%s]", recv, inst.MemberName, inst.Descriptor, callArgs, inst.OwnerClass)
	case OpConstString:
		return fmt.Sprintf("%s = %q", inst.Result, inst.Descriptor)
	case OpConstClass:
		return fmt.Sprintf("%s = class %s", inst.Result, inst.Descriptor)
	case OpConstNull:
		return fmt.Sprintf("%s = null", inst.Result)
	case OpNewArray, OpANewArray:
		return fmt.Sprintf("%s = new %s[%s]", inst.Result, inst.Descriptor, inst.Args[0])
	case OpArrayLength:
		return fmt.Sprintf("%s = len(%s)", inst.Result, inst.Args[0])
	case OpArrayLoad:
		return fmt.Sprintf("%s = %s[%s]", inst.Result, inst.Args[0], inst.Args[1])
	case OpArrayStore:
		return fmt.Sprintf("%s[%s] = %s", inst.Args[0], inst.Args[1], inst.Args[2])
	case OpThrow:
		return fmt.Sprintf("throw %s", inst.Args[0])
	case OpLoadException:
		return fmt.Sprintf("%s = <caught exception>", inst.Result)
	default:
		return fmt.Sprintf("%s (?)", inst.Op)
	}
}

func binarySymbol(op OpKind) string {
	switch op {
	case OpAdd:
		return "+"
	case OpSub:
		return "-"
	case OpMul:
		return "*"
	case OpDiv:
		return "/"
	case OpAnd:
		return "&"
	default:
		return "?"
	}
}

// ExceptionTrap describes one protected bytecode range and its catch target.
type ExceptionTrap struct {
	StartPC    int
	EndPC      int
	HandlerPC  int
	CatchClass string
}

// MethodIR holds the lowered SSA instructions for a method.
type MethodIR struct {
	OwnerClass     string
	Name           string
	Descriptor     string
	Instructions   []Instruction
	CFG            *ControlFlowGraph
	ExceptionTraps []ExceptionTrap
	ParamCount     int
	HasReturnVal   bool
	IsStatic       bool
}
