package main

// The short form of the dispatch manifest, which a dry run prints unless --dry-run=full asks for the whole text: what will be
// launched (lane, task, agent, model, effort, account, worktree, base, what runs at close), and any row of the whole manifest
// that carries a problem. The mechanism behind each row stays in the full text, in the run directory's `dispatch` file and in
// the receipt. Warnings and refusals are printed by the callers as before and never pass through here.

import "strings"

// manifestAlertWords mark a row that says something is wrong, missing, refused or about to be inherited. A row that has one
// is printed whole by the short form, whatever its place.
var manifestAlertWords = []string{
	"UNRESOLVED", "REFUSED", "NOT APPLIED", "NOT LAUNCHABLE", "NOT LOADED", "MISSING", "WILL TIME OUT", "UNESTABLISHED", "NO — shares",
	"WAITS on", "(REUSED", "BELOW", "NOT READY", "! ",
}

func manifestAlert(s string) bool {
	for _, w := range manifestAlertWords {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// manifestShortField is a row's value without its explanation (the text after " — "), unless the row is an alert.
func manifestShortField(s string) string {
	if i := strings.Index(s, " — "); i >= 0 && !manifestAlert(s) {
		return s[:i]
	}
	return s
}

// shortManifest is dispatchManifest's text of a dry run, from the same values the whole text is built from.
func (d *dsp) shortManifest(runLabel, routeLine, modelLine, effortLine, agentLine, permissionsLine, invokerLine, dbLine, wtLine, baseLine, verifyLine, looseRows string) string {
	verify, _, _ := strings.Cut(verifyLine, "  (hw runs this")
	if manifestAlert(verifyLine) { // WILL TIME OUT, or a setup build with no fence
		verify = verifyLine
	}
	model := strings.TrimRight(manifestShortField(firstLine(modelLine)), " ") // the pin note stays in the full text; the short form says where the alias lands
	if d.modelPinned != "" && d.dispatchRoute != "reuse" {
		model += " → " + d.modelPinned
	}
	rows := []string{"dispatch " + d.proj + ":" + d.task + "  run=" + runLabel}
	if d.dispatchRoute == "reuse" {
		rows = append(rows, "  route       "+firstLine(routeLine))
	}
	rows = append(rows,
		"  agent       "+manifestShortField(agentLine),
		"  "+model,
		"  "+firstLine(effortLine),
		"  account     "+manifestShortField(d.accountLine),
		"  worktree    "+manifestShortField(wtLine),
		"  base        "+manifestShortField(baseLine),
		"  verify      "+manifestShortField(verify))
	// the rows the short form does not carry, when one of them is a problem
	for _, r := range []struct{ key, text string }{
		{"floor", d.modelFloorLine}, {"after", d.afterLine}, {"permissions", permissionsLine}, {"database", dbLine},
		{"dependencies", d.depsLine}, {"invoker", invokerLine},
	} {
		if manifestAlert(r.text) {
			rows = append(rows, "  "+padRight(r.key+" ", 12)+r.text)
		}
	}
	// the rows whose text is built elsewhere (the effort's continuation, the brain guard, pi, plugins, review): a line of
	// them that carries a problem is a row of its own, with its key as the full text prints it
	_, effortMore, _ := strings.Cut(effortLine, "\n") // pi's `pi runs` line follows the effort row
	rows = append(rows, manifestAlertRows(effortMore+"\n"+looseRows)...)
	return strings.Join(rows, "\n")
}

// manifestAlertRows are the lines of text that carry a problem, as they are.
func manifestAlertRows(text string) []string {
	var rows []string
	for _, l := range strings.Split(text, "\n") {
		if manifestAlert(l) {
			rows = append(rows, l)
		}
	}
	return rows
}
