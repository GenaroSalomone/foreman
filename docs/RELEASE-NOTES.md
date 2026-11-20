# foreman 1.2.0-rc.2

foreman 1.2.0-rc.2 puts the review on the screen and keeps the machine clean on its own: UI builds get a QA reviewer, and leftover worktrees and caches stop piling up.

## In short

- **A UI build is reviewed by looking at it.** The review list takes `qa`: the reviewer runs the lane's `qa-tester` on a dev server at the reviewed head, at 375 and 1440 px, and a UI diff gets it by default even with an explicit `review:` list.
- **Leftovers clean themselves up.** `hw reap --apply` removes dead-executor worktrees, old detached cache trees and stale reported ones (archiving uncommitted work first, never deleting a branch), and `hw status` starts it once a day when a lane is over its threshold.
- **The suite reruns only what changed.** Each bash subject is keyed to the files it read, stale rows are re-measured on their own, and the release verdict is the one cold run.
- **The bottleneck is measured daily.** `hw status` prints the test-wait share, cache replay and review rounds, with an `ALERT` line when one crosses its threshold.
- **Review rounds count right.** A fix brief knows its parent build, and a second `hw review` of the same head dispatches nothing.

## What changed

17 changes since 1.2.0-rc.1.

### Added
- `hw reap --apply` removes what it used to keep, so leftover worktrees stop piling up: a worktree whose hw lock outlived its
  dead executor (no pane in it), a detached tree under the cache (a gate, a cut, a triage base) once it is over a day old,
  and the dirty or unmerged worktree of a task that reported over a day ago. Before a dirty or unmerged tree goes, its
  uncommitted diff, untracked files and unrebuildable ignored files are copied and verified into
  `<archive>/<lane>/<date>/<task>/`; its branch is never deleted. A live pane, a live lease and a lock of anyone else's
  still keep a tree. What reap archived is pruned after 30 days.
- `hw status` starts `hw reap <lane> --apply` in the background (`nice`, never waited for) when a lane is over its worktree
  threshold, at most once a day per lane; the `! N worktrees … hw reap` line says that a reap was started.
- The independent review of a UI build looks at the screen: the `review:` list takes `qa`. The reviewer runs the lane's own
  `qa-tester` agent once, on the dev server hw starts on its worktree at the reviewed head (`--dev-server` is passed for a lane
  whose dev server is opt-in), with the build brief's `## Do` and `## Verification` as its cases, at 375 and 1440 px. One
  browser launch, closed with `bin/browser-close`; the captures go into the PR description's one `## Review` section through
  `bin/gh-attach`, never a comment; a failed case is a blocker. done-invoker proves the Agent call of `qa-tester` from the
  reviewer's transcript and refuses a `review-skip:` of it.
- `qa` is refused at dispatch in a lane with no `qa-tester` (`<checkout>/.claude/agents/qa-tester.md`) or no dev server
  (`devserver.start` always or opt-in), and by `hw review --here`, which starts none: every path to the reviewer names what is missing.
- A lane's `ui_paths` (projects.json: a `dir/` prefix, a base-name glob such as `*.tsx`, or a whole-path glob) says what its diffs
  call UI. A build whose diff touches it gets `qa` by default in a lane that has `qa-tester` and a dev server, unless its
  `review:` is a list or `none`. The two lanes that have a `qa-tester` set it.
- `hw status` says, once a day, whether the build → merge bottleneck is gone: one line with the last report and its date
  (test-wait share of build wall, verdict-cache replay overall and per actor, review and fix rounds per build), and an
  `ALERT` line naming every number over its threshold (`setup/measure/throughput-thresholds.json`: test-wait > 50%,
  replay < 50%, review rounds > 2, fix rounds > 1). The measurement is `setup/measure/throughput.py`: read-only over
  the last 24 h of transcripts, the one-test guard's log and the runner's `verdict cache` lines; `hw status` starts it
  in the background (`nice`) when the last report is over a day old and never waits for it.

### Changed
- A reported task's artifacts are removed after 14 days instead of 30 (`retention.artifact_retention_days`; backups keep 60).
- The release's full-suite verdict is the one cold run. `setup/verify-for-push` records `cold=1` in the full verdict it
  caches when `HW_TEST_NO_CACHE` was in force for the whole run and `cold=0` otherwise, and `setup/release/cut`'s `test-hw`
  gate refuses a `cold=0` verdict, or an incremental one whose base is `cold=0`, naming the command that records the cold
  one: `HW_TEST_NO_CACHE=1 bash setup/verify-for-push --full`. The push gate and the batch gates keep replaying.
- A build whose diff touches the lane's `ui_paths` gets `qa` added to an explicit `review:` list too (a lane with `qa-tester` and
  a dev server); only `review: none` turns it off. The dispatch manifest says `qa` was added by the UI default and names the
  first UI file. Before, an explicit list such as `review: [tests]` kept the reviewer off the screen.
- `setup/suite-data/subject-deps.txt` is now measured (`setup/measure/subject-deps.py`, access-time method, every
  `setup/tests` subject): `setup/verdict-cache` keys a bash subject to the files it read instead of the whole tree,
  so a subject reruns only when something it read changed. Until now the file was header-only and the cache replayed
  0–1% of a gate. Subjects red in the measuring clone stay unmeasured (keyed to the whole tree).
- A green whole fast gate run outside an executor re-measures its stale rows on its own: `setup/test-hw` starts
  `setup/measure/subject-deps-refresh` detached, which measures them under `setup/heavy` and commits only
  `subject-deps.txt` on main (`setup/commit-tree --onto main`; a refused commit retries once). The gate does not wait,
  and an executor never re-measures: it only falls back to the whole-tree key.

### Fixed
- `bin/statusline-usage-tee` no longer fills `~/.cache/hw/statusline-tail`: a copy cut off by the status line's next refresh
  left a `.<key>.<pid>` file for good (404 files, 1.9 GB measured); a trap removes it, and a refresh no longer rewrites the
  tail of a transcript that did not move (or grew under 256 KB).
- `setup/test-hw` removes the detached merge-base worktree of its label pass when the runner is interrupted or killed inside
  the pass; it used to stay registered in the repository.
- The daily auto-reap no longer removes a detached tree under the cache (`~/.cache/hw`, `hw-run-tmp`) that a live process
  works in: `hw reap` keeps it at any age, and its plan line says why (or that `lsof` could not say). Two `hw status` at
  once now start exactly one background reap: the daily stamp is claimed atomically (`flock`), not checked and then written.
- `review: []` in a build brief is the same as `review: none` for the UI default: a UI diff no longer gets `qa` added to a
  reviewer the brief turned off.
- The review round cap counts each round once and knows a fix brief's parent. A build brief with `base: task/<parent>` and no
  `round_of:` is a round of `<parent>` (a trailing `-rN`/`-fixN` is stripped when that family's brief exists); an explicit
  `round_of:` still wins. A second `hw review` of the same build at the same head, while the first is dispatching or recorded,
  dispatches and records nothing and names the reviewer that owns the round. When `hw next` to the kept reviewer's pane fails,
  `hw review` reports it once and dispatches no second reviewer for that round.
- A measured row of `setup/suite-data/subject-deps.txt` that went stale no longer replays a green. Each row carries the
  stamp it was measured at (the subject plus every library it visibly sources: `setup/verdict-cache stamp`); when the
  stamp moves, `setup/verdict-cache` keys that subject to the whole tree instead of its old read set, so an edit that
  makes a subject or its `_common.sh` read a new file reruns it when that file changes. `verdict-cache stale` lists them.

## Known limits

- macOS and Linux only. Windows, native or under WSL2, is not supported or
  tested (KNOWN-LIMITATIONS, L1).
- The Go core is on by default when the installer builds it; `HW_IMPL=bash`
  goes back. Every limit a user can hit is in
  [`KNOWN-LIMITATIONS.md`](KNOWN-LIMITATIONS.md).

## Upgrading from 1.2.0-rc.1

Run `foreman-setup upgrade --brain DIR`. A brain installed before `upgrade`
existed has no record of its flags yet: run your install command once more,
and `upgrade` works from then on.

Every release's changes are in [`CHANGELOG.md`](CHANGELOG.md).
