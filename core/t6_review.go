package main

// `hw review <lane> <task> [--dry-run] [--here] [--at <sha>] [--account <name>] [--model <m>] [--effort <e>]`: the independent review of a build (design: brain/archive/setup/
// review-by-default-design/design.md §2, §3, §6 «Brief 1», §8 C1 C3 S1 S2). It measures the build's diff, picks the
// tier, writes the reviewer's brief from docs/REVIEW-BRIEF.md and dispatches a fresh claude opus executor on a branch
// cut at the reviewed SHA. It is a brainer verb: an executor reaches only its `--dry-run`.
//
//	tier 0   no code-only line changed        no reviewer; review_head is recorded all the same
//	tier S   1..400 code-only lines           thermo, test-review
//	tier L   more than 400                    + ponytail-review, pstack:principle-minimize-reader-load, pstack:principle-subtract-before-you-add
//	+JD      the build brief's review: names judgment-day (high risk, declared by the brainer)   the reviewer also runs Judgment Day
//
// What the build's receipt gains: review_head and review_tier always, review_run (the reviewer's brief) when one was
// dispatched. `--at <sha>` is the exception: a blind review of a past head, for benchmarking, records nothing in the build's
// receipt, so its verdict can never satisfy `hw done`'s merge gate. `--account <name>` is the dispatch's own flag, carried as it
// is: the reviewer's account is chosen, checked for parity and printed in the manifest by the one code path every dispatch uses
// (t5_vendor_account.go), and without it the reviewer takes the lane's default. review_owed=1 is the build dispatch's (t5l_env.go): a failed `hw review` leaves it standing, so the
// absence of a verdict is never read as "nothing owed" (C1).

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// reviewSmallMax is the largest diff (code-only lines) that gets the lean pass: above setup's p90 (375), below a product lane's median (665).
const reviewSmallMax = 400

// reviewDocOnly is the S2 allowlist: the paths whose change is prose a reader reads and nothing runs. Everything else is
// code, including `.claude/**`, `_agents/`, `_skills/`, `plugins/`, `CLAUDE.md`, `*.golden` and `testdata/`: a markdown file
// an agent obeys is executable. The three templates under docs/ are executable too.
func reviewDocOnly(path string) bool {
	switch path {
	case "docs/REVIEW-BRIEF.md", "docs/REVIEW-DELTA-BRIEF.md", "docs/BRIEF-TEMPLATE.md":
		return false
	}
	if strings.HasPrefix(path, "docs/") || strings.HasPrefix(path, "setup/export/overlay/changes/") {
		return true
	}
	parts := strings.Split(path, "/")
	for i := 0; i < len(parts)-1 && i < 2; i++ { // a directory at the root or one lane below it: setup/decisions/, <lane>/briefs/
		switch parts[i] {
		case "decisions", "briefs", "changelog":
			return true
		}
	}
	return false
}

// reviewMeasure reads `git diff --no-renames --numstat -z` and counts code-only lines. A binary file (`-`) counts as owed:
// one line, so it can never leave the diff at tier 0.
func reviewMeasure(numstatZ string) (code int, codeFiles []string, docLines int) {
	for _, rec := range strings.Split(numstatZ, "\x00") {
		f := strings.SplitN(rec, "\t", 3)
		if len(f) != 3 {
			continue
		}
		n := 1
		if f[0] != "-" || f[1] != "-" {
			a, e1 := strconv.Atoi(f[0])
			d, e2 := strconv.Atoi(f[1])
			if e1 != nil || e2 != nil {
				continue
			}
			n = a + d
		}
		if reviewDocOnly(f[2]) {
			docLines += n
			continue
		}
		code += n
		codeFiles = append(codeFiles, f[2])
	}
	return code, codeFiles, docLines
}

// reviewerOwed is false only when no reviewer is dispatched: tier 0, and no changes-needed to answer. A prose-only fix after
// changes-needed is still owed one, because `hw done` denies until a review says approve (a prose delta carries an approve, never a
// changes-needed), and a reviewer is who marks the prior rows resolved or open.
func (p *reviewPlan) reviewerOwed() bool {
	return p.tier != "0" || (p.delta != nil && p.delta.verdict == "changes-needed")
}

// reviewTierOf is 0, S or L by code-only lines.
func reviewTierOf(code int) string {
	switch {
	case code <= 0:
		return "0"
	case code <= reviewSmallMax:
		return "S"
	}
	return "L"
}

// reviewTierList is the `review:` list of the reviewer's brief: the plugins its tier loads, then judgment-day when the build declared it.
func reviewTierList(tier string, jd bool) []string { return reviewTierListWith(tier, jd, nil) }

// reviewTierListWith adds the plugin items the build's `review:` named (S4: reviewExtraPlugins), once each, before judgment-day.
func reviewTierListWith(tier string, jd bool, extra []string) []string {
	var out []string
	switch tier {
	case "S":
		out = []string{"thermo", "tests"}
	case "L":
		out = []string{"thermo", "tests", "ponytail", "pstack"}
	default:
		return nil
	}
	for _, n := range extra {
		if !has(out, n) {
			out = append(out, n)
		}
	}
	if jd {
		out = append(out, "judgment-day")
	}
	return out
}

// reviewTierMissing is the plugins the tier loads by default that this brain has not installed (the pinned review plugins are an
// optional install step, so a public install has none): the review falls back to what the install has. A plugin the build's review: or
// the lane's review_default named is not a default: it stays, and is refused where it was declared.
func (a *app) reviewTierMissing(tier string, extra []string) []string {
	var out []string
	for _, n := range reviewTierListWith(tier, false, nil) {
		if !has(extra, n) && !pluginAt(a.c.brain, n) {
			out = append(out, n)
		}
	}
	return out
}

// reviewerPieces is the pieces a reviewer proves, by full name, from its `review:` list. In a review_of brief `pstack` stands
// for its two principles (a Skill call each, like every other piece), not for the four reviewers it carries on a build.
func reviewerPieces(list []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, it := range list {
		switch it {
		case "judgment-day":
		case "pstack":
			add("pstack:principle-minimize-reader-load")
			add("pstack:principle-subtract-before-you-add")
		default:
			if only, ok := reviewerOnlyItems[it]; ok {
				add(only.piece)
				continue
			}
			for _, q := range reviewSkillsQualified([]string{it}) {
				add(q)
			}
		}
	}
	return out
}

// reviewShortLen is the SHA prefix a reviewer's task name carries; herdr caps a task name at 32 characters.
const (
	reviewShortLen = 7
	reviewTaskMax  = 32
)

// reviewTaskName is `<task>-review-<short sha>`, the build's name cut so the whole fits the cap. The SHA keeps it unique
// per dispatch (C3) however much of the name goes.
func reviewTaskName(task, short string) string { return reviewTaskNameOf(task, "-review-", short) }

// reviewAtTaskName is `<task>-review-at-<short sha>`: a past head's reviewer, which never collides with the review of the same SHA as a tip.
func reviewAtTaskName(task, short string) string { return reviewTaskNameOf(task, "-review-at-", short) }

func reviewTaskNameOf(task, infix, short string) string {
	suffix := infix + short
	if room := reviewTaskMax - len(suffix); len(task) > room {
		task = strings.TrimRight(task[:room], "-")
	}
	return task + suffix
}

// reviewTierLabel is what review_tier holds: `S`, `L+JD`, `0`.
func reviewTierLabel(tier string, jd bool) string {
	if jd && tier != "0" {
		return tier + "+JD"
	}
	return tier
}

// ── the functions a diff changed (§3.5) ─────────────────────────────────────

var (
	reviewHunkRe = regexp.MustCompile(`^@@ [^@]* @@ ?(.*)$`)
	reviewDefRes = []*regexp.Regexp{
		regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`^\s*(?:async\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`^\s*class\s+([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s*([A-Za-z_$][A-Za-z0-9_$]*)`),
		regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][A-Za-z0-9_$]*)\s*=>`),
	}
)

// reviewDefName is the function a line defines, "" when it defines none.
func reviewDefName(line string) string {
	for _, re := range reviewDefRes {
		if m := re.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	return ""
}

// reviewFunctionsOf reads `git diff -U0` and lists "name\tpath", once each, sorted: the function a hunk header names (the
// attributes file hands git a driver per language) and the definitions the hunk adds. A name hw cannot extract is not owed.
func reviewFunctionsOf(diff string) []string {
	seen := map[string]bool{}
	file := ""
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "):
			file = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
			if line == "+++ /dev/null" {
				file = ""
			}
		case strings.HasPrefix(line, "@@"):
			if m := reviewHunkRe.FindStringSubmatch(line); m != nil && file != "" {
				if n := reviewDefName(m[1]); n != "" {
					seen[n+"\t"+file] = true
				}
			}
		case strings.HasPrefix(line, "+") && file != "":
			if n := reviewDefName(line[1:]); n != "" {
				seen[n+"\t"+file] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// reviewJSDriver is the funcname pattern of the one language git has no driver for; the .go and .py ones are git's own.
const reviewJSDriver = `^[ \t]*((export[ \t]+)?(async[ \t]+)?function[ \t*].*|(export[ \t]+)?(const|let|var)[ \t]+[A-Za-z_$][A-Za-z0-9_$]*[ \t]*=.*=>.*|class[ \t].*)$`

// ── the verb ────────────────────────────────────────────────────────────────

type reviewPlan struct {
	lane, task, branch, head, base, baseHow string
	wt                                      string
	code                                    int
	codeFiles                               []string
	docLines                                int
	tier                                    string
	jd                                      bool
	list, pieces                            []string
	tierMissing                             []string // the tier's own plugins this brain has not installed: the review runs without them
	extraBrief, extraLane                   []string // what the reviewer's tier adds: the build's review: items, then the lane's review_default ones
	uiFiles                                 []string // the code files the lane's ui_paths call UI, when they add qa by default (reviewQADefault)
	post                                    string   // the lane's review_post: pr-description or none
	rtask, rbranch, rbase                   string
	briefPath                               string
	functions                               []string
	buildBrief, requestedBy                 string
	buildRunDir                             string
	dirty                                   string
	at                                      bool         // --at: a past head; the build's receipt is not touched
	rounds                                  int          // the reviews this build already had, this dispatch not counted
	liftWhy                                 string       // --lift-cap's reason when this dispatch is past the cap (set once, by reviewRoundCapCheck); reviewRecord writes it
	from                                    string       // the diff starts here: base, or a delta's T (a commit, or the tree of V with the base merged in)
	delta                                   *reviewDelta // a re-check of V..head; nil is the full review
	fullWhy                                 string       // why a build with an earlier review is re-checked in full
	rwtTask                                 string       // the task whose worktree holds the reviewer: rtask, or the kept reviewer's
	retask                                  string       // the pane of the kept reviewer the delta goes to; "" is a fresh reviewer
	retaskWhy                               string       // what ruled the kept reviewer out
	jdDropped                               bool         // the build declared judgment-day and the delta is below tier L, so the re-check does not run it
	model                                   string       // the reviewer's model: opus unless --model, and what its brief declares
}

const reviewUsage = "hw review <lane> <task> [--dry-run] [--here] [--no-report] [--at <sha>] [--account <name>] [--model <m>] [--effort <e>] [--lift-cap \"<why>\"]"

// reviewRoundCap is the most reviews one build gets: 75 of 82 builds finished within 3, and the ones past it never converged
// (setup/evidence/review-system-eval-2026-11-19.md §2, §6.1). The 4th dispatch is refused unless --lift-cap says why.
const reviewRoundCap = 3

// reviewValuedNeeds is what each value-taking flag of `hw review` wants, for the refusal that names it.
var reviewValuedNeeds = map[string]string{"--at": "a commit", "--account": "a name", "--model": "a model", "--effort": "a level", "--lift-cap": "the reason"}

// reviewValuedFlag reads args[i] as one of reviewValuedNeeds' flags, spelled `--flag v` or `--flag=v`: the flag, its value
// ("" when none follows) and the index of the last argument it consumed.
func reviewValuedFlag(args []string, i int, x string) (flag, v string, next int, ok bool) {
	for flag = range reviewValuedNeeds {
		switch {
		case x == flag && i+1 < len(args):
			return flag, args[i+1], i + 1, true
		case x == flag:
			return flag, "", i, true
		case strings.HasPrefix(x, flag+"="):
			return flag, strings.TrimPrefix(x, flag+"="), i, true
		}
	}
	return "", "", i, false
}

// reviewIsFlag is true when x is one of hw review's own options, valued or not.
func reviewIsFlag(x string) bool {
	if _, _, _, ok := reviewValuedFlag(nil, 0, x); ok {
		return true
	}
	return x == "--dry-run" || x == "--here" || x == "--no-report"
}

func (a *app) cmdReview() int {
	dry, here, noReport := false, false, false
	at, account, liftCap, model, effort := "", "", "", "", ""
	var pos []string
	for i := 0; i < len(a.args); i++ {
		x := a.args[i]
		if flag, v, next, ok := reviewValuedFlag(a.args, i, x); ok {
			if i = next; v == "" || (strings.HasPrefix(v, "-") && flag != "--lift-cap") || (flag == "--lift-cap" && reviewIsFlag(v)) { // a reason may start with a dash, but not be one of our flags (`--lift-cap --dry-run` made the dry run real); a commit or an account may not start with one
				a.die("hw review: " + flag + " needs " + reviewValuedNeeds[flag] + " (usage: " + reviewUsage + ")")
			}
			switch flag {
			case "--at":
				at = v
			case "--lift-cap":
				liftCap = v
			case "--model":
				model = v
			case "--effort":
				effort = v
			default:
				account = v
			}
			continue
		}
		switch x {
		case "--dry-run":
			dry = true
		case "--here":
			here = true
		case "--no-report": // dispatch's own flag, carried: a rehearsal from a shell that is no brainer's pane has nowhere to report
			noReport = true
		default:
			if strings.HasPrefix(x, "-") {
				a.die("hw review: unknown option " + x + " (usage: " + reviewUsage + ")")
			}
			pos = append(pos, x)
		}
	}
	if len(pos) != 2 {
		a.die("usage: " + reviewUsage)
	}
	p := a.reviewPlan(pos[0], pos[1], at)
	a.reviewPrint(p, dry)

	tmp, err := os.MkdirTemp("", "hw-review-")
	if err != nil {
		a.die("hw review: cannot make a scratch directory: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	if !p.reviewerOwed() {
		if p.at {
			a.ok("no code changed at " + p.head + ": no reviewer dispatched, and nothing recorded (--at).")
			return 0
		}
		if dry {
			a.info("dry run: would record review_head=" + p.head + " and review_tier=0 in the build's receipt; no reviewer.")
			return 0
		}
		a.reviewRecord(p, false)
		a.ok("no code changed: review_head=" + p.head + " recorded, no reviewer dispatched.")
		return 0
	}

	a.reviewQAGate(p, here) // before anything is claimed

	if !p.at { // a past head is a benchmark, never a round of the build
		release, owned := a.reviewClaim(p, dry)
		defer release()
		if owned {
			return 0
		}
		a.reviewRoundCapCheck(p, liftCap)
	}
	// the reviewer's effort follows the tier (plain S: medium, L and S+JD: high, a declared high-risk build is not lowered; a delta's tier is
	// its own); --model and --effort override, and the dispatch validates them
	p.model = model // the brief declares it too: the dispatch refuses a brief and a flag that disagree
	if p.model == "" {
		p.model = "opus"
	}
	reviewEffort := effort
	if reviewEffort == "" {
		reviewEffort = "high"
		if p.tier == "S" && !p.jd {
			reviewEffort = "medium"
		}
	}
	a.reviewRoute(p, account)
	a.reviewPrintRoute(p)
	path := p.briefPath
	if dry { // a dry run creates nothing in the brain: the dispatch's own check reads a scratch copy
		path = filepath.Join(tmp, p.rtask+".md")
	} else if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		a.die("hw review: " + err.Error())
	}
	writeBrief := func() {
		brief, err := a.reviewBriefText(p)
		if err != nil {
			a.die("hw review: " + err.Error())
		}
		if err := os.WriteFile(path, []byte(brief), 0o666); err != nil {
			a.die("hw review: cannot write the reviewer's brief " + path + ": " + err.Error())
		}
	}
	writeBrief()
	if p.retask != "" {
		if a.reviewRetask(p, path, dry) {
			if !dry {
				a.reviewRecord(p, true)
				a.reviewWriteFunctions(p)
			}
			return 0
		}
		p.retask, p.retaskWhy, p.rwtTask, p.delta.kept = "", "the re-task did not go through", p.rtask, ""
		writeBrief() // the fresh reviewer reads V's own review.md, not the copy that was to sit beside the kept reviewer's
	}
	// --fresh always: a reviewer is a fresh session by contract (docs/REVIEW-BRIEF.md), and without it the dispatch may re-task a kept
	// pane of the lane (a prior reviewer, the builder) that runs on another account, and refuse with a remedy this verb cannot take
	args := []string{p.lane, p.rtask, "--agent", "claude", "--model", p.model, "--effort", reviewEffort, "--base", p.rbase, "--brief", path, "--fresh"}
	if !noReport { // --keep-pane needs a report to chain after, so a no-report rehearsal drops it
		args = append(args, "--keep-pane", "--why", "the re-check: the brainer re-tasks this reviewer with hw next after the builder's one fix round (review-by-default design §3.4)")
	}
	if account != "" { // the dispatch parses it, verifies parity (or refuses) and prints the manifest's account line
		args = append(args, "--account", account)
	}
	if has(p.list, reviewQAItem) && a.c.devServerMode(p.lane) == "opt-in" { // qa needs the dev server a lane only starts when asked
		args = append(args, "--dev-server")
	}
	if here {
		args = append(args, "--here")
	} else {
		// the reviewer's brief is asked: investigate, which defaults a dispatch to worktree none (e38cf7ec0); the reviewer must
		// check `git rev-parse HEAD` == the reviewed SHA and done-invoker's head gate reads the workdir HEAD, so it needs a checkout
		args = append(args, "--worktree", "new")
	}
	if noReport {
		args = append(args, "--no-report")
	}
	if dry {
		args = append(args, "--dry-run")
	}
	a.step("dispatch  hw " + strings.Join(args, " "))
	if err := a.reviewRunHW(args); err != nil {
		retry := "`hw review " + p.lane + " " + p.task
		if p.at {
			retry += " --at " + p.head
		}
		if p.liftWhy != "" { // without it the retry is refused at the cap, and the remedy printed here would be a loop
			retry += " --lift-cap " + strconv.Quote(p.liftWhy)
		}
		if account != "" { // the retry must run the reviewer where this one was asked to run, not on the lane's default
			retry += " --account " + account
		}
		if model != "" { // nor with another model or effort than the ones asked
			retry += " --model " + model
		}
		if effort != "" {
			retry += " --effort " + effort
		}
		retry += "`"
		if p.at { // a past head never touches the build's receipt: do not claim anything about it
			a.die("the reviewer's dispatch failed (" + err.Error() + "): this was a review of a past head, so the build's receipt is untouched and nothing was recorded. Fix the cause above and run " + retry + " again.")
		}
		a.die("the reviewer's dispatch failed (" + err.Error() + "): review_owed=1 still stands in the build's receipt and nothing else was recorded. Fix the cause above and run " + retry + " again.")
	}
	if dry {
		return 0
	}
	if !p.at {
		a.reviewRecord(p, true)
	}
	a.reviewWriteFunctions(p)
	return 0
}

// reviewRoundsOf counts the reviews a build already had: the review_run records in the receipts of the build's runs, one per
// dispatch `hw review` made (a failed dispatch records none, a --at benchmark records none). A re-tasked reviewer is one
// directory holding several reviews, so its directories and briefs would undercount; the build's own receipt does not. A fix task
// is a build of its own, so the cap is the chain's: the reviews of the task and of every parent its `round_of:` names, each once.
func (a *app) reviewRoundsOf(p *reviewPlan) int {
	n := 0
	for _, t := range a.reviewChainOf(p.lane, p.task) {
		wt := a.workTree(p.lane, t) // a parent whose worktree is gone has no receipt left to count
		for _, r := range runsNewestFirst(wt + "/.hw") {
			n += len(receiptValues(wt+"/.hw/"+r, "review_run"))
		}
	}
	return n
}

// reviewChainOf is the task and the parents its brief's `round_of:` chain names, nearest first, each once (a cycle ends the walk).
func (a *app) reviewChainOf(lane, task string) []string {
	var chain []string
	for ; task != "" && !has(chain, task); task = briefRoundOf(a.reviewBuildBrief(lane, task)) {
		chain = append(chain, task)
	}
	return chain
}

// reviewRoundCapCheck refuses the dispatch past the cap, naming both exits; --lift-cap "<why>" is the one that dispatches.
func (a *app) reviewRoundCapCheck(p *reviewPlan, liftCap string) {
	p.rounds = a.reviewRoundsOf(p)
	if p.rounds < reviewRoundCap {
		return
	}
	next, limit := strconv.Itoa(p.rounds+1), strconv.Itoa(reviewRoundCap)
	if liftCap != "" {
		p.liftWhy = liftCap
		a.warn("review round cap lifted: this is review " + next + " of " + p.lane + ":" + p.task + " (cap " + limit + "); the reason goes in the build's receipt as review_cap_lifted")
		return
	}
	chain := ""
	if c := a.reviewChainOf(p.lane, p.task); len(c) > 1 {
		chain = ", counted across its round_of chain " + strings.Join(c, " <- ")
	}
	a.die("hw review: " + p.lane + ":" + p.task + " already had " + strconv.Itoa(p.rounds) + " reviews" + chain + " (the cap is " + limit + "): review " + next + " is refused. Either lift the cap with `hw review " + p.lane + " " + p.task + " --lift-cap \"<why>\"` (recorded in the receipt), or close the build and move its open review rows to KNOWN-LIMITATIONS.")
}

// reviewPlan measures the build and fixes everything the dispatch will say, or dies. A non-empty `at` is --at: the head is that
// commit instead of the branch tip, there is no origin comparison (the reviewer is cut at the SHA itself) and no receipt to touch.
func (a *app) reviewPlan(laneArg, task, at string) *reviewPlan {
	lane := a.c.t.canonProject(laneArg)
	if !a.c.t.known(lane) {
		a.die("hw review: " + laneArg + " is not a lane in projects.json")
	}
	if strings.ContainsAny(task, "/ ") || task == "" {
		a.die("hw review: a task name has no path separator or space (got: " + task + ")")
	}
	checkout := a.c.checkout(lane)
	if checkout == "" || !isDir(checkout) {
		a.die("hw review: " + lane + " has no checkout to read a branch from")
	}
	p := &reviewPlan{lane: lane, task: task}
	p.branch = a.c.lanePath(lane, "branch", task)
	if p.branch == "" {
		p.branch = "task/" + task
	}
	p.wt = a.workTree(lane, task)
	// the head: the branch tip, which is what the worktree holds when the build committed; under --at, the commit asked for
	if at != "" {
		out, ok := gitRun(checkout, nil, "rev-parse", "--verify", "--quiet", at+"^{commit}")
		if !ok {
			a.die("hw review: --at " + at + " is not a commit in " + checkout + " — nothing to review (it is cut locally: no fetch, no push)")
		}
		p.head, p.at = strings.TrimSpace(string(out)), true
	} else {
		out, ok := gitRun(checkout, nil, "rev-parse", "--verify", "--quiet", "refs/heads/"+p.branch+"^{commit}")
		if !ok {
			a.die("hw review: " + lane + ":" + task + " has no branch " + p.branch + " in " + checkout + " — nothing to review (a merged and reaped task is gone)")
		}
		p.head = strings.TrimSpace(string(out))
	}
	if !p.at && (isDir(p.wt+"/.git") || isFile(p.wt+"/.git")) {
		if st, ok := gitRun(p.wt, nil, "status", "--porcelain"); ok && strings.TrimSpace(string(st)) != "" {
			p.dirty = strconv.Itoa(len(strings.Split(strings.TrimSpace(string(st)), "\n"))) + " path(s) uncommitted in " + p.wt + ": the review judges the committed head only"
		}
	}
	prefix := a.c.t.get(lane, "base_ref_prefix")
	p.rbase = p.head
	if prefix != "" && !p.at { // C3: the reviewer is cut from the PR branch, so the PR branch must BE the head being stamped
		ref := prefix + "task/" + task
		r, ok := gitRun(checkout, nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
		remote := strings.TrimSpace(string(r))
		if !ok || remote != p.head {
			shown := "missing"
			if ok {
				shown = remote
			}
			a.die("hw review: " + ref + " is " + shown + " but the local head of " + p.branch + " is " + p.head + ". The reviewer is cut from " + ref + " (the PR branch), so push the branch, or fetch it, until they agree: a review stamped with a SHA the reviewer's tree is not at is not a review of that SHA.")
		}
		p.rbase = "task/" + task
	}
	p.buildBrief = a.reviewBuildBrief(lane, task)
	if nextFmValue("review_of", p.buildBrief) != "" {
		a.die("hw review: " + task + " is itself a review run (its brief declares review_of): a review is not reviewed, and its branch is never offered for merge")
	}
	p.requestedBy = nextFmValue("requested_by", p.buildBrief)
	review := reviewRawOf(nextFmValue("review", p.buildBrief))
	mode, _ := reviewModeOf(review)
	p.jd = mode == "judgment-day"
	p.buildRunDir = a.reviewRunDir(p.wt)

	base, how := a.reviewBaseOf(checkout, lane, p.head, p.buildBrief, p.buildRunDir)
	if p.at && base == p.head { // a merged head with no receipt fork point: base..head is empty, so measure the commit against its first parent
		if par, ok := gitRun(checkout, nil, "rev-parse", "--verify", "--quiet", p.head+"^1^{commit}"); ok {
			base, how = strings.TrimSpace(string(par)), "first parent of --at "+p.head+" — it is already in the lane's base"
		}
	}
	if base == "" {
		a.die("hw review: no base to measure " + p.branch + " against (no review_base in its receipt and no merge-base with " + prefix + a.c.t.get(lane, "base") + ")")
	}
	p.base, p.baseHow = base, how
	p.from = base
	mm := a.reviewLaneMergeBase(checkout, lane, p.head, base)
	if !p.at { // a past head is a blind benchmark: always the whole range
		a.reviewDeltaPlan(p, checkout, mm)
	}
	if p.delta == nil {
		a.reviewFromWithoutLane(p, checkout, mm)
	}

	num, _ := gitRun(checkout, nil, "diff", "--no-renames", "--numstat", "-z", p.from, p.head)
	p.code, p.codeFiles, p.docLines = reviewMeasure(string(num))
	p.tier = reviewTierOf(p.code)
	if p.delta != nil && p.tier != "L" && p.jd { // a fix round's check is not the high-risk read the whole build got: Judgment Day only when the delta is itself L
		p.jd, p.jdDropped = false, true
	}
	p.extraBrief = reviewExtraPlugins(review, pluginsOfValue(nextDecl(nextBriefDeclares(p.buildBrief), "plugins")))
	for _, n := range a.c.t.reviewLaneDefault(lane) {
		if !has(p.extraBrief, n) && !has(p.extraLane, n) {
			p.extraLane = append(p.extraLane, n)
		}
	}
	p.post = a.c.t.get(lane, "review_post")
	extra := append(slices.Clone(p.extraBrief), p.extraLane...)
	if p.uiFiles = a.reviewQADefault(lane, review, p.codeFiles); p.uiFiles != nil {
		extra = append(extra, reviewQAItem)
	}
	p.tierMissing = a.reviewTierMissing(p.tier, extra)
	p.list = slices.DeleteFunc(reviewTierListWith(p.tier, p.jd, extra), func(n string) bool { return has(p.tierMissing, n) })
	p.pieces = reviewerPieces(p.list)
	short := p.head
	if len(short) > reviewShortLen {
		short = short[:reviewShortLen]
	}
	p.rtask = reviewTaskName(task, short)
	if p.at {
		p.rtask = reviewAtTaskName(task, short)
	}
	p.rwtTask = p.rtask
	p.rbranch = a.c.lanePath(lane, "branch", p.rtask)
	if p.rbranch == "" {
		p.rbranch = "task/" + p.rtask
	}
	p.briefPath = a.c.brain + "/" + lane + "/briefs/" + p.rtask + ".md"
	if p.tier != "0" { // a prose-only delta owed a reviewer (reviewerOwed) has no code files, so no functions to list
		p.functions = a.reviewFunctions(checkout, p)
	}
	return p
}

// reviewBaseOf is the base a branch's diff is measured from: the merge-base with the lane's base NOW (a branch that merged main in
// has a later fork point than the one recorded at its dispatch, and base..head must not count main's commits). Once the branch is
// merged that is the head itself, and the merge-base recorded at the build's dispatch, while it is still an ancestor of the head, is
// the only fork point left. "" when there is none. `hw done` measures with it; `hw review` also takes the lane's merged-in commits out
// of it (reviewFromWithoutLane).
func (a *app) reviewBaseOf(checkout, lane, head, buildBrief, runDir string) (base, how string) {
	prefix := a.c.t.get(lane, "base_ref_prefix")
	lb := nextFmValue("base", buildBrief)
	if lb == "" {
		lb = a.c.t.get(lane, "base")
	}
	if m, ref := reviewMergeBaseOf(checkout, head, prefix, lb); m != "" {
		base, how = m, "git merge-base "+ref+" at the head"
	}
	if (base == "" || base == head) && runDir != "" {
		if b, _ := doneReceiptLast(runDir+"/receipt.jsonl", "review_base"); b != "" {
			if _, ok := gitRun(checkout, nil, "merge-base", "--is-ancestor", b, head); ok {
				base, how = b, "review_base of the build's receipt — the branch is already in "+lb+", so its merge-base now is the head"
			}
		}
	}
	return base, how
}

// reviewLaneMergeBase is the merge-base of head with the lane's own base (main), which differs from the build's base when a fix branch is
// cut from another task branch: what the branch merged from main is not the build's, whatever its base. It is fallback (the build's base)
// when the lane's base has no merge-base with head, or head is already in it (a merged branch measures from its recorded fork point).
func (a *app) reviewLaneMergeBase(checkout, lane, head, fallback string) string {
	prefix := a.c.t.get(lane, "base_ref_prefix")
	lb := a.c.t.get(lane, "base")
	if mm, _ := reviewMergeBaseOf(checkout, head, prefix, lb); mm != "" && mm != head {
		return mm
	}
	return fallback
}

// reviewMergeBaseOf is `git merge-base head ref` for the first of prefix+lb and lb that has one, and that ref; "" when neither does.
func reviewMergeBaseOf(checkout, head, prefix, lb string) (mb, ref string) {
	for _, ref := range []string{prefix + lb, lb} {
		if m, ok := gitRun(checkout, nil, "merge-base", head, ref); ok && strings.TrimSpace(string(m)) != "" {
			return strings.TrimSpace(string(m)), ref
		}
	}
	return "", ""
}

// reviewFromWithoutLane starts a full review's diff at the merge-tree of its base and the lane's newer commits when the branch merged
// them in (a fix branch that merged main: 162 files, tier L, against its own 30 lines, 2026-11-19), and says so in the plan's base line.
func (a *app) reviewFromWithoutLane(p *reviewPlan, checkout, mm string) {
	if mm == "" || mm == p.base {
		return
	}
	if _, anc := gitRun(checkout, nil, "merge-base", "--is-ancestor", mm, p.base); anc {
		return
	}
	if tree := reviewMergeTree(checkout, p.base, mm); tree != "" {
		p.from = tree
		p.baseHow += "; measured from git merge-tree " + reviewShort(tree) + ": that base with the lane's newer commits (" + reviewShort(mm) + ") merged in, so the commits the branch took from the lane are not the build's"
	}
}

// reviewBuildBrief is brain/<lane>/briefs/<task>.md, "" when the build had none.
func (a *app) reviewBuildBrief(lane, task string) string {
	if p := a.c.brain + "/" + lane + "/briefs/" + task + ".md"; isFile(p) {
		return p
	}
	return ""
}

// reviewRunDir is the newest run directory of a worktree that holds a receipt.
func (a *app) reviewRunDir(wt string) string {
	for _, r := range runsNewestFirst(wt + "/.hw") {
		if isFile(wt + "/.hw/" + r + "/receipt.jsonl") {
			return wt + "/.hw/" + r
		}
	}
	return ""
}

// reviewFunctions is `git diff -U0` of the code files with the brain's attributes file, read-only on the product repo.
func (a *app) reviewFunctions(checkout string, p *reviewPlan) []string {
	if len(p.codeFiles) == 0 {
		return nil
	}
	args := []string{"-c", "core.attributesFile=" + a.c.brain + "/setup/review/gitattributes", "-c", "diff.jsfn.xfuncname=" + reviewJSDriver,
		"diff", "-U0", "--no-renames", "--no-color", p.from, p.head, "--"}
	args = append(args, p.codeFiles...)
	out, ok := gitRun(checkout, nil, args...)
	if !ok {
		return nil
	}
	return reviewFunctionsOf(string(out))
}

func (a *app) reviewPrint(p *reviewPlan, dry bool) {
	a.step("hw review " + p.lane + ":" + p.task + func() string {
		if dry {
			return "  (dry run)"
		}
		return ""
	}())
	a.printf("  build        %s @ %s\n", p.branch, p.head)
	if p.at {
		a.printf("  past head    --at %s: a benchmark review cut locally (no push, no origin ref); the build's receipt is NOT touched (no review_head, review_tier or review_run), so this verdict never satisfies `hw done`'s merge gate\n", p.head)
	}
	if p.dirty != "" {
		a.warn(p.dirty)
	}
	a.printf("  base         %s  (%s)\n", p.base, p.baseHow)
	switch {
	case p.delta != nil:
		a.printf("  delta        re-check of %s..%s: the last review said %s at %s; %s\n", reviewShort(p.delta.v), reviewShort(p.head), p.delta.verdict, reviewShort(p.delta.v), p.delta.tHow)
	case p.fullWhy != "":
		a.printf("  delta        none, a full re-check: %s\n", p.fullWhy)
	}
	a.printf("  diff         %d code-only line(s) in %d file(s); %d doc-only line(s) not counted\n", p.code, len(p.codeFiles), p.docLines)
	a.printf("  tier         %s  (0: none · S: 1-%d · L: more)%s\n", reviewTierLabel(p.tier, p.jd), reviewSmallMax, func() string {
		if p.jd && p.tier == "0" {
			return " — the brief lists judgment-day, but no code changed, so there is nothing to judge"
		}
		return ""
	}())
	if !p.reviewerOwed() {
		return
	}
	a.printf("  pieces       %s\n", orDefaultStr(strings.Join(p.pieces, ", "), "none"))
	if len(p.tierMissing) > 0 {
		a.printf("  - plugins    %s not installed here: this tier reviews without them (optional: install.sh --brain %s --with-review-plugins)\n", strings.Join(p.tierMissing, ", "), a.c.brain)
	}
	if p.jdDropped {
		a.printf("  - JD         the build brief's review: names judgment-day, which a delta runs only at tier L\n")
	}
	if p.jd {
		a.printf("  + JD         the build brief's review: names judgment-day: the reviewer orchestrates the two judges\n")
	}
	if len(p.extraBrief) > 0 {
		a.printf("  + plugins    the build's review: adds %s to the reviewer's tier\n", strings.Join(p.extraBrief, ", "))
	}
	if len(p.extraLane) > 0 {
		a.printf("  + lane       %s's review_default adds %s to the reviewer's tier\n", p.lane, strings.Join(p.extraLane, ", "))
	}
	if p.uiFiles != nil {
		a.printf("  + qa         added by the UI default: the diff touches UI (%s, by %s's ui_paths) and the lane has qa-tester and a dev server: the reviewer looks at the screen\n", p.uiFiles[0], p.lane)
	}
	if p.post == reviewPostPR {
		a.printf("  + post       %s's review_post: the reviewer writes its findings into the PR body's `## Review` section\n", p.lane)
	}
	a.printf("  reviewer     task %s on %s, cut from %s (the %s)\n", p.rtask, p.rbranch, p.rbase, func() string {
		if p.rbase == p.head {
			return "reviewed SHA"
		}
		return "PR branch, which is the reviewed SHA"
	}())
	a.printf("  brief        %s\n", p.briefPath)
	a.printf("  functions    %d changed function(s) listed into the reviewer's $HW_ARTIFACTS/functions.txt\n", len(p.functions))
}

// reviewPrintRoute says who takes a re-check's delta.
func (a *app) reviewPrintRoute(p *reviewPlan) {
	switch {
	case p.delta == nil:
	case p.retask != "":
		a.printf("  re-task      pane %s, the reviewer of %s: its worktree fast-forwards to %s, then hw next --brief; it keeps the model and effort it was launched with (a live session's cannot change), so the tier's effort and --model/--effort apply only to a fresh reviewer\n", p.retask, p.rwtTask, reviewShort(p.head))
	default:
		a.printf("  re-task      none, a fresh reviewer takes the delta: %s\n", p.retaskWhy)
	}
}

// reviewRecord appends the build's receipt lines. review_owed=1 is the build dispatch's own and is left alone.
func (a *app) reviewRecord(p *reviewPlan, dispatched bool) {
	dir := p.buildRunDir
	if dir == "" {
		a.warn("no run directory with a receipt under " + p.wt + "/.hw: review_head / review_tier were NOT recorded")
		return
	}
	src := "hw review " + p.lane + " " + p.task + " — the branch tip " + p.branch + " measured against " + p.base
	if dispatched {
		a.receiptInto(dir, "review_run", p.briefPath, "hw review dispatched "+p.lane+":"+p.rwtTask+" with this brief")
		a.receiptInto(dir, "review_task", p.rwtTask, "hw review: the task whose worktree holds the reviewer ("+orDefaultStr(p.retask, "a fresh one")+") — hw done reads its review.md, the next hw review its pane")
		if p.liftWhy != "" {
			a.receiptInto(dir, "review_cap_lifted", p.liftWhy, "hw review "+p.lane+" "+p.task+" --lift-cap: review "+strconv.Itoa(p.rounds+1)+", past the cap of "+strconv.Itoa(reviewRoundCap))
		}
	}
	a.receiptInto(dir, "review_head", p.head, src)
	a.receiptInto(dir, "review_tier", reviewTierLabel(p.tier, p.jd), src+" — "+strconv.Itoa(p.code)+" code-only line(s)")
}

var reviewArtifactsRe = regexp.MustCompile(`(?m)^HW_ARTIFACTS='(.*)'$`)

// reviewArtifactsOf is the $HW_ARTIFACTS of a worktree's newest run, read off its env file; "" when there is none.
func (a *app) reviewArtifactsOf(wt string) string {
	run := a.reviewRunDir(wt)
	if run == "" {
		return ""
	}
	b, err := os.ReadFile(run + "/env")
	if err != nil {
		return ""
	}
	if m := reviewArtifactsRe.FindStringSubmatch(string(b)); m != nil {
		return m[1]
	}
	return ""
}

// reviewWriteFunctions puts functions.txt in the reviewer's $HW_ARTIFACTS, read off its run's env file.
func (a *app) reviewWriteFunctions(p *reviewPlan) {
	rwt := a.workTree(p.lane, p.rwtTask)
	art := a.reviewArtifactsOf(rwt)
	if art == "" {
		a.warn("functions.txt was NOT written: the reviewer's run directory has no HW_ARTIFACTS (looked in " + rwt + "/.hw). The brief carries the same list.")
		return
	}
	if err := os.MkdirAll(art, 0o777); err != nil {
		a.warn("functions.txt was NOT written: " + err.Error())
		return
	}
	body := strings.Join(p.functions, "\n")
	if body != "" {
		body += "\n"
	}
	if err := os.WriteFile(art+"/functions.txt", []byte(body), 0o666); err != nil {
		a.warn("functions.txt was NOT written: " + err.Error())
		return
	}
	a.ok("functions.txt: " + strconv.Itoa(len(p.functions)) + " function(s) in " + art)
	calls := strings.Join(a.reviewCallersOf(a.c.checkout(p.lane), p), "\n")
	if calls != "" {
		calls += "\n"
	}
	if err := os.WriteFile(art+"/callers.txt", []byte(calls), 0o666); err != nil {
		a.warn("callers.txt was NOT written: " + err.Error())
	}
}

const (
	reviewCallersCap     = 5 // hits listed per function
	reviewCallersMinName = 4 // a shorter name (`run`, `get`) matches everything: it is not searched
)

// reviewCallersOf is `name<TAB>file:line<TAB>text` for the places the changed functions are used, read from the tree at the head
// (git grep -w), once per name.
func (a *app) reviewCallersOf(checkout string, p *reviewPlan) []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range p.functions {
		name, _, _ := strings.Cut(f, "\t")
		if seen[name] {
			continue
		}
		seen[name] = true
		if len(name) < reviewCallersMinName {
			out = append(out, name+"\t(not searched: a name under "+strconv.Itoa(reviewCallersMinName)+" characters matches too much)")
			continue
		}
		grep, _ := gitRun(checkout, nil, "grep", "-n", "-w", "-I", "--no-color", "-e", name, p.head, "--")
		out = append(out, reviewCallerLines(name, p.head+":", string(grep))...)
	}
	return out
}

// reviewCallerLines turns one name's `git grep -n` output (each line `<prefix>file:line:text`) into its callers.txt lines: the
// line that defines the name is not a caller, and at most reviewCallersCap hits are listed, the rest counted.
func reviewCallerLines(name, prefix, grep string) []string {
	var out []string
	hits := 0
	for _, l := range strings.Split(grep, "\n") {
		c := strings.SplitN(strings.TrimPrefix(l, prefix), ":", 3)
		if len(c) < 3 || reviewDefName(c[2]) == name {
			continue
		}
		if hits++; hits <= reviewCallersCap {
			text := strings.TrimSpace(c[2])
			if r := []rune(text); len(r) > 140 {
				text = string(r[:140])
			}
			out = append(out, name+"\t"+c[0]+":"+c[1]+"\t"+text)
		}
	}
	if hits > reviewCallersCap {
		out = append(out, name+"\t("+strconv.Itoa(hits-reviewCallersCap)+" more hit(s) not listed)")
	}
	return out
}

// ── the reviewer's brief ────────────────────────────────────────────────────

// reviewCommonFrom is where the sections the full brief and the delta brief share begin in docs/REVIEW-BRIEF.md.
const reviewCommonFrom = "## What you run"

// reviewBriefText fills docs/REVIEW-BRIEF.md: its frontmatter and body carry {{key}} slots, and nothing else is rewritten.
func (a *app) reviewBriefText(p *reviewPlan) (string, error) {
	tpl, err := os.ReadFile(a.c.brain + "/docs/REVIEW-BRIEF.md")
	if err != nil {
		return "", err
	}
	if p.delta != nil { // the delta's own head, then the full template's sections from «What you run» on, so the two never disagree about them
		head, err := os.ReadFile(a.c.brain + "/docs/REVIEW-DELTA-BRIEF.md")
		if err != nil {
			return "", err
		}
		i := strings.Index(string(tpl), reviewCommonFrom)
		if i < 0 {
			return "", errors.New("docs/REVIEW-BRIEF.md has no `" + strings.TrimSpace(reviewCommonFrom) + "` section: the delta brief is built from it")
		}
		tpl = []byte(strings.Replace(string(head), "{{common}}", strings.TrimSpace(string(tpl)[i:]), 1))
	}
	prior := reviewDelta{}
	if p.delta != nil {
		prior = *p.delta
	}
	fns := "(hw extracted none: read the diff and list every function you judged)"
	if len(p.functions) > 0 {
		fns = ""
		for _, f := range p.functions {
			n, path, _ := strings.Cut(f, "\t")
			fns += "- `" + n + "` — " + path + "\n"
		}
		fns = strings.TrimRight(fns, "\n")
	}
	skills := ""
	for _, q := range p.pieces {
		skills += "- `" + q + "`\n"
	}
	if skills == "" && len(p.tierMissing) > 0 {
		skills = "- none: " + strings.Join(p.tierMissing, ", ") + " are not installed here, so read the diff yourself against the list below"
	}
	jd := ""
	if p.jd {
		jd = "This build is declared high risk (its brief lists `judgment-day`: money, mail or live data). After the pieces above, run Judgment Day over the same range with the `judgment-day` skill, once, and fold its confirmed findings into `review.md`. Then write ONE line in `review.md`, with no `|` in it: `jd: confirmed=<n> suspects=<n> exclusive=<n>` (confirmed: findings both judges agreed on; suspects: findings one judge raised that you could not confirm; exclusive: findings only Judgment Day found, that no other piece of this review did). `done-invoker` refuses a review that ran Judgment Day without it."
	}
	// a slot's value is not rescanned by the replacer below, so the two clauses take their own slots here
	own := strings.NewReplacer("{{base}}", p.from, "{{head}}", p.head, "{{branch}}", p.branch)
	agents, post, qa, postWhy := "", "", "", ""
	if has(p.pieces, reviewerOnlyItems[reviewerOnlyItem].piece) { // the agents clause is comment-sicko's report-only contract; qa has its own
		agents = own.Replace(reviewAgentsPrompt)
	}
	switch {
	case p.post == reviewPostPR:
		postWhy = "This lane sets `review_post: pr-description`"
	case has(p.list, reviewQAItem):
		postWhy = "This review runs `qa`"
	}
	if postWhy != "" {
		post = own.Replace(strings.Replace(reviewPostPrompt, "{{post_why}}", postWhy, 1))
	}
	if has(p.list, reviewQAItem) {
		cases := briefSections(p.buildBrief, "Do", "Verification")
		if cases == "" {
			cases = "(the build brief has no `## Do` or `## Verification` section: take the cases from what it asks for, and say so in `review.md`)"
		}
		qa = strings.NewReplacer("{{brain}}", a.c.brain, "{{qa_cases}}", cases).Replace(own.Replace(reviewQAPrompt))
	}
	guards := a.reviewGuardsSlot(p)
	asked := "investigate"
	rb := p.requestedBy
	if rb == "" {
		rb = "hw review " + p.lane + " " + p.task
	}
	r := strings.NewReplacer(
		"{{review_list}}", strings.Join(p.list, ", "),
		"{{review_of}}", p.lane+":"+p.task+"@"+p.head,
		"{{lane}}", p.lane, "{{task}}", p.task, "{{head}}", p.head, "{{base}}", p.from,
		"{{tier}}", reviewTierLabel(p.tier, p.jd), "{{code_lines}}", strconv.Itoa(p.code),
		"{{build_brief}}", orDefaultStr(p.buildBrief, "(the build had no brief on disk)"),
		"{{skills}}", strings.TrimRight(skills, "\n"), "{{jd}}", jd, "{{agents}}", agents, "{{qa}}", qa, "{{guards}}", guards, "{{post_exception}}", post, "{{functions}}", fns,
		"{{requested_by}}", strings.ReplaceAll(rb, `"`, `'`), "{{asked}}", asked, "{{model}}", p.model,
		"{{delta_from}}", p.from, "{{delta_how}}", prior.tHow, "{{prior_head}}", prior.v, "{{prior_verdict}}", prior.verdict, "{{prior_review}}", prior.priorPath(),
	)
	return r.Replace(string(tpl)), nil
}

// reviewGuardsPath is the one path whose reviews carry a checklist (setup/guards/REVIEW.md): the 2026-11 retro found 22 of the 82
// blocker rows in 13 of 167 reviews under it. It is not a general per-path mechanism.
const reviewGuardsPath = "setup/guards/"

// reviewGuardsSlot fills {{guards}}: the guards checklist when the diff changes a file under setup/guards/, "" otherwise.
func (a *app) reviewGuardsSlot(p *reviewPlan) string {
	if !slices.ContainsFunc(p.codeFiles, func(f string) bool { return strings.HasPrefix(f, reviewGuardsPath) }) {
		return ""
	}
	b, err := os.ReadFile(a.c.brain + "/" + reviewGuardsPath + "REVIEW.md")
	if err != nil {
		return "## Guard changes\n\nThis diff touches `setup/guards/` and this brain has no `setup/guards/REVIEW.md`: walk the guard bypass classes of `setup/decisions.md` (`decisions index setup --all | rg guard`) yourself."
	}
	return strings.TrimSpace(string(b))
}

// reviewPostPR is the lane's `review_post` that has the reviewer write its findings into the PR.
const reviewPostPR = "pr-description"

// reviewAgentsPrompt fills {{agents}}: how a reviewer runs an agent piece (comment-sicko). The agent's own definition edits comments, and
// a reviewer changes nothing, so the prompt is report-only and the tree is checked after; its findings map to review.md severities here,
// never a blocker (the review-lane-default design verdict).
const reviewAgentsPrompt = `The pieces above that name an agent (` + "`pstack:comment-sicko`" + `) are not skills: launch each with the Agent tool, its ` + "`subagent_type`" + ` that full name. Foreground or background both prove it, but a background agent proves only once it **completes**: wait for its completion notice (the task-notification) before you write ` + "`review.md`" + ` and close, since done-invoker refuses one that never completed.

- Its prompt is report-only: "Review the lines added by ` + "`{{base}}..{{head}}`" + ` (read ` + "`git diff {{base}}..{{head}}`" + `). Report: files touched, deletion count, ` + "`MUST KILL`" + ` flags with one line each, skips. Change no file."
- Afterwards ` + "`git status --porcelain`" + ` prints nothing. If it prints a path, the agent edited it against the prompt: restore exactly those paths to ` + "`{{head}}`" + ` (delete an added file) and say so in ` + "`review.md`" + `.
- What it reports becomes rows of ` + "`review.md`" + `, and never a blocker:

        agent reports                                    row
        a comment it kills on a line this diff added     should
        a MUST KILL flag on a symbol this diff added     should
        a kill or flag on a line that was already there  follow-up`

// reviewPostPrompt fills {{post_exception}} when the lane's review_post is pr-description: the one thing a reviewer writes outside its
// artifacts. One section of the PR body, replaced in place or appended: never a comment, never a review, never an approval. The skeleton
// is the whole text it posts, so it names no skill, plugin or agent (2026-11-19: reviews live only in the PR description).
const reviewPostPrompt = `## The one thing you write outside ` + "`$HW_ARTIFACTS`" + `

{{post_why}}: it overrides the opening line's "post nothing to GitHub" for this one edit. After ` + "`review.md`" + ` is written, put your findings in the build's pull request **body**, in ONE section titled ` + "`## Review`" + `, replacing that section if the body has it and appending it if not. Never a comment, never a submitted review, never an approval, and no other part of the body changes.

Write the section as the person who opened the PR would: findings and what was done about them, plain sentences, ` + "`file:line`" + ` for each. It names no skill, plugin, agent or tool, carries no AI attribution and no memory ids. Write it to ` + "`$HW_ARTIFACTS/review-section.md`" + `:

        ## Review

        Verdict: approved | changes needed (head <short sha>).

        - ` + "`file:line`" + ` — what is wrong and what it breaks. Fixed in <commit> | Open.

Then replace it in the body (if ` + "`gh pr view`" + ` fails (no pull request for the branch, a network error), the block stops before any edit: say so in ` + "`review.md`" + ` and post nothing):

` + "```sh" + `
gh pr view {{branch}} --json body -q .body > "$HW_ARTIFACTS/pr-body-old.md" || exit 1
awk -v sec="$HW_ARTIFACTS/review-section.md" '
  { sub(/\r$/, "") }
  function put(  l) { while ((getline l < sec) > 0) print l; close(sec); done = 1 }
  /^## Review[ \t]*$/ { put(); skip = 1; next }
  skip && /^## / { skip = 0; print "" }
  !skip { print }
  END { if (!done) { print ""; put() } }' "$HW_ARTIFACTS/pr-body-old.md" > "$HW_ARTIFACTS/pr-body.md" || exit 1
gh pr edit {{branch}} --body-file "$HW_ARTIFACTS/pr-body.md"
` + "```"
