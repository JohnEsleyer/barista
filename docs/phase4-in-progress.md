# Phase 4 — Follow-up Notes

> **Historical note:** This document records compiler work from the earlier generic-JVM phase. Barista now targets new code written against the Barista SDK; see [`barista-first.md`](barista-first.md) for the current direction. The technical findings and open compiler problems below remain useful, but compatibility work should now be justified by a concrete Barista-first use case.

**Date:** 2026-10-23
**Status:** Historical compiler notes; deferred work remains open
**Scope:** Kotlin engine: switches, intrinsics, default-parameter masks, Longs, casts, PHI joins, and the Kotlin fixture.

## What Works

- `tableswitch` and `lookupswitch` are decoded, lowered, added to the CFG, and emitted as Go `switch` statements.
- JVM branch, loop, and switch targets are emitted as `pc_<offset>` labels.
- Operand-stack merges at branch and switch targets lower to PHI variables with per-edge assignments. `return this == other;` and ternary expressions now compile.
- `long` support covers `lconst_0/1`, `ldc2_w`, `lload`/`lstore`, `lreturn`, `ladd`/`lsub`/`lmul`/`ldiv`/`lneg`, `lcmp`, `i2l`, `l2i`, and `laload`/`lastore`.
- `iand`, `swap`, `if_acmp*`, and `ifnull`/`ifnonnull` are lowered.
- `checkcast` and `instanceof` use generic Go runtime helpers.
- `java.lang.Object` virtual `equals`, `hashCode`, and `toString` dispatch through native helpers for generated and runtime values.
- Kotlin `Intrinsics` null checks, `AreEqual`, lateinit helpers, and `kotlin.io.ConsoleKt` are available.
- Kotlin default-parameter `$default` methods work through ordinary `iand` + `ifeq` lowering; generated Go method names replace `$` with `_`.
- `tests/fixtures/kotlin/Greeting.kt` compiles with Kotlin, lowers through Barista, generates Go, compiles, and runs.

## Open Problems

### 1. No cross-class dispatch or inheritance

Generated code still supports only the class being compiled plus the native runtime shims. Kotlin stdlib calls, `object` singletons, `Companion` methods, interfaces, and inherited members outside the shim set are rejected. `Greeting.kt` works because it uses same-class methods and runtime-backed `String`/`StringBuilder`/`ConsoleKt`.

### 2. Kotlin object singletons and coroutines are not implemented

Data classes and default-parameter bridges work. Kotlin `object` singletons, `Companion`, enums, property delegates, full `DefaultConstructorMarker` constructor semantics, and coroutine state-machine optimisation remain open. `tableswitch`/`lookupswitch` are ready for coroutine state machines, but no coroutine fixture or suspension bridge exists yet.

### 3. Casts and `instanceof` are concrete-type checks

`checkcast` and `instanceof` use Go type assertions such as:

```go
lang.CheckCast[*User](value)
lang.IsInstanceOf[*User](value)
```

They do not model JVM interfaces, class inheritance, or `java.lang.Object` as a universal supertype. A JVM-valid cast can therefore fail if the generated Go type hierarchy differs from the JVM hierarchy. Dynamic `Object` methods do not change the cast model.

### 4. PHI joins are slot-based

The lowering merges operand-stack slots, not full JVM types. It trusts the bytecode verifier that incoming paths have compatible stack shapes. If incompatible bytecode reaches a join, the generated Go may fail to compile or behave incorrectly instead of producing an IR diagnostic.

### 5. Kotlin compiler availability in CI

`kotlinc` is not on `PATH` in the current environment. The Kotlin integration test skips unless `KOTLINC` points at a working compiler. Local verification used a cached Kotlin 2.1.10 compiler wrapper. CI should install `kotlinc` or provide a pinned compiler path so the Kotlin fixture is not silently skipped.

### 6. Narrow primitive returns and arithmetic

`ireturn` in a boolean method is normalized to a Go `bool`. `byte`, `char`, and `short` arithmetic and returns are still not narrowed/zero-extended, and `float`/`double` opcodes remain unsupported.

## Next Steps

1. Add Kotlin `object`, `Companion`, enum, and full `DefaultConstructorMarker` support.
2. Add a coroutine fixture, then state-machine recognition and the goroutine/channel bridge.
3. Add cross-class dispatch, inheritance, and interface support so Kotlin stdlib calls can resolve beyond runtime shims.
4. Expand casts/`instanceof` to respect generated inheritance and interfaces.
5. Pin or install `kotlinc` in CI and require the Kotlin integration test when the compiler is available.

## References

- `internal/ir/stack_eval.go` — operand-stack evaluation, PHI joins, switches, casts
- `internal/codegen/function.go` — `pc_` labels, phi assignments, switches, casts, runtime calls
- `internal/codegen/runtime.go` — runtime and Kotlin import detection
- `tests/integration_test.go` — switch, reference-op, and Kotlin end-to-end tests
- `tests/fixtures/java/ReferenceOps.java` — PHI joins, `ternary`, casts, and Long arrays
- `tests/fixtures/kotlin/Greeting.kt` — Kotlin data class and default-parameter fixture
- `docs/case-studies/2026-10-23-jvm-stack-merge-phi-lowering.md` — PHI implementation details
