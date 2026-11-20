package main

// The review item `qa` (brief review-ui-qa, 2026-11-20): the independent reviewer of a UI build runs the lane's qa-tester on the
// dev server hw starts on its worktree. Split out of t6_review.go.

import (
	"os"
	"strings"
)

// reviewQAPrompt fills {{qa}} when the reviewer's list has `qa`: the lane's qa-tester, once, against the dev server hw started on the
// reviewer's own worktree at the head, at two viewports, with the build brief's cases. The captures go into the PR body's one
// `## Review` section ({{post_exception}} carries the edit), never a comment.
const reviewQAPrompt = `## What you look at: ` + "`qa`" + `

This review runs ` + "`qa`" + `: the build changes what a person sees, so you check it on the screen, at ` + "`{{head}}`" + `, before you write ` + "`review.md`" + `.

- Launch the lane's ` + "`qa-tester`" + ` agent with the Agent tool, ` + "`subagent_type`" + ` ` + "`qa-tester`" + `, **once**, in the foreground (a background one proves only once it completes: wait for its notice). done-invoker proves that call from your transcript and refuses a review without it.
- Target: ` + "`http://localhost:$HW_PORT_WEB`" + `, the dev server hw started on this worktree. A cold boot takes tens of seconds: prove an HTTP 200 before the launch. Never staging, never another port.
- Its prompt carries the cases below, each at two viewports, **375** and **1440** px wide, one capture per case and viewport in ` + "`$HW_ARTIFACTS/qa/`" + ` (` + "`<case>-375.png`, `<case>-1440.png`" + `); ONE browser launch for the whole run, never a retry of a failed launch, closed at the end with ` + "`{{brain}}/bin/browser-close`" + `; it changes no file.
- A case that fails, at either viewport, is a ` + "`blocker`" + ` row of ` + "`review.md`" + ` naming its capture. A QA that cannot run (no 200, a failed launch, no ` + "`qa-tester`" + ` among your agents, ` + "`gh-attach --check`" + ` refuses) is ` + "`done-invoker --blocked`" + ` naming it: never a ` + "`review-skip:`" + `, never an approve.
- The captures go into the PR description's one ` + "`## Review`" + ` section (below), never a comment: ` + "`{{brain}}/bin/gh-attach \"$(gh repo view --json nameWithOwner -q .nameWithOwner)\" \"$(gh pr view {{branch}} --json number -q .number)\" \"$HW_ARTIFACTS\"/qa/*.png`" + ` (no ` + "`--comment`" + `) prints one markdown image line per capture — that upload and the edit below are the GitHub writes qa adds; put each in ` + "`review-section.md`" + ` under a line naming its case, its viewport and passed or failed.

The cases, from the build brief:

{{qa_cases}}`

// reviewQAGate refuses a reviewer whose list has qa but who would get no dev server, naming what is missing: every path to a
// reviewer (the tab, --here, done-invoker's auto-dispatch) is `hw review`, so this one check covers them all. --here gets none
// even in a lane that has one: it exits before a dev server would start (t5_manifest.go).
func (a *app) reviewQAGate(p *reviewPlan, here bool) {
	if !has(p.list, reviewQAItem) {
		return
	}
	miss := a.c.reviewQAMissing(p.lane)
	if here && a.c.devServerMode(p.lane) != "" { // otherwise reviewQAMissing named it
		if miss != "" {
			miss += " and "
		}
		miss += "no dev server (--here starts none: dispatch the reviewer without --here)"
	}
	if miss != "" {
		a.die("hw review: the reviewer's list has qa, which runs the lane's qa-tester against the dev server hw starts on the reviewer's worktree, but lane " + p.lane + " has " + miss + ". NOTHING was dispatched.")
	}
}

// reviewQADefault is the code files that give a build's reviewer `qa` by default, nil when none do: the build's review: is not none
// (or an empty list, which is the same mode) and a list that does not already name qa (an explicit list gets it added too, brief review-qa-on-explicit-list), the lane has
// qa-tester and a dev server (reviewQAMissing), and a code file of the diff is UI by the lane's ui_paths (reviewUIFiles). What counts
// as UI is the lane's config, never a list hw carries.
func (a *app) reviewQADefault(lane, raw string, codeFiles []string) []string {
	list, _ := reviewListOf(raw)
	if mode, _ := reviewModeOf(raw); raw != "" && mode == "none" || has(list, reviewQAItem) { // "" is no review: line, which gets the default
		return nil
	}
	if ui := reviewUIFiles(codeFiles, a.c.t.reviewUIPaths(lane)); ui != nil && a.c.reviewQAMissing(lane) == "" {
		return ui
	}
	return nil
}

// briefSections is the body of the brief's `## <name>` sections (a heading that is the name, or starts with it and a space), in
// the order asked, each with its heading; "" when the brief has none of them.
func briefSections(brief string, names ...string) string {
	b, err := os.ReadFile(brief)
	if err != nil || brief == "" {
		return ""
	}
	found := map[string]string{}
	cur, fence := "", false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fence = !fence
		}
		if !fence && (strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ")) {
			cur = ""
			title := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			for _, n := range names {
				if strings.HasPrefix(line, "## ") && (strings.EqualFold(title, n) || strings.HasPrefix(strings.ToLower(title), strings.ToLower(n)+" ")) {
					cur = n
				}
			}
		}
		if cur != "" {
			found[cur] += line + "\n"
		}
	}
	var out []string
	for _, n := range names {
		if v := strings.TrimSpace(found[n]); v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, "\n\n")
}
