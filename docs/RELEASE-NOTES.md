# foreman 1.0.0-rc.2

foreman 1.0.0-rc.2 makes a reboot a non-event: the herdr app brings every lane's last brainer back on its own, and the toolchain stops trusting stale state left behind by the restart.

## In short

- **After a reboot, the brainers come back by themselves.** The cockpit mod runs `brain restore --on-start` on every app start: each lane's last conversation is resumed, the setup brainer last, and a LaunchAgent installer starts `engram serve` at login.
- **Stale state after a restart is read as stale.** `hw revive` and `brain restore` count a session as live only when its process runs, an unreadable terminal stays unknown, and `setup/heavy` frees slots held by runs that died in the reboot.
- **One command merges a task into main.** `setup/merge-task` applies what a branch deleted or renamed, measures the result, and `hw train push` advances the base through it without deleting ignored files.
- **The brain guard sees through `nice` and `timeout`,** and the repo-write guard opens a lane's untracked `.claude/settings.local.json`.
- **The Agents list in the app can be reordered by dragging** rows and whole spaces, and product worktrees get the main checkout's local skills.

## What changed

44 changes since 1.0.0-rc.1.

### Added
- The Agents list in the herdr app can be reordered by dragging a row, as the Spaces list can: inside its group
  (a space, or Pinned), never the brainer, and the order is kept across restarts. The priority sort keeps its
  urgency order and has no drag.
- `brain restore [<lane>…] [--dry-run]`: after a reboot, one command resumes every
  lane's last conversation (the newest transcript under the lane's directory, in the
  lane's own account) into its `brain-<lane>` herdr-core terminal. A lane that already
  runs claude, or has nothing to resume, is skipped and named.
- A brainer's SessionStart now starts its cockpit writer (`cockpit-state --loop 5 --bind-pane`) for its
  pane, so a brainer resumed by a bare `claude --resume` is no longer drawn as «NO STATE». Idempotent
  (the writer's own lock); not for an executor or with `FOREMAN_COCKPIT=0`.
- `setup/merge-task <branch> [-m msg]`: the brainer's merge into main as one command. It computes the merge, refuses a conflict or a dirty path it would
  touch (added, modified or deleted), refuses a `main` that moved after the merge was computed (git's own fast-forward check, not an exact compare-and-swap: KNOWN-LIMITATIONS L35), and updates the checkout **including the files the branch deleted or renamed**,
  which the hand-typed `merge-tree`/`commit-tree`/`checkout <merge> -- <paths>` recipe left on disk (a stale `824-…sh` beside its renamed copy
  stopped the batch gate).
  It is not atomic (git writes the checkout, then moves the ref): after the merge it measures main, HEAD and the checkout and exits 0 (consistent), 1 (nothing written) or 3 (NOT CONSISTENT, with the measured state and the one `git read-tree` that reconciles it); a ref lock after the checkout was written is no longer reported as «Nothing was changed». Documented as L35 in KNOWN-LIMITATIONS.
  It also refuses, before writing anything, a path whose name holds a newline or a `\001` byte and an unstaged deletion of a path the merge modifies (git would put the file back without a word), and lists a dangling symlink git could not remove as STILL ON DISK.
- `brain restore --on-start`: the cockpit mod runs it on every start of the herdr app, so after a reboot every lane's last
  brainer comes back without anyone typing. It detaches, waits up to a minute for the core, and under a lock restores each
  lane once per core holder: a lane is stamped only when it settled (restored, already running, or nothing to resume), so
  a terminal the core had not listed yet, or could not report, is looked at again on the next start. It logs to
  `~/.cache/hw/brain-restore.log`, and the cockpit logs the outcome of each start.
- `setup/install/install-engram-serve-agent.sh`: a LaunchAgent that starts `engram serve` at login unless a serve already
  answers on its port.
- A product lane's worktree (the one with local-only skills) now gets the main checkout's git-ignored skills (`.claude/skills/` and `.agents/skills/`) as
  symlinks, so an executor can load `testing`, `impeccable`, `agent-browser` and the rest without absolute paths.
  Tracked or already-present entries are never touched; `prod-*` skills stay out, as `.env.prod.local` does.
- Brief field `skills: [name, …]`: each name must resolve for the executor (worktree `.claude/skills/<name>/SKILL.md`
  after replication, or the account's user-level skills dir) or `hw` refuses the dispatch before building anything.

### Changed
- The Agents list in the herdr app (Grouped by space) now drags whole spaces: drop a row, the brainer included, on a row of another
  group and its space moves there, with the same `space.move` the Spaces list sends, so both lists agree and the order is kept like
  any space's. The groups are drawn in the Spaces list's order. Dropping on a row of its own group still reorders it (the brainer stays
  first); Pinned and the priority sort behave as before. The dragged row, or its whole group, dims and an insertion line marks where it lands.
- The Agents list in the herdr app (Grouped by space) now drags a space by its header: press a space's header and drag it over another group
  and the whole space moves there, with the same `space.move` the Spaces list sends. The insertion line is drawn between groups and the
  dragged group dims entirely; a click on a header still folds it. Agent rows no longer cross groups (dropped on another group they do
  nothing); reordering inside a group is unchanged. Pinned and the priority sort have no header drag.
- `brain <lane> --resume <id>` and `brain relaunch` use the herdr-core terminals when the
  caller is not itself an app terminal, the herdr server is not running and the core
  answers, instead of refusing on `herdr workspace list`. Where the herdr server is up
  nothing changes.
- `hw status`, `hw done` and `hw reap` read a worktree's occupants from the core's terminals
  (by cwd) when the herdr server is not running: `hw status` no longer prints
  `herdr unreachable`, `spaces · unknown` or a wall of `UNDETERMINED` rows while the core answers.
- The repo-write guard (Claude hook, OpenCode plugin, Codex guard, the executor guard) opens one more path inside a product
  repo: `<repo>/.claude/settings.local.json`, Claude Code's own per-user permission file, so a brainer can remove a stale
  «always allow» rule from it. Only that exact file, only while git does not track it, and never through a symlink or a
  hard link; `.claude/settings.json`, any other file under `.claude/` and a tracked `settings.local.json` stay shut.
- `hw train push` moves the checked-out base with `setup/merge-task --base <b> --advance <old> <new>`, the one guarded update `setup/merge-task` already uses, instead of a second implementation of it. The end state is measured from git (the ref, HEAD, the index and the disk over every touched path, modified, deleted and added ones alike) and the exit follows it: a write that fails half-way (a read-only directory, a full disk) is reported as NOT CONSISTENT with the one `git read-tree` that reconciles it, never as «the checkout is unchanged». It attempts no rollback, so it can no longer overwrite ignored data, write a carried edit over a half-written checkout, or roll back under a HEAD that left the base (another branch, a detached HEAD): those are measured and named. A local edit the train also changes is restored when nothing was written, written back when the advance succeeded, and left in `<git common dir>/hw-train/advance-bak-<pid>/` (named in the message) when the checkout is not consistent.
- `setup/merge-task` takes `--base <branch>` (another branch than main, which must be what HEAD holds) and `--advance <old> <new>` (a fast-forward the caller built; `<old>` must still be what the base holds).

### Fixed
- The brain guard skips a `nice -n` value or a `timeout` duration only when it is a literal number: a shell-expanded word there (`{10,tee}`, `$(…)`, `$V`) is judged as the verb, so it can no longer hide a write into brain.
- The brain guard sees through `nice` with its options: `nice -n 10 rg x <brain path>` (and `-n10`, `-10`,
  `--adjustment=10`, `--adjustment 10`) is judged as the bare `rg`, so a read stays allowed; before, it took `10`
  for the verb and refused the very form `brain/CLAUDE.md` tells executors to use. A write behind `nice` stays refused.
- `channel-send`/`done-invoker` to a brainer in an herdr-core terminal no longer exit 5 (delivery
  unconfirmed) while the brainer is mid-turn: the receiver's own transcript, which claude writes at
  once, proves the landing (any account), and the terminal's counters stay the fallback.
- `setup/e2e-cheap-executors` runs from an herdr-core terminal (`HC_TERMINAL_ID`) without
  `HERDR_ENV=1`, and no longer passes the caller's `HC_*` variables to the isolated server's
  children.
- The test runner's source snapshot no longer aborts a whole gate because a lane brainer edited its own
  `decisions.md`, a brief or an archive during the capture: a change confined to lane documentation the suite
  does not read (the pre-push cover's rule; a `decisions.md` only while its header is unchanged) is tolerated and
  named, and anything else is retried up to 3 times before the run is refused, naming the paths that kept changing.
- `brain` re-pointing the runs of a replaced brainer no longer takes a run that names no core (`hc-core` row absent) because THIS core's
  `hc ls` does not list its terminal: that address may be a brainer of another core, and its reports would then be delivered here. Only a
  run recorded on this core is judged by this core's listing; every run is still taken by a finished record or by the reset that ended it.
- `brain <lane> --resume`/`--reset` on the app no longer starts a second brainer beside one that may be live: `hc get` is read as
  "gone" only for a record with its address and state `finished` (the one predicate `hcRecordGone`, also used by `brain restore`).
  A reply with no known gone state (`{}`, an error object, `starting`, `lost`, `exited`) is unknown, and `brain` refuses.
  The detached `--await-idle` arm keeps waiting on such a reply, and after `hc kill` the wait for the terminal to end needs a
  finished record, not just an unread one (`hc rm` now runs once the record is finished).
- `exited` is no longer read as a gone state: `hc help` names only `live` and `finished`.
- The occupancy merge of the app core's terminals compares addresses and renders each `(hc, live)` line once.
- Tests can no longer reach the machine's real `hc`: the test run puts a refusing `hc` first on PATH.
- `hw done` of an app run no longer closes past its in-use check when `hc get` is unknown, at the first read or while it waits for the
  pane's last turn to end: a finished record is a gone terminal and proceeds, an unknown one keeps the wait going (bounded) and then
  proceeds only when an answered `hc ls` does not list the pane, and otherwise refuses (`--force` closes anyway).
- `brain` re-pointing the runs of a replaced brainer (`repointRuns`) takes only a terminal that is gone by evidence (a finished record, the one this
  reset just ended, or one an answered `hc ls` no longer lists, so a reset whose start failed repoints on the next `brain`). A terminal whose
  `hc get` is unknown and still listed keeps its runs. `hc get` read with a failing exit is no answer. UNESTABLISHED: what the real core answers after `hc rm`.
- `brain restore` reads a `hc get` reply as "terminal gone" only for a record with its address and a state of `finished`/`exited`:
  an exit-0 reply with no state (`{}`, an error object, `starting`, `lost`) is unknown, so the lane is skipped instead of being
  handed to `brain --resume` over a live brainer.
- `hw reap`/`hw done` occupancy is undetermined for every directory while herdr has no server and the app core's `hc ls` does not
  answer, including a directory with an hc run recorded (a label-filtered `hc ls` answering empty no longer reads as "nobody works in it").
- The cwd-less-terminal exemption of occupancy compares terminal addresses instead of parsing the printed `(hc, live)` line.
- `brain` judges every herdr reply on its own for "no server": only the question to the core is memoized, so a later `internal`
  failure or a success can no longer read as "gone" after the first decision.
- `brain restore` no longer replaces a terminal whose `hc get` could not be read: an unread terminal is unknown, not "no claude",
  so it is skipped and named instead of being handed to `brain --resume` (which ends a live idle brainer).
- A core terminal listed with no cwd makes `hw reap`/`hw done` occupancy undetermined instead of empty, so a worktree someone
  works in is no longer removed while herdr has no server.
- Whether herdr is absent (no server, the app's core answering) is decided once per `brain` process, in one place, and launch,
  relaunch and restore read that decision.
- `setup/heavy` no longer keeps a slot for a run that died in a reboot: a holder whose start time is
  before the machine's boot is dead even when its pid now belongs to another program (after a reboot,
  slot 1 stayed held by a killed release cut because macOS had handed its pid to an unrelated process).
- `brain restore` finds a lane's last conversation under the old directories its brain aliases name, not only under its
  current one.
- `brain restore` brings the setup lane back last, and replaces the shell of the terminal it runs in last, from a detached
  process, instead of skipping it.
- `hw revive` on the app counts a session as live only when claude or opencode runs in its terminal: a shell left under the
  task's labels after a reboot no longer refuses the revive, and a terminal it cannot read still does, naming why.
- `hw revive` on the app names the revived terminal's tab after its task, as a dispatch does.
- `hw revive` on the app ends the stale shell that still holds the task's name after a reboot before it starts the new
  terminal, instead of being refused by the core for a name already in use.
- Clearing a half-made worktree directory no longer touches anything but that worktree. The recovery ran a
  repo-wide `git worktree prune`, which also dropped other tasks' stale admin entries, and left a locked entry
  (the lock `hw` itself puts on a worktree) in place, so the rebuild then died "missing but locked". It now
  removes only that task's own entry (`git worktree remove -f -f`). The check for what `hw` itself makes now
  looks at the kind of entry as well as its name: `hw` writes symlinks and files, so a directory named `.envrc`
  (or `.env`, `.env.local`) is a stranger's and refuses the launch instead of being deleted.
- A launch killed while it was making a worktree no longer wedges the task name. `hw` used to read the leftover
  directory (hw's own `.env*` replication and `.hw/`, no checkout) as "worktree already exists, reusing", build
  on it and abort, on every retry, and `hw done` could not remove it. For the lanes that create a worktree and
  reuse an existing directory, a target with no `.git` whose every entry is something `hw` itself makes is now
  removed and the worktree built fresh, with one line saying so; git's own stale admin entry for it is pruned.
  Any other entry (a file `hw` does not make, a production env file included) refuses the launch naming it, and
  nothing is deleted. A directory with a `.git` is never touched, and no database is dropped.
- The merge that moves a checkout no longer spawns `git maintenance run --auto --detach` in the shared checkout (`maintenance.auto=false`, `gc.auto=0` are pinned with the rest of the merge's behaviour).
- `hw train push` no longer deletes ignored files in silence when it advances the checked-out main: it moves the
  checkout with git's own guarded fast-forward, which refuses (nothing moves, local edits put back) when ignored or
  untracked data is in the way of what the train adds.

## Known limits

- macOS and Linux only. Windows, native or under WSL2, is not supported or
  tested (KNOWN-LIMITATIONS, L1b).
- The Go core is on by default when the installer builds it; `HW_IMPL=bash`
  goes back. Every limit, with its evidence, is in
  [`KNOWN-LIMITATIONS.md`](KNOWN-LIMITATIONS.md).

## Upgrading from 1.0.0-rc.1

Run `foreman-setup upgrade --brain DIR`. A brain installed before `upgrade`
existed has no record of its flags yet: run your install command once more,
and `upgrade` works from then on.

Every release's changes are in [`CHANGELOG.md`](CHANGELOG.md).
