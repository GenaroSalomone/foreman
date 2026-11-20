# foreman 1.2.0-rc.1

foreman 1.2.0-rc.1 reworks the independent review: cheaper reviewers, delta-only re-checks, a round cap and lane defaults, plus a lane ledger that outlives compaction.

## In short

- **Reviews cost what the change costs.** Tier S reviewers run at medium effort, a fix round is re-checked on its own delta (a merged main no longer counts), and `--model`/`--effort` override both.
- **Review rounds end.** At most three reviews per build, and a delta re-check reports only the blockers its delta caused.
- **Reviewers write less that nobody reads.** No more `checks.md` requirement rows or `nit` findings; a lane can add its own review pieces with `review_default` and post the review in the PR description.
- **A brainer's agreements survive compaction.** `bin/ledger` keeps orders, owed items and handed-over facts per lane, written by `hw` and done-invoker and re-injected at start and compaction.
- **Fewer false stops.** `hw next` reads a pane's model past its mark, two dispatches of one task wait for each other, and `commit-tree` refuses only reds that are not red on the base too.
- **Executors get `gh-attach`** for PR captures, and talk to hc-hosted sessions through `channel-send`.

## What changed

48 changes since 1.1.0.

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

## Known limits

- macOS and Linux only. Windows, native or under WSL2, is not supported or
  tested (KNOWN-LIMITATIONS, L1).
- The Go core is on by default when the installer builds it; `HW_IMPL=bash`
  goes back. Every limit a user can hit is in
  [`KNOWN-LIMITATIONS.md`](KNOWN-LIMITATIONS.md).

## Upgrading from 1.1.0

Run `foreman-setup upgrade --brain DIR`. A brain installed before `upgrade`
existed has no record of its flags yet: run your install command once more,
and `upgrade` works from then on.

Every release's changes are in [`CHANGELOG.md`](CHANGELOG.md).
