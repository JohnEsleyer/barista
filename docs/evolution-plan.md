# Barista Evolution Plan

**Status:** Proposed delivery plan
**Related:** [`barista-first.md`](barista-first.md) defines the product direction; [`progress.md`](progress.md) tracks verified implementation.

This document turns the Barista-first direction into a sequenced engineering plan. It is intentionally opinionated: every phase moves Barista closer to a productive, finite framework and further away from arbitrary JVM compatibility.

The code sketches in this plan are design guidance, not a claim that the features already exist.

---

## 1. Positioning

> **Write expressive Kotlin or Java. Ship a single static Go binary.**

Barista targets greenfield backend services, APIs, workers, and CLI tools. It is not a migration tool for Spring Boot, Hibernate, or arbitrary enterprise JVM applications.

The operating promise:

- Kotlin/Java syntax and IDE support.
- A curated SDK that maps to Go primitives.
- No reflection, runtime classpath scanning, or dynamic proxies.
- A stripped static binary, fast startup, and low idle memory.

---

## 2. Target Developer Experience

```sh
# 1. Create a new service
barista new auth-service --template kotlin-api
cd auth-service

# 2. Develop with a standard Gradle/Maven project and barista-sdk
#    IntelliJ IDEA or VS Code provides completion and type checking.

# 3. Compile and run in memory during development
barista serve

# 4. Build a production binary
barista build -o bin/auth-service -target linux/amd64
```

The project should feel like a framework, not a bytecode tool.

---

## 3. Architecture

```text
Developer workspace
  IntelliJ / VS Code + Gradle plugin + barista-sdk
              │
              │ kotlinc / javac
              ▼
Application bytecode + barista-sdk metadata
              │
              │ barista CLI
              ▼
Class/JAR reader → CFG and stack de-stacker → specialized lowerers
              │
              ├── native Go invocation
              ├── coroutines → goroutines/channels
              ├── structural interfaces
              └── Spring-like annotations → static routes and wiring
              │
              ▼
Generated Go AST + embedded runtime shims + go.mod
              │
              │ CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"
              ▼
Single static native binary
```

---

## 4. Delivery Phases

### Phase 7: Multi-Class Linking and JAR Ingestion

**Goal:** Compile a complete application module, not a single class.

- Parse `.jar` and `.zip` archives with `archive/zip`.
- Read `META-INF/MANIFEST.MF` to discover `Main-Class`.
- Build an in-memory class hierarchy keyed by JVM internal name.
- Resolve method, field, and constructor references across classes in the same module.
- Generate all classes into one Go package with collision-free static method and field names.
- Add whole-program reachability analysis to prune unused classes and methods before code generation.
- Extend `grind`, `brew`, `build`, and `serve` to accept `.jar` inputs.

**Verification:** A multi-file Java fixture packaged as a JAR builds and runs, and a cross-class method call returns the expected output.

### Phase 8: Barista SDK and Native Concurrency

**Goal:** Provide the official application API where Kotlin/Java syntax binds directly to Go runtime capabilities.

- Publish `barista-sdk` as a lightweight JAR.
- Add `barista.async.Channel<T>` and `go { ... }`.
- Lower channel creation, send, and receive to Go channel operations.
- Lower `go { ... }` to `go func() { ... }()`.
- Add `barista.http.BaristaApp`, routing verbs, and request/response context.
- Map route definitions to a static `net/http` route table.
- Add AOT-friendly JSON, configuration, logging, and common data-access APIs.

**Verification:** A Kotlin service uses only the SDK to serve HTTP, run a background goroutine, and exchange messages over a channel.

### Phase 9: Go Interop

**Goal:** Give developers access to the Go ecosystem without writing Go.

- Add `@NativeGo(pkg, symbol)` for direct Go calls.
- Generate imports and wrapper declarations from annotation metadata.
- Add a controlled inline Go escape hatch for advanced use cases.
- Build `barista-bind`, which reads a Go package with `go/types` and emits Kotlin/Java bindings.
- Add Redis, Postgres, or another real Go client as an interop fixture.

**Verification:** A Barista service calls a third-party Go package through `@NativeGo` and builds without hand-written Go.

### Phase 10: Tooling, Linting, and Ecosystem

**Goal:** Make unsupported code fail inside the IDE, and prove the operating claims.

- Ship a Gradle/Maven plugin that validates the Barista subset before a native build.
- Emit actionable diagnostics for forbidden packages and constructs.
- Certify a small allowlist of pure Kotlin Multiplatform libraries.
- Add production benchmarks for startup latency, RSS, build duration, and binary size.
- Package the CLI through GitHub Releases, Homebrew, and Scoop.

**Verification:** A developer gets an IDE diagnostic for `java.lang.Thread.sleep`, and benchmark results are reproducible.

---

## 5. Spring-Like AOT Surface

Barista should feel familiar to Spring developers without reproducing Spring's runtime.

### 5.1 Green list: compile-time mappable

- `@RestController`, `@Controller`
- `@RequestMapping`, `@GetMapping`, `@PostMapping`, `@PutMapping`, `@DeleteMapping`, `@PatchMapping`
- `@PathVariable`, `@RequestParam`, `@RequestBody`, `@RequestHeader`
- `@Service`, `@Component`, `@Repository`
- `@Configuration`, `@Bean`
- `@Value` for environment-backed configuration

These annotations live in `barista.spring.annotation`, not `org.springframework`. They are a familiar surface for Barista, not a Spring compatibility promise.

### 5.2 Red list: permanently unsupported

- AOP and runtime interceptors.
- CGLIB or dynamic proxies.
- `ApplicationContext.getBean(...)` and runtime bean lookup.
- Classpath scanning and runtime annotation discovery.
- Arbitrary Spring Boot starters and auto-configuration.

### 5.3 Lowering model

At compile time, Barista reads annotation metadata from class files and emits:

- Static constructor wiring for controllers and services.
- A hardcoded `net/http` route table.
- Direct handler calls with generated request parsing.
- Generated response serialization for supported return types.

No reflection, no proxies, no runtime scanning.

---

## 6. Concurrency and Type Mapping

| Kotlin / Java construct | Barista IR | Emitted Go |
| --- | --- | --- |
| `Channel<String>(10)` | `OpMakeChan` | `make(chan *lang.String, 10)` |
| `ch.send(value)` | `OpChanSend` | `ch <- value` |
| `ch.receive()` | `OpChanRecv` | `<-ch` |
| `go { work() }` | `OpGoRoutine` | `go func() { ... }()` |
| `data class User(val id: Long)` | `OpNewStruct` | `type User struct { Id int64 }` |
| `interface Greeter` | `OpInterface` | `type Greeter interface { ... }` |
| `fun String.ext()` | `OpInvokeStatic` | `func String_Ext(this *lang.String)` |

---

## 7. Success Metrics

These are release gates, not marketing claims. Measure them on documented hardware and workloads.

| Metric | Target | Verification |
| --- | --- | --- |
| Startup latency | under 8 ms | `hyperfine ./binary --health-check` |
| Idle RSS | under 18 MB | Linux `time -v` on an idle HTTP server |
| Build duration | under 1.5 s | Full `barista build` on a 10-class service |
| Binary size | under 15 MB | Stripped static Linux/amd64 binary |

Publish the measurement environment, commands, and raw results.

---

## 8. Immediate Next Task

Start with Phase 7 because the framework cannot be real until a service can span multiple files.

1. Add `internal/classfile/jar.go` with a class hierarchy, JAR parsing, manifest handling, and a class-or-JAR loader.
2. Add multi-class Go generation with a global method and static-field naming table.
3. Add hierarchy-aware entrypoint discovery and builder support.
4. Extend the CLI to accept `.jar` inputs.
5. Add a multi-class fixture and an end-to-end build/run test.

Phase 8 should not begin until the Phase 7 fixture passes reliably.

---

## 9. Risks

- **Scope creep:** Every compatibility request must be tied to a Barista SDK use case.
- **IDE expectations:** Standard compilers work, but unsupported code must be caught early by linting.
- **Dependency expectations:** The SDK must cover common backend needs, with `@NativeGo` as the escape hatch.
- **Performance claims:** Benchmarks must be reproducible and honestly scoped.
- **Spring familiarity:** The surface can be familiar, but the implementation must stay static and reflection-free.

---

## 10. Relationship to Existing Work

The existing class-file parser, IR, code generator, runtime shims, builder, and CLI are the compiler foundation. They are not discarded.

What changes is the target: compile the Barista subset and make the SDK contract productive. Technical case studies remain valid because they document compiler internals independent of the product scope.
