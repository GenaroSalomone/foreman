package invokers

// The lane ledger (bin/ledger, <brain>/<lane>/ledger.md) is not the dispatch ledger `hw ledger` reads (release.go's qaLedgerRow).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ledgerReported marks the lane ledger's `<task> running in <pane>` line, which the dispatch wrote, with the report the brainer
// just admitted (bin/ledger: ` — done <date>`, or ` — blocked`). A ledger that cannot be written is said, never a failed report:
// the report is already delivered.
func (c *Ctx) ledgerReported(lane, task, status string) {
	if _, err := os.Stat(c.BinDir + "/ledger"); err != nil {
		return
	}
	if st, err := os.Stat(c.BinDir + "/../" + lane); err != nil || !st.IsDir() { // no lane directory, no ledger to mark
		return
	}
	cmd := exec.Command(c.BinDir+"/ledger", "--lane", lane, "report", task, status)
	cmd.Env = append(os.Environ(), "HW_BRAIN_ROOT="+filepath.Dir(filepath.Clean(c.BinDir))) // the root the guard above looked in, not the caller's
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.errf("done-invoker: warning: the report is delivered, but the %s ledger could not be marked (%s): `bin/ledger --lane %s report %s %s`\n", lane, strings.TrimSpace(string(out)), lane, task, status)
	}
}
