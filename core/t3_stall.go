package main

// The stall watcher: an executor whose turn ended LEGALLY (waiting on a background child, a reply to its ask, a human
// boundary) was never bounded: a product executor sat 17 h, `working`, behind a lock nobody would release. Each such
// turn end starts bin/stall-watch, detached, which alerts the brainer once if no new turn comes in HW_STALL_MIN minutes
// (default 20; 0 turns it off). The timer is that one short-lived process, in the shape of setup/heavy's frozen-run
// watchdog and started by the hook that already runs at every turn end: no LaunchAgent, no daemon, and a new turn (the
// `turns` count moves), the report or a gone pane ends it.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// t3RunningTasks is the shape rule of `background_tasks`, the one reader every caller uses (t3LiveChildren, t3RunningWhat, t3OnlyShells); it
// mirrors the jq program of bash's _executor_live_children, quirks included: an array's elements or an object's values; a falsy value holds none;
// anything else is not a shape (ok false). Only the literal "running" counts and only objects can be children.
func t3RunningTasks(payload *jval) (tasks []*jval, ok bool) {
	bt, ok := payload.get("background_tasks")
	if !ok {
		return nil, false
	}
	var elems []*jval
	switch {
	case !bt.truthy():
	case bt.kind == vArr:
		elems = bt.arr
	case bt.kind == vObj:
		for _, m := range bt.obj {
			elems = append(elems, m.v)
		}
	default:
		return nil, false
	}
	for _, e := range elems {
		if e.kind != vObj {
			continue
		}
		if st, _ := e.get("status"); st.kind == vStr && st.s == "running" { // # MUTATION-ANCHOR: 841-C01
			tasks = append(tasks, e)
		}
	}
	return tasks, true
}

// t3RunningWhat names what the running background children are: "shell `sleep 3600`", "subagent explore".
func t3RunningWhat(payload string) string {
	vals, _ := parseStream([]byte(payload))
	var parts []string
	for _, v := range vals {
		tasks, ok := t3RunningTasks(v)
		if !ok {
			break
		}
		for _, e := range tasks {
			ty, _ := e.get("type")
			name := "task"
			if ty.kind == vStr && ty.s != "" {
				name = ty.s
			}
			what := ""
			for _, k := range []string{"command", "description"} {
				if w, ok := e.get(k); ok && w.kind == vStr && w.s != "" {
					what = w.s
					break
				}
			}
			if what != "" {
				name += " `" + what + "`"
			}
			parts = append(parts, name)
		}
	}
	out := strings.Join(parts, ", ")
	if len(out) > 300 {
		out = out[:300] + "…"
	}
	return out
}

// t3OnlyShells is true when every running background child of the payload is a shell: the one kind whose life the process
// table shows (a subagent lives inside claude, with no process of its own).
func t3OnlyShells(payload string) bool {
	vals, _ := parseStream([]byte(payload))
	n := 0
	for _, v := range vals {
		tasks, ok := t3RunningTasks(v)
		if !ok {
			break
		}
		for _, e := range tasks {
			if ty, _ := e.get("type"); !(ty.kind == vStr && ty.s == "shell") {
				return false
			}
			n++
		}
	}
	return n > 0
}

// t3WatchedClaude is the pid (as the watcher's argument) of the claude whose background shells the stall watcher should look at:
// the nearest claude above this process, when every running child is a shell; "" otherwise.
func t3WatchedClaude(payload string) string {
	if !t3OnlyShells(payload) {
		return ""
	}
	return t3ClaudeAbove(t3ProcTable(), os.Getpid())
}

// MEASURED EXTERNAL CONTRACT (2026-11-19, Claude Code 2.1.328, macOS): a background Bash call runs as `zsh -c source
// ~/.claude*/shell-snapshots/snapshot-zsh-….sh … && eval '<command>' … && pwd -P`, a direct child of the `claude …` process (the reviewer of
// f0b09e3ef saw pid 36267 under claude 24092; `ps -A -o pid=,ppid=,command=` shows it). If a release changes the shape the baseline is 0 and
// nothing is lowered, which is the safe side; a claude pid reused by another process could be lowered by mistake.
// A background shell of Claude Code is a direct child of the claude process whose command line names its shell snapshot. A
// foreground tool call has one too, and at a turn end there is none: what is left is a background child.
const t3ShellSnapshot = "/shell-snapshots/snapshot-"

type t3Proc struct {
	ppid int
	cmd  string
}

// t3PsOut is the process table as `ps` prints it; the hook's unit tests run it in process, where no PATH stub reaches.
var t3PsOut = func() []byte {
	out, _ := exec.Command("ps", "-A", "-o", "pid=,ppid=,command=").Output()
	return out
}

// t3ProcTable is `ps -A -o pid=,ppid=,command=` by pid; empty when ps cannot say.
func t3ProcTable() map[int]t3Proc {
	tab := map[int]t3Proc{}
	for _, l := range strings.Split(string(t3PsOut()), "\n") {
		a, rest, _ := strings.Cut(strings.TrimSpace(l), " ")
		b, cmd, _ := strings.Cut(strings.TrimSpace(rest), " ")
		pid, e1 := strconv.Atoi(a)
		ppid, e2 := strconv.Atoi(b)
		if e1 == nil && e2 == nil {
			tab[pid] = t3Proc{ppid, strings.TrimSpace(cmd)}
		}
	}
	return tab
}

// t3ClaudeAbove is the pid of the claude process this one runs under: the nearest ancestor whose program is `claude`; "" when there is none.
func t3ClaudeAbove(tab map[int]t3Proc, pid int) string {
	for i := 0; i < 16 && pid > 1; i++ {
		p, ok := tab[pid]
		if !ok {
			return ""
		}
		if f := strings.Fields(p.cmd); len(f) > 0 && filepath.Base(f[0]) == "claude" {
			return strconv.Itoa(pid)
		}
		pid = p.ppid
	}
	return ""
}

// t3ShellKids counts the background shells of the claude process pid; -1 when that process is not there.
func t3ShellKids(tab map[int]t3Proc, pid int) int {
	if _, ok := tab[pid]; !ok {
		return -1
	}
	n := 0
	for _, p := range tab {
		if p.ppid == pid && strings.Contains(p.cmd, t3ShellSnapshot) {
			n++
		}
	}
	return n
}

// stallWatchStart starts bin/stall-watch for this turn end; why is what the executor ended its turn waiting on. It does
// nothing when the watcher is off (HW_STALL_MIN=0), the run is a shadow one, or the tool is not there.
func (t *t3) stallWatchStart(stateDir string, turns int64, why string) {
	if t.getenv("HW_STALL_MIN") == "0" {
		return
	}
	t.watcherStart(stateDir, strconv.FormatInt(turns, 10), why)
}

// childWatchStart starts the child watch of a turn that ended on background shells (`stall-watch --children`): it lowers the pane's
// children_running claim once claude has no background shell left, for as long as the pane awaits one. It is not the stall alert:
// HW_STALL_MIN neither delays it nor turns it off.
func (t *t3) childWatchStart(stateDir string, turns int64, claude string) {
	if claude == "" {
		return
	}
	t.watcherStart("--children", stateDir, strconv.FormatInt(turns, 10), claude)
}

func (t *t3) watcherStart(args ...string) {
	if t.fs.readonly {
		return
	}
	bin := t.binDir() + "/stall-watch"
	if !t3IsExec(bin) {
		return
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	t3Detach(cmd)
	if cmd.Start() == nil {
		cmd.Process.Release()
	}
}
