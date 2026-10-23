# Development Progress

This is the canonical tracker for Barista's implementation progress. Update it at the end of every meaningful change so a new contributor can understand what works, what does not, and what comes next without reading the entire commit history.

**Last updated:** 2026-10-23
**Current phase:** Phase 2 complete; Phase 3 next
**Latest implementation commit:** Phase 2 implementation commit (see `git log --oneline`)

---

## Status at a Glance

| Area | Status | Evidence |
| --- | --- | --- |
| Build | Passing | `go build ./...` |
| Vet | Passing | `go vet ./...` |
| Tests | Passing | `go test ./...` |
| Race detector | Passing | `go test -race ./...` |
| Class-file parsing | Complete for Phase 2 | `internal/classfile` |
| IR lowering | Complete for Phase 2 | `internal/ir` |
| Control flow graph | Basic blocks, edges, predecessors, and successors | `internal/ir/block.go` |
| Go code generation | Objects, fields, dispatch, CFG-safe branching | `internal/codegen` |
| CLI | `grind` and `brew` work for Phase 2 classes; `serve` is a stub | `cmd/barista` |
| End-to-end fixtures | Calculator and Counter compile, generate Go, compile, and run | `tests/integration_test.go` |

The integration suite now has two end-to-end fixtures:

- `Calculator.java` proves static `int` arithmetic, conditionals, and loops.
- `Counter.java` proves constructors, instance and static fields, `new`/`dup`, `invokespecial`, `invokevirtual`, `invokestatic`, `<clinit>`, static initialization, and generated pointer-receiver methods.

The generated Counter module is compiled and executed with assertions for field reads/writes, increments, constructors, static state, and dispatches.

---

## Phase Checklist

### Phase 1 — Primitive Calculator: Complete

- [x] Bounds-checked big-endian class-file reader
- [x] `0xCAFEBABE` validation and supported-version checks
- [x] Full constant-pool parsing, including `Dynamic`, `InvokeDynamic`, `Module`, and `Package`
- [x] Long/Double double-slot accounting
- [x] Class metadata, interfaces, fields, and methods
- [x] `Code` attributes with `max_stack`, `max_locals`, bytecode, and exception tables
- [x] `LineNumberTable` parsing
- [x] Abstract operand-stack evaluation into SSA-style IR
- [x] Integer loads, stores, constants, arithmetic, negation, and `iinc`
- [x] Conditional branches, labels, `goto`, and backward jumps for loops
- [x] `ldc` / `ldc_w` integer constants
- [x] Go AST generation through `go/ast` and `go/format`
- [x] Branch-safe codegen for methods containing labels and `goto`
- [x] `barista grind` IR/disassembly output
- [x] `barista brew` Go source output, including `-o`
- [x] Decoder, IR, and end-to-end integration tests

### Phase 2 — Objects & Control Flow: Complete

- [x] `new` and `dup`
- [x] `getfield` and `putfield`
- [x] `getstatic` and `putstatic`
- [x] `invokespecial` and constructors
- [x] `invokevirtual` and `invokestatic`
- [x] Generated class model with pointer-receiver methods
- [x] Go-native allocation through pointers (`&ClassName{}`)
- [x] JVM descriptor parsing for parameters, return types, and local slots
- [x] Primitive and reference type mapping (`I` → `int32`, `J` → `int64`, `L…;` → pointers, arrays → slices)
- [x] Explicit basic-block partitioning and CFG data structures
- [x] CFG predecessor/successor edges exposed by `barista grind`
- [x] `<clinit>` lowered to a Go `init` function
- [x] `<init>` lowered to a Go `Init` method
- [x] Static fields lowered to package-level Go variables
- [x] Object-oriented integration fixture
- [ ] PHI/stack-merge support at branch targets (deferred; branch targets must still have an empty operand stack)

### Phase 3 — Runtime & Exceptions: Next

- [ ] Exception-table lowering to `panic` / `recover`
- [ ] `java.lang.String` and `StringBuilder` runtime shims
- [ ] `newarray`, `anewarray`, and array opcodes
- [ ] Runtime fixture for strings, arrays, and caught exceptions
- [ ] Replace the generated `JavaLangString` type placeholder with the real runtime wrapper

### Phase 4 — Kotlin Engine: Planned

- [ ] `tableswitch` and `lookupswitch`
- [ ] `kotlin.jvm.internal.Intrinsics` shims
- [ ] Data classes, object singletons, and default-parameter wrappers
- [ ] Coroutine state-machine support and the goroutine/channel optimisation
- [ ] Kotlin integration fixture

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

## Current Capabilities

- Parse `.class` files from a stream or path.
- Resolve symbolic class names, strings, field references, method references, and name/type pairs.
- Extract method bytecode and line-number metadata.
- Parse JVM method descriptors, including primitive, reference, and array parameters, return types, and local-slot widths.
- Lower integer loads/stores, constants, arithmetic, negation, `iinc`, conditionals, `goto`, and `goto_w`.
- Lower `new`, `dup`, `getfield`, `putfield`, `getstatic`, `putstatic`, `invokevirtual`, `invokespecial`, and `invokestatic`.
- Split methods into basic blocks with bytecode ranges, instructions, predecessors, successors, and an entry block.
- Lower `<init>` and `<clinit>` into generated constructor and package `init` functions.
- Generate Go structs for instance fields and package-level variables for static fields.
- Generate pointer-receiver methods for instance methods and package functions for static methods.
- Resolve same-class method overloads by constant-pool descriptor.
- Emit formatted Go to stdout or a file.
- Print method IR and CFG edges with `barista grind`.

---

## Known Limitations

These are intentionally out of scope for Phase 2, but should be visible to avoid false expectations:

- `.jar` archive input is not implemented; the CLI accepts a single `.class` file.
- Only one class is compiled at a time. Cross-class field access, static calls, virtual calls, inheritance, and interface dispatch are rejected with an explicit diagnostic. The only external call currently handled is `java/lang/Object.<init>`, which is a no-op after Go allocation.
- Only nominal `int` lowering is implemented. `long`, `float`, `double`, `boolean`, `byte`, `char`, and `short` appear in type mapping and descriptors, but their arithmetic/load opcodes are not lowered.
- Branch targets must have an empty operand stack. There are no PHI nodes or stack-merge values yet.
- No arrays, strings, exceptions, or runtime shims are lowered yet. A minimal `JavaLangString` struct is emitted only when a field or method signature references `Ljava/lang/String;`, so such code can compile, but no String operations are supported.
- No `tableswitch`, `lookupswitch`, `wide`, `invokeinterface`, or `invokedynamic`.
- No `aconst_null`, `checkcast`, `instanceof`, or array opcodes.
- Generated methods are still linear SSA with labels and `goto`; the CFG is currently an analysis artifact and is not used for structured control-flow reconstruction.
- `brew` emits source only; it does not invoke `go build` or produce a linked binary yet.
- `serve` is not implemented.
- Kotlin is parsed as JVM bytecode but Kotlin runtime behaviour is not implemented.

---

## Verification

Run the full verification suite before updating status:

```sh
go build ./...
go vet ./...
go test ./...
go test -race ./...
```

Current result: all pass on 2026-10-23.

Manual CLI check:

```sh
go build -o bin/barista ./cmd/barista
javac -d tests/fixtures/out tests/fixtures/java/Calculator.java
javac -d tests/fixtures/out tests/fixtures/java/Counter.java
./bin/barista grind tests/fixtures/out/Counter.class
./bin/barista brew -o out/counter.go tests/fixtures/out/Counter.class
```

`grind` prints the IR, a CFG block count, and each block's bytecode range and successors. `brew` emits a Go source file containing the class struct, static fields, constructor, instance methods, and static methods.

---

## Next Task

Implement Phase 3 starting with the smallest end-to-end slice:

1. Add `newarray` / `anewarray` and the integer array load/store opcodes.
2. Introduce the `pkg/runtime/java/lang` String wrapper and replace the generated placeholder.
3. Lower exception tables to `panic` / `recover` with a fixture that catches and rethrows.
4. Add an integration fixture that uses an array, a String, and a caught exception.
5. Update this file and add a case study for any non-obvious issue encountered.

---

## Progress Log

| Date | Change | Reference |
| --- | --- | --- |
| 2026-10-23 | Initial repository scaffold, README, Git, Textify | `493b00a` |
| 2026-10-23 | Added the Java/Kotlin-to-Go deployment pitch | `ab5faa9` |
| 2026-10-23 | Established AI-agent case-study conventions | `af8801a` |
| 2026-10-23 | Implemented Phase 1 classfile parsing, IR lowering, codegen, CLI, and tests | `10fbff6` |
| 2026-10-23 | Added the canonical development progress tracker | `841af41` |
| 2026-10-23 | Implemented Phase 2 objects, fields, dispatch, descriptors, and CFG | Phase 2 implementation commit |

## Related Records

- [AI agent conventions](conventions.md)
- [Case study index](case-studies/README.md)
- [JVM branch offset case study](case-studies/2026-10-23-jvm-branch-offset-origin.md)
- [Go goto/declaration case study](case-studies/2026-10-23-go-goto-over-declarations.md)
- [Go untyped integer constant case study](case-studies/2026-10-23-go-untyped-integer-constants.md)

---

## Maintenance Rule

Update this file whenever a task changes what the compiler can do, what it cannot do, or how it is verified. A phase item may only be checked off when there is a test or a reproducible command that proves it. Always update the current phase, known limitations, verification result, next task, and latest implementation commit.
