package main

// The pinned verification of `hw done` ends, whole, at its ceiling.
//
// MEASURED 2026-11-06: two `hw done` fast gates ran 70 min under `timeout 900`. `timeout` signals the command and its own
// group; heavy puts its job in a group of its own (`set -m`), so the gate's grandchildren never saw the signal and kept
// running (and holding the output pipe) after `hw done` gave up. The command now leads a group and, at the ceiling, the
// group and every descendant `ps` can name are KILLed; the status is `timeout`'s, 124.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// doneTreePids is root and every descendant of it, by ps's parent pids.
func doneTreePids(root int) []int {
	out, _ := doneTool("", nil, "ps", "-axo", "pid=,ppid=")
	kids := map[int][]int{}
	for _, ln := range strings.Split(out, "\n") {
		var p, pp int
		f := strings.Fields(ln)
		if len(f) != 2 {
			continue
		}
		p, _ = strconv.Atoi(f[0])
		pp, _ = strconv.Atoi(f[1])
		kids[pp] = append(kids[pp], p)
	}
	order := []int{root}
	for i := 0; i < len(order); i++ {
		order = append(order, kids[order[i]]...)
	}
	return order
}

// doneRunGate is `out="$( cd "$dir" && timeout "$secs" bash -c "$cmd" 2>&1 )"`, with a ceiling (secs > 0) that reaches the whole tree.
func doneRunGate(dir string, secs int64, cmd string, env []string) (string, int) {
	ctx := context.Background()
	if secs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(secs)*time.Second)
		defer cancel()
	}
	c := exec.CommandContext(ctx, "bash", "-c", cmd)
	c.Dir, c.Env, c.Stdin = dir, env, os.Stdin
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var buf strings.Builder
	c.Stdout, c.Stderr = &buf, &buf
	c.Cancel = func() error {
		pgid := c.Process.Pid
		for _, p := range doneTreePids(pgid) {
			syscall.Kill(p, syscall.SIGKILL)
		}
		return syscall.Kill(-pgid, syscall.SIGKILL)
	}
	c.WaitDelay = 2 * time.Second // a survivor holding the output pipe must not hold hw done
	err := c.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return bashSub(buf.String() + "\nhw done: the verification outlived its " + strconv.FormatInt(secs, 10) + " s ceiling (HW_VERIFY_TIMEOUT_SECONDS): its process group and every descendant were killed\n" + doneCeilingWaited(cmd, buf.String())), 124
	}
	rc := doneExit(err)
	if errors.Is(err, exec.ErrWaitDelay) { // it exited by itself and only a descendant still held the pipe: the command's own status stands
		rc = c.ProcessState.ExitCode()
	}
	return bashSub(buf.String()), rc
}

// doneCeilingWaited says what the killed command was doing, so a timeout reads as the work it was and not as a hang: the command,
// the last line it printed, and — for the fast gate with nothing to select — why that is the whole gate (MEASURED 2026-11-18: an
// empty `git diff` makes setup/gate-select answer `all`, and the whole fast gate takes 8-12 min, more behind a suite slot).
func doneCeilingWaited(cmd, out string) string {
	last := ""
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			last = strings.TrimSpace(l)
		}
	}
	if len(last) > 200 {
		last = last[:200] + "…"
	}
	w := "hw done: it was still running `" + cmd + "`; its last output line was: " + last + "\n"
	if last == "" {
		w = "hw done: it was still running `" + cmd + "` and had printed nothing (a gate waiting for a suite slot or lock prints nothing)\n"
	}
	if strings.Contains(cmd, "setup/test-hw") {
		w += "hw done: setup/test-hw with no changed paths (an empty `git diff`, a merged branch) runs the WHOLE fast gate, 8-12 minutes uncontended; that is slow work, not a hang\n"
	}
	return w
}

// doneCeiling is HW_VERIFY_TIMEOUT_SECONDS as a number of seconds. Anything that is not a positive integer (`15m`, `abc`, a
// negative) falls back to the 900 s default instead of removing the ceiling; only a literal 0 means none, as it did for `timeout`.
func doneCeiling(s string) int64 {
	if n := doneBashArith(s); n > 0 || strings.TrimSpace(s) == "0" {
		return n
	}
	return 900
}
