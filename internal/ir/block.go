package ir

import "fmt"

// BasicBlock is a maximal sequence of instructions with a single entry and a
// single exit. The final instruction is normally a branch, goto, or return.
type BasicBlock struct {
	ID           int
	StartPC      int
	EndPC        int
	Instructions []Instruction
	Predecessors []*BasicBlock
	Successors   []*BasicBlock
}

// Label returns the stable label used by diagnostics and IR dumps.
func (b *BasicBlock) Label() string {
	return fmt.Sprintf("bb_%d", b.ID)
}

// ControlFlowGraph is the directed graph of basic blocks for one method.
type ControlFlowGraph struct {
	Blocks []*BasicBlock
	Entry  *BasicBlock
}

// FindBlockByPC returns the block whose bytecode range contains pc.
func (cfg *ControlFlowGraph) FindBlockByPC(pc int) *BasicBlock {
	if cfg == nil {
		return nil
	}
	for _, block := range cfg.Blocks {
		if pc >= block.StartPC && pc < block.EndPC {
			return block
		}
	}
	return nil
}
