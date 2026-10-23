# Mapping JVM HttpServer and HttpExchange to Go net/http

**Date:** 2026-10-24
**Area:** runtime, codegen
**Status:** Resolved

## Problem

Phase 5 requires ahead-of-time compiled JVM HTTP servers to execute without any JVM runtime dependencies. Traditional JVM HTTP servers depend on `com.sun.net.httpserver.HttpServer`, `HttpHandler`, and `HttpExchange` with blocking streams and explicit response header handshakes.

The goal is to map these JVM entry points onto Go's idiomatic `net/http` standard library while maintaining sub-millisecond boot latency, lightweight memory footprint, and standard Go goroutine scheduling.

## Context

- Go 1.24+
- Java SE built-in HTTP server (`com.sun.net.httpserver.*`)
- Fixture: `tests/fixtures/java/RestServer.java`

## Investigation

Three strategies were analyzed:

1. **Custom Socket Parsing:** Writing a raw HTTP/1.1 parser over `net.Listener`. This introduces custom state machines, lacks HTTP/2 readiness, and wastes Go's mature `net/http` package.
2. **Subprocess/Proxying:** Forwarding traffic to an external daemon. This violates Barista's core promise of a single static binary.
3. **Native Adapter Layer:** Bridging `com.sun.net.httpserver.HttpServer` directly to Go's `http.ServeMux` and `http.Server`. Each incoming request spawns a lightweight Go goroutine where a generated `HttpExchange` shim wraps Go's `http.ResponseWriter` and `*http.Request`.

## Root Cause

JVM `HttpServer` uses explicit method calls (`createContext`, `start`, `sendResponseHeaders`, `getResponseBody().write()`). Go's `net/http` uses callback handlers (`http.HandlerFunc`).

Furthermore, in Java, classes implement `HttpHandler` explicitly (`class RestServer implements HttpHandler`), which invokes `handle` via `invokeinterface` (opcode `0xb9`).

## Resolution

1. **Interface Lowering:** Added `BytecodeInvokeinterface` (opcode `0xb9`, 5 bytes) to `internal/ir/stack_eval.go`, lowering directly into `OpInvokeVirtual` because Go's interface and pointer-receiver dispatch syntax are identical.
2. **Go-Native HTTP Engine:** Built `pkg/runtime/java/net/http_server.go`:
   - `HttpServerCreate` initializes a Go `http.ServeMux` and `http.Server`.
   - `CreateContext(path, handler)` registers a Go `http.HandlerFunc`.
   - On request arrival, an `HttpExchange` wraps `w` and `r`.
   - `exchange.SendResponseHeaders` copies Java headers and calls `w.WriteHeader`.
   - `exchange.GetResponseBody()` writes directly through an `OutputStream` wrapping `w`.
   - In Go, any class with `Handle(*HttpExchange)` satisfies Go's `HttpHandler` duck typing automatically.
3. **Zero-Overhead Memory:** Java byte arrays (`[B`) map directly to Go `[]int8`, with streaming helpers avoiding intermediate buffers where possible.

## Verification

- `go test ./pkg/runtime/java/net/...` passes.
- Unit and integration tests verify:
  - Dynamic ephemeral port binding (port 0) with `server.GetAddress().GetPort()`.
  - HTTP GET response headers and JSON body serialization.
  - HTTP POST request body streaming with `is.readAllBytes()`.
  - Clean server shutdown (`server.stop(0)`).

## Guidance for Future Agents

- Do not reimplement HTTP parsing in runtime shims; Go's `net/http` is production-hardened and handles TLS, timeouts, and chunked transfer encoding out of the box.
- Java interface method calls (`invokeinterface`) lower into standard virtual calls in Barista IR because Go interface dispatch requires no JVM-style vtable lookups.
- Keep the `jnet` package name distinct from Go's standard `net` to avoid namespace collisions in generated code.

## References

- `internal/ir/stack_eval.go`
- `internal/codegen/type_map.go`
- `pkg/runtime/java/net/http_server.go`
- `pkg/runtime/java/net/net_test.go`
- `tests/fixtures/java/RestServer.java`
