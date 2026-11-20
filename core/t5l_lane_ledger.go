package main

// The lane ledger (bin/ledger, <brain>/<lane>/ledger.md: what a brainer was told and owes) is not the dispatch ledger
// (.hw-ledger/<lane>.jsonl, t5l_receipt.go). This file is the dispatch's write to the first: the `running` line of the task.

import "strings"

// laneLedgerRunning leaves `<task> running in <pane>` in the dispatching lane's Owed (`bin/ledger running`), written by the dispatch
// and not by a prose rule: bin/ledger keys it by the task, so a re-dispatch replaces the line. Only a brief that landed (drc 0), and
// never a review run's, which is the build's own review and not something the brainer owes. A lane with no directory under the brain
// (a test world) has no ledger and is skipped quietly; a lane with one that cannot be written warns, and the dispatch goes on.
func (l *lch) laneLedgerRunning(pane string, drc int) {
	if drc != 0 || l.reviewerOf() != "" || !isDir(l.c.brain+"/"+l.proj) {
		return
	}
	out, rc := l.sh(cmdSpec{name: l.binDir() + "/ledger", args: []string{"--lane", l.proj, "running", l.task, pane},
		env: []string{"HW_BRAIN_ROOT=" + l.c.brain}, mut: true, merge: true})
	if rc != 0 {
		l.warn("could not write the " + l.proj + " ledger (" + strings.TrimSpace(out) + "): `bin/ledger --lane " + l.proj + " running " + l.task + " " + pane + "`")
	}
}
