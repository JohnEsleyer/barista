# Development Progress

This is the canonical tracker for Barista's implementation progress. Update it at the end of every meaningful change so a new contributor can understand what works, what does not, and what comes next without reading the entire commit history.

**Last updated:** 2026-10-23
**Current phase:** Phase 3 complete; Phase 4 next
**Latest implementation commit:** `d12ee43` — Implement Phase 3 runtime, strings, arrays, and exceptions

---

## Status at a Glance

| Area | Status | Evidence |
| --- | --- | --- |
| Build | Passing | `go build ./...` |
| Vet | Passing | `go vet ./...` |
| Tests | Passing | `go test ./...` |
| Race detector | Passing | `go test -race ./...` |
| Class-file parsing | Complete for Phase 3 | `internal/classfile` |
| IR lowering | Complete for Phase 3 | `internal/ir` |
| Control flow graph | Basic blocks, edges, predecessors, and successors; exception traps are separate metadata | `internal/ir/block.go` |
| Go code generation | Objects, fields, dispatch, strings, arrays, exception dispatch, CFG-safe branching | `internal/codegen` |
| Runtime shims | `Object`, `String`, `StringBuilder`, `Throwable`/`Exception`, `System`, arrays | `pkg/runtime/java/lang` |
| CLI | `grind` and `brew` work for Phase 3 classes; `serve` is a stub | `cmd/barista` |
| End-to-end fixtures | Calculator, Counter, Greeter, and TryCatch compile, generate Go, compile, and run | `tests/integration_test.go` |

The integration suite now has four end-to-end fixtures:

- `Calculator.java` proves static `int` arithmetic, conditionals, and loops.
- `Counter.java` proves constructors, instance and static fields, `new`/`dup`, `invokespecial`, `invokevirtual`, `invokestatic`, `<clinit>`, static initialization, and generated pointer-receiver methods.
- `Greeter.java` proves string constants and methods, `StringBuilder`, `System.out.println`, class literals, `newarray`, `anewarray`, `arraylength`, and array load/store.
- `TryCatch.java` proves exception-table lowering, `athrow`, Go `panic`/`recover`, catch-type matching, handler dispatch, and `Throwable.getMessage`.

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

### Phase 3 — Runtime & Exceptions: Complete

- [x] Exception-table parsing and lowering into `MethodIR.ExceptionTraps`
- [x] `athrow` lowered to Go `panic`
- [x] Protected ranges wrapped in recover closures with `handlerPC` dispatch
- [x] Catch-type matching for the standard `java.lang` exception hierarchy
- [x] `java.lang.String` and `StringBuilder` runtime shims
- [x] `java.lang.Object`, `Throwable`, `Exception`, `RuntimeException`, `NullPointerException`, and `IndexOutOfBoundsException` shims
- [x] `System.out` / `System.err` `PrintStream` shim and `System.currentTimeMillis`
- [x] `newarray`, `anewarray`, `arraylength`, `iaload`, `iastore`, `aaload`, and `aastore`
- [x] `ldc` / `ldc_w` support for String, integer, and class constants
- [x] `aconst_null`, `pop`, and bounds-checked array helpers
- [x] Replace the generated `JavaLangString` placeholder with `*lang.String`
- [x] Runtime-backed `Greeter.java` and `TryCatch.java` fixtures
- [ ] User-defined exception classes and `finally` / overlapping / nested trap lowering (deferred)
- [ ] Returns inside a protected range or branches out of a protected range (deferred)

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
- Lower `ldc` / `ldc_w` integer, String, and class constants.
- Lower `newarray`, `anewarray`, `arraylength`, `iaload`, `iastore`, `aaload`, and `aastore` to Go slices and bounds-checked runtime helpers.
- Lower `athrow` and JVM exception tables into Go `panic` / `recover` dispatch for standard runtime exceptions.
- Resolve `java/lang/String`, `StringBuilder`, `Object`, `Throwable`, `Exception`, `RuntimeException`, `NullPointerException`, `IndexOutOfBoundsException`, `Class`, and `java/io/PrintStream` to native Go shims.
- Split methods into basic blocks with bytecode ranges, instructions, predecessors, successors, and an entry block.
- Lower `<init>` and `<clinit>` into generated constructor and package `init` functions.
- Generate Go structs for instance fields and package-level variables for static fields.
- Generate pointer-receiver methods for instance methods and package functions for static methods.
- Emit formatted Go to stdout or a file.
- Print method IR, exception traps, and CFG edges with `barista grind`.

---

## Known Limitations

These are intentionally out of scope for Phase 3, but should be visible to avoid false expectations:

- `.jar` archive input is not implemented; the CLI accepts a single `.class` file.
- Only one class is compiled at a time. Cross-class field access, static calls, virtual calls, inheritance, and interface dispatch are rejected with an explicit diagnostic. Runtime class calls are supported only for the native shims listed above.
- Only nominal `int` arithmetic is lowered. `long`, `float`, `double`, `byte`, `char`, and `short` appear in type mapping and descriptors, but their arithmetic and array opcodes are not lowered. `boolean` parameters and fields are supported through `iload`/`istore` type inference.
- Branch targets must have an empty operand stack. There are no PHI nodes or stack-merge values yet.
- Exception support covers the standard runtime hierarchy and simple protected ranges. User-defined exception classes, `finally`, overlapping or nested trap ranges, returns inside a protected range, and branches out of a protected range are not supported.
- Go panics that are not Barista runtime exceptions (for example a nil generated pointer) are re-panicked rather than matched to a JVM catch type.
- String and StringBuilder support is intentionally small: length, equality, substring, concat, char access, append, and toString. Other JDK methods are rejected.
- `aconst_null` is represented as Go `any`; it works for runtime calls and `any` contexts but is not yet context-typed for assignment to a specific reference field or array.
- No `tableswitch`, `lookupswitch`, `wide`, `invokeinterface`, `invokedynamic`, `checkcast`, or `instanceof`.
- Go identifier collisions after export casing (for example JVM fields `foo` and `Foo`) are not resolved; such a class will produce invalid Go rather than a diagnostic.
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
javac -d tests/fixtures/out tests/fixtures/java/Greeter.java
javac -d tests/fixtures/out tests/fixtures/java/TryCatch.java
./bin/barista grind tests/fixtures/out/Counter.class
./bin/barista grind tests/fixtures/out/TryCatch.class
./bin/barista brew -o out/counter.go tests/fixtures/out/Counter.class
./bin/barista brew -o out/greeter.go tests/fixtures/out/Greeter.class
```

`grind` prints the IR, exception traps, a CFG block count, and each block's bytecode range and successors. `brew` emits a Go source file containing the class struct, static fields, constructor, instance methods, static methods, runtime shim calls, array operations, and exception dispatch. Generated runtime-backed modules must be built in a module that can resolve `github.com/JohnEsleyer/barista/pkg/runtime/java/lang`; the integration suite supplies a `require`/`replace` pair automatically.

---

## Next Task

Begin Phase 4 with the smallest Kotlin-capable end-to-end slice:

1. Add `tableswitch` and `lookupswitch` lowering to labels and a Kotlin or Java fixture.
2. Add `kotlin.jvm.internal.Intrinsics` null-check and assertion runtime shims.
3. Add cross-class dispatch or a small generated-stdlib bridge so Kotlin fixture calls can resolve beyond the runtime package.
4. Record any new non-obvious findings as case studies and update this tracker.

---

## Progress Log

| Date | Change | Reference |
| --- | --- | --- |
| 2026-10-23 | Initial repository scaffold, README, Git, Textify | `493b00a` |
| 2026-10-23 | Added the Java/Kotlin-to-Go deployment pitch | `ab5faa9` |
| 2026-10-23 | Established AI-agent case-study conventions | `af8801a` |
| 2026-10-23 | Implemented Phase 1 classfile parsing, IR lowering, codegen, CLI, and tests | `10fbff6` |
| 2026-10-23 | Added the canonical development progress tracker | `841af41` |
| 2026-10-23 | Implemented Phase 2 objects, fields, dispatch, descriptors, and CFG | `e17b3cd` |
| 2026-10-23 | Implemented Phase 3 runtime shims, strings, arrays, exception dispatch, and `ldc` constants | `d12ee43` |

## Related Records

- [AI agent conventions](conventions.md)
- [Case study index](case-studies/README.md)
- [Go recover/handler dispatch case study](case-studies/2026-10-23-go-recover-handler-dispatch.md)
- [JVM boolean iload case study](case-studies/2026-10-23-jvm-boolean-iload.md)
- [JVM branch offset case study](case-studies/2026-10-23-jvm-branch-offset-origin.md)
- [Go goto/declaration case study](case-studies/2026-10-23-go-goto-over-declarations.md)
- [Go untyped integer constant case study](case-studies/2026-10-23-go-untyped-integer-constants.md)

---

## Maintenance Rule

Update this file whenever a task changes what the compiler can do, what it cannot do, or how it is verified. A phase item may only be checked off when there is a test or a reproducible command that proves it. Always update the current phase, known limitations, verification result, next task, and latest implementation commit.
