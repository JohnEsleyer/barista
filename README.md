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

> **Status:** Barista is under active development and features an end-to-end ahead-of-time pipeline: disassembling (`grind`), Go transpilation (`brew`), stripped static native binary compilation with cross-compilation (`build`), and instant memory compilation and execution (`serve`), accompanied by a real-time terminal progress bar. Kotlin `object` singletons, coroutines, and cross-class dispatch remain deferred; see the [known limitations](#known-limitations).
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

The generated program does not ship `rt.jar`. Critical-path JDK and Kotlin APIs are reimplemented in Go under `pkg/runtime` and embedded into the compiler binary with `embed.FS`:

- `java.lang.Object` — identity hash codes, default equality, and dynamic `equals`/`hashCode`/`toString` helpers; `java.lang.Object` parameters map to Go `any`.
- `java.lang.String` — immutable wrapper over Go strings with length, equality, substring, concat, character access, `getBytes`, and Java-compatible `hashCode`.
- `java.lang.StringBuilder` — mutable string builder backed by `strings.Builder`, including `append(char)` for Kotlin interpolation.
- `java.lang.Long` — boxed `Long`, static `hashCode(J)`, value equality, and string rendering.
- `java.lang.System` — `System.out` and `System.err` mapped to `os.Stdout` and `os.Stderr`, plus `currentTimeMillis` and `exit`.
- `java.lang.Throwable` / `Exception` / `RuntimeException` — exception values with message, cause, and string rendering.
- `java.lang.NullPointerException` / `IndexOutOfBoundsException` / `ClassCastException` — typed runtime exceptions that generated catch handlers can match.
- `java.lang.Class` — class-literal metadata for `ldc` class constants.
- `java.util.ArrayList` — slice-backed list implementation (planned).
- `kotlin.jvm.internal.Intrinsics` — null checks, `AreEqual`, lateinit/property assertion helpers, and parameter validation emitted by `kotlinc`.
- `kotlin.io.ConsoleKt` — Kotlin `println` and `print`.
- `java.net.ServerSocket` / `Socket` / `URI` / `URL` / `HttpURLConnection` — sockets backed by Go's `net` package.
- `java.io.InputStream` / `OutputStream` / `ByteArrayInputStream` / `ByteArrayOutputStream` — stream shims used by networking and HTTP handlers.
- `com.sun.net.httpserver.HttpServer` / `HttpHandler` / `HttpExchange` / `Headers` — mapped directly onto Go's standard library `net/http` server.
- Cast helpers — `CheckCast[T]`, `IsInstanceOf[T]`, and `CompareLong` for `checkcast`, `instanceof`, and `lcmp`.

---

## Repository Layout

```text
barista/
├── assets/
│   └── logo.svg                    # Vector brand logo
├── cmd/
│   └── barista/
│       └── main.go                 # CLI entrypoint: grind, brew, build, serve
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
│   ├── builder/                    # Toolchain invocation, static linking, serve
│   │   ├── builder.go              # Build pipeline and cross-compilation targets
│   │   ├── builder_test.go         # Builder unit tests
│   │   └── doc.go                  # Package documentation
│   └── ui/                         # Blue terminal progress bar and summary cards
│       ├── progress.go             # Progress bar rendering
│       ├── progress_test.go        # UI unit tests
│       └── terminal.go             # TTY detection and formatting helpers
├── pkg/
│   └── runtime/                    # Embedded shims baked into generated output
│       ├── runtime.go              # embed.FS packaging for self-contained compilation
│       ├── java/
│       │   ├── lang/               # Object, String, StringBuilder, System, exceptions
│       │   ├── net/                # Sockets, streams, URI, net/http server shims
│       │   └── util/               # java.util shims (ArrayList planned)
│       └── kotlin/
│           └── jvm/                # Intrinsics and Kotlin runtime helpers
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

The CLI exposes four commands, following the coffee theme:

| Command | Purpose | Status |
| --- | --- | --- |
| `barista grind <input.class>` | Disassemble a class file into a readable IR dump | implemented |
| `barista brew [-o output.go] <input.class>` | Run the transpilation pipeline and emit Go source | implemented |
| `barista build [options] <input.class>` | Compile a class file to a stripped static native executable | implemented |
| `barista serve [options] <input.class> [args...]` | Compile in memory and execute immediately | implemented |

`build` and `serve` accept `-entry` to override the auto-detected entrypoint and `-q` to suppress the progress bar. `build` also accepts `-o` for the output path and `-target os/arch` for cross-compilation.

```sh
./bin/barista --help
./bin/barista version

# 1. Compile a fixture
javac -d tests/fixtures/out tests/fixtures/java/HelloBarista.java

# 2. Inspect the intermediate representation
./bin/barista grind tests/fixtures/out/HelloBarista.class

# 3. Compile to a standalone, stripped static executable
./bin/barista build -o bin/hello tests/fixtures/out/HelloBarista.class

# 4. Cross-compile for another platform
./bin/barista build -target linux/arm64 -o bin/hello-arm64 tests/fixtures/out/HelloBarista.class

# 5. Or compile and execute immediately with arguments
./bin/barista serve tests/fixtures/out/HelloBarista.class World
```

Barista's production tooling is fully wired: `build` emits stripped, statically linked binaries (`-ldflags="-s -w"`, `CGO_ENABLED=0`) and `serve` compiles in a temporary workspace and runs the result immediately. The supported lowering covers static and instance methods using `int`/`long` arithmetic, conditionals, loops, operand-stack PHI joins, objects, instance and static fields, constructors, same-class dispatch, strings, arrays, `System.out`, caught runtime exceptions, dense/sparse switches, `checkcast`, `instanceof`, dynamic `Object` methods, Kotlin `Intrinsics`, `invokeinterface`, sockets, and `net/http`-backed HTTP servers. Cross-class dispatch, inheritance, user-defined exception classes, Kotlin `object` singletons, and coroutines are skipped or rejected with a diagnostic until support lands.

---

## Kotlin Support

Kotlin compiles to standard JVM bytecode, so the parser needs no Kotlin-specific changes. The runtime and lowering do need to honour Kotlin's conventions:

- **Null checks:** `Intrinsics.checkNotNullParameter`, `checkNotNullExpressionValue`, `checkNotNull`, and the lateinit helpers are translated to Go runtime calls.
- **Data classes:** generated `equals`, `hashCode`, `toString`, `copy`, and `componentN` methods lower as ordinary same-class bytecode, including `checkcast`, `instanceof`, `lcmp`, `Long.hashCode(J)I`, and `String.hashCode()I`.
- **Default parameters:** `$default` bridges use ordinary `iand` + `ifeq` masks. No Kotlin-specific IR pass is required; `$` in method names is sanitised to `_` in Go.
- **Switches:** `tableswitch` and `lookupswitch` are lowered to Go `switch` statements with `pc_<offset>` labels. This is also the foundation for coroutine state machines.
- **Coroutines:** `tableswitch` / `lookupswitch` can represent Kotlin's continuation state machines. A later optimisation will detect `kotlin.coroutines.Continuation` parameters and dispatch the body as a goroutine over channels.
- **Still planned:** `object` singletons, `Companion`, enums, property delegates, and the coroutine bridge.
- **CI note:** the Kotlin integration test runs when `KOTLINC` or `kotlinc` is available and skips otherwise; pin a compiler in CI so it cannot skip silently.

---

## Known Limitations

Barista compiles a single class at a time and does not yet model the full JVM runtime. Current gaps:

- Cross-class dispatch, inheritance, and interfaces are not resolved beyond same-class calls and runtime shims.
- User-defined exception classes are not supported; standard `java.lang` exception types are.
- Kotlin `object` singletons, `Companion` objects, enums, property delegates, and full `DefaultConstructorMarker` semantics are not supported.
- Coroutines are not bridged to goroutines and channels yet.
- `java.util.ArrayList` is not implemented.
- `.jar` archive input is not supported; pass individual `.class` files.

See [`docs/progress.md`](docs/progress.md) for the canonical tracker of implementation status and test coverage.

---

## Design Principles

1. **No JVM at runtime.** Generated programs depend only on the Go standard library and small, auditable shims.
2. **Go-native memory.** No custom allocator, write barrier, or GC; pointers map directly onto Go's heap.
3. **Deterministic output.** Same input bytecode and configuration should always produce the same Go source.
4. **Fail loudly.** Unsupported opcodes and malformed class files produce actionable diagnostics rather than silently wrong code.
5. **Incremental correctness.** Each capability is verified end-to-end with compiled fixtures before the next is added.

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
