---
kind: review
model: opus
review: [{{review_list}}]
review_of: {{review_of}}
requested_by: "{{requested_by}}"
asked: {{asked}}
sdd: none
---

# Independent re-check of {{lane}}:{{task}} @ {{head}}, since {{prior_head}}

This file is the template `hw review` fills for a build that was reviewed before (core/t6_review_delta.go): `{{slot}}` words are
its slots. The last review judged `{{prior_head}}` and said **{{prior_verdict}}**. You judge what changed since: the builder's fix
round. Judge the bytes at `{{head}}`; change nothing and post nothing to GitHub.

## What you review

- Branch head: `{{head}}` (check `git rev-parse HEAD` in your worktree: it must print exactly that, or close `--blocked`).
- Delta: `git diff {{delta_from}} {{head}}` — {{code_lines}} code-only line(s), tier **{{tier}}**: {{delta_how}}. Review this
  diff, not the whole build; `hw review-range` prints an empty range here, because this worktree is cut at the reviewed SHA.
- The prior review: `{{prior_review}}`. Read it before the diff. For **each of its rows** write one line in `review.md`, under a
  `## Prior rows` heading, **with no `|` in it** (review.md's finding rows are the only lines that carry one):
  `<id>: resolved — <file:line at the head, what now holds>` or `<id>: open — <why it still stands>`. A prior `blocker` that is
  open is a `blocker` row again, in the finding rows, so the verdict stays `changes-needed`. A prior `should` or `follow-up` that is
  open is only its line.
- **You report a `blocker` this delta introduced (a fix that broke something, or a path the fix opened), or you approve.** It carries
  a failure scenario and a file:line, and it is the only new finding row: no new `should`, no new `follow-up`. A `should` or a defect
  outside the delta that you see goes in ONE line under a `## Backlog` heading in `review.md`, with no `|` in it, never a row (this narrows «What you leave» below). Read the files the delta touches, not only its hunks.
- Intent: the build's brief, {{build_brief}}. Read it for what was asked, never for what was done.
- The builder's `checks.md`, PR body and test results are **claims, not evidence**. Re-derive what you rely on: run the tests
  yourself when one decides a finding.

{{common}}
