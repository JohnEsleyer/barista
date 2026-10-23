# JVM switch payloads are 4-byte aligned relative to the method start

**Date:** 2026-10-23
**Area:** ir
**Status:** Resolved

## Problem

Phase 4 needed to decode `tableswitch` and `lookupswitch`. A naive decoder that reads the opcode followed immediately by the default offset misreads the payload whenever padding is present, producing garbage case counts or invalid branch targets.

## Context

- JVMS `tableswitch` opcode `0xaa`
- JVMS `lookupswitch` opcode `0xab`
- Barista decodes a method's `code[]` slice starting at bytecode offset 0

The layout is:

```text
tableswitch
  <0-3 bytes padding>
  defaultbyte1..4
  lowbyte1..4
  highbyte1..4
  jump offsets...

lookupswitch
  <0-3 bytes padding>
  defaultbyte1..4
  npairs1..4
  match/offset pairs...
```

## Investigation

The first attempt is easy to get subtly wrong in two ways:

1. Aligning relative to the switch opcode instead of the byte after the opcode.
2. Treating branch offsets as relative to the end of the switch payload.

The JVM spec requires the padding so that the default offset starts on a 4-byte boundary measured from the beginning of the current method's code array. Switch jump offsets are relative to the address of the switch opcode itself, not the end of the payload.

## Root Cause

The payload is variable-length and alignment depends on the absolute bytecode offset of the switch. `opPC` must be preserved for both padding and target calculation.

## Resolution

`parseSwitch` in `internal/ir/stack_eval.go`:

```go
cursor := opPC + 1
padding := (4 - (cursor % 4)) % 4
cursor += padding
defaultOffset := int32(binary.BigEndian.Uint32(code[cursor : cursor+4]))
cursor += 4
// ...
target := opPC + int(offset)
```

`decodeInstruction` returns `nextPC - pc` as the decoded size, and leader collection records the default plus every case target.

## Verification

- `TestLowerBytecodeTableSwitch` builds a hand-written `tableswitch` with two padding bytes and checks the default and case targets.
- `TestIntegrationSwitch` compiles `Switch.java` and runs both `tableswitch` and `lookupswitch` output.
- Generated Go uses `pc_<offset>` labels and a Go `switch`.

## Guidance for Future Agents

- Always decode switch payloads from the switch opcode's absolute offset.
- Validate that the declared case count does not read past `len(code)`.
- Keep a hand-written bytecode unit test for switch decoding; integration tests alone make alignment bugs hard to localise.

## References

- `internal/ir/stack_eval.go` — `parseSwitch`, `decodeInstruction`
- `internal/ir/stack_eval_test.go` — `TestLowerBytecodeTableSwitch`
- `tests/fixtures/java/Switch.java`
- `tests/integration_test.go` — `TestIntegrationSwitch`
