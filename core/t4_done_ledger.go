package main

// An `asked: investigate` task that reported done leaves what its brainer owes in the lane ledger (bin/ledger): "act on
// <lane>:<task> — engram #<ids>". It is written by `hw done`, not by a prose rule, so a compaction cannot forget it; it stays
// there, re-injected at every start and compaction, until `bin/ledger done <n>`. A build, or a task that reported blocked,
// adds nothing, and a second `hw done` of the same run adds nothing (the `ledger-owed` marker in the run's task directory).
//
// shortcut: the decision is the session's, taken once at `hw done` from the brief of the task `hw done` names and the LAST
// report in the receipt. A chained session (`hw next`) whose earlier task was an `asked: investigate` that reported done leaves
// no Owed line for it: the receipt keeps only the last report and `hw next` is not handed the previous task's brief, so a
// per-task line (written when each report is recorded) is more than a few lines here. The brainer sees that report anyway.

import (
	"os"
	"regexp"
	"strings"
)

// An engram id: `engram #N`, or a bare `#` + 4 or more digits. `#3` alone ("step #3", "issue #12") is not one.
var ledgerEngramID = regexp.MustCompile(`engram (#\d+)|(#\d{4,})`)

func (d *doneRun) ledgerOwed(proj, task, rundir string) {
	if rundir == "" {
		return
	}
	a, marker, rc := d.app, doneTaskDir(rundir)+"/ledger-owed", rundir+"/receipt.jsonl"
	if isFile(marker) || briefFmValue("asked", a.reviewBuildBrief(proj, task)) != "investigate" {
		return
	}
	if st, _ := doneReceiptLast(rc, "report_status"); st != "done" {
		return
	}
	sum, _ := doneReceiptLast(rc, "report_summary")
	ids, seen := []string{}, map[string]bool{}
	for _, m := range ledgerEngramID.FindAllStringSubmatch(sum, -1) {
		id := m[1] + m[2] // one of the two groups matched
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	item := "act on " + proj + ":" + task + " — engram " + strings.Join(ids, ", ")
	if len(ids) == 0 {
		item = "act on " + proj + ":" + task + " — read its report"
	}
	if !d.ledgerCall(proj, "add", "owed", item) {
		return
	}
	os.WriteFile(marker, nil, 0o666)
	a.info("ledger: owed — " + item)
}

// ledgerReported marks the lane ledger's `<task> running in <pane>` line (written by the dispatch) with what `hw done` closes: the
// report the receipt holds (` — done <date>`, ` — blocked`), or ` — closed <date>` when the run never reported (a killed or crashed
// executor), so the hook stops telling the brainer about a pane that is gone. bin/ledger marks nothing for a task it has no line for.
func (d *doneRun) ledgerReported(proj, task, rundir string) {
	if rundir == "" || !isDir(d.app.c.brain+"/"+proj) {
		return
	}
	st, _ := doneReceiptLast(rundir+"/receipt.jsonl", "report_status")
	if st != "done" && st != "blocked" {
		st = "closed"
	}
	d.ledgerCall(proj, "report", task, st)
}

// ledgerCall runs `bin/ledger --lane <proj> <args…>` against this brain; a refusal is a warning naming the command, never a failure.
func (d *doneRun) ledgerCall(proj string, args ...string) bool {
	out, code := doneTool("", append(os.Environ(), "HW_BRAIN_ROOT="+d.app.c.brain), d.binDir+"/ledger", append([]string{"--lane", proj}, args...)...)
	if code != 0 {
		d.app.warn("could not write the " + proj + " ledger (" + strings.TrimSpace(out) + "): `bin/ledger --lane " + proj + " " + strings.Join(args, " ") + "`")
	}
	return code == 0
}
