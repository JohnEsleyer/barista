# Development Progress

This is the canonical tracker for Barista's implementation progress. Update it at the end of every meaningful change so a new contributor can understand what works, what does not, and what comes next without reading the entire commit history.

**Last updated:** 2026-10-24
**Current phase:** Phase 5 complete; Phase 6 next
**Latest implementation commit:** Implement Phase 5 networking, socket interception, and net/http pipeline

---

## Status at a Glance

| Area | Status | Evidence |
| --- | --- | --- |
| Build | Passing | `go build ./...` |
| Vet | Passing | `go vet ./...` |
| Tests | Passing | `go test ./...` |
| Race detector | Passing | `go test -race ./...` |
| Class-file parsing | Complete | `internal/classfile` |
| IR lowering | Complete for Phase 5 (includes PHI joins, switches, invokeinterface) | `internal/ir` |
| Control flow graph | Basic blocks, switch edges, predecessors, successors | `internal/ir/block.go` |
| Go code generation | Objects, fields, switches, casts, dynamic Object dispatch, net/http pipelines | `internal/codegen` |
| Runtime shims | `Object`, `String`, `StringBuilder`, `Throwable`, `System`, `Long`, `Intrinsics`, `HttpServer`, `Socket`, `Streams` | `pkg/runtime` |
| CLI | `grind` and `brew` work for Phase 5 classes; `serve` is next | `cmd/barista` |
| End-to-end fixtures | Calculator, Counter, Greeter, TryCatch, Switch, ReferenceOps, Greeting, RestServer | `tests/integration_test.go` |

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
- [x] Kotlin data classes, default parameters, and `DefaultConstructorMarker`
- [x] Kotlin integration fixture (`Greeting.kt`)

### Phase 5 — Networking & I/O: Complete
- [x] `invokeinterface` (opcode `0xb9`) lowering into normalized virtual dispatch
- [x] `java.net.ServerSocket` and `Socket` interception backed by `net.Listen` and `net.Dial`
- [x] `java.io.InputStream`, `OutputStream`, `ByteArrayInputStream`, `ByteArrayOutputStream`
- [x] `java.net.URI`, `URL`, and `HttpURLConnection`
- [x] `com.sun.net.httpserver.HttpServer`, `HttpHandler`, `HttpExchange`, `Headers` mapped onto Go's `net/http`
- [x] Multi-package runtime import detection and type registry (`lang` and `jnet`)
- [x] `String.getBytes()` and byte-array string constructors
- [x] Full REST fixture (`RestServer.java`) handling GET, POST, headers, and 404s

### Phase 6 — Production Tooling: Planned
- [ ] `barista serve` command
- [ ] Invoke `go build -ldflags="-s -w"`
- [ ] Static linking and cross-compilation packaging
