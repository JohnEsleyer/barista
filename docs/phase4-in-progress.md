# Phase 4 — In Progress Notes

**Date:** 2026-10-23
**Status:** In progress
**Scope:** Kotlin engine: switches, intrinsics, default-parameter masks, Longs, casts, and the Kotlin fixture.

## What Works

- `tableswitch` and `lookupswitch` are decoded, lowered, added to the CFG, and emitted as Go `switch` statements.
- JVM branch and switch targets are emitted as `pc_<offset>` labels.
- `long` support covers `lconst_0/1`, `ldc2_w`, `lload`/`lstore`, `lreturn`, `ladd`/`lsub`/`lmul`/`ldiv`/`lneg`, `lcmp`, `i2l`, `l2i`, and `laload`/`lastore`.
- `iand`, `swap`, `if_acmp*`, and `ifnull`/`ifnonnull` are lowered.
- `checkcast` and `instanceof` use generic Go runtime helpers.
- Kotlin `Intrinsics` null checks, `AreEqual`, and lateinit helpers live in `pkg/runtime/kotlin/jvm`.
- Kotlin default-parameter `$default` methods work through ordinary `iand` + `ifeq` lowering; generated Go method names replace `$` with `_`.
- `tests/fixtures/kotlin/Greeting.kt` compiles with Kotlin, lowers through Barista, generates Go, compiles, and runs.

## Open Problems

### 1. Branch targets with merged operand-stack values (PHI nodes)

The first `ReferenceOps.java` version implemented:

```java
public boolean same(Object other) {
    return this == other;
}
```

`javac` emits:

```text
0: aload_0
1: aload_1
2: if_acmpne 9
5: iconst_1
6: goto      10
9: iconst_0
10: ireturn
```

Barista rejects this with:

```text
branch to pc 10 carries 1 operand-stack values; phi nodes are not supported yet
```

The fixture was rewritten to use explicit early returns, which keeps the stack empty at every branch target. This is a workaround, not a real fix. General Kotlin and Java control flow can produce stack merges, so proper edge values or PHI nodes are the main remaining lowering gap.

### 2. No cross-class dispatch or inheritance

Generated code still supports only the class being compiled plus the native runtime shims. Kotlin stdlib calls, `object` singletons, `Companion` methods, interfaces, and inherited members outside the shim set are rejected. `Greeting.kt` works because it uses same-class methods and runtime-backed `String`/`StringBuilder`.

### 3. Kotlin object singletons and coroutines are not implemented

Data classes and default-parameter bridges work. Kotlin `object` singletons, `DefaultConstructorMarker`, enums, property delegates, and coroutine state-machine optimisation remain open. `tableswitch`/`lookupswitch` are ready for coroutine state machines, but no coroutine fixture or suspension bridge exists yet.

### 4. Casts and `instanceof` are concrete-type checks

`checkcast` and `instanceof` use Go type assertions such as:

```go
lang.CheckCast[*User](value)
lang.IsInstanceOf[*User](value)
```

They do not model JVM interfaces, class inheritance, or `java.lang.Object` as a universal supertype. A JVM-valid cast can therefore fail if the generated Go type hierarchy differs from the JVM hierarchy.

### 5. `java.lang.Object` is mapped to `any`

This keeps Kotlin `Any?` parameters and identity comparisons usable, but arbitrary generated classes do not share a native `Object` implementation. Virtual `Object.hashCode`, `Object.equals`, and `Object.toString` calls on non-runtime values are not yet routed through generic helpers.

### 6. Kotlin compiler availability in CI

`kotlinc` is not on `PATH` in the current environment. The Kotlin integration test skips unless `KOTLINC` points at a working compiler. Local verification used a cached Kotlin 2.1.10 compiler wrapper. CI should install `kotlinc` or provide a pinned compiler path so the Kotlin fixture is not silently skipped.

### 7. Narrow primitive returns and arithmetic

`ireturn` in a boolean method is normalized to a Go `bool`. `byte`, `char`, and `short` arithmetic and returns are still not narrowed/zero-extended, and `float`/`double` opcodes remain unsupported.

## Next Steps

1. Implement PHI/edge-value handling for non-empty stacks at branch and switch targets.
2. Add generic `Object` method helpers and include interfaces/inheritance in cast checks.
3. Add Kotlin `object`, `Companion`, enum, and `DefaultConstructorMarker` support.
4. Add a coroutine fixture and state-machine recognition.
5. Pin or install `kotlinc` in CI and require the Kotlin integration test when the compiler is available.

## References

- `internal/ir/stack_eval.go` — stack validation and branch lowering
- `internal/codegen/function.go` — `pc_` labels, switches, casts, runtime calls
- `tests/integration_test.go` — switch, reference-op, and Kotlin end-to-end tests
- `tests/fixtures/java/ReferenceOps.java` — current early-return workaround for stack merges
- `tests/fixtures/kotlin/Greeting.kt` — Kotlin data class and default-parameter fixture
