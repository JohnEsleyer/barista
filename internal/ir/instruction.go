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

// Value represents a typed symbolic value in the SSA-style IR.
type Value struct {
	ID   int
	Type string // e.g. "int32"
}

func (v Value) String() string { return fmt.Sprintf("v%d", v.ID) }

// Instruction represents a single normalized intermediate instruction.
type Instruction struct {
	Op        OpKind
	Result    *Value
	Args      []Value
	Constant  int32
	LocalVar  int
	Label     int
	Target    int
	Condition BranchCondition
}

func (inst Instruction) String() string {
	switch inst.Op {
	case OpConstInt:
		return fmt.Sprintf("%s = %d", inst.Result, inst.Constant)
	case OpLoadVar:
		return fmt.Sprintf("%s = local_%d", inst.Result, inst.LocalVar)
	case OpStoreVar:
		return fmt.Sprintf("local_%d = %s", inst.LocalVar, inst.Args[0])
	case OpAdd, OpSub, OpMul, OpDiv:
		return fmt.Sprintf("%s = %s %s %s", inst.Result, inst.Args[0], binarySymbol(inst.Op), inst.Args[1])
	case OpNeg:
		return fmt.Sprintf("%s = -%s", inst.Result, inst.Args[0])
	case OpLabel:
		return fmt.Sprintf("L%d:", inst.Label)
	case OpGoto:
		return fmt.Sprintf("goto L%d", inst.Target)
	case OpBranch:
		if len(inst.Args) == 1 {
			return fmt.Sprintf("if %s %s 0 goto L%d", inst.Args[0], inst.Condition, inst.Target)
		}
		if len(inst.Args) == 2 {
			return fmt.Sprintf("if %s %s %s goto L%d", inst.Args[0], inst.Condition, inst.Args[1], inst.Target)
		}
		return fmt.Sprintf("if <invalid> goto L%d", inst.Target)
	case OpReturn:
		if len(inst.Args) > 0 {
			return fmt.Sprintf("return %s", inst.Args[0])
		}
		return "return"
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
	default:
		return "?"
	}
}

// MethodIR holds the lowered SSA instructions for a method.
type MethodIR struct {
	Name         string
	Descriptor   string
	Instructions []Instruction
	ParamCount   int
	HasReturnVal bool
	IsStatic     bool
}
