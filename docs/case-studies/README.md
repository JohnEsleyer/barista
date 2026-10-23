# Case Studies

Short, searchable records of non-obvious problems encountered while building Barista and how they were resolved. The goal is to give future contributors — human or AI — enough context to avoid repeating the same investigation.

Each case study follows the naming convention, structure, and quality bar defined in [`../conventions.md`](../conventions.md).

## Index

| Date | Area | Title | Status |
| --- | --- | --- | --- |
| 2026-10-23 | codegen | [Go infers untyped integer literals as `int`](2026-10-23-go-untyped-integer-constants.md) | Resolved |
| 2026-10-23 | ir | [JVM branch offsets are relative to the opcode, not the next instruction](2026-10-23-jvm-branch-offset-origin.md) | Resolved |
| 2026-10-23 | codegen | [Go rejects goto across SSA variable declarations](2026-10-23-go-goto-over-declarations.md) | Resolved |

When adding a case study, add a row above and keep the table ordered newest first.
