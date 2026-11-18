package main

// secret-unlock: the wrapper that does one thing and leaves.
// The line number below is the `exec` line of the bash file: a shell names the line of a failed exec
// (`bin/secret-unlock: line 22: exec: …/hw-secret: not found`) and the Go says the same. wrappers_parity_test.go runs both
// against a missing program, so a line that moves in the bash is a red here.

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const (
	suExecLine = 22 // bin/secret-unlock: the `exec …/hw-secret unlock "$@"`
)

// bashIsLegacy: the bash the wrappers run under (`#!/usr/bin/env bash`, the first one on PATH) is older than 4: macOS's /bin/bash 3.2. A
// bash 4+ (5.2.15 measured in the Linux container, 2026-10-19) answers differently where a program cannot be started: bashExecFail and
// bcSleep say how. Asked once, of that bash itself; with none to ask, the platform's: only macOS ships 3.2.
var bashIsLegacy = sync.OnceValue(func() bool {
	if out, err := exec.Command("bash", "-c", `printf %s "${BASH_VERSINFO[0]}"`).Output(); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil {
			return n < 4
		}
	}
	return runtime.GOOS == "darwin"
})

// bashExecFail is what bash 3.2 prints, and exits with, when `exec <name>` cannot start the program: one line for a name looked up
// on PATH, and for a path the two lines it prints (the failed open, then the exec's own).
func (w *wenv) bashExecFail(line int, name string, err error) int {
	reason := "cannot execute"
	switch {
	case errors.Is(err, exec.ErrNotFound) || (errors.Is(err, os.ErrNotExist) && !strings.Contains(name, "/")):
		w.errf("%s: line %d: exec: %s: not found\n", w.argv0, line, name)
		return 127
	case errors.Is(err, os.ErrNotExist):
		if !bashIsLegacy() { // bash 4+: the one line of the failed open, and the 127 of "not found"
			w.errf("%s: line %d: %s: No such file or directory\n", w.argv0, line, name)
			return 127
		}
		reason = "No such file or directory"
	case errors.Is(err, os.ErrPermission):
		reason = "Permission denied"
	default:
		reason = err.Error()
	}
	w.errf("%s: line %d: %s: %s\n", w.argv0, line, name, reason)
	w.errf("%s: line %d: exec: %s: cannot execute: %s\n", w.argv0, line, name, reason)
	return 126
}

// ── secret-unlock ────────────────────────────────────────────────────────────

// suRun is `exec <dir>/hw-secret unlock "$@"`: the logic lives in hw-secret, the one owner of the session file.
func suRun(w *wenv, args []string) int {
	hs := w.selfDir() + "/hw-secret"
	argv := append([]string{hs, "unlock"}, args...) // MUTATION-ANCHOR: s6-su-args
	err := execReplace(hs, argv, os.Environ())
	return w.bashExecFail(suExecLine, hs, err)
}
