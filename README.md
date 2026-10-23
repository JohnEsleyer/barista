<p align="center">
  <picture>
    <img src="assets/logo.svg" alt="Barista Logo" width="160" height="160">
  </picture>
</p>

# Barista

> **Write Kotlin or Java. Ship a native Go binary.**
>
> Barista is an ahead-of-time compiler and lightweight application framework. It lets you build high-performance backend services using expressive Kotlin or Java syntax and compiles them directly into single, static Go executables—with sub-5ms boot times, tiny memory footprints, and zero JVM dependencies at runtime.

**Barista is not a legacy JVM compatibility layer.** It does not try to compile Spring Boot, Hibernate, reflection-heavy libraries, or arbitrary existing `.jar` files. That treadmill is effectively unbounded. Instead, Barista targets **new code written with Barista in mind**: expressive Kotlin or Java source, a small and explicit runtime contract, and Go's operational model underneath.

You keep the language and toolchain you already love. You leave behind the JVM's operational baggage: no 200MB base runtime, no JIT warm-up spikes, no JRE packaging, and no guessing whether your service will fit on a $4/month box. Write Kotlin or Java; ship one native binary.

> **Status:** Active development. The bytecode-to-Go compiler pipeline works end to end for the supported subset, including `grind`, `brew`, `build`, and `serve`. The Barista SDK, IDE lint plugin, framework APIs, and project scaffolding described below are the next major deliverable and are **not shipped yet**. See [`docs/progress.md`](docs/progress.md) for verified status and [`docs/barista-first.md`](docs/barista-first.md) for the product and engineering direction.

---

## Why Barista?

Most developers choose Kotlin or Java for the language: null safety, data classes, expressive collections, checked APIs, and mature IDE support. The JVM is often accepted as the price of admission, not the reason for the choice. Barista separates the two.

GraalVM Native Image and Quarkus prove that developers want Go-like deployment without giving up JVM ergonomics. They still carry the complexity of the JVM ecosystem. Barista takes a narrower, more honest path: keep the syntax, define a focused SDK, and compile only what that SDK promises.

The target operating model:

| Concern | Traditional JVM deployment | Barista-first deployment |
| --- | --- | --- |
| **Language** | Kotlin or Java | Kotlin or Java syntax |
| **Runtime** | JVM plus JRE | Go runtime, no VM |
| **Build** | Gradle/Maven plus JRE packaging | `kotlinc`/`javac` plus Barista and `go build` |
| **Startup** | Hundreds of milliseconds to seconds | Target sub-5ms startup |
| **Memory** | 100MB+ before application work | Target around 15MB RSS |
| **Deployment** | JRE plus application JAR | One self-contained static executable |
| **Cross-compilation** | Platform-specific JREs | Inherited from `GOOS`/`GOARCH` |
| **Concurrency** | Thread pools and JVM schedulers | Goroutines and channels via the SDK |
| **Scope** | Arbitrary legacy libraries | Curated Barista SDK and subset |
| **Reflection** | Pervasive in frameworks | Rejected by the Barista subset |
| **Hosting cost** | Needs headroom for the runtime itself | Smooth operation on $4/month VPS boxes |

---

## The Barista Model

Barista is deliberately opinionated. The contract is what makes the project finite, testable, and fast.

### 1. New code, written for Barista

Barista is for services, APIs, workers, and CLI tools that you start today. It is not a migration tool for existing JVM applications. If a program depends on arbitrary JVM libraries, runtime class loading, bytecode instrumentation, or deep framework magic, it is outside the Barista subset.

### 2. A curated SDK instead of the entire JVM

Barista defines a lightweight `barista-sdk` for the APIs it supports. The SDK maps directly onto Go runtime primitives:

- `barista.async.Channel<T>` and `go { ... }` for goroutines and channels.
- `barista.http` for routing and request/response handling backed by `net/http`.
- AOT-friendly JSON, configuration, logging, and database clients.
- Annotations and interfaces that the compiler understands without reflection.

### 3. An explicit, linted subset

The supported language and library surface is documented and enforced. A Gradle/Maven lint plugin should reject unsupported constructs inside the IDE, before `barista build` runs. The compiler remains the final authority, but developers should not discover unsupported opcodes or library calls at the end of a build.

### 4. Go semantics, Kotlin ergonomics

Barista exposes Go's operational strengths directly rather than emulating JVM concurrency:

- Goroutines replace ad-hoc thread pools.
- Channels replace blocking queues.
- `net/http` replaces servlet containers.
- Static binaries replace JRE packaging.

### 5. Composition over hierarchy

Barista favors `data class`, `interface`, and composition. Deep abstract-class hierarchies, reflective dependency injection, and dynamic proxies are not part of the model. This keeps the generated Go small and the compiler tractable.

### 6. AOT by default

Routing, serialization, configuration, and dependency wiring should be generated ahead of time. No reflection, no classpath scanning, no startup-time graph construction.

### 7. Native Go interop as an escape hatch

The planned `@NativeGo` annotation lets Barista code call a Go package directly. That gives Kotlin and Java developers access to Go's Redis, Postgres, Kafka, cloud, and container clients without waiting for a JVM-compatible shim.

### Non-goals

Barista explicitly does **not** aim to support:

- Existing Spring Boot, Hibernate, Micronaut, or Quarkus applications.
- Arbitrary Maven/Gradle dependencies that were not written for Barista.
- Reflection, dynamic proxies, runtime bytecode generation, or classpath scanning.
- The complete JVM specification, JIT compilation, or JVM thread semantics.
- Drop-in compatibility with legacy `.jar` archives.

---

## How It Works

Barista stays bytecode-driven so that `kotlinc`, `javac`, and existing IDE tooling remain useful. The difference is that the input is a **Barista-targeted** program, not arbitrary JVM code.

```text
Input: Barista-targeted .class files
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
│    functions; inject native runtime and framework shims;     │
│    format with go/ast and go/printer.                        │
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

These shims are the compatibility floor for the Barista subset, not a promise to emulate the full JDK.

---

## The Barista SDK (Planned)

The SDK is the product surface that makes Barista coherent. It is not shipped yet; the examples below define the target contract.

### Dependency

```kotlin
// build.gradle.kts
dependencies {
    implementation("io.github.johnesleyer:barista-sdk:0.1.0")
}
```

### Concurrency with Go semantics

```kotlin
import barista.async.*

fun main() {
    val ch = Channel<String>(buffer = 10)

    go {
        ch.send("Hello from a goroutine!")
    }

    println(ch.receive())
}
```

`Channel<T>` maps to a Go channel. `go { ... }` maps to `go func() { ... }()`. There is no JVM thread pool and no continuation state machine to lower.

### HTTP without a servlet container

```kotlin
import barista.http.*

class UserApi : BaristaApp() {
    override fun configure() {
        get("/api/greet") { ctx ->
            ctx.json(mapOf("status" to "ok", "message" to "Hello, World!"))
        }

        post("/api/echo") { ctx ->
            ctx.text(ctx.bodyString())
        }
    }
}
```

Routes are baked into the generated Go program and served by `net/http`. No reflection, classpath scanning, or runtime route discovery.

### Spring-like annotations without Spring's runtime

Barista should feel familiar to Spring developers without reproducing Spring's runtime machinery. The planned `barista.spring` annotation package is a compile-time surface, not a Spring compatibility layer:

```kotlin
import barista.spring.annotation.*

@RestController
@RequestMapping("/api/users")
class UserController(private val userService: UserService) {

    @GetMapping("/{id}")
    fun getUser(@PathVariable id: Long): String {
        return userService.findUser(id)
    }

    @PostMapping
    fun createUser(@RequestBody name: String): Map<String, Any> {
        return mapOf("status" to "created", "name" to name)
    }
}
```

The compiler reads the annotations from class metadata and emits:

- Static constructor wiring instead of runtime dependency injection.
- A hardcoded `net/http` route table instead of classpath scanning.
- Direct handler calls instead of reflection or dynamic proxies.
- Request parameter parsing generated from the method signature.

The supported annotation set is intentionally small: routing, request binding, and simple component wiring. AOP, CGLIB proxies, runtime bean lookup, and annotation scanning are permanently out of scope.

### Native Go interop

```kotlin
@NativeGo(pkg = "github.com/redis/go-redis/v9", symbol = "NewClient")
external fun newRedisClient(options: RedisOptions): RedisClient
```

The compiler sees the annotation, adds the Go import, and emits a direct call. This is the planned escape hatch for libraries that are not part of the SDK.

### IDE feedback before build time

The SDK should ship with a Gradle/Maven lint plugin that rejects unsupported APIs and language constructs in the editor. The compiler remains the source of truth, but the feedback loop should not wait for a full native build.

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
│   ├── barista-first.md            # Product direction and SDK contract
│   ├── evolution-plan.md           # Phased delivery plan
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

### Installation

Install via the one-line installer:

```sh
curl -fsSL https://raw.githubusercontent.com/JohnEsleyer/barista/main/install.sh | bash
```

Or from a cloned repository:

```sh
./install.sh
```

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
javac -d tests/fixtures/out tests/fixtures/java/HelloBarista.java
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

---

## Current Capabilities

The compiler currently supports:

- Class-file parsing, constant-pool resolution, and method code attributes.
- Control-flow graphs, branches, loops, and operand-stack PHI joins.
- `int` and `long` arithmetic, conditionals, and switch bytecodes.
- Objects, fields, constructors, and same-class method dispatch.
- Strings, arrays, standard `java.lang` exceptions, and Kotlin intrinsics.
- `checkcast`, `instanceof`, dynamic `Object` methods, and `invokeinterface`.
- Sockets, streams, and `net/http`-backed HTTP servers through runtime shims.
- Stripped, statically linked binaries with cross-compilation targets.
- In-memory compile-and-run via `barista serve`.

---

## Known Limitations

Barista compiles a single class at a time and does not yet model the full JVM runtime. Current gaps:

- Cross-class dispatch, inheritance, and interfaces are not resolved beyond same-class calls and runtime shims.
- User-defined exception classes are not supported; standard `java.lang` exception types are.
- Kotlin `object` singletons, `Companion` objects, enums, property delegates, and full `DefaultConstructorMarker` semantics are not supported.
- Coroutines are not bridged to goroutines and channels yet.
- `java.util.ArrayList` is not implemented.
- `.jar` archive input is not supported; pass individual `.class` files.
- The Barista SDK, lint plugin, framework APIs, and `barista new` scaffolding do not exist yet.

See [`docs/progress.md`](docs/progress.md) for the canonical tracker of implementation status and test coverage.

---

## Where Barista Is Going

The next work is organized around making the Barista-first contract real, not around expanding arbitrary JVM compatibility:

1. **Link multi-class programs and ingest JARs.** Compile a complete application module, not just one `.class` file.
2. **Publish the Barista subset specification.** Define the supported language constructs, SDK APIs, and compiler diagnostics.
3. **Build `barista-sdk`.** Ship annotations, HTTP routing, async primitives, JSON, configuration, and logging APIs that map cleanly to Go.
4. **Add the Spring-like AOT surface.** Support `barista.spring` routing and wiring annotations with zero reflection or runtime scanning.
5. **Ship the IDE lint plugin.** Reject unsupported constructs in Gradle/Maven before a native build starts.
6. **Add `barista new`.** Scaffold a Kotlin or Java service with the SDK, a dev server, and a production build.
7. **Implement `@NativeGo` and `barista-bind`.** Generate direct Go imports and calls, plus Kotlin bindings for Go packages.
8. **Grow conformance tests and benchmarks.** Every SDK promise needs a compiled fixture, an end-to-end assertion, and measured startup, memory, and binary-size results.

The detailed product and engineering rationale lives in [`docs/barista-first.md`](docs/barista-first.md), and the phased delivery plan lives in [`docs/evolution-plan.md`](docs/evolution-plan.md).

---

## Design Principles

1. **No JVM at runtime.** Generated programs depend only on the Go standard library and small, auditable shims.
2. **Go-native memory.** No custom allocator, write barrier, or GC; pointers map directly onto Go's heap.
3. **Deterministic output.** Same input bytecode and configuration should always produce the same Go source.
4. **Fail loudly.** Unsupported opcodes and malformed class files produce actionable diagnostics rather than silently wrong code.
5. **Incremental correctness.** Each capability is verified end-to-end with compiled fixtures before the next is added.
6. **Finite scope.** The SDK contract, not the JVM ecosystem, defines what Barista supports.

---

## Contributing

1. Fork the repository.
2. Create a feature branch: `git checkout -b feature/my-feature`.
3. Commit your changes: `git commit -m "Add my feature"`.
4. Push and open a pull request.

Please include a fixture and an integration test with compiler changes whenever possible. Changes that expand arbitrary JVM compatibility without a corresponding Barista SDK use case are out of scope.

Before investigating a non-obvious issue, check [`docs/case-studies/`](docs/case-studies/) for prior findings. After solving a unique problem, record it using the template in [`docs/conventions.md`](docs/conventions.md) so the next contributor — human or AI — does not have to rediscover it.

---

## License

License to be determined. Add a `LICENSE` file before distributing binaries or source.
