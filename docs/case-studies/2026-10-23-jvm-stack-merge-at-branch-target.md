# JVM branch targets can merge operand-stack values

**Date:** 2026-10-23
**Area:** ir
**Status:** Open

## Problem

Lowering `ReferenceOps.same(Object)` failed with:

```text
lower same: branch to pc 10 carries 1 operand-stack values; phi nodes are not supported yet
```

The source was the natural Java expression:

```java
public boolean same(Object other) {
    return this == other;
}
```

## Context

- Go 1.24
- JDK `javac` used by the integration suite
- `ReferenceOps.java` added while testing `if_acmp*` for Phase 4

`javap -c -p` showed:

```text
0: aload_0
1: aload_1
2: if_acmpne 9
5: iconst_1
6: goto      10
9: iconst_0
10: ireturn
```

## Investigation

The lowerer deliberately requires an empty operand stack at every branch target. It pops both references at `if_acmpne`, but the `goto` at pc 6 still carries the `iconst_1` result. Both the fallthrough path and the goto path arrive at pc 10 with one value on the stack.

This is not a decoder problem: the CFG and labels are correct. It is a data-flow problem. A complete lowering needs either PHI nodes at the merge label or edge-specific assignments that copy each incoming value into a shared merge variable.

## Root Cause

JVM bytecode permits different predecessor edges to reach the same instruction with non-empty operand stacks as long as their stack shapes are compatible. Barista currently rejects that case instead of merging values.

## Resolution

This is still open. The fixture was rewritten to force early returns, which keeps every branch target stack-empty:

```java
public boolean same(Object other) {
    if (this == other) {
        return true;
    }
    return false;
}
```

`javac` then emits `if_acmpne 7; iconst_1; ireturn; 7: iconst_0; ireturn`, and lowering succeeds. This is a fixture workaround, not a compiler fix.

## Verification

- The rewritten fixture passes `TestReferenceOpsPipeline`.
- `go test ./...` passes.
- The original expression still fails with the same diagnostic if restored.

## Guidance for Future Agents

- Do not assume that `javac`/`kotlinc` avoid stack merges. Simple `return a == b` expressions can produce them.
- A real fix needs per-edge value movement or PHI nodes in the CFG, plus codegen support for merge variables.
- Keep the explicit diagnostic until the fix lands; silently dropping the stack would generate wrong code.
- `docs/phase4-in-progress.md` tracks this as the main remaining Phase 4 lowering gap.

## References

- `internal/ir/stack_eval.go` — `requireEmptyStackAtBranch`
- `internal/codegen/function.go` — `pc_` labels and predeclarations
- `tests/fixtures/java/ReferenceOps.java`
- `docs/phase4-in-progress.md`
