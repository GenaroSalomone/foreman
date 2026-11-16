package main

// hw done / hw reap / occupancy for an executor that runs on herdr-core (herdr-app-done-reap, step 4).
//
// An hc run says so on disk: dispatch writes <rundir>/hc ("<project> <task> <address>"), the way a TUI run
// writes <rundir>/tab. Every hc call here is behind that record, so a TUI run makes none. The terminals a run
// owns are found by their labels (hw_project + hw_task), never by the recorded address or the name alone: a
// terminal with other labels is somebody else's and is not touched.

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// hcRunRecord is the run's hc record: its project, task and dispatch address, ok=false for a TUI run.
func hcRunRecord(rundir string) (proj, task, addr string, ok bool) {
	if rundir == "" {
		return "", "", "", false
	}
	b, err := os.ReadFile(strings.TrimSuffix(rundir, "/") + "/hc")
	f := strings.Fields(string(b))
	if err != nil || len(f) != 3 {
		return "", "", "", false
	}
	return f[0], f[1], f[2], true
}

// hcOwned lists the live terminals carrying this project+task. ok=false is "hc could not answer", which is
// not "none": reading it so reports a running executor as gone.
func hcOwned(proj, task string) (addrs []string, ok bool) {
	raw, rc := hcOut(10*time.Second, "ls", "--json", "--label", "hw_project="+proj, "--label", "hw_task="+task)
	v, parsed := bjParse(brTrimNL(raw))
	list, isList := bjGet(v, "terminals").([]any)
	if !rc || !parsed || !isList {
		return nil, false
	}
	for _, e := range list {
		if m, isMap := e.(jmap); isMap && hcStr(m, "address") != "" {
			addrs = append(addrs, hcStr(m, "address"))
		}
	}
	return addrs, true
}

// closeHC ends the run's hc terminals: kill, then remove. It answers like closeObject: a terminal `hc ls`
// no longer lists is closed, one it still lists after kill is a failure.
func (d *doneRun) closeHC(proj, task string) bool {
	addrs, ok := hcOwned(proj, task)
	if !ok {
		d.closeResult = "failed"
		d.warn("CLOSE FAILED: `hc ls` could not answer, so whether " + proj + ":" + task + " is still running on herdr-core is UNKNOWN")
		return false
	}
	d.closeResult = "absent"
	for _, a := range addrs {
		hcOut(10*time.Second, "kill", a)
		hcOut(10*time.Second, "rm", a)
		d.closeResult = "closed"
		d.ok("terminal " + a + " closed (" + proj + ":" + task + ")")
	}
	if left, ok := hcOwned(proj, task); !ok || len(left) > 0 {
		d.closeResult = "failed"
		d.warn("CLOSE FAILED: " + proj + ":" + task + " still has a live terminal on herdr-core")
		return false
	}
	if len(addrs) == 0 {
		d.info("terminal for " + proj + ":" + task + " is already gone")
	}
	return true
}

// hcLiveLine is one live core terminal as occupantsIn prints it: the only place that format is written.
func hcLiveLine(addr string) string { return addr + " (hc, live)" }

// hcOwnedIn is the live terminals of the hc runs recorded under dir. observed=false when an hc run is
// recorded and `hc ls` could not say.
func hcOwnedIn(dir string) (addrs []string, observed bool, why string) {
	recs, _ := filepath.Glob(dir + "/.hw/*/hc")
	seen := map[string]bool{}
	for _, r := range recs {
		proj, task, _, ok := hcRunRecord(filepath.Dir(r))
		if !ok || seen[proj+"/"+task] {
			continue
		}
		seen[proj+"/"+task] = true
		got, ok := hcOwned(proj, task)
		if !ok {
			return nil, false, "hc ls could not answer for " + proj + ":" + task
		}
		addrs = append(addrs, got...)
	}
	return addrs, true, ""
}

// hcOccupants is hcOwnedIn as occupantsIn's lines.
func hcOccupants(dir string) (list string, observed bool, why string) {
	addrs, observed, why := hcOwnedIn(dir)
	lines := make([]string, len(addrs))
	for i, addr := range addrs {
		lines[i] = hcLiveLine(addr)
	}
	return strings.Join(lines, "\n"), observed, why
}

// hcCwdOccupants is the addresses of the live terminals of the app's core whose cwd is dir or under it (the caller renders them
// with hcLiveLine). answered is false when `hc ls` could not say. unplaced are the listed terminals that name no cwd: where
// they stand is unknown, so they are neither in dir nor outside it.
func hcCwdOccupants(dir string, win bool) (addrs, unplaced []string, answered bool) {
	raw, ok := hcOut(hcReadTimeout, "ls", "--json")
	if !ok {
		return nil, nil, false
	}
	terms, ok := hcLsTerminals(raw)
	if !ok {
		return nil, nil, false
	}
	d := onePath(dir, win)
	for _, e := range terms {
		m, isMap := e.(jmap)
		if !isMap || hcStr(m, "address") == "" {
			continue
		}
		if hcStr(m, "cwd") == "" {
			unplaced = append(unplaced, hcStr(m, "address"))
			continue
		}
		if underDir(d, onePath(hcStr(m, "cwd"), win)) {
			addrs = append(addrs, hcStr(m, "address"))
		}
	}
	return addrs, unplaced, true
}

// addHCPanes makes an hc run's live terminals the live panes of its task, as herdr's pane list does for a TUI run.
// An `hc ls` that cannot answer adds nothing: status then shows what herdr says, as before.
func (d *statusData) addHCPanes() {
	for _, key := range d.taskOrder {
		t := d.tasks[key]
		if addrs, ok, _ := hcOwnedIn(t.workdir); ok {
			for _, addr := range addrs {
				d.panesByTask[key] = append(d.panesByTask[key], jmap{"pane_id": addr, "label": t.task, "cwd": t.workdir})
			}
		}
	}
}
