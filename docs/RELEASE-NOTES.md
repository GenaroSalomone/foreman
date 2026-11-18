# foreman 1.0.0

foreman 1.0.0 is the first stable release: the interface of `hw`, `brain`, the invokers, the brief format, the install and the herdr app is now a promise.

## In short

- **Smaller on purpose.** Two agents (Claude Code and pi) share one guard implementation; Codex, OpenCode, the sandbox and the Spec Kit flow are gone.
- **The brain guard fails closed.** A command naming a brain path is judged only when flat, and anything it cannot read is refused with the spelling that passes.
- **The consultant is a standard.** `kind: design` + `asked: investigate` + `decision:` runs fable at effort low, and the top tier outside a build needs a named decision.
- **Three honest limitations.** macOS and Linux only, guards are a tripwire not a boundary, and foreman inherits herdr's and Claude Code's behaviour.

## What changed

264 changes since 0.9.0-rc.1 (1.0.0-rc.1, rc.2, rc.3 and rc.4 folded in).

### Added
- In the herdr app's Agents list, each row's icon tells a brainer from an executor: a brain for a brainer
  (a terminal named `brain-<lane>` with no `hw_task`), a hard hat for an executor (one carrying `hw_task`, and no
  `hw_role`); every other terminal keeps its program icon. It is `herdr-app/mods/foreman-agents`, a clone of the app's
  `agents` mod that the three foreman layouts now place; link it into `~/.config/herdr-app/mods/` as the README says.
- `foreman-files` copies a local image it opens into `~/.config/herdr-app/cache/foreman-files/`,
  named by its content, because the app draws images only from its own folder; copies not opened
  for 30 days are removed.
- A Files block can name the file it shows first with `props.open`.
- `setup/env-prefix-check` (run by subject 1356) fails when an environment prefix that `bin/` or `core/` reads is neither swept in `setup/tests/_common.sh` nor on `setup/suite-data/env-prefix-allowlist.tsv` with a reason. It would have caught the `HC_*` leak that lasted about 2.7 days.
- `setup/test-hw --base <ref> <subject>...` answers «is this red the branch's or the base's?» for bash subjects the way `go-test-core.sh --base` does for Go: one fresh clone at `<ref>`, each named `setup/tests` subject run there and on this tree under `env -i`, `red`/`green`/`none` for each, exit = this tree's.
- In the herdr app's Cockpit, the queue's rows have buttons: **Reply** on an ask, **Rule** on a challenge (or a queued
  ruling), **Done** on a reported task (it asks first). Each runs the argv `cockpit/verbs.txt` gives, read from that file
  (`props.verbs` in the layout's cockpit block); a failed verb shows its output and the row stays, a cut-off send shows
  `unknown`, and a row clears only once the next state read shows the change. There is no merge button.
- Every whole fast gate run from a linked worktree (the brainer's batch gate) appends one row
  to the main checkout's `setup/suite-data/gate-runs.tsv`: date, tree, the `HW_GATE_BATCH` label
  if set, result (`green`, `red`, `known-flake`, `killed`), the first failing job and the wall
  seconds. The first-try rate of the batch gates is now a query. A gate run in the main
  checkout itself says it leaves no row: a row in the tree under test would change what the
  next gate diffs.
- A red subject listed in `setup/suite-data/flaky-ledger.tsv` that is red at the merge base
  too is printed `known flake` and recorded `known-flake` in `gate-runs.tsv`; the gate itself
  stays red.
- Herdr app: `cmd-shift-f` (`mod.open:foreman-paths`) lists the file paths on the focused terminal's screen, newest first,
  and opens the chosen one in the Files panel; a missing file and a screen with no paths say so. `herdr-app/README.md`
  names which printed links it reaches.
- `hw <lane> <task> --agent pi|opencode|codex` now launches from a terminal of the herdr app, as it does from a herdr TUI
  pane: one `hc run` terminal running the vendor's own binary with the brief as its launch task. Only `--sandbox` and `--here`
  are still refused there.
- **`--sandbox` in the herdr app.** A sandboxed dispatch from an app terminal was refused ("the executor would run
  UNCONFINED"). The run is now armed before `hc run` and the shim's directory goes first on the terminal's `PATH`; the
  broker's heartbeat proves the agent runs inside the profile, and a terminal without one is killed. `hw revive` arms a
  sandboxed run again the same way instead of refusing it.
- **A brainer in the herdr app can reset itself: `brain <lane> --reset --self`.** It was refused ("has no herdr-core
  mapping yet"). The brainer arms the reset and a detached process waits for that turn to end, ends the terminal and
  starts a new one; the lane's open runs are repointed to the new terminal. `--force` and the unreported-executors
  gate work as in a herdr pane.
- `projects.json` names the agents root once, as `root`: brain is `<root>/brain`, work `<root>/work`
  (unless `work` is set), the archive `<root>/archive`. `guards.json` spells the root `{brain}` and
  `{work}`, resolved from the `projects.json` beside it; the guards protect exactly what they did.
- `projects.json` names organisations: `orgs` maps each to the directory its product checkouts live
  under, and every lane names its `org`. A table where a lane names an unknown org, or a
  `product_repo` checkout outside its org's directory, does not load. Organisation-only rules read
  `org`, never a list of lane names.
- `bin/move-root --to <dir> [--dry-run]` moves the agents root (`brain/`, `work/`, `archive/`) by rename
  and repairs what names it: git worktree links and remotes, every account's `settings.json` and
  `.claude.json`, `~/.codex/hooks.json` and `config.toml`, herdr, herdr-app and hc state, LaunchAgents,
  hw's run state, symlinks into the trees and `<root>/CLAUDE.md`, the accounts' project directories,
  and the brain's root-bearing files (committed). It checks the engram labels do not change, refuses
  while any process stands in the trees or herdr/hc is running, and journals every step;
  `bin/move-root --undo` puts it back.
- `bin/move-root --audit`: a hard-coded `<root>/brain|work` outside history, or a root-bearing file on
  another root, is red (`setup/tests/1361`).
- `docs/layout.md` defines each piece once (agents root, brain, work, archive, lane, org, product
  checkout), where each kind of file lives, how to add a lane and choose its org, and how to move the root.
- The Cockpit of the herdr app has a **Dispatch…** control under each brainer: it lists the briefs of that lane that were
  never dispatched, prints the `hw <lane> <task> --dry-run` manifest, and runs `hw <lane> <task> --report-to <the brainer>`
  only after a confirmation. Its three verbs are in `cockpit/dispatch-verbs.txt`, beside the frozen `verbs.txt`.
- The Agents panel of the herdr app shows each executor's model beside its row, from the same `model_running` the Cockpit row reads.
- `hw review <lane> <task> [--dry-run]`: the independent review of a build. It measures the branch's diff (code-only lines: only `decisions/`, `briefs/`, changelog fragments and `docs/` are prose; `.claude/**`, `CLAUDE.md`, `_agents/`, `_skills/`, `plugins/`, `*.golden`, `testdata/` and binaries count), picks tier 0, S (1-400 lines) or L (more), plus Judgment Day when the build's brief lists `judgment-day`, writes the reviewer's brief from `docs/REVIEW-BRIEF.md` and dispatches a fresh claude opus executor on `task/<task>-review-<short sha>`, cut at the reviewed SHA (a product lane is cut from `origin/task/<task>` and refuses a head that branch is not at). The build's receipt gains `review_run`, `review_head` and `review_tier`; the reviewer's run gets `functions.txt` (the functions the diff changed). Tier 0 dispatches nothing and records `review_head`.
- A build's dispatch writes `review_owed=1` to its receipt (not for `review: none` with a `review-skip:` reason, nor for a kind that is not a build), so a forgotten or failed `hw review` is never read as "nothing owed".
- A `kind: review` brief with `review_of: <lane>:<task>@<sha>` gets its own gate keys at dispatch (`plugin_review_t1`, `jd_agents_t1` when Judgment Day is listed, `checks_required_t1`, `review_of_t1`), so done-invoker proves each review piece from the reviewer's transcript. `review-skip` and `review_of` are known frontmatter keys.
- `hw done <lane> <task> --accept-review "<why>"`: a brainer's override of the review gate, recorded in the receipt as
  `review_accepted`. Refused when `HW_TASK` or `HW_RUN` is set, so an executor cannot use it.
- `hw review <lane> <task> --at <sha>`: a blind review of a past head, for benchmarking (a PR before its fixes). The SHA must resolve to a commit in the lane's checkout; the reviewer runs on `task/<task>-review-at-<short sha>`, cut locally at it, so nothing is pushed and no remote ref is needed (a product lane's `origin/task/<task>` comparison is skipped: the tree is cut at the stamped SHA itself, and the reviewer's close gate, `HEAD` equal to the reviewed SHA, is unchanged). The base is the merge-base of the SHA with the lane's base, as before. The build's receipt is not touched (no `review_head`, `review_tier` or `review_run`), so a past-head verdict can never satisfy `hw done`'s merge gate; the dry run says so. `hw help advanced` lists it.
- An independent reviewer's `done-invoker` is held to its review: when the run's receipt has `review_of_t<n>` (written by `hw review`'s dispatch), `review.md` must exist, its first line must be `verdict: approve|changes-needed  head: <sha>` with the reviewed SHA, every row `id | blocker|should|nit|follow-up | file:line | …` must carry a known severity and a file:line, `approve` may carry no `blocker` row and `changes-needed` needs one, and the function table in `checks.md` (`function | what it does | verdict | action`, verdict `simple|split|merge|inline`) must have a row for every function in `functions.txt`. The reviewer's worktree `git rev-parse HEAD` must equal the reviewed SHA, so the bytes it read are the bytes the verdict names.
- `checks.md` of a reviewer may hold that function table: the build's checks gate skips it for a run with `review_of_t<n>`.
- `bin/statusline-look`: the Claude Code status line in foreman's look (`Fable 5.2 · 9.0% · branch · +0 -0`, theme colors, no labels), the same pinned ccstatusline on both accounts, with backups and `revert`.
- `herdr-app/mods/foreman-tabs`: brainer and executor tabs draw the brain and hard-hat icons in the tab bar.
- `setup/commit-tree` refuses a new `setup/tests` subject whose number another subject of the tree already holds (`setup/test-number-check`, the check `setup/test-hw` runs first), naming `setup/next-test-number`.
- `setup/test-hw` repeats the touched subjects the budget cut (`SKIPPED, names your diff and was NOT run: …`) in the last lines of the run, uncapped, so a `tail` shows them.
- `docs/BRIEF-TEMPLATE.md`: a mutant goes through `setup/mutation/safe-mutate` and must compile; a "costs no visible time" claim is 6 interleaved old/new pairs, medians; a pi leg starts with `pi -p --model <m> "say ok"` and is UNVERIFIED if it fails.
- `hw status <lane>` now says when a lane holds 25 or more finished briefs:
  `N finished briefs → hw briefs archive <lane> --apply`. The count is what that verb would move, so a
  fresh archive leaves the status quiet.
- A brief can ask for a review of its tests: `review: [..., tests]` loads foreman's own `test-review`
  skill (plugin `tests`), whose pass criterion is a test that fails on the old code and passes on the new,
  OLD/NEW output pasted. The close is held to it like any review plugin: a Skill call in the transcript and
  a `test-review` row in `checks.md`, and no `review-skip:` for it. Pages for Go, vitest with Testing
  Library and pytest with FastAPI are picked by the diff's file types.
- `webguide`, a vendored plugin: Chrome's Modern Web Guidance (`GoogleChrome/modern-web-guidance`, Apache-2.0, pinned at `4dd9683`). A brief that touches UI loads it with `plugins: [webguide]` after `setup/vendor/plugins/install`; it is not a review piece, and the lane's `CLAUDE.md` wins when its guidance disagrees.
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
- `hw review <lane> <task> --account <name>` runs the reviewer on that account.
  The flag goes to the reviewer's dispatch as it is, so the same parity check,
  refusal and `account` manifest line apply (`--dry-run` shows it). Without it
  the reviewer keeps the lane's default account.
- The consultant is a standard hw holds: `kind: design` + `asked: investigate` +
  `decision:`. It defaults to fable at effort low (`effort:` in the brief raises
  it) and its prompt carries the delivery format (verdict first, facts with
  file:line, one recommendation, steps as one brief each, UNESTABLISHED marked).

### Changed
- The herdr app's Files panel shows a file the way the file viewer does: markdown is rendered
  (headings, bold, tables, code blocks), code and plain text keep their syntax colors and wrap
  long lines, and a local image is drawn. The three foreman layouts place `foreman-files`, a
  clone of the app's `files` mod, with the tree unchanged; link it with
  `ln -sfn "$PWD/herdr-app/mods/foreman-files" ~/.config/herdr-app/mods/foreman-files`.
- `bin/herdr-isolated start --brain` also puts `cockpit/verbs.txt` and `projects.json` under the brain root the layouts name inside `<dir>/h` (the
  layouts and the History mod read them from the instance's HOME; without them the Cockpit said "buttons off" and History
  listed no product lane). Its stand-in `claude` treats a launch prompt as a turn and runs a line that begins with `!` in its
  own environment, so a QA can ask, challenge, report and dispatch as an executor or brainer. `herdr-app/README.md` has the
  product-lane (dev-server) cycle: a fixture lane under `<dir>/h`, a stand-in `npx` that serves 200 on `HW_PORT_WEB`, and
  `--keep-pane` so the report row stays for **Done**.
- In the herdr app, `brain <lane>` opens its tab named `brain` and `hw <lane> <task>`
  opens its tab named after the task, instead of the app's numbers («1», «2»).
  An executor's dev-server terminals keep the app's numbering, and a tab a person
  already named is left alone.
- foreman-sh is now `foreman`: the repository, the Homebrew formula (`foreman`
  in the tap) and the environment variables
  (`FOREMAN_SH_*` became `FOREMAN_*`). The installer command is `foreman-setup`,
  and nothing links an executable called `foreman`, because homebrew-core already
  has a formula with that name.
- An install made by 0.9.x upgrades to this release with `foreman-setup upgrade
  --brain DIR`. The three variables a 0.9.x installer hands to the new one
  (`FOREMAN_SH_VERSION`, `FOREMAN_SH_INSTALLED_FROM`, `FOREMAN_SH_UPGRADE_TARGET`)
  are still read. The tap records the rename in `formula_renames.json`, and the formula
  keeps a deprecated `foreman-sh` command so a 0.9.x install's own `foreman-sh upgrade`
  completes.
- The README and the reference state what 1.0 promises: `hw`, `brain`, the invokers,
  the brief format, the install and the herdr app, with its mods, layouts and the
  `hc` backend. A change that breaks a working setup takes a new major version.
- An edit to `setup/mutation/mutation-anchors.tsv` no longer selects every subject that builds a mutant: `mutate_anchor` (in `_common.sh`) names the table, so the selector read it as named by every caller, and one row pulled 12 slow subjects (2285 s) into a touched list with no red of the branch's. `setup/gate-select` now selects, for the table, only the subjects whose own text spells it (and 540, which judges it); the 28 `# gate-when:` lines that listed it lose it.
- The sleep/poll guard no longer refuses a bounded `tail -f … | grep --line-buffered` in the `Monitor` tool: a Monitor command is judged as a background run bounded by its `timeout_ms`. Without a numeric `timeout_ms` an unbounded follow or loop is still refused, and the refusal names `timeout_ms` as the form that passes. Nothing is loosened for `Bash`.
- `setup/heavy` keeps the last 40 lines of every run's output (stdout and stderr, the short lane too) in `~/.cache/hw/heavy-out/<tag>-<timestamp>.log`, and the `heavy.log` line now ends in ` out=<that file>`: a red of a batch gate can be classified after the fact. The caller still receives both streams, apart. Files older than 14 days are removed.
- `setup/heavy -- bash setup/tests/<n>-*.sh` (a lone subject, nothing else on the line) takes the short lane by itself when `suite-data/test-budgets.json` budgets it at 120 s or less, instead of queueing behind a frozen holder. A subject over the cap, one the table does not list, or a line with another word queues as before.
- The runner's `ENV-ONLY` label (a red subject that passes alone under `env -i`) now names the variable that turns it red again: each group of the gate's exported variables (the name up to its first `_`) is put back alone into the clean run, and the label reads `ENV-ONLY (red again when only HC_* is put back …)`, the variable's own name for a lone one, or says that no single group flips it and how many were tried.
- `setup/commit-tree` now runs, before it commits, the subjects `setup/gate-select` names for the commit's paths whose budget is 20 s or less (cheapest first, up to 180 budgeted seconds), refuses the commit when one is red (the ref unmoved, the subject and its first `not ok` line named), and lists the slower ones as `NOT RUN here`. Each subject runs in its own session under a wall limit (its budget, at least 20 s) and the whole check under a 150 s allowance: a subject that never returns (the memory guard freezes a tree under 26% free memory) is killed and listed as `FROZEN/TIMEOUT`, never counted as red, and the rest are not started. A red used to surface 30 minutes to 29 hours later, in a batch gate.
- `setup/go-test-core.sh --bless <patterns>` inside an executor narrows itself instead of exiting 75: it runs only the cases whose golden holds a line a pattern matches, through each corpus' own `HW_*_CASES` selector (estimate: partition seconds x the share of cases kept, held to `touched_budget_seconds`). When it cannot narrow (a NEW line is in no golden, a matching case no cases-* file names, or the narrowed run is still over the budget) it exits 75 and prints the focused commands and the `golden-bless` step to run by hand. Outside an executor the whole batch runs as before.
- `cockpit/verbs.txt`: the app's `exec` has no stdin, so for the `stdin=text` verbs the app passes the person's text as one
  argv word in place of the lone `-` (no shell, nothing expanded); a text that `hw` or `channel-send` would read as a flag
  is refused with a message, never rewritten. The Claude cockpit mod still uses stdin.
- The mutation partitions (`m844`, `m848`, `m840`, `m826`, `m861`-`m866`, `m871`, `mbrain`: 21 of the 40 Go partitions, about half the batch's Go work) leave the fast gate and
  run in the release gate: the partition table's `RELEASE=` line names them (`go-test-core.sh --release`), the fast gate in every form queues none and lists the ones a diff
  reaches as `DEFERRED, names your diff: mutation partitions …`, and `setup/release/cut` runs them as one batch after the release suite and refuses the cut when one is red.
  No mutant is dropped; the full run still queues them. The worst-case fast gate estimate falls from 1253 s to 818 s.
- The fast gate names what it did not run: a slow subject that names the diff and was left out
  is listed in the last lines as `DEFERRED, names your diff: <n> -> bash setup/tests/<file>`.
- An executor's touched form that would queue Go partitions over `touched_budget_seconds` exits
  75 before any subject runs, naming the focused `go test -run` form; the refusal for a path no
  subject names now prints a loop that runs the reference subjects, and the executor's touched
  run says how to check a freeze (`ps -o state=`), since it runs outside `setup/heavy`.
- `mutate_anchor` fails at its first use, naming `setup/mutation/mutation-anchors.tsv`, for an
  anchor id the registry does not list (it failed only at subject 540, the end of the gate).
- Herdr app cockpit: the «N reports to read» block folds from its header (kept across restarts; a new report reopens it), and a report's summary line shows only when it says more than «Done».
- Herdr app Agents, «Grouped by space»: the brainer is the first row of its space, its executors follow; a group's fold is kept across restarts.
- **What the herdr app still refuses says why.** `brain --in-pane`, `hw … --here` and `hw revive` of a pi, opencode or
  codex run on the app used to say "no herdr-core mapping yet: this step…" or name a state that was no longer true
  ("only claude executors are launched"); each now gives the measured reason and the command that does the job there.
- The readers of the lane table (`bin/project-spaces.sh`, hw-core, the label proxy, cockpit-state,
  brainer-metrics, suite-gate, the invokers (Go and their shell fallback), the suite runner, `install.sh`, the zsh
  completion) take `work` from `root` when the table does not set it.
- The rules, docs, skills and scripts say `<brain>` and `<work>` instead of one machine's paths; an
  organisation-only rule names the `org` it applies to.
- The README has a "Tools foreman works with" table: every tool foreman installs, pins, launches or reads,
  whether it is required, and its version floor or pinned commit, including the `tests` review piece and `webguide`.
- The README's diagram is a 1920×1080 architecture picture (`docs/foreman-architecture.png`, source
  `docs/foreman-architecture.html`): brainer, `hw`, executors, the return channel, engram, review pieces, `setup/heavy` and the way to a merge.
- A build owes the independent review by default: every `review:` value on a `kind: build` (or no kind) owes it, whether the key is absent, `judgment-day`, `rdd`, `plugins`, a list or a bare `none`. The one opt-out is `review: none` WITH a `review-skip: <why>` line. A bare `review: none` still owes the review and warns for one release (it is refused after that).
- The builder no longer runs Judgment Day or a review skill. Its closing is: commit per unit, its own OLD/NEW tests, and `/simplify` on claude (proved by the Skill call `simplify`, receipt `plugin_review_t<n>`; pi: `pstack:deslop` when loaded, unproved; codex and opencode: not available). The reviewer runs Judgment Day when the brief declares `judgment-day`, `rdd` or a list naming it.
- `hw review` adds the plugin items of the build's `review:` list to the reviewer's tier, and `review: plugins` adds the brief's `plugins:` (S4); `rdd` is accepted in a list as Judgment Day.
- A build that owes the review launches with `--keep-pane` by default (a pane is needed for the one fix round); the manifest says it was the default. The manifest's `review` line names the independent review for every build.
- The docs now say what the independent review is: every `kind: build` owes it whatever `review:` says (only `review: none` + `review-skip:` opts out), `hw review` picks the tier by code-only lines (0 / S 1-400 / L +ponytail and two pstack principles / +Judgment Day when the brief names it), the builder closes with OLD/NEW tests and `/simplify` and runs no Judgment Day, and `hw done` offers «Merge it» only for an approved head or the brainer's `--accept-review`. Updated: the `review:` paragraph of `BRIEF-TEMPLATE.md`, `INSTALL.md`, `REFERENCE.md`, and the brainer rule that still called the review opt-in.
- `hw review --at <sha>` (a benchmark of a past head, never the merge gate) is documented, with its one known gap: on an already-merged commit the base is the current base, so it counts 0 lines and prints tier 0 (KNOWN-LIMITATIONS L34).
- `hw help flags` says what `review:` means now (a build owes the independent review whatever it says; `review: none` opts out only with `review-skip:`), instead of «a review is opt-in». The export README's «Reviews you opt into» bullet says the same, and two of its feature bullets were joined onto one line each to stay inside test 1024's 380-line budget (no wording dropped).
- `hw done` no longer prints «Merge it» for every branch with a commit ahead. It recomputes the code-only lines at the branch head
  and offers the merge only for an approval at that head (the reviewer's `review.md`), a diff with no code, a recorded
  `review-skip:`, or `--accept-review "<why>"`. Otherwise it prints `NOT MERGEABLE: review <verdict> at <sha>, head is <sha>`
  and why. A missing verdict is a denial, never «nothing owed». Owed follows the diff, not the brief's kind, and a review run's
  own branch is never offered.
- On a product lane `hw done` compares the reviewed SHA with the PR's `headRefOid` (read-only `gh pr view`) and prints
  `gh pr ready <n>` only for a draft PR at that head. Builds open their PRs with `gh pr create --draft`.
- The three foreman layouts show `foreman-tabs` instead of the shipped `tabs` mod.
- The two accounts' `statusLine` no longer differ (`@2.2.27` vs `@latest`).
- `setup/heavy --short` says, on its exit-124 kill, to rerun without `--short` (a cold `go test` takes a slot) and that `--max <s>` only lowers the 120 s cap; the printed focused-run forms (`setup/go-test-core.sh`, `docs/BRIEF-TEMPLATE.md`) say the same.
- The Claude status line says the context once, `▓▓░░░░░░░░ 170k/1M`, and colors the bar by use:
  muted below 60%, amber below 85%, red above.
- Its git counts are the branch's commits against `main`, and absent on `main` itself, instead of the
  working tree's uncommitted changes.
- The refusals for a bad `review:` list and for `review: plugins` with no review plugin now name `tests`
  among the plugins that carry a review skill.
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
- The guard's verb finder and builtin check read one table of precommand words. hw-core's "brain guard present" checks (dispatch, revive, `hw done`'s leak scan) read one list, which also requires the two modules the guard imports, `setup/guards/deny_brain_writes_shell.py` and `setup/guards/deny_repo_writes.py` (without either it fails closed on every call).
- Fail-closed costs, measured on 5426 product-executor commands (KNOWN-LIMITATIONS L2): 154 refused (the last round 127). Among the 27 more, a brain-naming command that reads an assignment while its text holds a `(` even inside quotes (`MSG="done (x)"; done-invoker "$MSG"`), and a heredoc fed to `python3` in a command that names brain. Write such a value where it is used, and such a script to a file.
- Fail-closed cost, measured on 5426 product-executor commands (KNOWN-LIMITATIONS L2): 162 refused (the last round 154). The 8 more are a `cat > file <<'EOF'` in the same command as a brain program called with a `(` in a quoted argument (`done-invoker "done (x)"`): run the heredoc in a command of its own.
- Fail-closed cost, measured on 5426 product-executor commands (KNOWN-LIMITATIONS L2): 169 refused (the last round 162). The 7 more are a `cat > file <<'EOF'` in a command that also holds a `|` and names brain: run the heredoc in a command of its own.
- An unknown word that is only data is allowed again (round 15 refused it anywhere in a writing command): `make test; echo "rc=$?" >> "$HW_ARTIFACTS/x"`, `cmd | tail -5; echo "exit=$?"`, `ls -la ${TMPDIR:-/tmp}/x.* | head`, `ps > /tmp/ps.$$`. Still refused, fail closed: an unknown word as an operand of a writing command even where it cannot reach brain (`cp "$f" "${f%.txt}.bak"`, `gh … ${PR:-1}`), and an unknown word in a command with a rename, `exec`, a function or a pipe into `tee`. Spell the word literally.
- Docs: the README is a short tour (what foreman is, install, the first dispatch, links); the herdr app, the tools table and the comparison with similar tools moved to REFERENCE.md. The docs no longer describe Codex, OpenCode, the Spec Kit flow or `hw unstick`.
- `hw wait` and `hw next` judge a blocked pane by its screen alone: the "settled" verdict and
  `hw wait` exit 5 came from an OpenCode endpoint and are gone, and the
  `hw unstick` advice with them (`channel-send` keeps its own exit 5, an unconfirmed
  delivery). A blocked/stuck pane whose completion tokens are
  delivered is still re-tasked without `--force`.
- A `projects.json` written by `install.sh --vendor opencode` still loads: `opencode_config_dir` is accepted and
  ignored for one release, and a lane with `vendor: opencode` or `codex` opens a Claude brainer. Dispatching an
  executor of that vendor is refused naming the removal.
- The pi executor now asks the same Python write guards the Claude hooks run (`deny_repo_writes.py` through the new `setup/guards/deny_repo_writes_ask.py`, and `deny_brain_writes.py` directly), one spawn per question (measured 2026-11-17: median 40 ms, p95 76 ms a spawn). A guard that does not run, or answers anything but the shape asked for, refuses that call (and only that one: the next call asks again). Every guard now starts with `python3 -I` (the pi spawns, the Claude shims and the hook commands), so a `usercustomize.py` in the user site can no longer stand in for it.
- `hw help launch` says a lane whose `vendor:` is `codex` or `opencode` is no longer supported, in the words of the
  dispatch refusal, instead of "no flag needed".
- `hw wait` and the `hw next` gate share one witness rule (a moved screen is live, anything else unknown).
- `docs/KNOWN-LIMITATIONS.md` is three entries now (macOS and Linux only; the guards are a tripwire, not a sandbox; foreman needs herdr and Claude Code), each as the limit, why, and the workaround. The version floors in `docs/REFERENCE.md` are stated as the versions foreman was tested on; the Windows, first-run, `--permissions ask` and install notes live in `docs/INSTALL.md`.
- `hw next`: the `t<seq>` run directory is made only once the lease gate has passed, instead of being made first and removed on a refusal (no change in what a refusal leaves behind).
- The pi guard's measured cost in the changelog agrees with `docs/pi-vendor.md` (median 40 ms, p95 76 ms a spawn), and INSTALL
  says the pi adapter needs `/usr/bin/python3`.
- New test 1392: no public doc offers Codex, OpenCode, Spec Kit, `hw unstick` or `--sandbox` outside the files that are history.
- `asked: investigate` defaults to `worktree: none` unless `requires:` names `repo` or `database`; `--worktree new` still wins
  and the manifest says which one decided.
- Opus or fable on `kind: explore`, `audit` or `design` is refused unless the
  brief has a `decision:` line or the dispatch has `--model-why` (`review` is
  exempt). A model hw cannot rank is not top tier.
- `kind: design` with `asked: build` goes through the build ceiling: opus needs
  `top_tier_why`. `design` + build keeps opus as its default.
- `fable` is ranked beside `opus`, so the manifest's suggestion note reads it.

### Deprecated
- Codex and OpenCode are outside the 1.0 promise. They keep working, and either
  can be removed without a major version.

### Removed
- The Cockpit's **Dispatch…** control and its `cockpit/dispatch-verbs.txt`. A brief is dispatched from its
  brainer with `hw <lane> <task>`.
- The herdr app's **Artifacts** panel (`foreman-artifacts`): its mod, its entry in the right column's switch
  and stack in the three looks (C, D, G), its tests and its README sections.
- The brain guard's simulator of what `${A#…}`, `${A/x/y}` and `${A:2}` give. Such a read keeps its unknown piece and names brain when brain's name or `..` is in its text or in a value it reads, which also refuses `${A:-${B:-../../..}}/brain/…`, `${A/n[^a]/n}` and zsh's `${A:s/…/}` and `${A[2,20]}`.
- `bin/invoker-common.sh` and `bin/state-witness.sh`, the two bash libraries no program read since the invokers,
  `channel-send` and the witness run in Go, with the subjects and mutation guards that only sourced them. The Go witness
  keeps its answers as goldens frozen from the bash's last run, and `hw next`, `hw ruling` (resuming a blocked task) and a dispatch's brief delivery no longer refuse a
  `brain/bin` for lacking `invoker-common.sh`.
- The Spec Kit flow. `--sdd speckit` and a brief's `sdd: speckit` now die with one message, `hw next --sdd speckit` too;
  hw no longer looks for `speckit-*` skills on disk, names an entry command, refuses a sonnet or haiku
  orchestrator, or prints the `framework` and `entry` lines of a dispatch manifest. `--sdd none` and `sdd: none`
  stay accepted, and ignored, for one release. Limitation L14 and the Spec Kit entry of the dependencies table go with it.
- Codex and OpenCode are no longer supported: foreman runs Claude Code and pi only.
  `--agent codex|opencode` is refused with a message naming the two supported agents.
- `brain --opencode`, the `hw unstick` command and `install.sh --vendor` are gone, with
  the OpenCode and Codex routes of `channel-send` (`opencode`, `opencode-tui`, `codex`),
  the OpenCode plugins and turn-end adapter, the Codex guards and hook, `opencode-auto`
  and `codex-channel-session-hook.sh`.
- KNOWN-LIMITATIONS no longer lists Codex or OpenCode; the file holds three limitations, renumbered L1-L3.
- The JavaScript ports of the guards (`setup/guards/deny-repo-writes.js`, `setup/guards/deny-brain-writes.js`) and the JavaScript half of the guard conformance suite and its mutation arm: there is one implementation of each rule now, so the two can no longer drift.
- `--sandbox` and `sandbox: true` in a brief: hw no longer runs an executor under a Seatbelt profile. The flag and `sandbox: true` die
  with one message before anything is built (a dispatch that asked for confinement must not run without it); `sandbox: false` is
  accepted and ignored. `bin/hw-sandbox`, its outbox broker, the arming on `hc` and on `hw revive`, and the `HW_SANDBOX` branch of
  `done-invoker` and `ask-invoker` are gone. The docs that offered it (README, THREAT-MODEL, REFERENCE, INSTALL, BRIEF-TEMPLATE,
  pi-vendor, `hw help flags`) say so no more. Containment is the operating system's job; the guards stay a tripwire.

### Fixed
- `herdr-app/mods/foreman-cockpit/looks.test.mjs` (run by test 1272) no longer asserts QA-recipe strings that the README
  moved into `bin/herdr-isolated`.
- In the herdr app (the `hc` route, where a brainer and an executor are core terminals, not herdr panes), the Cockpit now
  offers **Reply** and **Rule**: the state file's `ask` carries the hold's `route` (`herdr` or `hc`, null with
  `pending_reply`) and `cockpit/verbs.txt` hands it to `channel-send` as `<route>` where it said `herdr`. Before, the writer
  set `pending_reply` only for a herdr-route hold, so an ask or a challenge raised under the app showed no button.
- The Issues hand-off finds the lane's brainer under the app's core: when `herdr pane list` has none it lists `hc ls --json
  --label label=brain` and delivers with `channel-send hc <terminal>`. Before, it answered "The brainers could not be listed:
  no herdr server is running". A refused send now says its cause, not channel-send's closing "my stdout is NOT a terminal" line.
- `hw done` no longer refuses a report as STRANDED when its task branch is already merged into
  the lane's base by ancestry: a `done-invoker` exit 5 (delivery unconfirmed) whose report the brainer
  received and merged is treated as delivered. A branch still at its start point proves nothing and stays refused.
- `herdr-isolated stop` ends every process whose command line names the instance directory (the cockpit
  writers of a `--brain` instance, which hold no socket and used to respawn a core under it), and
  TERMs again on every wait round instead of leaving a respawn to the KILL.
- `herdr-isolated start` canonicalizes `<dir>` to its physical path, so the app's home and the pane cwd agree
  (`/tmp/x` is `/private/tmp/x`).
- `herdr-isolated shot` puts the app that was in front back in front once it has taken the frame.
- `setup/commit-tree` refuses a commit that names the suite's time-budget table (`test-budgets.json`, its ceilings,
  the retired list or the partition table) when `setup/test-budget-gate` finds the new tree's table over budget, and
  `setup/test-budget-gate` now prints the headroom (full suite and fast gate) on green as well as on red.
- `setup/release/cut` with no `--out` and no `HW_ARTIFACTS` writes its evidence to `~/.cache/hw/release-<VERSION>-<n>`
  (the next free `n`) instead of a mktemp directory, and prints `evidence: <dir>` as its first line and its last.
- `done-invoker --check` with no summary prints a usage line that names `--check`, and its exit footer says
  `CHECK REFUSED` (a rehearsal delivers nothing by design) instead of `NOT DELIVERED`.
- `hw`'s dispatch manifest, for a setup `kind: build` brief with no runnable Verification fence, shows
  `verify      ! setup build with no fence: paste the BRIEF-TEMPLATE Done-when fence` instead of a plain `none —` line.
- `hw done` and `hw reap` no longer list `gpui-kit.d.ts` and `jsconfig.json` (ignored, regenerable) as «DO NOT REMOVE THIS WORKTREE YET». The regenerable list was copied in three places (`core/reap.go`, `core/t4_done_wt.go`, `core/t4_reap_wt.go`); it is now one constant in `core/wt_regenerable.go` that names both files.
- `setup/test-budget-gate` leaves the release-gate partitions (the `RELEASE=` line of `go-test-partitions.txt`: the `m*` mutation partitions) out of its worst-case fast-gate estimate; the full-suite estimate still counts them. Measured on this tree: 1252 s before, 817 s after.
- `HEAVY_ONLY=<F..> bash setup/tests/1040-…` runs only the claims it names; it ran every mutant
  after them.
- A gate run in the main checkout no longer reads the runner's own `gate-runs.tsv` rows as a changed path (it narrowed the
  in-process suite to 0 Go partitions); a full run in an executor ignores `HW_TEST_CHANGED` again instead of hitting the touched
  form's refusal.
- The close after a report now ends the herdr app terminal of a `--worktree none` task in a repo lane:
  `hw done` looked for its run record only under the lane's worktree path, found none, searched herdr's tabs and said
  "NOTHING TO CLOSE" with exit 0, so the terminal stayed open and the report piled up in the cockpit.
- The close that `done-invoker` starts after a report no longer exits 0 when it found no tab, space or terminal to close:
  it says "NOT CLOSED — NOTHING FOUND" and exits non-zero, so the task keeps the `close-failed` file `hw status` shows.
  A hand-run `hw done` on a task that opened nothing is unchanged.
- `done-invoker` and `ask-invoker` no longer refuse an opencode executor started with `--prompt <brief>`: the nested-headless
  guard read the word `run` inside the brief as `opencode run`.
- An opencode executor's turn-end plugin publishes in a herdr-app terminal (`HC_TERMINAL_ID`), not only in a herdr TUI pane.
- Herdr app dispatch (`hc run`) now records `model_running` in the receipt, as the TUI route does, so `bin/cockpit-state`
  and the cockpit row show `vendor · model · effort · ctx` for app-dispatched executors.
- **The stall alert and `setup/heavy`'s FROZEN alert reach a brainer in the herdr app.** Both run
  `channel-send --report herdr "$HW_INVOKER_PANE"`, and an app brainer's address is a terminal (`t24`) that herdr does not
  know: every alert since the move to the app ended in a log as "did not reach pane t24 (channel-send exit 1)".
  `channel-send` now sends the `herdr` word through `hc` when the target is an app terminal and the caller runs on
  herdr-core; a herdr pane id (it carries a colon) and every caller in a herdr TUI keep the herdr route. The vault alert
  uses the same command but still finds the brainer through herdr only (L29).
- **`hw status` for executors in the herdr app.** It read "context: not measured" for every app executor (it asked herdr
  for a terminal herdr does not have) and now reads the terminal's screen with `hc read`; a finished app run read "its
  placement record is gone, so what closed is not recorded" and now names its terminal ("app terminal t4 closed"); a
  stalled or stale app executor is to be read with `hc read <terminal>`, not `herdr pane read`.
- **A `--keep-pane` lease on the herdr app expires.** Its expiry closed nothing for an app run
  (`cleanup=no-owned-container`), so the terminal stayed open for ever; it now ends the run's terminals as `hw done` does.
  The lease also names the terminal that started it instead of `by=unknown`, and an expiry ends terminals only while
  its own run's terminal is still one of the task's.
- **`hw unstick` on an app terminal that names no agent** refused with "t5 is , not OpenCode"; the sentence now reads.
- The Rule button of the herdr app now sends: `hw ruling t5 …` run by the app's own `exec` (no `HC_TERMINAL_ID`, but the app's
  `HERDR_APP_HOME` and `XDG_RUNTIME_DIR`) was refused as "not running as a terminal of an app". A process the app started is now on
  the app's backend; a process with a herdr TUI ancestor, or without those variables, is still refused.
- The answer to an executor's ask in the herdr app now marks the ask `hw_ask_state=answered`; `hc get` kept `delivered` after the reply.
- `done-invoker` reads a background Judgment Day judge's answer: an Agent launched with `run_in_background` answers its tool call only with "Async agent launched", and its findings arrive as a `<task-notification>` (a `user` line, or a `queued_command` attachment). The protocol check used to say «not checked — the judges' results carry no readable findings» over a full ledger; it now reads the notification (once, however many lines repeat it) and holds a background re-judgment to the frozen ledger like a foreground one.
- `hw review --at <sha>` no longer says «review_owed=1 still stands in the build's receipt» when the dispatch fails: a past head never touches that receipt. It says so and prints `hw review <lane> <task> --at <sha>` to re-run.
- `docs/REVIEW-BRIEF.md`: the reviewer checks binaries itself and keeps them out of a judge's target paths; judge prompts carry intent, not reassurance (a test value is not "fine"); three more defect classes (a test or placeholder value reaching production, tests reaching the network through an unpatched client, user-visible text driven by placeholder data).
- A test run can no longer label a live herdr-core terminal: `setup/test-channel-send` puts a tripwire `hc`
  first on PATH (a call is recorded and refused, and fails the file), so the brainer's terminal is no longer
  drawn as an executor with the fixtures of that test.
- The herdr app's Cockpit buttons (Reply, Rule, Done) run `hw` and `channel-send` from the brain's `bin/`.
  They died with «cannot run hw: No such file or directory» because the app's PATH did not carry it.
- The Spaces list drags with a reducer (`foreman-spaces`, a clone of the app's `spaces`): a drop no longer needs a mouse-move
  to have reported the target, and leaving the list cancels the drag. The live pointer path is unverified.
- The gate's snapshot wrapper (`setup/test-hw-snapshot.py`) reads the runner's output as bytes. A subject that
  printed bytes that are not UTF-8 raised `UnicodeDecodeError` in the wrapper, which removed the private
  snapshot while the runner kept going, so every later bash subject read «does not parse».
- The release cut no longer re-runs the fast gate for the commit that only folds the release notes: a
  candidate that differs from a first-parent ancestor with a green fast-gate verdict only in the changelog,
  the release notes and the removed fragments reuses that verdict, and the gate line says which tree and paths.
- A partition of the in-process suite whose only failure is go test's `-timeout` panic, in a run whose process
  tree was seen stopped (the memory guard's freeze), is run once more instead of turning a frozen machine into
  a red cut.
- `bin/statusline-usage-tee` hands the status line command the last 8 MB of a long transcript (`HW_STATUSLINE_TAIL_MB`, 0 turns it off) instead of the whole file: ccstatusline read a 473 MB transcript on every refresh, at 2.8 GB RSS and 4.3 s each; now 178 MB and 0.56 s, same output.
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
- The brainer's batch gate under `setup/heavy` (no `HW_TASK`) now waits up to 3600 s for the Go test batch lock instead of dying at 300 s behind an executor's `--bless`; executors keep 300 s.
- The brain guard no longer lets an expanded or quoted value of `env -u`/`-C`/`--unset`, `nice -n` or `timeout` replace the verb after it: `env -u 'HW_TASK' hw reap setup --apply` and `env -u "$X" secret-unlock --auto` are denied again, and a quoted literal (`env -u "HOME" rg x <brain path>`) is a literal. A wrapper word that may split into several (dash-led `-$(…)`, attached `-u$(…)`, an option word or an assignment such as `A=$(…)`) is also judged as the verb, so `env -u -$(echo X tee) cat <brain path>` is denied instead of running `tee` over the file.
- The brain guard no longer tries to recognise which expanded values may split into a verb (the list missed `"${@:1}"`, zsh `"$arr[@]"`, a quote inside `$(…)`, and a 33-candidate cap let a late one through: each truncated a file in real bash/zsh). A wrapper's option, value or assignment (`env`, `nice`, `timeout`, `command`, `setup/heavy`, `hw secret run`) is skipped only when it is provably one plain word: bare `[A-Za-z0-9_.,:/@%+=-]`, `'…'`, or `"…"` with no `$` (other than `$NAME`/`${NAME}`), backtick, backslash or `[`. Any other leaves the verb undetermined, and a command that names a brain path is refused whatever follows, a reader included.
- Over-refusal that comes with it: `env PATH=$PATH rg x <brain path>` and `env -u X\ Y rg x <brain path>` are now refused. The refusal says why and names the spelling that passes: `env PATH="$PATH" rg …`, `env -u 'X Y' …`. A command that names no brain path is unaffected (`env -u $X rg x /tmp/f`).
- The brain guard judges the WHOLE command once its verb is undetermined, not only the words after the verb: `env -u $V <brain>/setup/heavy -- …`, `nice -n $N <brain>/bin/hw secret run …`, `timeout $D <brain>/setup/heavy -- …` and `env PATH=<brain>/bin:$PATH nice -n $N …` were allowed and truncated brain's own files in real bash/zsh. The refusal now also names single quotes (`env -u 'X Y'`).
- Refused beside it, each shape measured (the classes they belong to stayed open until the next entry, `task-brain-guard-env-unset-fix4`), each a way to run a brain program or write brain that the guard allowed: `timeout -s SIG` / `--signal` / `-k` values were read as the verb (`timeout -s KILL 5 hw reap setup --apply`); `exec -a NAME` and GNU `time -f FMT` / `-o FILE` likewise; a word that expands and spells a program of brain's `bin/` (`env -u {X,hw} reap …`, `$(echo hw) reap …`); a verb that expands to nothing (`E=; $E tee <brain>/x`); a reader's writing flag behind an expanded argument (`sort $O <brain>/x`, `rg $X`, `fd $X`, `find … $X`); a pipe from a brain read into an undetermined verb (`… | env -u $V cat`); and a variable changed by `export A=x`, `read A`, `printf -v A`, `A+=x`, `eval`, a nameref (`$A` after one is a guess, so a wrapper word or verb made of it is undetermined).
- Over-refusal that comes with it: `env PATH=<brain>/bin:$PATH rg x /tmp/q` and a reader with a writing flag (`sort`, `rg`, `fd`, `find`, `sed`, `tree`, `uniq`) given an unreadable argument alongside a brain path are refused. `exec -a NAME cat <brain path>` and `timeout -s KILL 5 rg x <brain path>` are now allowed (the option's value is no longer read as the verb).
- The brain guard skips the value of an `env` option (`-u`, `-C`, `--unset`, `--chdir`, …) only when it is a literal word: a shell-expanded one (`{X,tee}`, `$V`, `$(…)`) is judged as the verb, so `env -u {X,tee} cat <brain path>` is denied instead of running `tee` over the file.
- The brain guard's flat rule admits the two shapes executors use to call a brain program: a heredoc whose delimiter is quoted (`cat > "$HW_ARTIFACTS/checks.md" <<'EOF' … EOF`, then `<brain>/bin/done-invoker …`) and an unconditional literal assignment (`MSG="…"; done-invoker "$MSG"`, `B=<brain>/bin; $B/hw secret check x`), which has exactly one value. Measured on 5426 product-executor commands with the brain at today's root: main refuses 58, the round before last 91, the last round 210, this guard 127 (the 42 above the round before last were each a glob, `$(…)`, unquoted heredoc, block, subshell or brace in a command that names brain).
- Refused now, each allowed by the last round and each a truncation of a brain file or a run of `hw reap --apply` in real bash or zsh: `${X:?$(…)}` and `${X?`…`}` with X unset, a brace holding a quoted or escaped blank (`sort {-o,'<brain>/x/a b'} /dev/null`), a subshell after a reserved word (`! (…)`, `if (…); then`), an assignment chain five deep or after 64 earlier values (`A=<agents>; B=$A; … cat > $E/brain/x`), and zsh `noglob`/`nocorrect`/`-` and bash `jobs -x` before `hw reap setup --apply`.
- `S=<path>; $S "$(cat f)"` is no longer refused (S has one value), and when a verb may expand to nothing the refusal names that variable.
- The brain guard's two admissions are textual whitelists that fail closed (`setup/export/overlay/docs/KNOWN-LIMITATIONS.md` L2). A heredoc is data only behind exactly `'WORD'` or `"WORD"` and fed to no interpreter in its command or pipeline (`bash`, `python*`, `xargs` … one table); any other delimiter spelling (`<<\EOF`, `<<'EOF'x`, `<<'-EOF'`) refuses the command whole, since the guard cannot tell where its body ends. An assignment has one value only in a command whose text holds no `(`, backtick, `{` (but `${`), `[[`, background `&` or reserved word, with no `~`/`=` at its value's start or after a `:`.
- Refused now, each allowed by the last round and each a truncation of a brain file or a run of `hw reap setup --apply` in real bash or zsh: a heredoc after `<<\EOF`, `<<E\OF`, `<<'E'OF`, `<<'EOF'x` or `<<'-EOF'`; `A=~/…/brain/x/f` and `A=~/agents; cd $A` (also `export`, after a `:`, zsh `A==hw`); an assignment after `||` holding `$(:;)`; an assignment inside `if …; end; …; fi` or `time -p while`; `jobs -rx`/`-r -x`/`-xr hw reap setup --apply`; `bash <<'EOF'`, `cat <<'EOF' | bash`, `python3 - <<'EOF'` and `xargs rm <<'EOF'` with a relative brain path; `patch <<'EOF'` naming one; `A="-o ../brain/x/f"; sort $A`.
- The brain guard's last denylists are allowlists that fail closed (`setup/export/overlay/docs/KNOWN-LIMITATIONS.md` L2). A quoted heredoc is data only for a data consumer (`cat`, `tee`, `jq`, `git commit`, `gh pr`, `done-invoker`, `ask-invoker`, `hw`) alone in its pipeline, in a command whose text has no `(`, `{`, backtick or descriptor past 2; any other heredoc is code, and the interpreter table no longer decides it. A variable the command assigns is read flat only as `$NAME` or `${NAME}`; any other read of it (`${A:-}`, `${A#…}`, `${!A}`) is not flat and is read with every value it may give, empty and a literal default included. A heredoc body of more than 512 paths names brain instead of being cut at 512.
- Refused now, each allowed by the last round and each a truncation of a brain file or a run of `hw reap setup --apply` in real bash 3.2, zsh 5.9 or bash 5.3: `false && A=x; cp /dev/null ${A:-}../../../brain/x/f` (and `${A#…}`, `"${A-}"`, `sort -o`, `${A:-<brain path>}`); `cat <<'EOF' |` with the interpreter on the line after the body, `| (bash)`, `| { …; bash; }`, `| tee … | bash`, `> >(bash)`, `exec 3<<'EOF' … bash <&3`, `bash -c "$(cat <<'EOF' …)"`, `| awk '{system($0)}'`, `ed <<'EOF'`, a `patch` body padded past 512 paths; a `<<\EOF` heredoc inside a backtick; bash 5.3's `${ cmd; }` and `${| cmd; }`, read now as command lists.
- The brain guard no longer takes a brain program for a heredoc's data consumer: `hw secret run item --as V -- bash <<'EOF'` execs bash on the body, and was allowed. A quoted heredoc is data only for `cat`, `tee`, `jq`, `git commit` or `gh pr <verb>`, written as the bare name or a path in `/bin`, `/usr/bin`, `/usr/local/bin` or `/opt/homebrew/bin`, with no assignment before it and its subcommand as the next word (`git -c alias.zz='!bash -s' zz commit`, `./cat`, `PATH=/tmp/d cat` are code). The command around it may hold no `|`, `&`, reserved word (`for … done | bash`, `case … esac | bash`, `coproc bash`), no descriptor the guard cannot read (`>&p`, `>&"$FD"`), no `alias`, `hash` or other builtin outside the flat ones, and no PATH or array-element assignment (`setup/export/overlay/docs/KNOWN-LIMITATIONS.md` L2).
- A read of a variable that is not a plain `$NAME`/`${NAME}` keeps its unknown piece, so the known start before it still names brain (`../../../${A:+brain}/x/f`, `${A/x/brain}`, `${A%Z}`, `${!B}`), and is read with what its form may give: a default's or alternate's text (`${A:+../../..}`, `${HOME:+…}`, a quoted default), every part of a cut or substring, a replacement.
- An unknown piece of a word may be empty: `cat /dev/null > $UNSET../../../brain/x/f` (a flat command) is refused.
- bash 5.3's `${ cmd; }` inside an unquoted heredoc body, a `${…}` or an arithmetic is read as a command list.
- The brain guard no longer reads a heredoc fed to `git commit` or `gh pr` as data: each runs the editor that `GIT_EDITOR`, `EDITOR`, `core.editor`, `GIT_CONFIG_*` or `GH_EDITOR` names on the body (`export GIT_EDITOR='bash -s'; git commit <<'EOF'` ran the body). A quoted heredoc is data only for `cat`, `tee` or `jq`, in a command that exports nothing to it: no `export`, no assignment of `PATH` or of a name the environment holds (`setup/export/overlay/docs/KNOWN-LIMITATIONS.md` L2).
- A command the guard cannot finish reading is denied: past 5 seconds of its own work it denies naming the timeout (and the kernel ends it past twice that of CPU), and both of the brain guard's Claude Code hooks carry `"onFailure": "block"`, so a hook that fails or times out blocks the call. `echo ${PATH//*a*a*a*a*a*a*a*a*Q/x}; …` kept the last guard busy past Claude Code's 60-second hook timeout, which let the command run.
- `$PWD`, `${PWD}` and `~+` before any `cd` are the payload's cwd again: `cat /dev/null > "$PWD/../../../brain/x/f"` is refused, as on main.
- The brain guard refuses a path it cannot know instead of reading its text: a word, redirect or here-string that starts with a read the guard does not expand (`${A/x/}`, `${A%…}`, `${A:-…}` of an assigned name, `${!A}`, zsh's `${~A}`, `$~A`, `$A[…]`, `$A:s/…/`, `$_`, `$PWD` after a `cd` it cannot follow), or holds two, is refused with "spell the path literally". `A=<agents>/bxrain; cat /dev/null > ${A/x/}/x/f` and `> ${PWD%/work*}/b?ain/x/f` wrote into brain. A reader's argument (`echo ${PWD##*/}`) is still allowed when no pipe, subshell or substitution takes its output on.
- One cwd model: `cd`, `chdir` and `pushd` are followed where they surely run, `&&` runs what follows them in their target, and a `cd` that may fail or not run (`cd x 2>/dev/null; …`, `false && cd /tmp; …`), one in a subshell, a pipeline or a function, `cd -`, `popd` and `eval` leave the cwd unknown. `$PWD`, `${PWD}`, `~+`, `$OLDPWD` and `~-` are that cwd. `cd ../../..; cat /dev/null > $PWD/brain/x/f`, `chdir ../../..; … > brain/x/f` (zsh) and `(cd /tmp); … > ../../../brain/x/f` wrote into brain.
- No cap on the length of the cwd the guard follows: past 256 characters it stopped resolving relative paths, so a symlink to the agents directory under a long cwd reached brain.
- A brace after a `"\""` in the same command is read again: `echo {a"\""; env -u {X,hw} reap setup --apply` ran `hw reap --apply`.
- An array's elements are its values: `A=(../../../brain/x/f); cat /dev/null > $A` is refused, as on main.
- The brain guard is never weaker than main's: main's guard, frozen as `setup/guards/deny_brain_writes_main.py`, judges every call first, in the same process and deadline, and its refusal is the verdict, with its message. `true && cd ../../..; cat /dev/null > brain/x/f`, `cat /dev/null > l/brain/x/f; for i in 1; do cd /tmp; done` (`l` a symlink to the agents directory) and `A=('../../../brain/x/f'); cat /dev/null > $A` wrote into brain; main refused each.
- A word the guard cannot resolve to a literal is unknown, and an unknown word in a path position is refused (an output redirect target, an operand of a command that may write, a `cd` target before a write), and an assignment of one makes each later read of its name unknown: a `${…}` but `${NAME}` (any operator: `:-`, `/`, `%`, `#`, `@`, `^`, `,`, a substring, a length, an indirection), `$'…'`, a positional or special parameter (`$1`, `$@`, `$*`, `${1%x}`; not `$?`, `$#`, `$$`, `$!`, digits only), `$_`, `$PWD` after a `cd` it cannot follow. Ten rounds of resolvers each missed a spelling that wrote into brain: `A=x; cat /dev/null > d/${A:+../../../..}/brain/x/f`, `${PWD%/work*}/b?ain/x/f`, `${A:-l}/b?ain/x/f` (`l` a symlink to the agents directory), `${HW_TASK/t/l}/b?ain/x/f`, `${A:-$'<agents>/\x62rain'}/x/f`, `set -- <agents>/braix; > ${1%x}n/x/f`, `${@:-l}/b?ain/x/f`, `A=braix; B=${A%x}n; cd ../../..; > $B/x/f`. A reader whose output stays on screen may still print one.
- A reader's argument is data only where what it prints cannot come back as a path or code: no rename, `exec`, function definition or `printf -v` (nor an option word that is not a known literal), no pipe into a non-reader, never inside a substitution, and once a reader or a block wrote to a file, no input redirect, heredoc, substitution, interpreter, `xargs`, `parallel`, `source`, `.` or `eval` after it (anywhere in a loop). `f() { echo ${A%x}/x/f; }; f | xargs cp /dev/null`, `echo … | xargs cp /dev/null $(true)`, `echo ${A%x}/x/f > l; xargs cp /dev/null < l`, a bare `exec > l` before the reader, and `printf "-v" P …`, `printf \-v P …`, `F=-v; printf $F P …` wrote into brain.
- A command with any compound word reads every `cd` in it as uncertain again: a `case` pattern on its own line named `done` was counted as the end of a loop, and the `cd /tmp` behind it was taken as certain.
- The two leak scans of a push (main's guard's and this one's) run side by side, each within its own 40 s: a push waits for the slower scan, not for both in turn, and both fit the hook's 60 s.
- A reader's argument is never exempt after a rename in the same command: `hash -p /bin/rm cat; cat ${A%x}/x/f` (bash), `hash cat=/usr/bin/tee; cat …` (zsh) and `alias cat=tee` wrote into or deleted from brain through a "reader".
- A `cd` target is resolved as bash and zsh resolve it, logically (`..` cancels the name before it), and physically only under `cd -P`, `set -P`, `set -o physical` or zsh's `chase_links`; a start that leads toward brain either way counts. `cd s/../../../../brai$(printf n) && cat /dev/null > x/f` (`s` a symlink to `/var/empty`) wrote into brain: the guard resolved the symlink first.
- The brain guard judges a command that names a brain path only when it is flat — simple commands of known words (literal text, quotes, `~`, the executor's environment variables) joined by `;`, `&&`, `||`, `|`, with redirections and here-strings — and refuses any other, naming the construct and the flat spelling that passes (`setup/export/overlay/docs/KNOWN-LIMITATIONS.md` L2). It replaces the assignment tracker of the previous round, whose bookkeeping let six more spellings truncate a brain file in real bash/zsh (`A=tee; false &&⏎export A=cat⏎"$A" <brain>/x`, `"" <brain>/x` after a heredoc `${A:=tee}`, `[[ 1 ]]&&true; cat /dev/null > <brain>/x`, `printf -v A tee; "${A:-cat}" <brain>/x`, `set -o braceccl; env -u {Xt}ee cat <brain>/x`, `sort $(echo D/*) <brain>/x`). Also refused now, each allowed before: zsh `{ :; } always { A=tee; }`, `if [[ … ]] { … }`, `time -p A=tee`, `HOME=<brain>/x; … > ~/f`, `… | dash` fed a brain path, `timeout .5 hw reap setup --apply`, zsh `=hw reap setup --apply`.
- `B=<brain>/bin; $B/hw …` is refused, and the refusal spells the call that passes: `<brain>/bin/hw …`. So are a heredoc, a glob, `$(…)`, a loop or `if` in a command that names a brain path: write them in a command of their own.
- Measured on 5426 product-executor commands: main refuses 5, the previous round 34, this guard 38; of the 12 it newly refuses beside the previous round, 11 name brain and are not flat, 1 (`S=x; $S "$(cat f)"`) names none. No flat call of `hw`, `setup/heavy`, `done-invoker` or another `bin/` program is newly refused. `env PATH=$PATH rg x <brain path>` (blank-free PATH), `env -u X\ Y …` and `env -u "a\b" …` stay allowed.
- The reuse check and manifest, `hw reap --apply`'s report test and `hw-reconcile` pick the newest run through the one `RunIDs` instead of three private copies of it.
- `setup/tests/37-chaining-lease.sh`: the fixture panes and the sweep root follow the move to `~/agents`, so the subject runs past C01 again (C18a, C21 live).
- `hw next` no longer refuses with "has no chaining authority" a reviewer whose report was delivered. A second dispatch of a task whose terminal is already live launches nothing, and its run directory is now moved into the executor's own run (`.hw/<run>/reuse/<this run>`) instead of staying beside it as the newest run. Every reader of "the run" (`hw next`, `hw done`, `hw status`, `hw review`, the cockpit witness, the never-reported lists, the invoker's env fallback, reap, the brief archiver) therefore reads the run that holds the done marker and the lease, with no check of its own; the per-run `TMPDIR` of the reuse dispatch is removed.
- A reuse dispatch trusts the terminal's `hw_run` label only when it is a run id.
- A refused `hw next` ("NOTHING was sent or touched") no longer leaves an empty `t<N>/` directory in the run.
- Upgrade: a run directory already marked `reused-live-run` by the interim builds of 2026-11-16 is an ordinary run directory now; it stays the newest run of its task until that task is closed with `hw done`.
- Test 1241 asked the setup brain guard about the old root's brain, a path that stopped being brain when the agents root moved to `~/agents`; it now derives the `~` spelling from `guards.json`. The guard was right.
- Test 1391 now runs when a lane shim (`<lane>/.claude/hooks/deny-repo-writes.py`) or `.claude/settings.json` changes.
- The reviewer `hw review` dispatches is always a fresh executor (`--fresh`).
  Before, a kept pane of the lane on another account was picked for reuse and
  the dispatch refused with a `--fresh` remedy `hw review` does not accept; the
  one remedy left, dropping `--account`, ran the review on the default account.
- When the reviewer's dispatch fails, the command printed to retry keeps
  `--account <name>`.
- The review `hw done` runs after a delivered build carries the build's own
  account (the dispatch records it in the receipt as `account`), so a build
  on `personal` is reviewed on `personal`.
- The review `hw done` runs after a delivered build ignores a receipt account
  `hw` would refuse (it is not a known account): it says so and the review runs
  on the lane's own account, instead of carrying the value into the command.
- A release cut checks the `hw-core` its gates drive before its first gate: a tree with none, or one older than
  `core/`'s sources, is built with `setup/build-core --native`, and a build that cannot be done stops the cut at once
  naming that command (or the architecture mismatch when the build left it where `bin/hw` does not look), instead of the
  fast gate dying on it ~40 minutes in.
- `setup/heavy` refuses to start a job (exit 75) when the user's process table has less than a quarter of its limit left
  (at most 1000, at least 100; `HW_HEAVY_PROC_HEADROOM=<n>` sets it flat, 0 turns it off; the limit is the smaller of
  `kern.maxprocperuid` and `ulimit -u`), naming the count, the limit and the commands with most processes, orphans
  first, instead of the job dying on `fork: Resource temporarily unavailable` halfway through. The short lane
  (`build-core`, a focused test) is never refused, so `hw-core` can always be rebuilt.
- `setup/go-test-core.sh` says `FORK FAILED` (with the process count and the limit, exit 75) when it cannot start or
  count its partitions, instead of printing `[: : integer expression expected` or reporting a plain red; it waits for the
  partitions it had already started before it exits, so none outlives the batch lock.
- A new brief key whose "Known keys" evidence in the mutant table was not updated is caught in seconds by a unit test
  of the fast gate, not by the release's mutation batch.
- An ask or challenge that the brainer answers with a plain prompt (not `channel-send --reply-hold`) no longer stays «ask unanswered» in the cockpit while the executor works on the answer: the executor's first tool call or turn end after a `human` prompt retires the hold (the witness is the session transcript's `origin.kind`, so a queued ruling, a Stop-hook block or a background job's wake never retires an unanswered ask), and that turn end is then judged as a turn with no hold. The `post-tool` hook no longer returns early while a `pending-reply-N` hold exists.
- `setup/heavy`: a waiter, short or slot, whose own queue ticket has been removed (its queue directory deleted underneath it) now exits 75 at once, naming the ticket path, instead of waiting forever. Ten `--short --tag build-core` waiters slept ~4 h on 2026-11-16. A long queue is not stuck, so there is no time limit. A wait cut short under `--when-idle` no longer leaves the pause clock running.
- `bin/core-rebuild`: a builder that exits 75 (`setup/heavy` refused: no process headroom or memory, or its own ticket was gone) is no longer recorded as `rebuild-failed`, so the stale `hw-core` is retried on the next call instead of being left in place until its sources change.
- `hw <lane> <task> --dry-run` from a plain terminal prints the manifest (its return channel says `UNRESOLVED`) instead of stopping at `HW_INVOKER_PANE is UNRESOLVED`; a dry run builds nothing, so it needs no return channel. A real launch still refuses.
- `hw review --at <sha>` on a commit the lane's base already holds measures it against its first parent, so the review counts the commit's lines and dispatches a reviewer instead of reporting 0 code lines.
- The sleep/poll guard reads `nice --adj 10` / `--adj=10` (a long-option prefix) as a prefix, so `nice --adj 10 sleep 600` is judged as `sleep 600`.
- Setup test 1382: a crashing mutant no longer counts as killed; the b07/b08 rows are in order and `env -u -$(…)` has its row.
- Setup test 1391 selects on `guards.json` and `deny_repo_writes.py`; test 1010 names the agents-root brain path.
- `channel-send` (hc route): when the receiver's transcript watermark (`claude-delivery-receipt.py --mark`) cannot be taken, the transcript proof is skipped and the counters decide; before, the failure became "none" and the scan started at byte 0, so an earlier send of the same bytes could count as PROVEN delivery.
- `brain restore`: a lane named twice (or a lane and its alias) is restored once; the second resume ended the terminal the first had just made.
- `channel-send` (claude route): the same failed-watermark rule: no watermark, nothing is sent and the refusal says so (it used to treat the failure as an empty watermark and could report PROVEN from an earlier send).
- `setup/merge-task` no longer reports NOT CONSISTENT when a dirty tracked path with a newline in its name has a piece that
  equals a merged path, and no longer dies in `tr` on a non-UTF-8 tracked name under a UTF-8 locale (it runs in the C locale).
- `setup/merge-task` no longer reports a case-only rename of a dangling symlink as STILL ON DISK, and a deletion git could not
  apply is no longer also counted and listed as «removed».
- `hw train push` no longer drops the staged change of a path that is both staged and edited again (it is git's refusal, like
  any staged path), and refuses before touching an uncommitted edit whose backup cannot be written instead of losing it.
- `hw train push` with several carried edits: a failed backup on any of them puts every edit back (it used to lose the earlier
  ones), a write-back that fails after the advance exits non-zero naming the copy, and a path the train deletes with only a mode
  edit advances again. `setup/merge-task` no longer forks per pair when reporting case-only renames (300 renames: minutes -> seconds).
- `setup/merge-task` no longer reports NOT CONSISTENT (exit 3) for a merge that renames a directory only by case on a
  case-insensitive volume: the added path that is the same file as a removed one is not counted as untracked.
- Test sandboxes no longer leak processes: a contract world (`setup/tests/contract-hw/world-t3.sh`) now stops every process whose command line holds its path, and their descendants, so the `stall-watch` an `executor-turn-end` case starts (and the `bin/core-rebuild` / `setup/heavy` waiter a stale `hw-core` puts behind it) no longer outlives the case orphaned to pid 1. Test 1352 stops the `setup/heavy` waiters under its killed jobs too, not only their subshells.
- `bin/core-rebuild`: a builder refused by `setup/heavy` (exit 75) is now reported as "refused, not broken", not as a failed build; the next call still tries again.
- Test 164's fixture no longer copies the gitignored `core/dist` (82 MB of release binaries in a main checkout). Test 1387 does not fail on a user whose hard process limit is under 3500; it says it did not exercise those two cases.
- The runner snapshot (`setup/test-hw-snapshot.py`) removes a read-only directory its first capture copied, so the retry after a changed input no longer dies with `File exists`.
- `go-test-core.sh`: FORK FAILED says "most likely" the machine ran out of processes (a partition killed by a signal leaves no result either).
- `hw help launch` named only Codex and OpenCode as unsupported vendors, so a lane with any other vendor (`gemini`) read
  "no flag needed" while the dispatch refused it; both now share one predicate and one sentence.
- `hw next --brief` with `sdd: speckit` is refused like the launch refuses it, instead of delivering the brief.
- The Agents list's drop line for a group moved down now sits under the last row's unfolded sub-rows, not between that row and them.
- The Agents list's drop line no longer draws twice for a pinned sub-agent, and a row dragged down onto a row with sub-rows now puts the line under them.

## Known limits

- macOS and Linux only. Windows, native or under WSL2, is not supported or
  tested (KNOWN-LIMITATIONS, L1).
- The Go core is on by default when the installer builds it; `HW_IMPL=bash`
  goes back. Every limit a user can hit is in
  [`KNOWN-LIMITATIONS.md`](KNOWN-LIMITATIONS.md).

## Upgrading from 1.0.0-rc.4

Run `foreman-setup upgrade --brain DIR`. A brain installed before `upgrade`
existed has no record of its flags yet: run your install command once more,
and `upgrade` works from then on.

Every release's changes are in [`CHANGELOG.md`](CHANGELOG.md).
