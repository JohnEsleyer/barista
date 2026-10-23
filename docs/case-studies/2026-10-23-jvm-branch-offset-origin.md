# JVM branch offsets are relative to the opcode, not the next instruction

**Date:** 2026-10-23
**Area:** ir
**Status:** Resolved

## Problem

Lowering `Calculator.sumTo(int)` failed during validation with:

```text
invalid bytecode in sumTo: branch target pc 7 is not an instruction boundary
```

`javap` clearly showed the loop-back `goto 4` and the exit branch `if_icmpgt 19`, so a target of `7` was impossible.

## Context

- Go 1.26.1
- JDK 17.0.20.1 (`class` major version 61)
- Fixture: `tests/fixtures/java/Calculator.java`
- Failing method bytecode:

```text
03 3c 04 3d 1c 1a a3 00 0d 1b 1c 60 3c 84 02 01 a7 ff f4 1b ac
```

## Investigation

The bytecode scanner was computing every branch target as:

```go
target := pc + instructionSize + offset
```

For the `goto` at pc 16 with offset `0xfff4` (`-12`), that produced `16 + 3 - 12 = 7`. The `if_icmpgt` at pc 6 with offset `0x000d` (`13`) produced `22`.

Both values were wrong, which made the failure look like a decoding bug rather than an offset-origin bug.

## Root Cause

JVMS branch offsets are signed offsets from the address of the branch opcode itself, not from the instruction after the operands:

```text
target = branch_opcode_pc + offset
```

For the `goto` at pc 16: `16 + (-12) = 4`.
For the `if_icmpgt` at pc 6: `6 + 13 = 19`.

## Resolution

Anchor all branch calculations to the opcode address:

- In the bytecode scanner: `target := pc + offset` where `pc` points at the opcode.
- In lowering: capture `opPC := pc - 1` immediately after reading the opcode, then use `target := opPC + int(offset)`.
- Apply the same rule to `goto_w`.

## Verification

- The scanner now accepts all `Calculator` methods.
- `TestLowerBytecodeConditional` asserts the branch shape for `absolute(int)`.
- The integration test compiles and runs the generated module; `SumTo(10)` returns `55`.
- `barista grind` shows `if v2 > v3 goto L1` and `goto L0` for `sumTo`.

## Guidance for Future Agents

- Never assume a branch offset is relative to the end of the branch instruction.
- When a branch target looks impossible, dump the raw `Code` bytes and compare them with `javap -c -p`.
- Validate every branch target against the set of instruction-start PCs. This turns a silent misparse into an immediate, actionable error.

## References

- JVMS §4.9.1 and the branch instruction definitions in §6.5
- `internal/ir/stack_eval.go`
- `tests/fixtures/java/Calculator.java`
