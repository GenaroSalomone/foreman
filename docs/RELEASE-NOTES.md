# foreman 1.2.0

foreman 1.2.0 reworks the independent review and keeps the machine clean on its own: cheaper, bounded reviews that look at the screen for UI builds, a lane ledger that outlives compaction, and leftovers that clean themselves up.

## In short

- **Reviews cost what the change costs, and they end.** Tier S reviewers run at medium effort, a fix round is re-checked on its own delta, and a build gets at most three reviews.
- **A UI build is reviewed by looking at it.** The review list takes `qa`: the reviewer runs the lane's `qa-tester` on a dev server at the reviewed head, at 375 and 1440 px, by default on a UI diff.
- **A brainer's agreements survive compaction.** `bin/ledger` keeps orders, owed items and handed-over facts per lane, re-injected at start and compaction.
- **Leftovers clean themselves up.** `hw reap --apply` removes dead-executor worktrees and old cache trees, archiving uncommitted work first and never deleting a branch, and `hw status` starts it once a day.
- **The suite reruns only what changed.** Each bash subject is keyed to the files it read, and `hw status` reports the build → merge bottleneck daily.

## What changed

65 changes since 1.1.0 (1.2.0-rc.1 and rc.2 folded in).

### Added
- `bin/ledger`: one small file per lane (`<lane>/ledger.md`: orders in force, items owed, facts handed over)
  that a brainer writes with `ledger add|done|show`. A SessionStart hook (startup, resume, compact) puts it
  back into the brainer's context after compaction; executors never see it.
- `hw done` on a task whose brief says `asked: investigate` and whose run reported done adds
  `act on <lane>:<task> — engram #<ids>` to the lane ledger's Owed section, once per run; a build or a
  blocked report adds nothing.
- `hw review` re-checks a build that was reviewed before on the delta only: from the last reviewed head (whatever its
  verdict) to the new head, tiered on that delta, with the prior review.md to mark row by row as resolved or open. A
  merge of main into the branch is not counted as the fix. It re-tasks the live reviewer when it can (`hw next`) and
  starts a fresh one on the same delta otherwise. A brief may declare `round_of: <parent task>`; dispatch records the
  parent's last verdict in the receipt and refuses `review: none` on a fix round.
- `hw review <lane> <task> --model <m> --effort <e>` overrides the reviewer's model and effort; the dispatch validates them and the manifest shows them as chosen.
- A lane can add pieces to every reviewer of its builds: `review_default` in
  `projects.json` (`thermo`, `tests`, `ponytail`, `pstack`, `comment-sicko`),
  joined after the build brief's own `review:` list, each piece once.
  `judgment-day` stays a per-brief risk call and is refused there.
- `comment-sicko`, pstack's comment review agent, is now a `review:` list item
  for the reviewer; it is launched report-only, in the foreground or the
  background.
- `review_post: pr-description` (`projects.json`) has the reviewer write its
  findings into one `## Review` section of the PR body, never a comment.
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
- `docs/BRIEF-TEMPLATE.md` is a short fill-in form (goal, context with premises, limits, done when,
  verification fence, mechanism) with the used keys only; `hw help flags` owns what a key means.
- The executor prompt now carries three rules the template could not deliver: a dropped item is a
  `pass` row with `dropped: <proof>`, the smallest change that meets Done when, and (setup builds)
  the manifest row and test-budget row a new file needs.
- `hw help flags` now documents every brief key the template's comment block used to (release and its
  env-from/env-diff/screens/qa, rehearsal-of, authorizes, secrets, kind, requires, placement, worktree,
  database, invoker, ...), and says the template only lists the ones in use.
- The isolated-herdr QA recipe and the leak-detector text moved from the template to `hw help advanced`.
- The template's live frontmatter keys are bare (a hint is a comment line of its own): hw reads a value
  to the end of the line, so a brief filled in from the old template was refused at dispatch.
- The checks a setup build runs that no tool prints (budget gate, timing claims, mutants, a pi leg, PR
  checkouts, after-merge installs) live in `setup/runbooks/setup-build-checks.md`, named by the setup
  executor prompt; the smallest-change rule reaches every build, not only one that owes the review.
- `setup/commit-tree` refuses a red touched subject only when it is green on the base too: a red subject is
  re-run once on the parent through `setup/test-hw --base`, and red there as well prints
  `RED ON BASE TOO <subject> (not this commit's; allowed)` and the commit goes through. A base that cannot be
  measured (timeout, no verdict, over the 180 s allowance) still refuses, saying so.
- The lane ledger is written by the tools, not by a rule: `hw <lane> <task>` leaves `<task> running in <pane> (auto since <date>)`
  in the lane's Owed (one line per task, a re-dispatch replaces it; a dry run, a brief that did not land and a review run leave none), and the
  admitted report (`done-invoker` delivered, or `hw done`) marks it ` — done <date>` or ` — blocked`, and `hw done` of a run that
  never reported marks it ` — closed <date>`. The ledger cap still holds: a write that would pass 4096 bytes first drops auto
  entries (done, closed, blocked, then running; oldest first), never an order, a fact or an item a person added. Only the
  `(auto since <date>)` mark makes an entry auto, so a hand-written `x running in y` item is never reclaimed.
- `bin/ledger hook` ends with a footer naming the write commands (`ledger add order|owed|fact`, `ledger done <n>`), also for an
  empty ledger; the sentence in the brainer rules that said the same is gone.
- An executor's pre-commit no longer queues the Go partitions whose estimate is over
  `touched_budget_seconds`: it names them in the last lines for the brainer's batch gate
  and still runs its bash subjects (one `bin/` file paid ~12 minutes of Go before). A
  person's commit, and the batch gate, run them as before.
- The pre-commit no longer says "the whole fast gate's verdict is still owed" after a
  selected gate.
- `hw done` carries an approval at an ancestor of the head when the commits since have no code, and refuses
  `--accept-review` when they have: it prints the `hw review` command for the delta instead.
- `hw next` records `review_of_t<n>` for a re-tasked reviewer, so its verdict is held to the head its worktree is at.
- `hw review --lift-cap` takes a reason that starts with a dash, and a failed dispatch's retry carries the flag.
- `hw review` dispatches a reviewer for a prose-only fix after changes-needed (it said "no code changed" while `hw done`
  denied, so `--accept-review` was the only exit); after an approve it still dispatches nobody and `hw done` carries it.
- The review round cap counts the reviews of every task in a `round_of` chain (the builder and each fix task), so a 4th
  review of the chain needs `--lift-cap` instead of starting again at 1 on each fix task.
- `hw review` runs the reviewer at `--effort medium` on tier S (1-400 code lines; a delta re-check is tiered by its own delta); tier L, and S or L with `judgment-day`, keep `high`. A re-tasked reviewer keeps the effort it was launched with, and the plan says so. The model stays `opus`.
- `done-invoker` proves a review agent launched in the background (Claude) from
  its `<task-notification>` reporting it completed, the way a background judge
  is read: the launch alone proves the start, so a background
  `pstack:comment-sicko` that never completes (or fails) is refused like a
  missing one. The reviewer cannot force a foreground run, and its brief no
  longer asks for one.
- `hw review` dispatches at most 3 reviews per build. The 4th is refused, naming both exits:
  `--lift-cap "<why>"` (the reason lands in the build's receipt as `review_cap_lifted`), or close the
  build with its open review rows moved to KNOWN-LIMITATIONS.
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

### Removed
- The template's Committing, QA, Reviewing PRs, Access, When you finish, After merge, If you get
  blocked, Notes and In/Out of scope sections, and its 9 KB key-comment block.
- The independent reviewer no longer writes `checks.md` requirement rows (one `pass` per piece: 1096 `pass` and 0 `fail` in the
  review-system eval) nor the `nit` severity (427 rows nobody read). `review.md` keeps `blocker`, `should` and `follow-up`; `checks.md`
  holds only the function table. `hw review` stops writing `checks_required` for a reviewer, and `done-invoker` no longer asks a
  reviewer for a row per piece: each piece is still proved from its transcript.
- The delta re-check no longer asks for new `should` and `follow-up` rows: it reports a `blocker` its delta caused, or approves; a `should`
  it sees is one line under `## Backlog`.

### Fixed
- `channel-send claude <target> --reply-hold <file>` now works when the target is an
  hc terminal: the target is turned into an hc target before the hold is checked, so
  every flag the hc route takes works through `claude` too.
- `channel-send claude <target>` now reaches a claude session that lives in an hc
  terminal (a brainer): it names the terminal by address, name or session id and
  delivers through the hc route, with the same proof and exit codes. It used to
  refuse with "use the herdr route", a route those terminals do not have. A
  target held by no herdr pane and no hc terminal is still refused, now naming
  `channel-send hc <terminal-name>` and `hc ls`.
- A product executor can now run `gh-attach` (`gh-attach --check`, `gh-attach <owner/repo> <n> <image>… [--comment …]`)
  as `docs/gh-attach.md` tells a brief to hand it over. The brain guard refused it, so every UI brief that followed
  the documented pattern blocked. Only `gh-attach` was added; writes into brain are still refused.
- The ledger hook now prints for every lane's brainer (it no longer requires the engram label `brain`, which a lane's
  brainer does not carry), and also on `/clear`.
- `bin/ledger` keeps a line a person hand-edited into `ledger.md` instead of dropping it, takes a lock around
  `add`/`done`, and does not add an Owed item that is already there.
- `hw done` names engram ids only as `engram #N` or `#NNNN`+, not any `#N`. Limit: in a chained session only the last
  report's Owed line is written.
- A line above the first heading of `ledger.md` stays there; it was re-filed into Handed over on the next write.
- `ledger show` and `ledger done <n>` number from one place, and an expired order is dropped together with its indented
  continuation line only: an unindented line a person typed after it stays.
- The brainer handback guard (Stop hook) holds every lane's brainer, not only the one labelled `brain`: it asks the same question
  as the ledger hook (neither `HW_TASK` nor `HW_RUN` set).
- `hw next` no longer refuses a re-task brief whose `model:` or `effort:` equals what the pane runs when the
  dispatch line carries a suggestion after its `(chosen)` mark: the value is read up to the first mark, wherever
  it sits. `hw review`'s re-task of a kept reviewer stops falling back to a fresh dispatch for the same reason.
- `HW_TEST_CHANGED` set to a path that is not a readable file exits 2 naming it, instead
  of widening to the whole fast gate.
- The executor sleep-poll guard's refusal of a backgrounding `&` quotes the `&` with 20
  characters of context on each side instead of the first 120 characters of the command.
- Two dispatches of the same task no longer race: the second says `being dispatched by pid N`, waits (at most
  15 minutes) and then reuses what the first built, instead of failing half-way (seen as a duplicate
  `CREATE DATABASE` when a brainer ran `hw review` while done-invoker was already dispatching it). The lock
  leaves no file behind.
- A build's done report ends with `review: done-invoker dispatches it right after this report (reviewer <task>)`
  when the review is owed, so the brainer does not dispatch it a second time.
- `done-invoker` read `run_in_background` as a bool, but Claude Code records the
  string `"true"`: the whole assistant line was dropped, taking background
  judges and a Skill call of the same message with it. Both shapes are read.
- A review agent counts as background by what its `tool_result` says
  ("Async agent launched"), not by the input flag alone (most async launches
  carry no flag): such a call proves only once its notification says it
  completed, a foreground one proves as before, and judges are untouched.
- The `review_post: pr-description` block stops before any edit when
  `gh pr view` fails, instead of replacing the whole PR description with the
  Review section.
- The same block replaces the `## Review` section of a CRLF body instead of
  adding a second one.
- A fix branch that merged main is measured at its own lines: `hw review` starts the diff at the merge-tree of its base and the lane's
  newer commits (ledger-writers-fix2 counted 162 files, tier L, for a 30-line fix), and the plan's base line says how.
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

## Upgrading from 1.2.0-rc.2

Run `foreman-setup upgrade --brain DIR`. A brain installed before `upgrade`
existed has no record of its flags yet: run your install command once more,
and `upgrade` works from then on.

Every release's changes are in [`CHANGELOG.md`](CHANGELOG.md).
