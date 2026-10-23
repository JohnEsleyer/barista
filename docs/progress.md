# Development Progress

This is the canonical tracker for Barista's implementation progress. Update it at the end of every meaningful change so a new contributor can understand what works, what does not, and what comes next without reading the entire commit history.

**Last updated:** 2026-10-23
**Current phase:** Phase 4 core complete; Phase 5 next
**Latest implementation commit:** `e383f40` — Implement Phase 4 Kotlin engine, switches, and operand stack merge phi lowering

---

## Status at a Glance

| Area | Status | Evidence |
| --- | --- | --- |
| Build | Passing | `go build ./...` |
| Vet | Passing | `go vet ./...` |
| Tests | Passing | `go test ./...` |
| Race detector | Passing | `go test -race ./...` |
| Class-file parsing | Complete | `internal/classfile` |
| IR lowering | Complete for Phase 4 (includes PHI joins and switches) | `internal/ir` |
| Control flow graph | Basic blocks, switch edges, predecessors, successors | `internal/ir/block.go` |
| Go code generation | Objects, fields, switches, casts, dynamic Object dispatch, Kotlin intrinsics | `internal/codegen` |
| Runtime shims | `Object`, `String`, `StringBuilder`, `Throwable`, `System`, `Long`, `Intrinsics` | `pkg/runtime` |
| CLI | `grind` and `brew` work for Phase 4 classes; `serve` is a stub | `cmd/barista` |
| End-to-end fixtures | Calculator, Counter, Greeter, TryCatch, Switch, ReferenceOps, Greeting | `tests/integration_test.go` |
| Kotlin integration | Conditional on `KOTLINC` or `kotlinc` on `PATH`; verified locally with Kotlin 2.1.10 | `TestIntegrationKotlinGreeting` |

---

## Phase Checklist

### Phase 1 — Primitive Calculator: Complete
- [x] Bounds-checked big-endian class-file reader
- [x] `0xCAFEBABE` validation and supported-version checks
- [x] Full constant-pool parsing
- [x] Long/Double double-slot accounting
- [x] Class metadata, interfaces, fields, and methods
- [x] `Code` attributes with `max_stack`, `max_locals`, bytecode, and exception tables
- [x] Abstract operand-stack evaluation into SSA-style IR
- [x] Integer loads, stores, constants, arithmetic, negation, and `iinc`
- [x] Conditional branches, labels, `goto`, and backward jumps for loops
- [x] Go AST generation through `go/ast` and `go/format`
- [x] Branch-safe codegen for methods containing labels and `goto`

### Phase 2 — Objects & Control Flow: Complete
- [x] `new` and `dup`
- [x] `getfield` and `putfield`
- [x] `getstatic` and `putstatic`
- [x] `invokespecial` and constructors
- [x] `invokevirtual` and `invokestatic`
- [x] Generated class model with pointer-receiver methods
- [x] JVM descriptor parsing for parameters, return types, and local slots
- [x] Primitive and reference type mapping
- [x] Explicit basic-block partitioning and CFG data structures
- [x] `<clinit>` lowered to Go `init` function

### Phase 3 — Runtime & Exceptions: Complete
- [x] Exception-table parsing and lowering into `MethodIR.ExceptionTraps`
- [x] `athrow` lowered to Go `panic`
- [x] Protected ranges wrapped in recover closures with `handlerPC` dispatch
- [x] Catch-type matching for standard `java.lang` exception hierarchy
- [x] `java.lang.String` and `StringBuilder` runtime shims
- [x] `System.out` / `System.err` `PrintStream` shim and `System.currentTimeMillis`
- [x] `newarray`, `anewarray`, `arraylength`, `iaload`, `iastore`, `aaload`, `aastore`

### Phase 4 — Kotlin Engine: Complete
- [x] `tableswitch` and `lookupswitch` lowering and CFG construction
- [x] Operand stack merges (PHI nodes) at branch and switch targets
- [x] `long` primitive operations (`lconst`, `lload`, `lstore`, `ladd`, `lsub`, `lmul`, `ldiv`, `lneg`, `lcmp`, `i2l`, `l2i`, `laload`, `lastore`)
- [x] Bitwise operators (`iand`, `swap`)
- [x] Object equality and reference comparisons (`if_acmpeq`, `if_acmpne`, `ifnull`, `ifnonnull`)
- [x] `checkcast` and `instanceof`
- [x] Dynamic `java.lang.Object` virtual dispatch (`equals`, `hashCode`, `toString`)
- [x] `kotlin.jvm.internal.Intrinsics` (`checkNotNullParameter`, `checkNotNull`, `areEqual`, `stringPlus`)
- [x] `kotlin.io.ConsoleKt` (`println`, `print`)
- [x] Kotlin data classes and default-parameter masks (`DefaultConstructorMarker` is mapped to `any`; full constructor bridges are deferred)
- [x] Kotlin integration fixture (`Greeting.kt`)

### Phase 5 — Networking & I/O: Planned
- [ ] Intercept socket and HTTP entry points
- [ ] Map request handling onto `net/http`
- [ ] REST fixture

### Phase 6 — Production Tooling: Planned
- [ ] `barista serve`
- [ ] Invoke `go build -ldflags="-s -w"`
- [ ] Static linking and cross-compilation workflow
- [ ] Release packaging

---

## Deferred for Later Phases

The Phase 4 core is complete, but these advanced cases remain out of scope and should not be assumed to work:

- Cross-class dispatch, inheritance, interfaces, Kotlin stdlib classes outside the runtime shims, and `.jar` input.
- Kotlin `object` singletons, `Companion`, enums, property delegates, and full `DefaultConstructorMarker` constructor semantics.
- Coroutine continuation state machines and the goroutine/channel bridge.
- Casts and `instanceof` against generated inheritance or interfaces; current helpers check concrete Go types.
- `float`, `double`, `byte`, `char`, and `short` arithmetic, narrowing, array opcodes, and returns.
- User-defined exception classes, `finally`, nested/overlapping traps, and returns or branches across protected ranges.
- `wide`, `invokeinterface`, `invokedynamic`, `multianewarray`, and legacy `jsr`/`ret`.

`docs/phase4-in-progress.md` keeps the longer list of deferred Kotlin engine work.

---

## Verification

```sh
go build ./...
go vet ./...
go test ./...
go test -race ./...
KOTLINC=/path/to/kotlinc go test ./tests -run TestIntegrationKotlinGreeting -count=1 -v
```

All commands pass on 2026-10-23. The Kotlin suite was verified locally with Kotlin 2.1.10; it skips when no compiler is available.

---

## Related Records

- [AI agent conventions](conventions.md)
- [Phase 4 follow-up notes](phase4-in-progress.md)
- [Case study index](case-studies/README.md)
- [Operand-stack merge/PHI case study](case-studies/2026-10-23-jvm-stack-merge-phi-lowering.md)
- [JVM switch payload alignment case study](case-studies/2026-10-23-jvm-switch-payload-alignment.md)
- [Kotlin default-parameter mask case study](case-studies/2026-10-23-kotlin-default-parameter-masks.md)
