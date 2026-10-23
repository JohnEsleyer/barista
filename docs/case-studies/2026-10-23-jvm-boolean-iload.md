# JVM booleans use iload, so the result type cannot be hard-coded to int

**Date:** 2026-10-23
**Area:** ir
**Status:** Resolved

## Problem

The Phase 3 exception fixture takes a `boolean` parameter and branches on it. Lowering succeeded, but generated Go failed with:

```text
cannot use local_0 (variable of type bool) as int32 value in assignment
```

The branch also emitted `if v0 == 0`, which is invalid Go for a `bool`.

## Context

- Go 1.24
- Method descriptor: `run(Z)I`
- JVM bytecode starts with `iload_0` followed by `ifeq`

## Investigation

The lowerer's generic `iload` path passed a hard-coded `"I"` type:

```go
loadLocalAt(opPC, index, "I")
```

For descriptors, `Z` was already mapped to Go `bool`, but the load erased that information. The only place the real type survived was `localTypes[0]`, seeded from the method descriptor.

## Root Cause

The JVM uses the same `iload` instruction for `boolean`, `byte`, `char`, `short`, and `int` because all occupy one stack slot. The opcode alone does not determine the Java type; the local variable's declared descriptor does.

## Resolution

1. Change `iload` and `iload_0` through `iload_3` to infer the type from `localTypes` instead of forcing `"I"`.
2. Teach branch emission about `Z`: `ifeq` becomes `!value` and `ifne` becomes `value`.
3. Keep ordering comparisons on booleans as a codegen error rather than emitting invalid Go.

## Verification

- `go test ./...` compiles and runs the generated `TryCatch` package.
- `Run(false)` follows the `ifeq` path and returns `1`.
- `Run(true)` takes the throw path, is caught, and returns `2`.
- The generated branch is a boolean test, not `== 0`.

## Guidance for Future Agents

- Treat JVM stack-slot width as separate from Java type. `iload` is not proof of `int`.
- When adding `byte`, `char`, or `short` opcodes, infer from the local/descriptor where possible and normalize explicitly at the operation boundary.
- If a later phase introduces PHI nodes, the merged type must preserve `Z` rather than collapsing boolean locals to `I`.

## References

- `internal/ir/stack_eval.go` — local load lowering
- `internal/codegen/generator.go` — `branchExpr`
- `tests/fixtures/java/TryCatch.java`
