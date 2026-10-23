# Kotlin default-parameter masks are ordinary bitwise branches

**Date:** 2026-10-23
**Area:** ir
**Status:** Resolved

## Problem

Kotlin functions with default parameters compile to a synthetic `$default` bridge that receives a bit mask. It was not obvious whether Barista needed a Kotlin-specific AST pass to unpack defaults before lowering.

## Context

- Kotlin 2.1.10
- `tests/fixtures/kotlin/Greeting.kt`
- Generated class `GreetingKt`
- Method `farewell$default(String, String, int, Object)`

The fixture has:

```kotlin
fun farewell(name: String, punctuation: String = "!"): String =
    "Goodbye, $name$punctuation"
```

## Investigation

`javap` showed the bridge using the same generic bytecode as any other method:

```text
iload_2
iconst_2
iand
ifeq ...
ldc "!"
astore_1
```

The mask test is a plain `iand` followed by `ifeq`; the default value is loaded and stored into the parameter local on the branch. There is no special constant-pool entry or annotation needed at lowering time.

The only naming wrinkle is that JVM method names contain `$`. Barista's exported Go name helper replaces `$` with `_`, so `farewell$default` becomes `farewell_default`.

## Root Cause

Kotlin encodes default arguments as a generated bridge with explicit bit tests. Once `iand`, local stores, branches, and `$`-containing method names are supported, the bridge lowers without a Kotlin-aware pass.

## Resolution

1. Lower `iand` to the existing binary-expression path.
2. Emit `$default` methods with sanitized Go names.
3. Add the Kotlin `Intrinsics` import and null-check shims used by the bridge and constructors.
4. Verify the generated `Farewell_default` call with a nil default and a non-zero mask.

## Verification

- `TestLowerBytecodeStackMergePhi` covers operand-stack joins for `$default` masks and other branches.
- `TestIntegrationKotlinGreeting` calls `Farewell_default(lang.NewString("Barista"), nil, 2, nil)` and gets `"Goodbye, Barista!"`.
- `KOTLINC=<compiler> go test ./tests -run TestIntegrationKotlinGreeting` passes.

## Guidance for Future Agents

- Inspect `javap` output before designing Kotlin-specific lowering passes; many features are ordinary JVM patterns.
- Preserve JVM method identity while sanitizing Go names. `$` is legal in JVM names but not in Go identifiers.
- Reuse the generic branch and stack machinery for default masks rather than adding a parallel evaluator.

## References

- `tests/fixtures/kotlin/Greeting.kt`
- `internal/ir/stack_eval.go` — `iand` lowering
- `internal/codegen/function.go` — method-name resolution and runtime calls
- `pkg/runtime/kotlin/jvm/intrinsics.go`
- `tests/integration_test.go` — `TestIntegrationKotlinGreeting`
