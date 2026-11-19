package main

// browser_reap.go — close every browser a RUN started, wherever its launcher put it.
//
// bin/browser-close closes the browser a run started under the namespace and profile hw gave it
// (AGENT_BROWSER_NAMESPACE=hw-<run>, HW_BROWSER_PROFILE=$HW_ARTIFACTS/browser-profile). A lane script that exports its own
// AGENT_BROWSER_PROFILE / _SESSION / _NAMESPACE starts a daemon in a namespace hw never heard of, so the close never saw it:
// 2026-11-05, four headed Chrome for Testing windows from a lane's QA script outlived their closed executors
// (1d22h, 1d21h, 19h, 27min).
//
// THE ANCHOR is the run's own environment, read off the process. hw puts HW_RUN=<run> in every executor's environment and
// a per-run TMPDIR (<cache>/hw-run-tmp/<run>, core/run_tmp.go) beside it; the agent-browser daemon is spawned by the CLI the
// executor ran, and Chrome by the daemon, so they inherit both — MEASURED 2026-11-05 with an overridden namespace and profile:
// the daemon and every Chrome process listed HW_RUN and TMPDIR under `ps eww`. A launcher can override the browser's
// namespace and profile by accident; it does not by accident rewrite the environment's run id. (A launcher that scrubs the
// environment with `env -i` loses both markers: KNOWN-LIMITATIONS says so, and `hw status` names a browser it cannot attribute.)
// A registry written at launch was the other option: nothing of ours runs at launch (the lane script calls agent-browser
// directly), so a registry would only be as good as the launcher's cooperation — the very thing that failed.
//
// WHAT IT WILL NOT DO. Signal a process that does not carry THIS run's marker (another run's, a human's), signal any process
// that is not a browser by its command line (the executor's own agent may be `claude --chrome`), or signal anything while a
// non-browser process of the run is still alive: that is a run still running, so the browsers are named and left.

import (
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// brProc is one process: what `ps` lists without its environment.
type brProc struct {
	pid, ppid  int
	etime, cmd string
}

// brHost is the machine's process table and the way to signal it: real by default, a fake in a unit test.
type brHost struct {
	list   func() ([]brProc, error)
	envOf  func(pids []int) map[int]string // the environment of each pid as space-separated KEY=value text; "" when unreadable
	cmdOf  func(pid int) string            // the command line now, "" when the process is gone
	signal func(pid int, kill bool)
	sleep  func(time.Duration)
	self   int
}

var brPsLine = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s+(\S+)\s+(.*)$`)

// brBrowserCmd is "this command line is a browser stack process": the agent-browser daemon and client, Chrome for Testing and
// its helpers, and the Linux chrome/chromium binaries. A command that merely mentions chrome (`claude --chrome`) is not one.
var brBrowserCmd = regexp.MustCompile(`(?i)(/agent-browser[^/ ]*( |$)|^agent-browser( |$)|Google Chrome for Testing|/(chrome|chromium|chromium-browser|chrome-headless-shell|headless_shell)( |$))`)

func brIsBrowser(cmd string) bool { return brBrowserCmd.MatchString(cmd) }

func realBrHost() *brHost {
	h := &brHost{self: os.Getpid(), sleep: time.Sleep}
	h.list = func() ([]brProc, error) {
		out, err := exec.Command("ps", "-axo", "pid=,ppid=,etime=,command=").Output()
		if err != nil && len(out) == 0 {
			return nil, err
		}
		var ps []brProc
		for _, l := range strings.Split(string(out), "\n") {
			m := brPsLine.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			pid, _ := strconv.Atoi(m[1])
			ppid, _ := strconv.Atoi(m[2])
			ps = append(ps, brProc{pid: pid, ppid: ppid, etime: m[3], cmd: m[4]})
		}
		return ps, nil
	}
	h.envOf = func(pids []int) map[int]string {
		res := map[int]string{}
		if _, err := os.Stat("/proc/self/environ"); err == nil { // Linux: exact, NUL-separated
			for _, p := range pids {
				if b, err := os.ReadFile("/proc/" + strconv.Itoa(p) + "/environ"); err == nil {
					res[p] = strings.ReplaceAll(string(b), "\x00", " ")
				}
			}
			return res
		}
		for i := 0; i < len(pids); i += 200 { // macOS: `ps eww` appends the environment to the command
			end := i + 200
			if end > len(pids) {
				end = len(pids)
			}
			var csv []string
			for _, p := range pids[i:end] {
				csv = append(csv, strconv.Itoa(p))
			}
			out, _ := exec.Command("ps", "eww", "-o", "pid=,command=", "-p", strings.Join(csv, ",")).Output()
			for _, l := range strings.Split(string(out), "\n") {
				if m := brPsLine2.FindStringSubmatch(l); m != nil {
					p, _ := strconv.Atoi(m[1])
					res[p] = m[2]
				}
			}
		}
		return res
	}
	h.cmdOf = func(pid int) string {
		out, err := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
		if err != nil {
			return ""
		}
		return strings.TrimRight(string(out), "\n")
	}
	h.signal = func(pid int, kill bool) {
		sig := syscall.SIGTERM
		if kill {
			sig = syscall.SIGKILL
		}
		syscall.Kill(pid, sig)
	}
	return h
}

var brPsLine2 = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)

// brMarks is "this environment text belongs to run": HW_RUN=<run> as a whole word, or a TMPDIR that is this run's per-run one.
func brMarks(env, run string) bool {
	if run == "" || !RunIDPattern.MatchString(run) {
		return false // an empty or malformed id must never match everything
	}
	for _, f := range strings.Fields(env) {
		if f == "HW_RUN="+run {
			return true
		}
		if strings.HasPrefix(f, "TMPDIR=") && strings.HasSuffix(strings.TrimRight(f, "/"), "/hw-run-tmp/"+run) {
			return true
		}
	}
	return false
}

// brRunOf is the run id an environment text carries: HW_RUN, else the run of a per-run TMPDIR; "" when neither.
func brRunOf(env string) string {
	tmpRun := ""
	for _, f := range strings.Fields(env) {
		if v, ok := strings.CutPrefix(f, "HW_RUN="); ok && RunIDPattern.MatchString(v) {
			return v
		}
		if v, ok := strings.CutPrefix(f, "TMPDIR="); ok {
			v = strings.TrimRight(v, "/")
			if i := strings.LastIndex(v, "/hw-run-tmp/"); i >= 0 && RunIDPattern.MatchString(v[i+len("/hw-run-tmp/"):]) {
				tmpRun = v[i+len("/hw-run-tmp/"):]
			}
		}
	}
	return tmpRun
}

func brEnvVar(env, key string) string {
	for _, f := range strings.Fields(env) {
		if v, ok := strings.CutPrefix(f, key+"="); ok {
			return v
		}
	}
	return ""
}

// brBrowser is one browser stack process of a run, with what the report shows of it.
type brBrowser struct {
	brProc
	profile, namespace string
	root               bool // its parent is not another browser process of the same run: the line worth printing
	helpers            int  // for a root: how many browser processes of the run sit under it
}

func (b brBrowser) kind() string {
	if strings.Contains(strings.ToLower(strings.Fields(b.cmd + " x")[0]), "agent-browser") {
		return "agent-browser daemon"
	}
	return "Chrome"
}

// brAssemble turns browser stack processes into report entries: their profile and namespace, which of them is a root (its parent is
// not in the set: the line worth printing) and, for a root, how many of the others sit under it. The one place that rule lives.
func brAssemble(procs []brProc, envs map[int]string) []brBrowser {
	inSet := map[int]bool{}
	for _, p := range procs {
		inSet[p.pid] = true
	}
	bs := make([]brBrowser, 0, len(procs))
	for _, p := range procs {
		prof := brEnvVar(envs[p.pid], "AGENT_BROWSER_PROFILE")
		if i := strings.Index(p.cmd, "--user-data-dir="); i >= 0 {
			prof = strings.Fields(p.cmd[i+len("--user-data-dir="):] + " ")[0]
		}
		bs = append(bs, brBrowser{brProc: p, profile: prof, namespace: brEnvVar(envs[p.pid], "AGENT_BROWSER_NAMESPACE"), root: !inSet[p.ppid]})
	}
	sort.Slice(bs, func(i, j int) bool { return bs[i].pid < bs[j].pid })
	for i := range bs {
		if !bs[i].root {
			continue
		}
		for j := range bs {
			if j != i && brUnder(bs, bs[j], bs[i].pid) {
				bs[i].helpers++
			}
		}
	}
	return bs
}

// brSnapshot is what is running now, for one run.
type brSnapshot struct {
	browsers []brBrowser // browser stack processes carrying the run's marker
	alive    []brProc    // NON-browser processes carrying it, outside the caller's own process tree; read only when asked for or when a browser matched
}

// snapshotOf reads the process table once and picks out the run's processes. The environment of the browser candidates is read
// first and alone: a run with no browser costs one `ps` and a handful of environments, not the whole table's. The environment of
// every other process is read only when it can matter (a browser matched, or needAlive). The caller's own tree (itself, its
// ancestors, its descendants) is excluded: `hw done` is run by a closer that inherited the executor's environment. With
// keepAncestors the caller's ancestors count as the run's own live processes (`hw status` run from inside an executor).
func (h *brHost) snapshotOf(run string, keepAncestors, needAlive bool) (brSnapshot, error) {
	var s brSnapshot
	ps, err := h.list()
	if err != nil {
		return s, err
	}
	byPid := map[int]brProc{}
	kids := map[int][]int{}
	for _, p := range ps {
		byPid[p.pid] = p
		kids[p.ppid] = append(kids[p.ppid], p.pid)
	}
	mine := map[int]bool{h.self: true}
	for p, n := byPid[h.self].ppid, 0; !keepAncestors && p > 1 && n < 64; n++ { // ancestors
		mine[p] = true
		p = byPid[p].ppid
	}
	var walk func(int)
	walk = func(p int) {
		for _, k := range kids[p] {
			if !mine[k] {
				mine[k] = true
				walk(k)
			}
		}
	}
	walk(h.self)

	var cands, others []brProc
	var candPids []int
	for _, p := range ps {
		if brIsBrowser(p.cmd) {
			cands = append(cands, p)
			candPids = append(candPids, p.pid)
		} else if !mine[p.pid] {
			others = append(others, p)
		}
	}
	envs := h.envOf(candPids)
	var marked []brProc
	for _, p := range cands {
		if brMarks(envs[p.pid], run) && !mine[p.pid] { // never signal the caller's own tree, whatever its command says
			marked = append(marked, p)
		}
	}
	s.browsers = brAssemble(marked, envs)
	if needAlive || len(marked) > 0 {
		otherPids := make([]int, 0, len(others))
		for _, p := range others {
			otherPids = append(otherPids, p.pid)
		}
		oenvs := h.envOf(otherPids)
		for _, p := range others {
			if brMarks(oenvs[p.pid], run) {
				s.alive = append(s.alive, p)
			}
		}
	}
	return s, nil
}

// runAlive: the run still has a process of its own that is not a browser — an executor in its pane, or a child of it.
func (h *brHost) runAlive(run string) bool {
	s, err := h.snapshotOf(run, true, true)
	return err != nil || len(s.alive) > 0 // unreadable table: do not claim the run is closed
}

// brUnder: b descends from root through browser processes of the set.
func brUnder(all []brBrowser, b brBrowser, root int) bool {
	for n := 0; n < 16; n++ {
		if b.ppid == root {
			return true
		}
		found := false
		for _, o := range all {
			if o.pid == b.ppid {
				b, found = o, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return false
}

// brReport is the outcome of closing one run's browsers.
type brReport struct {
	run      string
	closed   []brBrowser // roots that are gone, for the printed lines; a browser that outlived the close is never here
	survived []brBrowser // still alive after SIGTERM, SIGKILL and the settle window (a replacement the dying daemon started included)
	held     []brProc    // non-browser processes of the run still alive: nothing was signalled
}

// brLine is the one-line description of a browser stack: pid, kind, profile, namespace, age, helpers.
func brLine(b brBrowser) string {
	l := b.kind() + " pid " + strconv.Itoa(b.pid)
	if b.profile != "" {
		l += ", profile " + b.profile
	}
	if b.namespace != "" {
		l += ", namespace " + b.namespace
	}
	l += ", up " + b.etime
	if b.helpers > 0 {
		l += ", +" + strconv.Itoa(b.helpers) + " helper processes"
	}
	return l
}

// reapRun closes every browser stack process that carries run's marker. settle bounds each wait (the run's own processes going
// away, then the browsers going away after SIGTERM, then after SIGKILL), in wall time as well as in sleeps. It signals nothing
// while a non-browser process of the run is alive. The process table is read in full a fixed number of times (first look, after
// a hold, last look); the waits only ask `ps` about the pids already known.
func (h *brHost) reapRun(run string, settle time.Duration) (brReport, error) {
	rep := brReport{run: run}
	const tick = 100 * time.Millisecond
	wait := func(until func() bool) {
		t0 := time.Now()
		for waited := time.Duration(0); !until() && waited < settle && time.Since(t0) < settle; waited += tick {
			h.sleep(tick)
		}
	}
	still := func(p brProc) bool { cur := h.cmdOf(p.pid); return cur != "" && cur == p.cmd } // the pid is still the process we listed
	anyStill := func(ps []brProc) bool {
		for _, p := range ps {
			if still(p) {
				return true
			}
		}
		return false
	}
	snap, err := h.snapshotOf(run, false, false)
	if err != nil || len(snap.browsers) == 0 {
		return rep, err
	}
	// the run still has a process that is not a browser: its executor, or a child of it. It may be about to exit (hw done has
	// just closed the pane): give it the settle window, then hold.
	if len(snap.alive) > 0 {
		wait(func() bool { return !anyStill(snap.alive) })
		if snap, err = h.snapshotOf(run, false, false); err != nil {
			return rep, err
		}
		if len(snap.alive) > 0 {
			rep.held = snap.alive
			return rep, nil
		}
	}
	targets := make([]brProc, 0, len(snap.browsers))
	for _, b := range snap.browsers {
		targets = append(targets, b.brProc)
	}
	for _, kill := range []bool{false, true} {
		for _, p := range targets {
			if still(p) {
				h.signal(p.pid, kill)
			}
		}
		wait(func() bool { return !anyStill(targets) })
		if !anyStill(targets) {
			break
		}
	}
	// the last look: what carries the marker now, a replacement a dying daemon started included, is a survivor, not a close
	last, err := h.snapshotOf(run, false, false)
	if err != nil {
		return rep, err
	}
	rep.survived = last.browsers
	left := map[int]bool{}
	for _, b := range last.browsers {
		left[b.pid] = true
	}
	for _, b := range snap.browsers {
		if b.root && !left[b.pid] {
			rep.closed = append(rep.closed, b)
		}
	}
	return rep, nil
}

// allBrowserRuns lists, by the run each carries, every browser stack process in the table: the browsers `hw status` can attribute
// to a run, whichever run that is.
func (h *brHost) allBrowserRuns() (map[string][]brBrowser, error) {
	ps, err := h.list()
	if err != nil {
		return nil, err
	}
	var cand []brProc
	var pids []int
	for _, p := range ps {
		if brIsBrowser(p.cmd) && p.pid != h.self {
			cand = append(cand, p)
			pids = append(pids, p.pid)
		}
	}
	out := map[string][]brBrowser{}
	if len(cand) == 0 {
		return out, nil
	}
	envs := h.envOf(pids)
	byRun := map[string][]brProc{}
	for _, p := range cand {
		if r := brRunOf(envs[p.pid]); r != "" {
			byRun[r] = append(byRun[r], p)
		}
	}
	for r, group := range byRun {
		out[r] = brAssemble(group, envs)
	}
	return out, nil
}

// reapBrowsersOfDir closes the browsers of every run recorded under <wt>/.hw — what `hw done` and `hw reap --apply` run for a
// task they close. A held run or a survivor is a warning with the command to retry, never a failed close: the pane and the
// worktree are closed either way. Idempotent: the second call finds nothing.
func (a *app) reapBrowsersOfDir(wt string) {
	if !isDir(wt + "/.hw") {
		return
	}
	const settle = 3 * time.Second
	h := bcHost()
	for _, run := range RunIDs(wt + "/.hw") {
		rep, err := h.reapRun(run, settle)
		if err != nil {
			a.warn("could not read the process table to close the browsers of run " + run + ": " + err.Error())
			continue
		}
		for _, b := range rep.closed {
			a.info("closed browser of run " + run + ": " + brLine(b))
		}
		if len(rep.held) > 0 {
			a.warn("run " + run + " still has a live process (pid " + strconv.Itoa(rep.held[0].pid) + "), so its browsers were NOT closed; once it is gone: browser-close --run " + run)
		}
		for _, b := range rep.survived {
			a.warn("browser of run " + run + " outlived SIGTERM and SIGKILL: " + brLine(b) + " — browser-close --run " + run)
		}
	}
}
