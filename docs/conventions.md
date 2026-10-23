# AI Agent Conventions

This document defines working conventions for AI agents contributing to Barista. Its purpose is to preserve engineering context so future agents can build on what has already been learned instead of rediscovering it.

## Progress Tracking

[`docs/progress.md`](progress.md) is the canonical development tracker. At the end of every meaningful task:

- Update the current phase and status.
- Check off completed capabilities only when a test or reproducible command proves them.
- Add or remove known limitations.
- Record the verification result and the latest implementation commit.
- Set the exact next task.
- Link any new case study.

Do not mark a phase complete based only on code being written. The tracker must reflect verified behaviour.

## The Case Study Rule

### Rule

Any agent that encounters a **unique, non-obvious problem** MUST write a short case study about it before considering the task complete.

A problem qualifies as unique and non-obvious if any of the following are true:

- It required meaningful investigation — multiple attempts, reading tool or language internals, or roughly 15 minutes or more of debugging.
- The observed behaviour contradicted documentation, expectations, or prior assumptions.
- The solution is a workaround, or depends on a specific tool, runtime, or platform version.
- The problem is likely to recur while developing, testing, or shipping Barista.
- Existing documentation and case studies do not already cover it.

Do **not** write a case study for typos, routine compile errors, or problems already documented. If a relevant case study exists, link to it and add new findings there instead of duplicating it.

### Location and naming

Write case studies under:

```text
docs/case-studies/YYYY-MM-DD-short-kebab-case-title.md
```

Use the date the problem was diagnosed and a descriptive title. Create `docs/case-studies/` when the first case study is added. Add a row for the new case study to the index in [`docs/case-studies/README.md`](case-studies/README.md).

Keep each case study small — usually 50–150 lines. Optimise for the next agent: include exact errors, versions, commands, the root cause, and reusable guidance. A short, precise note is more valuable than a long narrative.

### Required structure

```markdown
# <Descriptive title>

**Date:** YYYY-MM-DD
**Area:** classfile | ir | codegen | runtime | tooling | build | other
**Status:** Resolved | Workaround | Open | Superseded

## Problem

What failed, and what was the observable symptom?

## Context

Relevant versions, environment, input, and minimal reproduction.

## Investigation

What was tried, and which hypotheses were ruled out? Keep this brief.

## Root Cause

The actual explanation, distinct from any workaround.

## Resolution

The fix or workaround, including commands or code snippets.

## Verification

How the fix was confirmed. Include the exact command where possible.

## Guidance for Future Agents

The reusable lesson. What should the next agent check first?

## References

Related files, upstream issues, documentation, and other case studies.
```

### Quality requirements

- Record exact version numbers (Go, JDK, `kotlinc`, OS) when they matter.
- Include exact error messages and a minimal reproduction rather than paraphrasing.
- Clearly separate the root cause from the workaround.
- Add verification steps so a future agent can prove the conclusion still holds.
- Link to the relevant source files, tests, and upstream discussions.
- Never include credentials, tokens, personal data, or other secrets.
- If new findings invalidate an existing case study, update it and mark it `Superseded` rather than leaving conflicting guidance.
- Add the case study to the index in `docs/case-studies/README.md`.
- Commit the case study with the fix, or immediately afterwards if the fix must land first.

### Before starting non-trivial work

Search for existing context first:

```sh
ls docs/case-studies/
rg -i "<keyword>" docs/case-studies/
```

If the problem is already covered, follow the documented resolution or build on it. Do not repeat the investigation from scratch.

### Completion checklist

- [ ] I searched `docs/case-studies/` for related findings.
- [ ] I confirmed this problem is not already documented.
- [ ] I wrote a case study using the required structure.
- [ ] I added the case study to the index in `docs/case-studies/README.md`.
- [ ] I included versions, symptoms, root cause, resolution, and verification.
- [ ] I linked the relevant files and existing case studies.
- [ ] I committed the case study with the change.
