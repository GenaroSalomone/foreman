package main

// review-delta-recheck (setup/evidence/review-delta-recheck-2026-11-19.md): a re-check after a fix round reviews only what the
// fix changed. `hw review` finds V, the last reviewed head of the build, whatever its verdict; the diff it measures, tiers and
// hands the reviewer runs from T to the head, where T is V itself, or V with the base merged in when main was merged into the
// branch since (so main's commits are not the fix's). It is a full re-check only when V is not an ancestor of the head, when the
// merge of the base into V conflicts, or when there is no V. The reviewer is the kept pane of the last review when it can be
// re-tasked (`hw next`), a fresh one otherwise. `hw done` reads the same V: an approval at an ancestor carries when the delta is
// prose, and is refused otherwise with the delta's own command.

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// reviewDelta is a re-check of V..head: V's verdict and where its review.md is, the reviewer that wrote it, and T.
type reviewDelta struct {
	v, verdict string // V (full SHA) and the verdict the last review gave at it
	review     string // V's review.md, the reviewer's prior rows
	kept       string // where a re-tasked reviewer finds it: the copy hw leaves beside the review.md its new review replaces
	rtask      string // the task whose worktree holds V's reviewer ("" when only a prior_review names it)
	t, tHow    string // the delta starts here: V, or the merge-tree of V and the base; how it was derived
}

// reviewDeltaOf is T for a re-check of v..head: v itself when the head's merge-base with the base (mb) is already in v, else the tree
// `git merge-tree --write-tree v mb` (v with the base's newer commits in it), so a main merge is not counted as the fix. ok is false,
// and how says why, when the re-check must be a full one: v is the head or not an ancestor of it (a rebase or force-push), there is
// no base, or merging the base into v conflicts.
func (a *app) reviewDeltaOf(checkout, v, head, mb string) (t, how string, ok bool) {
	switch {
	case v == "" || v == head:
		return "", "there is no earlier head: nothing to measure a delta from", false
	case mb == "":
		return "", "the head has no merge-base with the base", false
	}
	if _, anc := gitRun(checkout, nil, "merge-base", "--is-ancestor", v, head); !anc {
		return "", "the last reviewed head " + reviewShort(v) + " is not an ancestor of " + reviewShort(head) + " (rebase or force-push)", false
	}
	if _, anc := gitRun(checkout, nil, "merge-base", "--is-ancestor", mb, v); anc {
		return v, "the last reviewed head " + reviewShort(v) + ", the base had not moved into the branch since", true
	}
	tree := reviewMergeTree(checkout, v, mb)
	if tree == "" {
		return "", "the base merged into the branch since " + reviewShort(v) + ", and merging it into that head conflicts", false
	}
	return tree, "the last reviewed head " + reviewShort(v) + " with the base's newer commits merged in (git merge-tree " + reviewShort(tree) + "), so the base's own commits are not the fix", true
}

// reviewMergeTree is the tree of `git merge-tree --write-tree a b` (a with b's newer commits in it), "" when the merge conflicts.
func reviewMergeTree(checkout, a, b string) string {
	out, ok := gitRun(checkout, nil, "merge-tree", "--write-tree", a, b)
	tree, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	if !ok || len(tree) < 40 {
		return ""
	}
	return tree
}

// reviewerTaskOf is the task whose worktree holds the reviewer the build's last `hw review` used: review_task, which a re-tasked
// reviewer needs (its brief is named for the new head, its worktree is the first reviewer's), else the review_run brief's name.
func reviewerTaskOf(runDir string) string {
	if v := lastLine(receiptValues(runDir, "review_task")); v != "" {
		return v
	}
	run := lastLine(receiptValues(runDir, "review_run"))
	if run == "" {
		return ""
	}
	return strings.TrimSuffix(doneBasename(run), ".md")
}

// reviewerArtifactsOf is the $HW_ARTIFACTS of the reviewer task's worktree, where its review.md is.
func (a *app) reviewerArtifactsOf(lane, rtask string) string {
	rwt := a.workTree(lane, rtask)
	if art := a.reviewArtifactsOf(rwt); art != "" {
		return art
	}
	return rwt + "/.artifacts"
}

// reviewPriorOf is V: the verdict of the build's last review. It reads the review.md of the newest run's review_run (through its
// review_task), and falls back to the receipt's prior_review, `<verdict>@<sha> <review.md>`, which a fix round's dispatch wrote from its
// parent's review. ok is false when the build has no earlier review whose verdict can be read.
func (a *app) reviewPriorOf(lane, wt string) (d reviewDelta, ok bool) {
	for _, r := range runsNewestFirst(wt + "/.hw") {
		dir := wt + "/.hw/" + r
		if rtask := reviewerTaskOf(dir); rtask != "" {
			path := a.reviewerArtifactsOf(lane, rtask) + "/review.md"
			if raw, err := os.ReadFile(path); err == nil {
				if verdict, sha, vok := reviewVerdictOf(string(raw)); vok {
					return reviewDelta{v: sha, verdict: verdict, review: path, rtask: rtask}, true
				}
			}
		}
		if pr := lastLine(receiptValues(dir, "prior_review")); pr != "" {
			vs, path, _ := strings.Cut(pr, " ")
			verdict, sha, found := strings.Cut(vs, "@")
			if found && sha != "" {
				return reviewDelta{v: sha, verdict: verdict, review: strings.TrimSpace(path)}, true
			}
		}
	}
	return reviewDelta{}, false
}

// reviewDeltaPlan makes the plan a re-check of V..head when the build has a V and the delta can be measured; p.from then starts at T.
// Otherwise it is the full review, and p.fullWhy says why when there was a V.
func (a *app) reviewDeltaPlan(p *reviewPlan, checkout, mm string) {
	d, ok := a.reviewPriorOf(p.lane, p.wt)
	if !ok {
		return
	}
	out, found := gitRun(checkout, nil, "rev-parse", "--verify", "--quiet", d.v+"^{commit}")
	if !found {
		p.fullWhy = "the last review judged " + d.v + ", which is not a commit here"
		return
	}
	d.v = strings.TrimSpace(string(out))
	t, how, dok := a.reviewDeltaOf(checkout, d.v, p.head, mm)
	if !dok {
		p.fullWhy = how
		return
	}
	d.t, d.tHow = t, how
	p.delta, p.from = &d, t
}

// reviewReviewerPane is the pane and run directory of a reviewer task: the `pane` of the receipt of the newest run of its worktree.
func (a *app) reviewReviewerPane(lane, rtask string) (pane, rundir string) {
	rundir = a.reviewRunDir(a.workTree(lane, rtask))
	if rundir == "" {
		return "", ""
	}
	return lastLine(receiptValues(rundir, "pane")), rundir
}

// reviewRoute decides whether the re-check re-tasks the delta's reviewer: its pane answers, its chaining lease is live, the pieces of
// the new brief are plugins it was launched with, and no `--account` is asked (an account is fixed at a pane's launch, and hw next
// has no account check). p.retask is its pane, or "" with p.retaskWhy saying what ruled it out, and the dispatch is a fresh reviewer.
func (a *app) reviewRoute(p *reviewPlan, account string) {
	if p.delta == nil {
		return
	}
	pane, rundir := "", ""
	if p.delta.rtask != "" {
		pane, rundir = a.reviewReviewerPane(p.lane, p.delta.rtask)
	}
	var missing []string
	if rundir != "" {
		missing = reviewMissingPlugins(p.list, nextPluginsLoaded(rundir))
	}
	switch {
	case account != "":
		p.retaskWhy = "--account " + account + " is chosen when a reviewer starts; a live pane cannot take it"
	case p.delta.rtask == "":
		p.retaskWhy = "the last review's reviewer is not recorded (its verdict came from a prior_review)"
	case pane == "":
		p.retaskWhy = "the reviewer " + p.delta.rtask + " has no pane in its receipt"
	case !isDir(a.workTree(p.lane, p.delta.rtask)):
		p.retaskWhy = "the reviewer's worktree " + a.workTree(p.lane, p.delta.rtask) + " is gone"
	default:
		switch st := nextLeaseEligibility(rundir, nextCurrentTask(rundir)); {
		case st != "live":
			p.retaskWhy = "the reviewer's chaining lease is " + st
		case len(missing) > 0:
			p.retaskWhy = "the delta's pieces need plugins the reviewer was not launched with: " + strings.Join(missing, ", ")
		case has(p.list, reviewQAItem) && len(receiptValues(rundir, "web_port")) == 0:
			p.retaskWhy = "the delta runs qa, and the reviewer was launched with no dev server (no web_port in its receipt)"
		case a.nextAgentGet(pane) == "":
			p.retaskWhy = "pane " + pane + " does not answer"
		default:
			p.retask = pane
		}
	}
	if p.retask != "" {
		p.rwtTask = p.delta.rtask
		p.delta.kept = a.reviewerArtifactsOf(p.lane, p.rwtTask) + "/review-prior.md" // hw copies it there before the re-task: the new review.md replaces it
	}
}

// reviewRunHW runs the brain's own `hw` with args, its output on ours: the verbs `hw review` hands the work to.
func (a *app) reviewRunHW(args []string) error {
	cmd := exec.Command(a.c.brain+"/bin/hw", args...)
	cmd.Stdout, cmd.Stderr = a.out, a.err
	return cmd.Run()
}

// reviewPriorPath is the review.md the reviewer is told to read: the copy beside its own when it is re-tasked, V's original otherwise.
func (d *reviewDelta) priorPath() string {
	if d.kept != "" {
		return d.kept
	}
	return d.review
}

// reviewMissingPlugins is the plugins the reviewer's `review:` list loads that its pane was not launched with.
func reviewMissingPlugins(list, loaded []string) []string {
	var out []string
	for _, n := range pluginsWithReview(nil, "["+strings.Join(list, ", ")+"]") {
		if !has(loaded, n) {
			out = append(out, n)
		}
	}
	return out
}

// reviewRetask hands the delta brief to the kept reviewer: its worktree fast-forwards to the head (the gate forbids a reviewer's
// commits, so it always can), V's review.md is kept beside the one the new review will replace, and `hw next <pane> --brief` delivers.
// It returns false, with what stopped it already printed, when nothing was handed over and the dispatch must be a fresh reviewer; a
// `hw next` that fails dies instead (it may have delivered). A dry run moves nothing.
func (a *app) reviewRetask(p *reviewPlan, brief string, dry bool) bool {
	rwt := a.workTree(p.lane, p.rwtTask)
	if !dry {
		if _, ok := gitRun(rwt, nil, "merge", "--ff-only", p.head); !ok {
			a.warn("the reviewer's worktree " + rwt + " would not fast-forward to " + p.head + ": a fresh reviewer takes the delta instead")
			return false
		}
		if out, ok := gitRun(rwt, nil, "rev-parse", "HEAD"); !ok || strings.TrimSpace(string(out)) != p.head {
			a.warn("the reviewer's worktree is not at " + p.head + " after the fast-forward: a fresh reviewer takes the delta instead")
			return false
		}
		prior, err := os.ReadFile(p.delta.review)
		if err != nil || os.WriteFile(p.delta.kept, prior, 0o666) != nil {
			a.warn("could not keep the prior review.md beside the new one (" + p.delta.kept + "): a fresh reviewer takes the delta instead")
			return false
		}
	}
	args := []string{"next", p.retask, "--brief", brief}
	if dry {
		args = append(args, "--dry-run")
	}
	a.step("re-task  hw " + strings.Join(args, " "))
	if err := a.reviewRunHW(args); err != nil {
		// hw next may have half-delivered: a fresh reviewer beside it is two reviewers for one round, so nothing more is dispatched
		a.die("hw next did not take the delta (" + err.Error() + "): the re-task of pane " + p.retask + " failed, so NO other reviewer was dispatched for this round and nothing was recorded. Look at pane " + p.retask + " (it may have taken the brief), then run `hw review " + p.lane + " " + p.task + "` again. If hw next refuses again, the rerun picks this same pane: `hw done " + p.lane + " " + p.rwtTask + "` (it takes <lane> <task>, not a pane), and the rerun dispatches a fresh reviewer.")
	}
	return true
}

// reviewDeltaCommand is what `hw done` prints for an approval that sits at an ancestor of a head whose delta has code.
func (a *app) reviewDeltaCommand(lane, task, rtask string, from, head string, code int) string {
	line := "run `hw review " + lane + " " + task + "`: it re-checks only " + reviewShort(from) + ".." + reviewShort(head) + " (" + strconv.Itoa(code) + " code-only line(s), tier " + reviewTierOf(code) + ")"
	if pane, rundir := a.reviewReviewerPane(lane, rtask); pane != "" && rundir != "" && nextLeaseEligibility(rundir, nextCurrentTask(rundir)) == "live" {
		until := nextLeaseField(rundir, "until_h")
		return line + ", and re-tasks " + pane + " until " + orDefaultStr(until, "its lease ends")
	}
	return line + " with a fresh reviewer (the last one's lease is over)"
}
