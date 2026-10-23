<p align="center">
  <picture>
    <img src="assets/logo.svg" alt="Barista Logo" width="160" height="160">
  </picture>
</p>

# Barista

> **Love writing Java or Kotlin? Deploy like Go.**
>
> If you love writing Java or Kotlin, but you want to deploy apps that feel like Go programs — **sub-second builds, zero-effort cross-compilation, tiny 15MB RAM footprints, and smooth operation on $4/month VPS boxes** — Barista is built for you.

**Barista** is an ahead-of-time (AOT) static de-stacking compiler for JVM bytecode, written in Go. It ingests standard `.class` files produced by `javac` or `kotlinc`, reconstructs their stack-based execution model as a register/variable-based IR, and emits idiomatic Go source that is compiled into a single, statically linked binary. `.jar` archive input is planned.

You keep the language and toolchain you already love. You leave behind the JVM's operational baggage: no 200MB base runtime, no JIT warm-up spikes, no JRE packaging, and no guessing whether your app will fit on a $4/month box. Write Java or Kotlin; ship one native binary.

> **Status:** Phase 4 core is implemented. Barista can parse class files, lower static and instance methods (including branches, loops, PHI joins, objects, fields, constructors, same-class dispatch, strings, arrays, caught exceptions, switches, Kotlin `Long` values, casts, dynamic `Object` methods, and Kotlin intrinsics) into de-stacked IR, build a control flow graph, and emit runnable Go source. Kotlin `object` singletons, coroutines, and cross-class dispatch are deferred; see the [roadmap](#roadmap).
>
> **Detailed progress:** [`docs/progress.md`](docs/progress.md) is the canonical tracker for completed capabilities, known limitations, test status, and the next task.

---

## Why Barista?

You should not have to choose between JVM ergonomics and Go-like operations. Barista closes the gap:

| | JVM deployment | Barista deployment |
| --- | --- | --- |
| **Build experience** | JVM build tooling plus JRE packaging | Go toolchain with sub-second rebuilds |
| **Base runtime overhead** | OpenJDK reserves roughly 100–250MB RSS at boot | One static binary with no VM |
| **JIT contention** | Compilation spikes compete for a single vCPU | No JIT; code is compiled ahead of time |
| **Delayed reclamation** | G1/ZGC assume memory is plentiful and shrink lazily | Delegate allocation and GC to the Go runtime |
| **Memory footprint** | 100MB+ before your application does any work | Target around 15MB RSS |
| **Deployment footprint** | JRE + application JAR | One self-contained executable |
| **Startup latency** | Hundreds of milliseconds to seconds | Target sub-5ms startup |
| **Cross-compilation** | Platform-specific JREs | Inherited from the Go toolchain |
| **Hosting cost** | Needs headroom for the runtime itself | Smooth operation on $4/month VPS boxes |

---

## How It Works

Barista is a non-reversing, five-stage pipeline:

```text
Input: application .jar / .class
              │
              ▼
┌──────────────────────────────────────────────────────────────┐
│ 1. Class File Deserialization & Constant Pool Resolution     │
│    Validate 0xCAFEBABE, parse the constant pool, extract     │
│    class metadata, fields, interfaces, methods and Code      │
│    attributes.                                               │
└──────────────────────────────┬───────────────────────────────┘
                               ▼
┌──────────────────────────────────────────────────────────────┐
│ 2. Bytecode Analysis & CFG Construction                      │
│    Map byte offsets to opcodes, split methods into basic     │
│    blocks, and build a directed control flow graph.          │
└──────────────────────────────┬───────────────────────────────┘
                               ▼
┌──────────────────────────────────────────────────────────────┐
│ 3. Symbolic Stack De-stacking & SSA Generation               │
│    Abstractly evaluate the operand stack, materialise        │
│    explicit variables, and infer concrete Go types.          │
└──────────────────────────────┬───────────────────────────────┘
                               ▼
┌──────────────────────────────────────────────────────────────┐
│ 4. AST Generation & Go Code Emission                         │
│    Map classes to structs and methods to pointer-receiver    │
│    functions; inject native runtime shims; format with       │
│    go/ast and go/printer.                                    │
└──────────────────────────────┬───────────────────────────────┘
                               ▼
┌──────────────────────────────────────────────────────────────┐
│ 5. Toolchain Invocation & Static Linking                     │
│    Invoke go build with dead-code elimination and stripped   │
│    symbols to produce a single static executable.            │
└──────────────────────────────────────────────────────────────┘
```

### The de-stacking transformation

The JVM pushes and pops values on an operand stack; Go uses named variables. Barista emulates the JVM stack at compile time and allocates SSA temporaries:

```text
JVM bytecode                     Barista IR
------------                     ----------
iload_1                          // push local 1
iload_2                          // push local 2
iadd                             t0 = local_1 + local_2
bipush 10                        // push constant 10
imul                             t1 = t0 * 10
istore_3                         local_3 = t1
```

### Type mapping

| JVM type | Go type |
| --- | --- |
| `boolean` | `bool` |
| `byte` | `int8` |
| `char` | `uint16` |
| `short` | `int16` |
| `int` | `int32` |
| `long` | `int64` |
| `float` | `float32` |
| `double` | `float64` |
| reference types | pointers to generated structs or `lang.*` runtime shims |
| `java.lang.Object` / `null` | `any` |
| arrays | Go slices (`[]int32`, `[]int64`, `[]*lang.String`, …) |

JVMS operands `boolean`, `byte`, `char`, `short`, and `int` occupy one stack slot. Barista preserves the JVM's value semantics at the IR level and narrows to the declared Go type where required. `long` occupies two JVM slots but maps to one Go `int64`; local-slot accounting and `lload`/`lstore` lowering preserve that difference.

### Object model

Each JVM class becomes a Go struct, and instance methods use pointer receivers:

```go
// com/example/User -> ComExampleUser
type ComExampleUser struct {
    Id   int64
    Name *lang.String
}

func (this *ComExampleUser) GetId() int64 {
    return this.Id
}
```

Instances are always heap-allocated through pointers (`&ComExampleUser{}`), so object lifetime is managed entirely by the Go garbage collector — no custom allocator or tracing runtime is required.

### Runtime shims

The generated program does not ship `rt.jar`. Critical-path JDK and Kotlin APIs are reimplemented in Go under `pkg/runtime`:

- `java.lang.Object` — identity hash codes, default equality, and dynamic `equals`/`hashCode`/`toString` helpers; `java.lang.Object` parameters map to Go `any`.
- `java.lang.String` — immutable wrapper over Go strings with length, equality, substring, concat, character access, and Java-compatible `hashCode`.
- `java.lang.StringBuilder` — mutable string builder backed by `strings.Builder`, including `append(char)` for Kotlin interpolation.
- `java.lang.Long` — boxed `Long`, static `hashCode(J)`, value equality, and string rendering.
- `java.lang.System` — `System.out` and `System.err` mapped to `os.Stdout` and `os.Stderr`, plus `currentTimeMillis`.
- `java.lang.Throwable` / `Exception` / `RuntimeException` — exception values with message, cause, and string rendering.
- `java.lang.NullPointerException` / `IndexOutOfBoundsException` / `ClassCastException` — typed runtime exceptions that generated catch handlers can match.
- `java.lang.Class` — class-literal metadata for `ldc` class constants.
- `java.util.ArrayList` — slice-backed list implementation (planned).
- `kotlin.jvm.internal.Intrinsics` — null checks, `AreEqual`, lateinit/property assertion helpers, and parameter validation emitted by `kotlinc`.
- Cast helpers — `CheckCast[T]`, `IsInstanceOf[T]`, and `CompareLong` for `checkcast`, `instanceof`, and `lcmp`.

---

## Repository Layout

```text
barista/
├── assets/
│   └── logo.svg                    # Vector brand logo
├── cmd/
│   └── barista/
│       └── main.go                 # CLI entrypoint: grind, brew, serve
├── internal/
│   ├── classfile/                  # Binary class-file decoding
│   │   ├── class.go                # Class headers, members, and attributes
│   │   ├── constant_pool.go        # All JVMS constant-pool tags and resolvers
│   │   ├── reader.go               # Bounds-checked big-endian reader
│   │   ├── attributes.go           # Code, exception, and line-number attributes
│   │   └── class_test.go           # Decoder unit tests
│   ├── ir/                         # Intermediate representation
│   │   ├── instruction.go          # Normalised SSA-style instructions
│   │   ├── block.go                # Basic blocks and control flow graph
│   │   ├── descriptor.go           # JVM method descriptor parsing
│   │   ├── stack_eval.go           # Operand-stack evaluator and branch lowering
│   │   └── stack_eval_test.go      # IR and CFG unit tests
│   ├── codegen/                    # Go transpilation
│   │   ├── generator.go            # Top-level IR to Go AST generation
│   │   ├── function.go             # Method bodies, exception dispatch, runtime calls
│   │   ├── runtime.go              # Runtime class/method mapping and import detection
│   │   └── type_map.go             # JVM descriptors to Go types and names
│   └── builder/                    # Toolchain execution (Phase 6)
├── pkg/
│   └── runtime/                    # Embedded shims baked into generated output
│       ├── java/
│       │   ├── lang/
│       │   └── util/
│       └── kotlin/
│           └── jvm/
├── docs/
│   ├── conventions.md              # AI-agent conventions and case-study rule
│   ├── progress.md                 # Canonical development progress tracker
│   └── case-studies/               # Recorded non-obvious problems and fixes
├── tests/
│   ├── fixtures/                   # Java and Kotlin sources for verification
│   │   ├── java/
│   │   └── kotlin/
│   └── integration_test.go         # End-to-end compile-and-run assertions
├── go.mod
└── README.md
```

---

## Getting Started

### Prerequisites

- Go 1.24 or newer
- A JDK (`javac`) and, optionally, `kotlinc` to build test fixtures
- Git

### Build the CLI

```sh
go build -o bin/barista ./cmd/barista
```

### Run the test suite

```sh
go test ./...
```

### Compile a fixture

```sh
javac -d tests/fixtures/out tests/fixtures/java/Calculator.java
```

---

## CLI

The CLI exposes three commands, following the coffee theme:

| Command | Purpose | Status |
| --- | --- | --- |
| `barista grind <input.class>` | Disassemble a class file into a readable IR dump | implemented |
| `barista brew [-o output.go] <input.class>` | Run the Phase 4 pipeline and emit Go source | implemented |
| `barista serve <input>` | Compile in memory and execute immediately | planned |

```sh
./bin/barista --help
./bin/barista version

javac -d tests/fixtures/out tests/fixtures/java/Calculator.java
./bin/barista grind tests/fixtures/out/Calculator.class
./bin/barista brew -o out/calculator.go tests/fixtures/out/Calculator.class
```

Phase 4 covers static and instance methods using `int`/`long` arithmetic, conditionals, loops, operand-stack PHI joins, objects, instance and static fields, constructors, same-class dispatch, strings, arrays, `System.out`, caught runtime exceptions, dense/sparse switches, `checkcast`, `instanceof`, dynamic `Object` methods, and Kotlin `Intrinsics`. Cross-class dispatch, inheritance, user-defined exception classes, Kotlin `object` singletons, and coroutines are skipped or rejected with a diagnostic until their phases land.

---

## Kotlin Support

Kotlin compiles to standard JVM bytecode, so the parser needs no Kotlin-specific changes. The runtime and lowering do need to honour Kotlin's conventions:

- **Null checks:** `Intrinsics.checkNotNullParameter`, `checkNotNullExpressionValue`, `checkNotNull`, and the lateinit helpers are translated to Go runtime calls.
- **Data classes:** generated `equals`, `hashCode`, `toString`, `copy`, and `componentN` methods lower as ordinary same-class bytecode, including `checkcast`, `instanceof`, `lcmp`, `Long.hashCode(J)I`, and `String.hashCode()I`.
- **Default parameters:** `$default` bridges use ordinary `iand` + `ifeq` masks. No Kotlin-specific IR pass is required; `$` in method names is sanitised to `_` in Go.
- **Switches:** `tableswitch` and `lookupswitch` are lowered to Go `switch` statements with `pc_<offset>` labels. This is also the foundation for coroutine state machines.
- **Coroutines:** `tableswitch` / `lookupswitch` can represent Kotlin's continuation state machines. A later optimisation will detect `kotlin.coroutines.Continuation` parameters and dispatch the body as a goroutine over channels.
- **Still planned:** `object` singletons, `Companion`, enums, full `DefaultConstructorMarker` constructor semantics, property delegates, and the coroutine bridge.
- **CI note:** the Kotlin integration test runs when `KOTLINC` or `kotlinc` is available and skips otherwise; pin a compiler in CI so it cannot skip silently.

---

## Roadmap

| Phase | Goal | Key work | Status |
| --- | --- | --- | --- |
| 1 | Primitive calculator | Class-file parsing, constant pool, core integer opcodes, branch/loop lowering, `go/ast` output, `grind`/`brew` | done |
| 2 | Objects & control flow | `new`, `dup`, field access, constructors, object model, basic-block partitioning | done |
| 3 | Runtime & exceptions | Exception tables, `String`/`StringBuilder`, arrays, `System.out`, runtime shims | done |
| 4 | Kotlin engine | Switch bytecodes, Kotlin intrinsics, Longs, casts, default masks, data classes, coroutine state machines | done |
| 5 | Networking | `net/http`-backed server pipelines, socket interception, REST fixture | done |
| 6 | Production tooling | `serve`, stripped static binaries, packaging | next |

### Immediate next steps

1. Implement Kotlin `object` singletons, `Companion`, enums, and full `DefaultConstructorMarker` constructor semantics.
2. Add a coroutine fixture and the goroutine/channel bridge.
3. Add cross-class dispatch and inheritance support so Kotlin stdlib calls can resolve beyond runtime shims.
4. Expand casts/`instanceof` to model generated inheritance and interfaces.
5. Start Phase 5 by mapping socket/HTTP entry points onto `net/http`.

---

## Design Principles

1. **No JVM at runtime.** Generated programs depend only on the Go standard library and small, auditable shims.
2. **Go-native memory.** No custom allocator, write barrier, or GC; pointers map directly onto Go's heap.
3. **Deterministic output.** Same input bytecode and configuration should always produce the same Go source.
4. **Fail loudly.** Unsupported opcodes and malformed class files produce actionable diagnostics rather than silently wrong code.
5. **Incremental correctness.** Each phase is verified end-to-end with compiled fixtures before the next begins.

---

## Contributing

1. Fork the repository.
2. Create a feature branch: `git checkout -b feature/my-feature`.
3. Commit your changes: `git commit -m "Add my feature"`.
4. Push and open a pull request.

Please include a fixture and an integration test with compiler changes whenever possible.

Before investigating a non-obvious issue, check [`docs/case-studies/`](docs/case-studies/) for prior findings. After solving a unique problem, record it using the template in [`docs/conventions.md`](docs/conventions.md) so the next contributor — human or AI — does not have to rediscover it.

---

## License

License to be determined. Add a `LICENSE` file before distributing binaries or source.
