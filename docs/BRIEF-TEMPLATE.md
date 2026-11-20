---
# A value is bare: hw reads everything after `key:` (no inline comments). Rare keys
# (release, rehearsal-of, skills, requires-agents, plugins, boundary, placement,
# worktree, database, invoker) and what each key does: `hw help flags`.
# kind: build | explore | audit | review | design
kind: build
# asked: build | investigate
asked: build
requested_by: "<who, date: «the request, quoted»>"
# base: any task whose result is measured
base: main
# model: a check against the dispatch, never a source
model: <alias>
effort: <level>
# Only when they apply:
# review: none (+ review-skip: <why>; absent = the independent review)
# decision: <what is undecided> (design, or opus/fable outside a build)
# requires: browser, subagents, repo, database
# authorizes: <destination> :: <operation> :: <handle|none>
# secrets: [<Bitwarden item>]
# round_of: <parent task> (a fix round dispatched as its own task: hw review re-checks only the delta)
---

# <task-name>

## Goal
<One sentence: what is true when this is done that is not true now.>

## Context
<Only what the executor cannot get from the code, the CLAUDE.md files or the hw prompt:
history, prior attempts, the engram #ids to read first.>
<One line per premise: <claim> — <file:line | command | engram #id>, or ASSUMED.>
<A file the executor must open: `briefs/assets/<file>` (hw copies it to $HW_ARTIFACTS/brief-assets/).>

## Limits
- <What THIS task must not touch or do, and why: who owns it, what breaks.>

## Done when
<build — the rules the behaviour must hold on real data, and what it is FOR:>
- [ ] Rule: <…>. Purpose: <…>.
- [ ] <observable, checkable condition>
<investigate — name this section `## Answer` and say what the answer contains.>

## Verification
```
<the command hw pins at dispatch and runs itself in `hw done`>
```
<A rule that only breaks on real data: a check that runs on that data. A deployed build: a
string only the new build carries. A target that may be unreachable: report the measurement and stop.>

## Mechanism
<Only when the work goes through a tool, MCP, skill, host or credential:>
Mechanism: `<exact name as invoked>`; credential: `secrets: [<item>]` or handle `<handle>`
Precheck:  `<the cheapest call that proves it is live>`
If it fails: report blocked. Not acceptable instead: <the substitutes, by name>.
