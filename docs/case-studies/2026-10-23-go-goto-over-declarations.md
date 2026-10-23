# Go rejects goto across SSA variable declarations

**Date:** 2026-10-23
**Area:** codegen
**Status:** Resolved

## Problem

After adding branch and loop lowering, the generated Go for `Calculator.absolute` and `Calculator.sumTo` failed to compile with errors of the form:

```text
goto L0 jumps over declaration of v3
```

The IR itself was correct; the failure only appeared once labels and `goto` were emitted.

## Context

- Go 1.26.1
- Generated from `tests/fixtures/java/Calculator.java`
- Codegen uses `go/ast` plus `go/format`

## Investigation

The original emitter declared SSA temporaries at their first assignment:

```go
v0 := local_0
if v0 >= 0 {
    goto L0
}
v1 := local_0
...
L0:
v3 := local_0
```

A `goto` that jumps forward over `v1 := ...` is illegal because the declaration is in scope at the label. The same problem exists for locals such as `local_1` and `local_2` in the loop.

## Root Cause

Go's `goto` statement may not jump over variable declarations that remain in scope at the target. Linear IR assigns SSA values at arbitrary points, which is incompatible with declaration-on-first-use once backward or forward jumps exist.

## Resolution

For methods that contain labels, `goto`, or conditional branches:

1. Predeclare every SSA value (`var vN int32`) and every non-parameter local (`var local_N int32`) at the top of the function.
2. Emit assignments (`=`) instead of short declarations (`:=`).
3. Add `_ = name` for declared values that are never read, because Go also rejects unused local variables.

Branch-free methods keep the compact `:=` form, so simple arithmetic still produces clean output.

Labels are buffered and attached to the next emitted statement, since Go labels must precede a statement.

## Verification

- The generated module for `Calculator` compiles and runs.
- `go test ./...` covers all seven static methods, including the `sumTo` loop and the `absolute`/`max` branches.
- `barista brew` emits code with predeclared `var` blocks for branchy methods only.

## Guidance for Future Agents

- Treat Go's `goto` restrictions as a codegen constraint, not an IR constraint.
- If you introduce PHI nodes or structured control flow later, this predeclaration strategy can be narrowed, but do not revert to `:=` for any method whose IR contains labels until then.
- Remember that a declared-but-never-read variable is a compile error in Go; track reads when predeclaring.

## References

- Go language specification: `goto` statements and declaration scoping
- `internal/codegen/generator.go`
- `internal/ir/stack_eval.go`
- `tests/integration_test.go`
