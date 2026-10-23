# Development Progress

This is the canonical tracker for Barista's implementation progress. Update it at the end of every meaningful change so a new contributor can understand what works, what does not, and what comes next without reading the entire commit history.

**Direction:** [`barista-first.md`](barista-first.md) defines the product scope: new Kotlin/Java services written against the Barista SDK, not arbitrary legacy JVM compatibility.

**Last updated:** 2026-10-23
**Current focus:** Barista-first SDK, subset specification, and developer tooling
**Latest compiler commit:** `e995376` — build, serve, and progress CLI

---

## Status at a Glance

| Area | Status | Evidence |
| --- | --- | --- |
| Build | Passing | `go build ./...` |
| Vet | Passing | `go vet ./...` |
| Tests | Passing | `go test ./...` |
| Race detector | Passing | `go test -race ./...` |
| Class-file parsing | Complete | `internal/classfile` |
| IR lowering | Complete for the supported subset | `internal/ir` |
| Control flow graph | Basic blocks, switch edges, predecessors, successors | `internal/ir/block.go` |
| Go code generation | Objects, fields, switches, casts, dynamic Object dispatch, net/http pipelines | `internal/codegen` |
| Runtime shims | `Object`, `String`, `StringBuilder`, `Throwable`, `System`, `Long`, `Intrinsics`, `HttpServer`, `Socket`, `Streams` | `pkg/runtime` |
| CLI | `grind`, `brew`, `build`, and `serve` implemented with progress UI | `cmd/barista` |
| Native build | Stripped static binaries and `-target os/arch` cross-compilation | `internal/builder` |
| End-to-end fixtures | Calculator, Counter, Greeter, TryCatch, Switch, ReferenceOps, Greeting | `tests/integration_test.go` |
| Barista SDK | Not started | — |
| JAR ingestion and multi-class linking | Not started | — |
| Subset specification | In progress | `docs/barista-first.md` |
| Gradle/Maven lint plugin | Not started | — |
| Spring-like AOT surface | Not started | — |
| HTTP framework DSL | Not started | — |
| `barista new` scaffolding | Not started | — |
| `@NativeGo` interop | Not started | — |
| `barista-bind` generator | Not started | — |
| Production benchmarks | Not started | — |

---

## Compiler Capabilities

These capabilities are implemented and covered by tests or reproducible commands.

### Class files and IR

- [x] Bounds-checked big-endian class-file reader
- [x] `0xCAFEBABE` validation and supported-version checks
- [x] Full constant-pool parsing, including long/double double-slot accounting
- [x] Class metadata, interfaces, fields, methods, and `Code` attributes
- [x] Exception-table parsing and lowering into `MethodIR.ExceptionTraps`
- [x] Abstract operand-stack evaluation into SSA-style IR
- [x] Basic-block partitioning, control-flow graphs, switch edges, predecessors, and successors
- [x] Operand-stack merges (PHI nodes) at branch and switch targets

### Lowering and code generation

- [x] Integer loads, stores, constants, arithmetic, negation, and `iinc`
- [x] Conditional branches, labels, `goto`, and backward jumps for loops
- [x] `long` operations, comparisons, conversions, and long arrays
- [x] `tableswitch` and `lookupswitch`
- [x] `new`, `dup`, instance and static fields, constructors, and `<clinit>`
- [x] `invokespecial`, `invokevirtual`, `invokestatic`, and `invokeinterface`
- [x] Same-class dispatch with pointer-receiver Go methods
- [x] Strings, `StringBuilder`, arrays, and `System.out` / `System.err`
- [x] Exception traps, `athrow`, recover closures, and catch-type matching
- [x] `checkcast`, `instanceof`, reference comparisons, and dynamic `Object` methods
- [x] Kotlin `Intrinsics`, default-parameter masks, data classes, and `ConsoleKt`
- [x] `java.net` sockets, streams, URI, and `com.sun.net.httpserver` mapped to `net/http`

### Tooling

- [x] `grind` IR dump
- [x] `brew` Go source emission
- [x] `build` with `-o`, `-target`, `-entry`, and `-q`
- [x] Stripped static linking with `-ldflags="-s -w"` and `CGO_ENABLED=0`
- [x] `serve` in-memory compile and execution
- [x] Embedded runtime shims via `embed.FS`
- [x] Blue terminal progress bar with TTY detection and duration timing

---

## Current Gaps

### Compiler

- Cross-class dispatch, inheritance, and interfaces are not resolved beyond same-class calls and runtime shims.
- User-defined exception classes are not supported.
- Kotlin `object` singletons, `Companion`, enums, property delegates, and full `DefaultConstructorMarker` semantics are not supported.
- Coroutines are not bridged to goroutines and channels.
- `java.util.ArrayList` is not implemented.
- `.jar` archive input is not supported; only individual `.class` files are accepted.
- Casts and `instanceof` use concrete Go type assertions rather than a generated type hierarchy.
- PHI joins are slot-based and trust the JVM verifier.
- Narrow primitive arithmetic (`byte`, `char`, `short`) and `float`/`double` opcodes are incomplete.

### Framework and developer experience

- `barista-sdk` does not exist yet.
- The Barista subset is documented as a direction, not yet as a versioned specification with diagnostics.
- No Gradle/Maven lint plugin exists.
- No HTTP routing DSL or async API exists.
- No `@NativeGo` annotation or code generation exists.
- No `barista new` project scaffolding exists.
- No SDK conformance suite or published benchmarks exist.

---

## Next Milestones

The work below is organized by outcome, not by phase.

### 1. Whole-application compilation

- [ ] Parse `.jar` and `.zip` archives and build a class hierarchy.
- [ ] Read `META-INF/MANIFEST.MF` for the main class.
- [ ] Resolve cross-class methods, fields, and constructors.
- [ ] Generate all classes into one Go package with collision-free names.
- [ ] Add reachability analysis to prune unused classes and methods.
- [ ] Extend `grind`, `brew`, `build`, and `serve` to accept `.jar` inputs.
- [ ] Add a multi-class JAR fixture and end-to-end test.

### 2. Subset specification

- [ ] Define supported language constructs and bytecode patterns.
- [ ] Define unsupported constructs and their diagnostics.
- [ ] Document the SDK API surface and versioning policy.
- [ ] Add a conformance matrix mapping each promise to a fixture.

### 3. Barista SDK

- [ ] Create the `barista-sdk` project and publishable artifact.
- [ ] Add `barista.async.Channel<T>` and `go { ... }`.
- [ ] Add `barista.http` routing and request/response helpers.
- [ ] Add AOT-friendly JSON, configuration, and logging APIs.
- [ ] Add annotations recognized by the compiler from class metadata.

### 4. Spring-like AOT surface

- [x] Create the `barista.spring.annotation` package (`RestController`, `RequestMapping`, `GetMapping`, `PostMapping`, `PutMapping`, `DeleteMapping`, `RequestBody`, `PathVariable`, `RequestParam`).
- [x] Parse `RuntimeVisibleAnnotations` from class files.
- [x] Detect controllers and route annotations at compile-time (`DetectSpringControllers`).
- [x] Generate static constructor wiring and a Go 1.22+ `net/http` route table (`GenerateSpringMainGo`).
- [x] Generate zero-reflection compile-time JSON serialization/deserialization with Go `json:"..."` struct tags.
- [x] Add runtime JSON marshalling for `lang.String` and `lang.Long`.
- [x] Add controller fixtures and automated integration tests (`TestSpringJSONSerialization`, `TestSpringOrderController`).

### 5. IDE feedback

- [ ] Build a Gradle plugin that validates the subset.
- [ ] Add Maven support.
- [ ] Emit actionable diagnostics for unsupported JVM and framework APIs.
- [ ] Test the plugin against SDK fixtures.

### 6. Project scaffolding and dev loop

- [ ] Add `barista new <name> --template kotlin-service`.
- [ ] Add a Java service template.
- [ ] Add a development loop around `barista serve`.
- [ ] Add generated project documentation and a runnable endpoint.

### 7. Native Go interop

- [ ] Define the `@NativeGo` annotation and metadata format.
- [ ] Generate Go imports and direct calls.
- [ ] Validate type mappings and report unsupported signatures.
- [ ] Add Redis or Postgres interop as a fixture.
- [ ] Build `barista-bind` to generate Kotlin/Java bindings from Go packages.

### 8. Compiler follow-through

- [ ] Add interface lowering needed by the SDK.
- [ ] Add Kotlin `object`, `Companion`, and enum support needed by the SDK.
- [ ] Add coroutine-to-goroutine bridging where the SDK requires it.
- [ ] Expand casts and `instanceof` to the generated type hierarchy.

### 9. Benchmarks and release

- [ ] Measure startup latency, RSS, build duration, and binary size.
- [ ] Publish reproducible benchmark commands and hardware details.
- [ ] Package the CLI through GitHub Releases, Homebrew, and Scoop.

Additions should be justified by a concrete Barista-first use case. Expanding arbitrary JVM compatibility is out of scope.

---

## Verification

Current verification commands:

```sh
go build ./...
go vet ./...
go test ./...
go test -race ./...
```

Manual smoke test:

```sh
# A plain Java HTTP API compiled with Barista and run without a JVM.
javac -d sandbox/java-hello-api/out sandbox/java-hello-api/HelloApiServer.java
./bin/barista build -o sandbox/java-hello-api/hello-api sandbox/java-hello-api/out/HelloApiServer.class
./sandbox/java-hello-api/hello-api
```

The resulting binary is statically linked, stripped, and returns JSON over HTTP. The `sandbox/` directory is ignored by git.

---

## Updating This Tracker

At the end of every meaningful task:

- Check off completed capabilities only when a test or reproducible command proves them.
- Add or remove known limitations.
- Record the verification result.
- Update the latest compiler commit when implementation changes.
- Keep the direction aligned with [`barista-first.md`](barista-first.md).
- Link any new case study.
