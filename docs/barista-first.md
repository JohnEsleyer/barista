# Barista-First: Product and Engineering Direction

**Status:** Active direction
**Audience:** Contributors, compiler engineers, SDK designers, and technical stakeholders

This document defines what Barista is, what it is not, and how the project stays technically viable. It is the source of truth for scope decisions. [`progress.md`](progress.md) tracks verified implementation status; [`evolution-plan.md`](evolution-plan.md) turns this direction into a delivery sequence.

---

## 1. The Thesis

Developers choose Kotlin or Java primarily for the language: null safety, data classes, expressive collections, checked APIs, and strong IDE support. The JVM is often accepted as the deployment cost, not the reason for the choice.

Barista separates language from runtime:

> **Write new services in Kotlin or Java syntax. Compile them into a single static Go binary.**

This is not a generic JVM-to-Go translator. It is an opinionated application framework and language frontend for code written with Barista in mind.

The project succeeds by making the supported surface small, explicit, and productive:

- A curated SDK instead of the entire JDK and Maven ecosystem.
- A documented language subset instead of arbitrary bytecode.
- Go concurrency and networking instead of JVM thread and servlet emulation.
- Ahead-of-time generation instead of reflection and classpath scanning.

---

## 2. What Barista Is

Barista is:

- A **language frontend** that accepts Kotlin and Java bytecode produced by standard compilers.
- A **compiler** that lowers the Barista subset into idiomatic Go source.
- A **toolchain** that produces stripped, statically linked binaries with cross-compilation.
- A **framework contract** expressed through `barista-sdk` and enforced by compiler diagnostics and IDE linting.
- A **deployment model** with no JVM, no JIT warm-up, and no JRE packaging.

Barista is not:

- A migration path for existing Spring Boot, Hibernate, Micronaut, or Quarkus applications.
- A drop-in replacement for the JVM.
- A general-purpose Maven/Gradle compatibility layer.
- A reflection, dynamic proxy, or runtime bytecode generation runtime.
- A complete implementation of the JVM specification.

Every scope decision should be evaluated against one question:

> **Does this make new, Barista-first applications more productive without turning Barista into a JVM compatibility treadmill?**

If the answer is no, the work is out of scope.

---

## 3. Why the Framework Model Wins

Compiling arbitrary legacy Java is an unbounded problem. The JVM ecosystem includes decades of libraries, reflection, dynamic class loading, annotation processing at runtime, bytecode instrumentation, and container-specific assumptions. Even heavily funded native-image projects struggle with compatibility.

The Barista-first model changes the problem:

| Aspect | Generic JVM compiler | Barista-first framework |
| --- | --- | --- |
| **Scope** | Unbounded JVM ecosystem | Curated SDK and language subset |
| **Feedback** | Fails during native compilation | Fails in the IDE through linting |
| **Concurrency** | Thread pools and JVM schedulers | Goroutines and channels |
| **Networking** | Servlet containers and framework stacks | `net/http` and a small routing API |
| **Serialization** | Reflection-based libraries | AOT-generated codecs |
| **Go interop** | Impossible or unsafe | First-class `@NativeGo` bindings |
| **Binary size** | Grows with compatibility shims | Small, because only the SDK is supported |

The constraint is the feature: a finite contract can be documented, tested, and made reliable.

---

## 4. The Developer Contract

### 4.1 Source languages

Barista accepts Kotlin and Java source compiled by standard `kotlinc` and `javac`. Standard compilers preserve IDE support, type checking, and debugging. Barista consumes the resulting `.class` files.

### 4.2 The SDK boundary

Developers depend on `barista-sdk`. The SDK is the supported standard library and framework surface. It should be small enough to document completely and stable enough to version deliberately.

The SDK is responsible for:

- HTTP routing and request/response helpers.
- Goroutine and channel primitives.
- JSON, configuration, logging, and common data access.
- Annotations that the compiler can read from class metadata.
- Runtime types that map directly to generated Go types.

Anything outside the SDK is unsupported unless it is explicitly bound through `@NativeGo`.

### 4.3 The Barista subset

The subset is defined by language features and bytecode patterns that the compiler can lower deterministically. Initial rules:

1. **Prefer composition.** Support `data class`, `interface`, and composition. Avoid deep abstract-class hierarchies and complex vtables.
2. **No reflection.** Reject `java.lang.reflect`, dynamic proxies, runtime annotation scanning, and classpath scanning.
3. **No dynamic class loading.** Reject custom class loaders, runtime bytecode generation, and instrumentation.
4. **No arbitrary JVM libraries.** Only the SDK, the documented runtime shims, and `@NativeGo` bindings are supported.
5. **Explicit concurrency.** Use `barista.async` primitives rather than raw `Thread`, `ExecutorService`, or JVM locking.
6. **AOT-friendly configuration.** Generate wiring, serializers, and route tables at compile time.
7. **Deterministic output.** The same inputs and SDK version must produce the same Go source.

The subset will evolve, but additions require a concrete SDK use case and end-to-end tests.

---

## 5. Framework Surface

The following APIs describe the target contract. They are not shipped yet.

### 5.1 Concurrency

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

Compiler mapping:

- `Channel<T>` → Go `chan T`.
- `go { ... }` → `go func() { ... }()`.
- `send` / `receive` → channel operations.
- No continuation state machines, thread pools, or JVM schedulers.

### 5.2 HTTP

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

Compiler mapping:

- Routes are emitted as a static Go route table.
- Handlers become Go functions compatible with `net/http`.
- Request and response helpers wrap `http.Request` and `http.ResponseWriter`.
- No servlet container, reflection, or runtime route discovery.

### 5.3 Native Go interop

```kotlin
@NativeGo(pkg = "github.com/redis/go-redis/v9", symbol = "NewClient")
external fun newRedisClient(options: RedisOptions): RedisClient
```

Compiler mapping:

- Read the annotation from class metadata.
- Add the Go import.
- Emit a direct call to the named symbol.
- Preserve type safety through generated wrapper declarations.

This is the planned escape hatch for databases, queues, cloud SDKs, and other Go-native libraries.

### 5.4 Spring-like AOT surface

Barista should feel familiar to Spring developers without reproducing Spring's runtime. The planned `barista.spring.annotation` package provides a small, compile-time surface:

- `@RestController`, `@Controller`
- `@RequestMapping`, `@GetMapping`, `@PostMapping`, `@PutMapping`, `@DeleteMapping`
- `@PathVariable`, `@RequestParam`, `@RequestBody`
- `@Service`, `@Component`, `@Repository`
- `@Configuration`, `@Bean`
- `@Value` for environment-backed configuration

The compiler reads these annotations from class metadata and emits static constructor wiring, a hardcoded `net/http` route table, direct handler calls, and generated request parsing. No reflection, classpath scanning, or dynamic proxies.

AOP, CGLIB proxies, runtime bean lookup, classpath scanning, and arbitrary Spring Boot starters are permanently out of scope.

### 5.5 IDE linting

A Gradle/Maven plugin should validate the Barista subset in the editor and during normal builds. Examples of diagnostics:

- `java.lang.reflect is not supported. Use Barista serialization instead.`
- `ExecutorService is not supported. Use barista.async.Channel or go { ... }.`
- `Spring annotations are not supported. Use barista.http routing.`
- `This class hierarchy exceeds the supported inheritance model.`

The compiler remains the final authority, but developers should not discover unsupported code after a full native build.

---

## 6. Compiler Strategy

The existing compiler pipeline remains the foundation:

1. Parse class files and resolve the constant pool.
2. Build basic blocks and a control-flow graph.
3. De-stack the operand stack into SSA-style variables.
4. Emit Go AST and inject runtime or SDK shims.
5. Invoke the Go toolchain to produce a static binary.

The Barista-first direction adds four layers:

### 6.1 Whole-application compilation

A real service spans multiple classes. The compiler must ingest `.jar` archives, build a class hierarchy, resolve cross-class calls and fields, and generate the entire module into one Go package. Reachability analysis should prune unused code before lowering.

### 6.2 SDK-aware code generation

The compiler must recognize SDK classes and annotations and lower them directly to Go constructs. This is more reliable than emulating a general JVM API.

### 6.3 Subset validation

Before code generation, the compiler should reject unsupported constructs with actionable diagnostics. Validation should identify the source class, method, and bytecode offset where possible.

### 6.4 Native interop metadata

`@NativeGo` bindings need a deterministic mapping from Kotlin/Java declarations to Go packages, symbols, and types. This metadata belongs in generated code and in the SDK documentation.

---

## 7. Developer Experience

The target workflow:

```sh
# 1. Scaffold a new service
barista new my-api --template kotlin-service

# 2. Develop with an in-memory compile-and-run loop
barista serve src/Main.class

# 3. Ship a single static binary
barista build -o bin/server src/Main.class
```

The experience should feel like a framework, not a bytecode tool:

- `barista new` creates a working project with the SDK and a runnable endpoint.
- `barista serve` provides fast feedback without a JVM.
- IDE linting catches unsupported code before the compiler runs.
- Error messages name the unsupported construct and suggest the Barista alternative.
- Generated binaries are small, static, and cross-compiled with `-target`.

---

## 8. Positioning

Do not position Barista as:

> "A compiler that translates JVM bytecode to Go."

Position it as:

> **"The high-ergonomics backend framework: write Kotlin, ship Go."**

Supporting message:

> Tired of 200MB JVM footprints, JIT warm-ups, and expensive hosting for simple services? Love Kotlin's data classes, null safety, and clean syntax, but not Go's boilerplate? Barista lets you write expressive Kotlin or Java services that compile into a single stripped native Go binary with sub-5ms boot time.

The audience is developers starting new services, APIs, workers, and CLI tools who value Kotlin or Java ergonomics and Go deployment characteristics.

---

## 9. Risks and Mitigations

### "Why not just write Go?"

Go is simple to learn, but some developers prefer Kotlin or Java ergonomics: data classes, null safety, collection pipelines, and mature IDE refactoring. Barista's answer is language experience plus Go operations, not JVM compatibility.

**Mitigation:** Demonstrate concrete ergonomic wins in the SDK and generated code. Keep the generated Go idiomatic and debuggable.

### IDE and tooling expectations

Kotlin and Java developers expect excellent IDE support. Because Barista consumes standard bytecode, standard tooling still works. The gap is knowing which constructs Barista supports.

**Mitigation:** Ship the lint plugin early and document the subset precisely.

### Dependency expectations

Developers expect libraries for JWT, Postgres, Redis, queues, and cloud services.

**Mitigation:** Grow the SDK around common backend needs and make `@NativeGo` the escape hatch for the Go ecosystem.

### Compiler complexity

Every supported language feature and library API adds compiler surface.

**Mitigation:** Keep the subset small, require an SDK use case for every addition, and test each capability end to end.

### Performance claims

Sub-5ms startup and ~15MB RSS are targets, not guarantees for every application.

**Mitigation:** Benchmark real SDK services and publish measured numbers with hardware and workload details.

---

## 10. Success Criteria

Barista is succeeding when a developer can:

1. Scaffold a new Kotlin or Java service with `barista new`.
2. Add HTTP routes, JSON, configuration, and concurrency using only the SDK.
3. Get unsupported-code diagnostics in the IDE before building.
4. Compile to a single static binary with `barista build`.
5. Cross-compile for Linux, macOS, and Windows from one machine.
6. Run the binary with no JVM installed and measure single-digit-millisecond startup.
7. Call a Go library through `@NativeGo` without leaving Kotlin or Java syntax.
8. Read a documented subset and know exactly what is supported.

If a proposed feature does not move at least one of these outcomes forward, it is probably out of scope.

---

## 11. Milestones

These are thematic workstreams, not a promise of sequencing:

- **Whole-application compilation:** JAR ingestion, cross-class linking, and reachability pruning.
- **Subset specification:** document supported language constructs, SDK APIs, and diagnostics.
- **`barista-sdk`:** ship HTTP, async, JSON, configuration, and logging APIs.
- **Spring-like AOT surface:** `barista.spring` routing and wiring annotations with zero reflection.
- **Lint plugin:** integrate subset validation into Gradle and Maven.
- **Scaffolding:** add `barista new` with a runnable service template.
- **Native interop:** implement `@NativeGo` metadata and code generation.
- **Conformance suite:** compile and run every documented SDK promise.
- **Benchmarks:** publish startup, memory, and binary-size measurements.

Progress against these workstreams belongs in [`progress.md`](progress.md); the delivery sequence is in [`evolution-plan.md`](evolution-plan.md).

---

## 12. Relationship to Existing Work

The existing class-file parser, IR, code generator, runtime shims, builder, and CLI are not discarded. They are the compiler foundation for the Barista-first framework.

What changes is the target:

- Before: compile arbitrary JVM bytecode and expand compatibility shims indefinitely.
- Now: compile the Barista subset and make the SDK contract productive.

Technical case studies remain valid. They document how the compiler works, independent of the product scope. Compatibility work should now be justified by a specific Barista-first use case rather than by the existence of a JVM API.
