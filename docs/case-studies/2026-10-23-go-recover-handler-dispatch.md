# Go recover cannot resume at a JVM handler offset

**Date:** 2026-10-23
**Area:** codegen
**Status:** Resolved

## Problem

JVM exception tables identify catch handlers by bytecode offset. The generated Go needs to recover a panic and continue at the matching `handlerPC`, which is an arbitrary label in the method body.

A method-level `defer` with `recover()` stops the panic, but the deferred function cannot `goto` a label in its enclosing function. Returning from the deferred function also cannot resume the original instruction stream.

## Context

- Go 1.24
- Generated from `tests/fixtures/java/TryCatch.java`
- JVM exception table: `[0,18) -> handler pc 21, java/lang/RuntimeException`

## Investigation

Three approaches were considered:

1. A method-level `defer` that jumps to the handler. Go forbids jumping from a deferred function into the outer function.
2. Duplicating handler bytecode inside the deferred function. This works for simple handlers but cannot reliably continue normal control flow or share locals.
3. Wrapping each protected range in an immediately-invoked closure, recovering inside that closure, then dispatching in the outer function.

## Root Cause

`recover()` only stops panicking. It does not provide a continuation or a way to transfer control into the outer function's label space. The JVM handler offset must therefore be represented as data that the outer function can consume after the protected closure returns.

## Resolution

For each distinct `[startPC,endPC)` exception range:

1. Reset `handlerPC = -1`.
2. Emit an immediately-invoked Go closure containing the protected instructions.
3. Install a `defer` in that closure that calls `recover()`.
4. Match the recovered value with `lang.MatchesException`.
5. On a match, set the shared `exc *lang.Throwable` and `handlerPC` variables.
6. Re-panic if no trap matches.
7. After the closure, emit `switch handlerPC { case 21: goto L1 }`.

The handler label itself remains in the outer function. Its first IR operation is `LOAD_EXCEPTION`, which reads the shared `exc` variable. All SSA values and locals are predeclared when a method has exception traps so the `goto` is legal.

The generated shape is:

```go
handlerPC = -1
func() {
    defer func() {
        if r := recover(); r != nil {
            switch {
            case lang.MatchesException(r, "java/lang/RuntimeException"):
                exc = lang.AsThrowable(r)
                handlerPC = 21
            default:
                panic(r)
            }
        }
    }()
    // protected bytecode
}()
switch handlerPC {
case 21:
    goto L1
}
```

## Verification

- `go test ./...` compiles and runs the generated `TryCatch` package.
- `Run(false)` returns `1` through the normal path.
- `Run(true)` returns `2` through the caught `RuntimeException`.
- The generated source contains `recover()`, `MatchesException`, `AsThrowable`, and a handler `goto`.

## Guidance for Future Agents

- Do not try to `goto` out of a deferred function.
- Model JVM handler offsets as explicit state (`handlerPC`) and dispatch after recovery.
- Keep all names predeclared before any handler `goto`.
- The current implementation rejects returns inside protected ranges and branches out of a protected range. Extending those cases requires a more general state-machine or structured CFG lowering rather than a larger deferred closure.

## References

- `internal/codegen/function.go` — `emitTryGroup` and `recoverDefer`
- `internal/ir/stack_eval.go` — exception-table lowering and handler injection
- `tests/fixtures/java/TryCatch.java`
- Go specification: `recover`, `defer`, and `goto`
