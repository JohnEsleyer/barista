# Barista

**Barista** is an ahead-of-time (AOT) static de-stacking compiler for JVM bytecode, written in Go. It ingests standard `.class` files and `.jar` archives produced by `javac` or `kotlinc`, reconstructs their stack-based execution model as a register/variable-based IR, and emits idiomatic Go source that is compiled into a single, statically linked binary.

The goal is to make JVM backends viable on very small machines — $4/month VPS nodes with 512MB–1GB of RAM, or containers with even less — by removing the JVM from the deployment equation entirely.

> **Status:** early scaffold. The repository currently contains the project structure and CLI skeleton. The compiler pipeline is under active development; see the [roadmap](#roadmap).

---

## Why Barista?

Running a JVM application on a small instance is expensive before your code even executes:

| Problem | JVM behaviour | Barista approach |
| --- | --- | --- |
| Base runtime overhead | OpenJDK reserves roughly 100–250MB RSS at boot | Compile to a native Go binary with no VM |
| JIT contention | Compilation spikes compete for a single vCPU | No JIT; code is compiled ahead of time |
| Delayed reclamation | G1/ZGC assume memory is plentiful and shrink lazily | Delegate allocation and GC to the Go runtime |
| Deployment footprint | JRE + application JAR | One self-contained executable |
| Startup latency | Hundreds of milliseconds to seconds | Target sub-5ms startup |
| Cross-compilation | Platform-specific JREs | Inherited from the Go toolchain |

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
| reference types | pointers to generated structs |

JVMS operands `boolean`, `byte`, `char`, `short`, and `int` occupy one stack slot. Barista preserves the JVM's value semantics at the IR level and narrows to the declared Go type where required.

### Object model

Each JVM class becomes a Go struct, and instance methods use pointer receivers:

```go
// com/example/User -> ComExampleUser
type ComExampleUser struct {
    Id   int64
    Name *JavaLangString
}

func (this *ComExampleUser) GetId() int64 {
    return this.Id
}
```

Instances are always heap-allocated through pointers (`&ComExampleUser{}`), so object lifetime is managed entirely by the Go garbage collector — no custom allocator or tracing runtime is required.

### Runtime shims

The generated program does not ship `rt.jar`. Critical-path JDK and Kotlin APIs are reimplemented in Go under `pkg/runtime`:

- `java.lang.Object` — identity hash codes and default equality.
- `java.lang.String` — immutable wrapper over Go strings with Java-compatible conversion helpers.
- `java.lang.System` — `System.out` / `System.err` mapped to `os.Stdout` / `os.Stderr`.
- `java.lang.Throwable` — exceptions lowered to Go error pairs or `panic`/`recover` depending on the exception table scope.
- `java.util.ArrayList` — slice-backed list implementation.
- `kotlin.jvm.internal.Intrinsics` — null-check and assertion intrinsics emitted by `kotlinc`.

---

## Repository Layout

```text
barista/
├── cmd/
│   └── barista/
│       └── main.go                 # CLI entrypoint
├── internal/
│   ├── classfile/                  # Binary class-file decoding
│   │   ├── class.go                # Class structure definitions
│   │   ├── constant_pool.go        # Constant pool tags and symbolic tables
│   │   ├── reader.go               # Bounds-checked big-endian reader
│   │   └── attributes.go           # Code, exception, and line-number attributes
│   ├── ir/                         # Intermediate representation
│   │   ├── block.go                # Basic blocks and CFG
│   │   ├── instruction.go          # Normalised SSA instructions
│   │   └── stack_eval.go           # Abstract operand-stack evaluator
│   ├── codegen/                    # Go transpilation
│   │   ├── generator.go            # Code generation driver
│   │   ├── translator.go           # IR blocks to Go AST nodes
│   │   └── type_map.go             # JVM descriptors to Go types
│   └── builder/                    # Toolchain execution
│       └── compiler.go             # Invokes go build with optimisation flags
├── pkg/
│   └── runtime/                    # Embedded shims baked into generated output
│       ├── java/
│       │   ├── lang/
│       │   └── util/
│       └── kotlin/
│           └── jvm/
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
| `barista grind <input>` | Disassemble a class or JAR into a readable IR dump | planned |
| `barista brew <input>` | Run the full pipeline and emit a Go module | planned |
| `barista serve <input>` | Compile in memory and execute immediately | planned |

```sh
./bin/barista --help
./bin/barista version
```

---

## Kotlin Support

Kotlin compiles to standard JVM bytecode, so the parser needs no Kotlin-specific changes. The runtime does need to honour Kotlin's conventions:

- **Null checks:** `Intrinsics.checkNotNullParameter` is translated to a Go null check.
- **Coroutines:** basic support comes from `tableswitch` / `lookupswitch`, which lower Kotlin's continuation state machines without special treatment. A later optimisation will detect `kotlin.coroutines.Continuation` parameters and dispatch the body as a goroutine over channels.
- **Language features:** data classes, object singletons, and default-parameter bridge methods are lowered in Phase 4.

---

## Roadmap

| Phase | Goal | Key work | Status |
| --- | --- | --- | --- |
| 1 | Primitive calculator | Class-file parsing, constant pool, core integer opcodes, CFG-free codegen, `go/ast` printer | in progress |
| 2 | Objects & control flow | `new`, `dup`, field access, constructors, basic blocks, labels | planned |
| 3 | Runtime & exceptions | Exception tables, `String`/`StringBuilder`, arrays | planned |
| 4 | Kotlin engine | Switch bytecodes, stdlib intrinsics, data classes, singletons | planned |
| 5 | Networking | `net/http`-backed server pipelines, socket interception | planned |
| 6 | Production tooling | `grind` / `brew` / `serve`, stripped static binaries | planned |

### Immediate next steps

1. Implement the bounds-checked big-endian reader in `internal/classfile/reader.go`.
2. Validate the `0xCAFEBABE` magic and supported class-file versions.
3. Parse the full constant pool (all tag types) into an indexed lookup table.
4. Parse class metadata, fields, methods, and `Code` attributes.
5. Compile `tests/fixtures/java/Calculator.java` and assert that method bytecodes are recovered exactly.

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

---

## License

License to be determined. Add a `LICENSE` file before distributing binaries or source.
