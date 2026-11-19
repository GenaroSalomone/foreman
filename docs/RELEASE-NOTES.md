# foreman 1.1.0-rc.1

foreman 1.1.0 closes a run's browsers with it and makes the release cut stand on a full suite run, not the fast gate.

## In short

- **A closed run takes its browsers with it.** `browser-close --run <run-id>` finds every browser a run started, and `hw done`, the report's own close and `hw reap --apply` use it; `hw status` names any that survive.
- **A release needs the full suite.** The cut reads a `gate=full` verdict recorded on its exact tree and refuses without one, so a subject the fast gate skips can no longer ship red.
- **`hw done` stops waiting on nothing.** A task already merged into its base skips the 900 s verification, and a timeout now says what it waited for.
- **Blocked builds resume on a ruling.** A kept pane that reports `--blocked` waits for `hw ruling`, and an expired lease ends that wait cleanly.
- **Reviews get their own checkout.** `hw review` dispatches the reviewer with its own worktree and no longer refuses a task named with "verification".
- **The suite cleans up after itself.** Each job gets its own `TMPDIR`, removed when it ends, and old leftovers are pruned at the start of a run.

## What changed

18 changes since 1.0.0.

### Added
- `browser-close --run <run-id>` closes every browser a run started, found by the run's own marker (`HW_RUN` or its per-run
  `TMPDIR`) in the process environment, so a lane script that picked its own profile and namespace no longer escapes. It lists
  what it closed by pid and profile, and signals nothing while the run still has a live process of its own (exit 3).
- `hw status` names a browser that is still alive for a run that is already closed ("browsers alive for closed runs").

### Changed
- The release cut now checks a FULL-suite verdict recorded outside it instead of running a suite of its own: a `gate=full` verdict (macOS from `setup/verify-for-push` or Linux from `run-suite.sh full`, any platform) for the exact tree, its docs-only cover, or the release-notes cover of an ancestor's. Covered, the cut skips the fast gate (a full run contains it) and says so; not covered, `test-hw` goes red (`REFUSED: no FULL-suite verdict`) naming the subjects no green verdict vouches for. Before, the cut passed on the fast gate alone, which runs only the subjects of 1.7 s or less, so the 1.0.0 cut shipped with 816, 821 and 842 red. The cut's fast-gate reuse, notes-over-`.fast` cover, Linux incremental run and own fast run are gone.
- `setup/release-standard.md` and the cut checklist say the new rule: the full run is the brainer's, once per merged batch (`setup/verify-for-push`), and a release needs it on the merge tree first.

### Fixed
- `hw done`, the close `done-invoker` starts and `hw reap --apply` now take the run's browsers with them. Before, the headed
  Chrome for Testing windows of a lane's QA script outlived their executor for days.
- Subjects 816 and 821 (the contract and anchors of receipt/log/handle/help) were retired and ran in no gate, so their drift went unseen; they run in the full suite again.
- An ambient `HW_TEST_ONLY`, `HW_TEST_RESULTS` or `HW_TEST_CHANGED` can no longer turn the export's fast gate (the cut's one suite) into a partial run.
- `hw review` no longer refuses the brief it just wrote when the reviewed task's name contains "verification" (for example `hw-done-900s-verification`). The generated title `# Independent review of <lane>:<task> @ <sha>` was read as a Verification section with no runnable command; the dispatch check now skips a level-1 document title. A `## Verification` heading still counts, first or not.
- `hw done`'s verification timeout now says what it waited for: the command, its last output line, and why `setup/test-hw` with no changed paths (an empty `git diff`, a merged branch) runs the whole fast gate, 8-12 minutes, rather than a hang.
- A branch merged into the lane's own base ref (`base_ref_prefix` + `base`, not a hard-coded `origin/`) is still skipped by `hw done`'s verification, and one merged only into a local base of a lane that reads `origin/` is verified.
- `hw review` skips front matter before looking for the brief's title, so a comment line there no longer hides the title from the Verification check.
- `hw done` no longer spends its whole 900 s verification ceiling on a task that has nothing left to verify. A clean checkout whose HEAD is already an ancestor of the lane's base (merged by `setup/merge-task` or a PR) is receipted `verify_run skipped — merged`, and a task with no checkout of its own (reaped) is receipted `not run — … holds no checkout of this task` instead of running the gate against the lane's main checkout. An unmerged branch is still verified.
- A build that reports `done-invoker --blocked` while its pane is kept (chaining lease or legacy keep) is now marked waiting for a ruling, so `hw ruling` resumes it as the executor prompt promises; the lease only decides how long the pane lives. The blocked report names whichever clock fires first (the lease's expiry or `hw reap`).
- A chaining lease that expires and closes the pane of a blocked task now ends the wait (`blocked-waiting` becomes `blocked-closed`) the way `hw done` does, so `hw status` and the cockpit stop offering a ruling to a closed pane.
- The top-tier refusal on explore/audit/design now names the real source of the model when a lane floor or a lane default chose it («<lane>'s floor is opus» / «<lane>'s lane default is opus») instead of saying the task «is declared» top tier, and for a floor it points at `decision:` / `--model-why` or `--model sonnet` with `--below-floor-why`, never a bare `--model sonnet` the floor would refuse.
- `hw review` now dispatches the reviewer with `--worktree new` (not under `--here`). The reviewer's brief says `asked: investigate`, which since the investigate default means `worktree none`: the reviewer got no checkout, so its `git rev-parse HEAD` check could not pass and `done-invoker`'s head gate read the workdir HEAD as `<unreadable>` and refused the close.
- The suite's temp root (`~/.cache/hw-test-<uid>`) no longer fills up: `setup/test-hw` gives each job its own `TMPDIR` and removes it after the job, whatever its exit (a KILLed subject, or one whose own `trap … EXIT` drops `$TMP`, leaves nothing); a contract world is removed when its case gets TERM or INT; the pi driver and the test-built `hw-core` remove their scratch directories.
- New `setup/hw-test-prune`, run by `setup/test-hw` at the start of a run: removes entries older than 24 h that no live process holds (versioned `*.vN` caches are kept) and says how many.

## Known limits

- macOS and Linux only. Windows, native or under WSL2, is not supported or
  tested (KNOWN-LIMITATIONS, L1).
- The Go core is on by default when the installer builds it; `HW_IMPL=bash`
  goes back. Every limit a user can hit is in
  [`KNOWN-LIMITATIONS.md`](KNOWN-LIMITATIONS.md).

## Upgrading from 1.0.0

Run `foreman-setup upgrade --brain DIR`. A brain installed before `upgrade`
existed has no record of its flags yet: run your install command once more,
and `upgrade` works from then on.

Every release's changes are in [`CHANGELOG.md`](CHANGELOG.md).
