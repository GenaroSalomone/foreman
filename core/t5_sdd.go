package main

// Train T5, area C: `_next_dispatch_model` and `_next_dispatch_effort`.

import (
	"os"
	"regexp"
	"strings"
)

// rtState is what area C keeps for itself (the bash globals only its functions read).
type rtState struct {
	rtInvokerExported bool // _resolve_invoker_context reached its `export` (HW_INVOKER_VENDOR/SESSION/ENDPOINT)
}

// rtRecords is awk's records of a file: split on newline, a last line without one still counts.
func rtRecords(data string) []string {
	if data == "" {
		return nil
	}
	recs := strings.Split(data, "\n")
	if recs[len(recs)-1] == "" {
		recs = recs[:len(recs)-1]
	}
	return recs
}

// rtReadable is bash's `[ -r f ]` followed by a read: the text, or false when either fails.
func rtReadable(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

var (
	rtMarkRe     = regexp.MustCompile(`  .*$`)
	rtModelLine  = regexp.MustCompile(`^[ \t\n\v\f\r]*model=`)
	rtEffortLine = regexp.MustCompile(`^[ \t\n\v\f\r]*effort=`)
)

// rtDispatchValue is the awk of `_next_dispatch_model`/`_next_dispatch_effort`: the value after the key on the
// first matching line, the manifest's tail from its first "  " taken off.
func rtDispatchValue(rundir string, key *regexp.Regexp) string {
	text, ok := rtReadable(rundir + "/dispatch")
	if !ok {
		return ""
	}
	for _, line := range rtRecords(text) {
		if loc := key.FindStringIndex(line); loc != nil {
			line = line[loc[1]:]
			if m := rtMarkRe.FindStringIndex(line); m != nil { // # MUTATION-ANCHOR: 844-136-M02
				line = line[:m[0]]
			}
			return line
		}
	}
	return ""
}

// nextDispatchModel is `_next_dispatch_model`.
func (d *dsp) nextDispatchModel(rundir string) string { return rtDispatchValue(rundir, rtModelLine) }

// nextDispatchEffort is `_next_dispatch_effort`.
func (d *dsp) nextDispatchEffort(rundir string) string { return rtDispatchValue(rundir, rtEffortLine) }
