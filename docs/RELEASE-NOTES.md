# foreman 1.3.0

foreman 1.3.0 is the first-contact release: fewer things to install, one word for a lane in every help page and message, and a dry run that fits on a screen.

## In short

- **Three requirements.** herdr, Claude Code and git; jq, fd, rg and sd are no longer required, and a release carries `hw-core` prebuilt, so Go is only for building from source.
- **One word for a lane.** Every help page and message says `lane`, and `hw help` no longer shows an example a fresh brain refuses.
- **A dry run that fits on a screen.** `hw <lane> <task> --dry-run` prints about ten lines; a row with a problem still shows, and `--dry-run=full` prints everything.
- **Spec-driven development is removed.** `--sdd` and `sdd:` are ignored with one line, and `hw` names the line of a brief body that still mentions them.
- **Reviews work on an installed brain.** `hw review` finds its brief templates, reviews without plugins that are not installed, and the reviewer's brief names the one tests command.
- **The Files panel reads executors' files.** A `file://` link or an absolute path opens in Files, a pipe table draws as columns, and a brief's front matter shows one key per line (herdr-app 0.1.17 or newer).

## What changed

49 changes since 1.2.0 (1.3.0-rc.1 folded in).

### Added
- A `CODE_OF_CONDUCT.md`.
- `setup/export/check` refuses a shipped doc that links a page or names a path the export does not carry.
- In the herdr app, a Ctrl/Cmd-click on a `file:///abs/path` link in a terminal opens that file in the Files panel, drawn as Files
  draws it (markdown rendered, code in the editor, an image drawn); a link to a file that does not exist shows a message. Other
  links keep their opener. Needs herdr-app 0.1.17 or newer.

### Changed
- `hw` (dispatch, `--dry-run` and `hw next --brief`) warns when a brief body names the removed `--sdd` / `sdd:` outside a fenced block, with the line number and "delete it from the brief". One table, `retiredBodyTokens`, holds the retired body tokens; the next removal adds a row.
- `statusline-look apply` now keeps any other key under `statusLine` (it replaced the whole object before).
- One word, `lane`, in every help page and message: `hw help` and every usage line say `<lane>`, and an unknown lane is named
  as that. `hw` used to say `unknown project` (or a missing-brief error when no brief was given), and `brain` said
  `unknown brainer`. Now, for a lane that does not exist (here a brain with only the `demo` lane):
      $ hw nolane some-task
        ✗ unknown lane: nolane — try: demo                     (exit 1)
      $ brain nolane
      unknown lane: nolane — try: demo                         (exit 64)
  With several lanes, `hw` separates them with ` | ` and `brain` with `, `. Exit codes are unchanged.
- The first page of `hw help` lists what the README's everyday table lists (`reap`, `train`, `review` included) and no longer
  lists `hw retro-done`.
- `ask-invoker` and `done-invoker` with no argument print their own usage (still exit 64), and an unknown flag is one line
  naming what they take, with no exit-code paragraph after it.
- `SECURITY.md` supports the latest stable release and `main`; the README requirements say macOS, with Linux checked by the
  test suite only; the Homebrew caveats give `foreman-setup init`, the README's first command.
- The export's `herdr-app/README.md` is now a short user page for the one mod it ships, Cockpit: what it shows, how to install
  it (a symlink and a layout block), how to update and remove it. The 1.0 promise and `REFERENCE.md` name that mod instead of
  "mods and layouts", and `REFERENCE.md` no longer shows a screenshot of a look the export does not carry.
- In the herdr app, a Ctrl/Cmd-click on a plain printed absolute path (`/opt/tools/bin/hw`) opens that file in the Files panel too, when the app verified it exists on this machine; a relative path or one it could not verify keeps the app's own opener.
- The ledger a brainer gets back at start and compaction now says what the session summary is for: evidence nothing else owns, never the next steps, owed items or orders the ledger already holds.
- A release installs without Go: it carries `hw-core` and the invokers prebuilt for macOS and Linux, arm64 and amd64
  (`hw-core-<os>-<arch>.tar.gz`, with `SHA256SUMS`). The Homebrew formula drops `depends_on "go"` for one resource per
  target, `curl | bash` and `upgrade` fetch the asset for their tag and check it against the release's `SHA256SUMS`, and
  `install.sh` installs a prebuilt that matches its sums. Go 1.22+ is required only for a source checkout without one.
- The README's requirements name three tools: herdr, Claude Code and git (its Command Line Tools bring `python3` on macOS; Linux needs python3 3.9 or newer). Go is only for building from source.
- The reviewer's brief now names the one tests command (`setup/go-test-core.sh --affected "$HW_ARTIFACTS/changed" --run`, in the
  background), the tools the reviewer really has (`rg`/`fd`, `sed -n`; `Glob`/`Grep` may be absent) and the read-only rule for
  a skill's references, so a reviewer stops running a foreground `go test` that `deny-executor-sleep-poll` refuses.
- A review of a diff that changes a file under `setup/guards/` carries a checklist of the guard bypass classes this system has
  already shipped (`setup/guards/REVIEW.md`, sourced from `setup/decisions.md`); any other diff gets none.
- `hw review` writes `callers.txt` beside `functions.txt` in the reviewer's artifacts: where each changed function is used (5 hits
  per function at most, the definition left out, no search for a name under 4 characters), and the brief points at it.
- A review that ran Judgment Day must leave a `jd: confirmed=<n> suspects=<n> exclusive=<n>` line in `review.md`:
  `done-invoker` refuses it without the line (the receipt carries `review_jd_t<n>`); a review that did not run Judgment Day, and
  an older `review.md`, owe nothing.
- A fix round's brief (`docs/BRIEF-TEMPLATE.md`) has a `## Findings` table with a `disposition` column (accepted, declined or
  deferred); the re-check brief copies it beside each prior row, so the next retro can count what builders dismiss.
- A reviewer runs each skill piece itself with the Skill tool; to parallelise one it
  uses a sonnet subagent that invokes the skill, never an unasked `general-purpose` one.
- One lane's subagents that name no model now run sonnet (`subagent_model`); an explicit
  `model: opus` and an agent pinned to opus keep opus.
- `hw <lane> <task> --dry-run` prints the short form of the dispatch by default: the lane and task, then agent, model, effort, account, worktree, base and what runs at close, about ten lines. A row that carries a problem (an unresolved invoker, a refused account, a missing guard, a pi or plugins row that is NOT APPLIED or NOT LAUNCHABLE, a verification that will time out, a reused worktree) is printed whole, and warnings and refusals print as before. `--dry-run=full` prints the whole manifest, as `--dry-run` used to. A launch still prints it whole and writes it to the run directory.
- `setup/heavy` runs three slots by default (`HW_HEAVY_SLOTS=1` or `=2` keep their meaning). A waiter takes slot 3
  only while at least 30% of memory is free (`HW_HEAVY_MIN_FREE_3`, next to slot 2's `HW_HEAVY_MIN_FREE` of 20%);
  under that it is skipped exactly as slot 2 is under pressure. `--status` lists slot 3 with its threshold.
  Waiters already running read the script they started with, so slot 3 is taken by waiters started after the update.

### Removed
- jq is no longer a requirement. `hw` reads the lane table and the status-line JSON itself (`hw-core json`), and the
  installer reads its own JSON with python3. `install.sh --check` stops asking for it, the Homebrew formula no longer
  depends on it, and `hw`, `hw revive` and `hw preview` stop refusing a machine without it. The zsh completion
  (`shell/_hw`) still uses jq when it is there and offers the fixed words when it is not.
- `fd`, `rg` and `sd` are no longer requirements: hw lists directories itself, so `install.sh --check`, `init`, the
  Homebrew formula and the README stop asking for them. `rg` stays a soft WARN; `engram` is optional in `lib/hw/deps.conf`.
- Spec-driven development is gone for good: `hw <lane> <task> --sdd <value>`, `hw next --sdd <value>` and a brief's `sdd:` key are no longer read. Each prints ONE line saying spec-driven development was removed and the flag or key is ignored, never a refusal, so older briefs that still carry `sdd:` dispatch as before. The `--sdd` help rows, the zsh completion entries, the `sdd` field of the run receipt, and the `--sdd` sections of INSTALL and REFERENCE are removed.

### Fixed
- An executor that reported `--blocked`, was resumed by `hw ruling`, and had reported `--blocked` a second time while the ruling was being sent (the stop hook nudges it, because the resume sets its `done` aside before the send) can now close with `done-invoker`: the ruling consumes that report's `done` and `blocked-waiting` as well, instead of leaving the executor refused with "no ruling has resumed this task" until the brainer sent an empty second ruling. A task nobody has answered is still refused.
- `brain relaunch`, `brain <lane> --resume ID` and `brain restore` resume a conversation under the account
  that holds its transcript, whatever the lane declares, so a lane declared `default` whose session lives in
  the personal account no longer comes back "No conversation found". A plain-shell `brain relaunch --all`
  finds those sessions, `brain restore` picks the newest transcript of the lane in any account, and stderr
  names whether the account came from the transcript or from the lane's default.
- `brain relaunch` refuses (FAILED, non-zero exit, the brainer left running) a brainer whose session has no
  transcript, instead of starting a new conversation in its place.
- `docs/REFERENCE.md` no longer describes `lib/hw/*.sh` modules and a `docs/layout.md` that do not exist, `docs/INSTALL.md`
  no longer names plugin scripts the export does not ship, and the link to `CHANGELOG.md` in `docs/RELEASE-NOTES.md` points
  at the file.
- `hw review` works on an installed brain. The installer now puts the reviewer's brief templates (`docs/REVIEW-BRIEF.md`,
  `docs/REVIEW-DELTA-BRIEF.md`) in the brain; before, the first `hw review` died on `open <brain>/docs/REVIEW-BRIEF.md`.
- A review tier whose plugins (`thermo`, `tests`, `ponytail`, `pstack`, optional installs) are not installed reviews without
  them instead of briefing a reviewer for skills it cannot call, and the dry run says which ones are missing in one line.
  A plugin the build's `review:` or the lane's `review_default` names is still required.
- `hw` on herdr-core no longer dies with "shows neither status=working nor a turn" when the executor is already on a first
  turn longer than 30 s: claude's spinner in the terminal title now proves the brief landed.
- In the herdr app, a markdown file that opens with YAML front matter (a brief's `kind:`, `requested_by:` …) shows it as a yaml block, one key per line, in Files, Issues and the viewer; it was joined into one paragraph.
- `hw reap` no longer panics when a task's `.hw` directory vanishes while it checks whether the task reported long enough ago.
  An expired cache tree whose occupants `lsof` cannot list is now judged by the same helper in both reap paths, and the
  unreadable case is tested.
- `hw review <lane> <task> --lift-cap --dry-run` no longer dispatches for real: a flag given as the `--lift-cap` reason is refused ("--lift-cap needs the reason").
- The plugin-review closing no longer tells an executor to launch the review agent in the foreground only; a background launch proves once it completes (as done-invoker already proves it).
- A green executor pre-commit prints the `DEFERRED` lines (the Go partitions left to the brainer's batch).
- A refused `hw next` re-task of a kept reviewer says how to leave the rerun loop (`hw done <pane>`).
- foreman-files link handler: a `file://localhost/` test case; the README no longer claims a remote file keeps the app's opener.
- The first page of `hw help` no longer shows an example that a fresh `foreman-setup init` brain refuses. `hw train add demo hello`
  failed there (`hw train: demo's checkout is not brain …, so it has no train`), because `init` makes `demo` on a toy repository and
  `hw train` is for a lane whose repo is the brain itself. The row now says so and carries no example.
- `hw status <lane>` and the `hw` picker say `lane`, not `project`: ``scoped to demo — `hw status` with no lane adds the cross-lane checks``,
  the picker's `lane >` prompt, and the `DRIFT` and `no agent detected` lines that said "the project's space".
- The Files panel and the issue and viewer panes draw a markdown pipe table (an executor's `checks.md`) as aligned columns
  instead of one run-on paragraph.
- The status line draws again in a session started outside a login shell (a `brain relaunch` from a detached process): `statusline-usage-tee` adds `~/.bun/bin` to `PATH` when the status line command is not found.
- `setup/go-test-core.sh --bless` no longer exits 75 inside an executor when run the documented way, under `setup/heavy` in the background: the 90 s executor budget caps only a foreground narrowed run.
- The release cut's notes cover accepts `setup/releases/highlights/VERSION.md`, the file `setup/release/notes` requires, in the notes commit.
- Two test subjects that run under `env -i` (828, 1381) pass `HW_HOUSEKEEPING=0`, so they no longer start a background reap.
- The batch gate's subject-deps re-measure commits rows on main only when the commit it verified is main or an ancestor of it; an unmerged batch branch no longer leaves a fresh row that replays a false green.
- `KNOWN-LIMITATIONS.md` L2 no longer names a Bitwarden guard or a Claude Code Keychain guard, which foreman does not ship.

## Known limits

- macOS and Linux only. Windows, native or under WSL2, is not supported or
  tested (KNOWN-LIMITATIONS, L1).
- The Go core is on by default when the installer builds it; `HW_IMPL=bash`
  goes back. Every limit a user can hit is in
  [`KNOWN-LIMITATIONS.md`](KNOWN-LIMITATIONS.md).

## Upgrading from 1.2.0 or 1.3.0-rc.1

Run `foreman-setup upgrade --brain DIR`. A brain installed before `upgrade`
existed has no record of its flags yet: run your install command once more,
and `upgrade` works from then on.

Every release's changes are in [`CHANGELOG.md`](../CHANGELOG.md).
