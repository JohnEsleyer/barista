# Development Progress

This is the canonical tracker for Barista's implementation progress. Update it at the end of every meaningful change so a new contributor can understand what works, what does not, and what comes next without reading the entire commit history.

**Last updated:** 2026-10-23
**Current phase:** Phase 1 complete; Phase 2 next
**Latest implementation commit:** `10fbff6` — Implement Phase 1 classfile parser, IR lowering, and Go codegen

---

## Status at a Glance

| Area | Status | Evidence |
| --- | --- | --- |
| Build | Passing | `go build ./...` |
| Vet | Passing | `go vet ./...` |
| Tests | Passing | `go test ./...` |
| Race detector | Passing | `go test -race ./...` |
| Class-file parsing | Complete for Phase 1 | `internal/classfile` |
| IR lowering | Complete for Phase 1 | `internal/ir` |
| Go code generation | Complete for Phase 1 | `internal/codegen` |
| CLI | `grind` and `brew` work; `serve` is a stub | `cmd/barista` |
| End-to-end fixture | Compiles, generates Go, compiles and runs it | `tests/integration_test.go` |

The integration test compiles `tests/fixtures/java/Calculator.java`, parses the class, lowers all static methods, emits Go, and then compiles and runs the generated module. It asserts arithmetic, a loop, and both branches:

- `Add(2, 3) == 5`
- `SumTo(10) == 55`
- `Absolute(-7) == 7`
- `Max(3, 9) == 9`

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

### Phase 2 — Objects & Control Flow: Next

- [ ] `new` and `dup`
- [ ] `getfield` and `putfield`
- [ ] `getstatic` and `putstatic`
- [ ] `invokespecial` and constructors
- [ ] Generated class model with pointer-receiver methods
- [ ] Go-native allocation through pointers (`&ClassName{}`)
- [ ] Explicit basic-block partitioning and CFG data structures
- [ ] PHI/stack-merge support at branch targets
- [ ] Object-oriented integration fixture

### Phase 3 — Runtime & Exceptions: Planned

- [ ] Exception-table lowering to `panic` / `recover`
- [ ] `java.lang.String` and `StringBuilder` shims
- [ ] `newarray`, `anewarray`, and array opcodes
- [ ] Runtime fixture for strings, arrays, and caught exceptions

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
- Resolve symbolic class names, strings, member references, and name/type pairs.
- Extract method bytecode and line-number metadata.
- Lower `int`-based static methods into linear SSA-style instructions.
- Lower `ifeq`, `ifne`, `iflt`, `ifge`, `ifgt`, `ifle`, the six `if_icmp*` comparisons, `goto`, and `goto_w`.
- Generate valid Go for static methods using `int32`.
- Skip instance methods and initializers with a clear diagnostic in `grind` and silently in `brew`.
- Emit formatted Go to stdout or a file.

---

## Known Limitations

These are intentionally out of scope for Phase 1, but should be visible to avoid false expectations:

- `.jar` archive input is not implemented; the CLI accepts a single `.class` file.
- Only static methods are compiled. Instance methods, constructors, and `<clinit>` are skipped.
- Only `int` parameters and `int`/`void` return types are supported by codegen.
- No object model, fields, arrays, strings, exceptions, or runtime shims yet.
- No `long`, `float`, `double`, `boolean`, `byte`, `char`, or `short` lowering.
- No `tableswitch`, `lookupswitch`, `wide`, `invokedynamic`, or method invocation.
- Branch targets must have an empty operand stack. There are no PHI nodes or stack-merge values yet.
- Generated methods are package-level Go functions; class namespacing and overload resolution are minimal.
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
./bin/barista grind tests/fixtures/out/Calculator.class
./bin/barista brew -o out/calculator.go tests/fixtures/out/Calculator.class
```

---

## Next Task

Implement the Phase 2 object model, starting with the smallest end-to-end slice:

1. Parse and lower `new`, `dup`, `getfield`, `putfield`, and `invokespecial`.
2. Generate a Go struct for the class and pointer-receiver methods.
3. Support a constructor and instance-field fixture end to end.
4. Add unit tests for the new opcodes and an integration test that compiles and runs the generated object code.
5. Update this file and add a case study for any non-obvious issue encountered.

---

## Progress Log

| Date | Change | Reference |
| --- | --- | --- |
| 2026-10-23 | Initial repository scaffold, README, Git, Textify | `493b00a` |
| 2026-10-23 | Added the Java/Kotlin-to-Go deployment pitch | `ab5faa9` |
| 2026-10-23 | Established AI-agent case-study conventions | `af8801a` |
| 2026-10-23 | Implemented Phase 1 classfile parsing, IR lowering, codegen, CLI, and tests | `10fbff6` |

## Related Records

- [AI agent conventions](conventions.md)
- [Case study index](case-studies/README.md)
- [JVM branch offset case study](case-studies/2026-10-23-jvm-branch-offset-origin.md)
- [Go goto/declaration case study](case-studies/2026-10-23-go-goto-over-declarations.md)

---

## Maintenance Rule

Update this file whenever a task changes what the compiler can do, what it cannot do, or how it is verified. A phase item may only be checked off when there is a test or a reproducible command that proves it. Always update the current phase, known limitations, verification result, next task, and latest implementation commit.
