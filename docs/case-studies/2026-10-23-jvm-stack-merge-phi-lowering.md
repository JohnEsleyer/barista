# Lowering JVM operand stack merges to branch-scoped phi assignments

**Date:** 2026-10-23
**Area:** ir, codegen
**Status:** Resolved

## Problem

Compiling idiomatic Java and Kotlin expressions containing ternary operators (`a ? b : c`), boolean returns (`return this == other;`), or if-else expressions failed during lowering with:

```text
branch to pc 10 carries 1 operand-stack values; phi nodes are not supported yet
```

Early fixtures worked around this by replacing expression-level joins with explicit early returns, which forced the operand stack to be empty at every branch target.

## Context

- JVM bytecode verifier guarantees that along all paths reaching any bytecode offset, the operand stack depth and types are identical.
- Java `return this == other;` emits:
  ```text
  0: aload_0
  1: aload_1
  2: if_acmpne 9
  5: iconst_1
  6: goto 10
  9: iconst_0
  10: ireturn
  ```
- At pc 10, the stack depth is 1 on both paths (incoming from goto 6, and incoming from fallthrough 9).

## Investigation

Traditional SSA compilers construct a dominance frontier and insert PHI nodes at join basic blocks. However, Barista emits linear SSA Go source with predeclared variables for branchy methods.

Because the operand stack depth and types at any label are known from incoming paths:
1. When a branch or goto targets a label with $k$ items on the stack, the label requires $k$ join temporaries (phi variables).
2. For an unconditional jump (`goto`), the assignments `phi_k = stack_k` are emitted immediately before the `goto`.
3. For a conditional branch (`if_...`), the assignments `phi_k = stack_k` are placed inside the Go `if` statement before the `goto`.
4. For fallthrough into a label, `phi_k = stack_k` is emitted immediately before the label.
5. At the label itself, the operand stack is initialized with `[phi_0, ..., phi_{k-1}]`.

## Root Cause

Barista previously assumed that any non-empty operand stack at a branch target required a full SSA dominator tree reconstruction, which caused lowering to bail out prematurely.

## Resolution

1. Added `OpAssign` to the intermediate representation.
2. Introduced `PhiAssignment` records on `Instruction` (for branches and switch default) and `SwitchCase` (for switch cases).
3. In `internal/ir/stack_eval.go`:
   - Evaluated target phi requirements using `getOrCreateTargetPhi(targetPC, stack)`.
   - Emitted assignments before `goto` and fallthrough labels.
   - Attached branch-specific assignments to `OpBranch`.
4. In `internal/codegen/function.go`:
   - Emitted phi assignments inside conditional `if` blocks prior to `goto`.
   - Included phi variables in `emitPredeclarations` so Go's declaration-scoping rules for `goto` are satisfied.

Generated Go shape:

```go
var v3 int32
...
if v0 != v1 {
    goto pc_9
}
v2 = 1
v3 = v2
goto pc_10
pc_9:
v4 = 0
v3 = v4
pc_10:
return v3 != 0
```

## Verification

- `go test ./...` passes.
- `ReferenceOps.java` was updated with `return this == other;` and `ternary(flag, a, b)`, both compiling and passing tests.
- `TestLowerBytecodeStackMergePhi` verifies the IR shape for stacked joins.
- Kotlin data class `equals` and ternary methods compile and pass end-to-end tests.

## Guidance for Future Agents

- In a stack de-stacking compiler, operand stack values at join points are simply variables with multiple incoming assignments.
- Always emit the fallthrough assignment before the target label, not after it, so jumping to the label does not execute the fallthrough assignment.
- Ensure all phi destination variables are registered with `emitPredeclarations` when the method contains branches or labels.

## References

- `internal/ir/instruction.go`
- `internal/ir/stack_eval.go`
- `internal/codegen/function.go`
- `tests/fixtures/java/ReferenceOps.java`
- `docs/case-studies/2026-10-23-go-goto-over-declarations.md`
