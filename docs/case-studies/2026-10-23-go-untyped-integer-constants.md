# Go infers untyped integer literals as `int`

**Date:** 2026-10-23
**Area:** codegen
**Status:** Resolved

## Problem

The first generated `Counter` module failed to compile:

```text
./counter.go:16:8: invalid operation: v3 + v4 (mismatched types int32 and int)
./counter.go:25:8: invalid operation: v2 + v3 (mismatched types int32 and int)
./counter.go:10:2: declared and not used: v0
```

The same code shape worked in Phase 1 because constants only appeared inside loop methods that predeclared every SSA value as `int32`.

## Context

- Go 1.26.1
- Generated from `tests/fixtures/java/Counter.java`
- Methods `Counter.<init>`, `Counter.increment`, and `Counter.<clinit>` are branch-free, so Phase 1 codegen used `:=` instead of predeclaring SSA variables.

Minimal generated shape:

```go
v3 := CounterCreated // int32
v4 := 1              // int
v5 := v3 + v4        // invalid: int32 + int
```

The constructor also contained:

```go
v0 := this
// Object.<init> was intentionally skipped
```

Since the skipped instruction was the only consumer of `v0`, Go reported it as unused.

## Investigation

The IR types were correct: `OpConstInt` produces an `I` value, and `CounterCreated` is `int32`. The mismatch was introduced only by Go's type inference for an untyped literal.

Three fixes were considered:

1. Predeclare every SSA value in every method. This works but makes branch-free output noisy.
2. Emit `int32(1)` for every constant. This fixes arithmetic, but branchy methods already have typed variables.
3. Emit a typed literal only when the SSA name is not predeclared.

Option 3 keeps branch-free output compact and leaves branchy output unchanged.

The unused-variable error is separate: the `java/lang/Object.<init>` no-op consumes its receiver in the IR, but the generated Go statement was removed. The fix is an explicit `_ = receiver` for that skipped call.

## Root Cause

An untyped Go integer constant defaults to `int`. It is assignable to `int32` in a declaration, but it does not make an expression such as `int32Var + untypedConstant` become `int32`; the constant is converted to the other operand's type only in contexts where that type is known. With `v4 := 1`, the constant is first materialized as an `int` variable, so the later addition is invalid.

Separately, skipping a JVM instruction during codegen can leave an SSA value with no generated read even though the IR records the value as an operand.

## Resolution

In `internal/codegen/generator.go`:

- When a constant's SSA value is not predeclared, emit `int32(<literal>)`.
- When `java/lang/Object.<init>` is skipped, emit `_ = <receiver>` so the receiver value remains used.
- Keep the Phase 1 predeclaration strategy for methods containing labels or branches.

Generated branch-free constructor now looks like:

```go
v3 := CounterCreated
v4 := int32(1)
v5 := v3 + v4
CounterCreated = v5
```

## Verification

```sh
go test ./...
```

The integration test compiles and runs the generated Counter module, including constructor static-state updates, field increments, and all three dispatch forms. The generated code also compiles under `go test ./...` in a temporary module with `GOWORK=off`.

## Guidance for Future Agents

- Treat every JVM constant as a typed value. If codegen does not predeclare the SSA name, wrap the literal in the target Go type.
- Audit skipped or fused IR instructions for hidden readers. Go rejects unused local variables, so removed statements may require a blank use.
- Do not assume `int32Var + 1` is safe after materializing `1` through `:=`.
- Re-run the object fixture whenever codegen changes branch-free declaration handling.

## References

- `internal/codegen/generator.go`
- `internal/ir/instruction.go`
- `tests/fixtures/java/Counter.java`
- `tests/integration_test.go`
- [Go rejects goto across SSA variable declarations](2026-10-23-go-goto-over-declarations.md)
