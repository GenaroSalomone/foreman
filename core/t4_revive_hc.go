package main

// hw revive of a run that lives in the herdr app (a run dispatched as an `hc run` terminal: <run>/hc exists).
//
// herdr-app-final-gaps G3: dispatch on hc now records session_resume, but revive spoke only to the herdr CLI (`agent list`,
// `tab create`), so on the app it refused ("herdr agent list did not answer") or, with a TUI herdr running, looked at the wrong
// agents. The same recorded facts (cwd, env, session resume) are repeated in a new `hc run` terminal instead of a tab.
//
// shortcut: claude only (the other vendors' resume flags are unmeasured on hc).

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// reviveReopen is the end of a revive of a run that had reported: the reopened marker replaces the done marker.
func (a *app) reviveReopen(rundir, marker string) {
	if !isFile(marker) { // # MUTATION-ANCHOR: 863-G09
		return
	}
	if f, err := os.Create(reviveReopenedMarker(rundir)); err == nil {
		f.Close()
	}
	// THE DONE MARKER GOES WITH THE REOPEN. channel-send's --report-state refuses EVERY report while it stands ("task
	// state already has a done marker", not even the blocked-pane bypass), so a reopened run could not report at all
	// and the executor moved the marker by hand (2026-11-04). The earlier report stays on record — it is in the
	// receipt and in the ledger — and the reopened marker is what says this task is open again.
	os.Remove(marker)
	a.receiptInto(rundir, "reopened", "yes", "the run had "+filepath.Base(marker)+" and hw revive gave it a live pane; its done marker was cleared so the next done-invoker can deliver (the earlier report stays in the receipt and the ledger)")
	a.ok("run marked reopened — its earlier report stands, a new done-invoker is expected (no --retask), and hw status shows it as reopened")
}

// reviveOnHC is reviveCmd's tail for an app run: nothing is created before the refusals, as on herdr.
func (a *app) reviveOnHC(st *reviveRunState, sid, vendor, cmdArgs string, envShown []string, marker string) {
	proj, task, rundir, cwd := st.proj, st.task, st.rundir, st.cwd
	if vendor != "claude" {
		// every vendor is LAUNCHED on the app since hc-dispatch-every-vendor; what is claude-only is the resume, and this says why
		a.reviveDie("this run lives in the herdr app, and hw revive resumes only claude there: the resumed session is proved by reading claude's <config>/sessions/<pid>.json for the new terminal, and " + vendor + " has no such read-back (pi records no session id on the app). session_vendor is " + vendor + ". Start it again with `hw " + proj + " " + task + "`: the worktree is intact")
	}
	// One task, one terminal. AN UNREAD LIST IS NOT AN EMPTY ONE.
	raw, ok := hcOut(20*time.Second, "ls", "--json", "--label", "hw_project="+proj, "--label", "hw_task="+task)
	v, parsed := bjParse(brTrimNL(raw))
	list, isList := bjGet(v, "terminals").([]any)
	if !ok || !parsed || !isList {
		a.reviveDie("could not read whether " + proj + ":" + task + " is already live — `hc ls` did not answer. Refusing rather than resuming a session twice.")
	}
	var addrs []string
	for _, e := range list {
		if m, isMap := e.(jmap); isMap && hcStr(m, "address") != "" {
			addrs = append(addrs, hcStr(m, "address"))
		}
	}
	// the task's dev-server terminals carry its labels too, and are not a session
	read := map[string]string{} // one `hc get` per terminal: hcNotDev's read is the one judged below
	get := func(addr string) (string, bool) {
		if raw, ok := read[addr]; ok {
			return raw, true
		}
		raw, ok := hcOut(20*time.Second, "get", addr, "--json")
		if ok {
			read[addr] = raw
		}
		return raw, ok
	}
	agents, readable := hcNotDev(addrs, get)
	if !readable {
		a.reviveDie("could not read whether " + proj + ":" + task + " is already live — `hc get` did not answer. Refusing rather than resuming a session twice.")
	}
	// A LABEL IS NOT A SESSION: after a reboot the core gives a kept terminal a shell and keeps its labels (MEASURED 2026-11-16:
	// a terminal holding only zsh refused the revive as "still live"). Only a claude process running in it is.
	var stale []string // live terminals holding only a shell under this task's labels: ended before the new one starts
	for _, addr := range agents {
		raw, _ := get(addr)
		v, _ := bjParse(brTrimNL(raw))
		m, _ := v.(jmap)
		switch verdict, what := hcSessionProcess(m, addr); verdict {
		case "live":
			a.reviveDie("session " + sid + " of run " + filepath.Base(rundir) + " is still live in " + addr + " (" + what + ") — a live session is never resumed twice")
		case "stale":
			a.warn(addr + " carries this task's labels but no session: " + what + ". Its labels are stale")
			if hcStr(m, "state") == "live" {
				stale = append(stale, addr)
			}
		default:
			a.reviveDie("could not tell whether session " + sid + " of " + proj + ":" + task + " is live in " + addr + ": " + what + ". Refusing rather than resuming a session twice; read it with `hc get " + addr + "`.")
		}
	}
	st.agentArgs = cmdArgs
	a.reviveResolvePermissions(st)
	a.reviveFrameworkArgs(st, vendor, cmdArgs, rundir)

	args := []string{"run", "--bg", "--json", "--no-parent", "--no-notify", "--name", task, "--cwd", cwd,
		"--label", "hw_project=" + proj, "--label", "hw_task=" + task, "--label", "hw_run=" + filepath.Base(rundir),
		"--label", "herdr.space=" + a.c.t.get(proj, "space"),
		"--", "env", "-u", "HERDR_ENV", "-u", "HERDR_PANE_ID", "-u", "HERDR_SOCKET_PATH", "-u", "HERDR_TAB_ID", "-u", "HERDR_WORKSPACE_ID",
		"-u", "HW_INVOKER_PANE", "-u", "CLAUDE_CODE_CHILD_SESSION", "CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1", "CLAUDE_CODE_BASH_EDIT_DIFF=0"}
	args = append(args, envShown...)
	args = append(args, hcPathWord(a.reviveBinDir(), a.getenv("PATH")), "claude")
	if st.explorerArm { // an hc terminal has no shell to type the launch function into: the JSON is a word of the argv
		if json := a.explorerFor(proj).claudeJSON(); json != "" {
			args = append(args, "--agents", json)
		}
	}
	args = append(args, strings.Fields(st.agentArgs)...)
	// a kept terminal keeps its name, and `hc run --name` wants one unique among LIVE terminals: the stale shell still holds
	// <task>, so it is ended first (it runs no session: hcSessionProcess said so), and proved finished before the start
	for _, addr := range stale {
		hcOut(20*time.Second, "kill", addr)
		if !hcEndedWithin(addr, 5) {
			a.reviveDie(addr + " holds only a shell under this task's name and could not be ended (`hc kill " + addr + "` left it live or unread), so `hc run --name " + task + "` would be refused. Nothing was started; end it and revive again.")
		}
		a.info("ended " + addr + ": a stale shell holding this task's name")
	}
	out, ok := hcOut(60*time.Second, args...)
	rv, _ := bjParse(brTrimNL(out))
	addr := ""
	if m, isMap := rv.(jmap); isMap {
		addr = hcStr(m, "address")
	}
	if !ok || addr == "" {
		a.reviveDie("`hc run` did not start " + vendor + " for " + proj + ":" + task + ": " + reviveHead(out, 3))
	}
	if err := os.WriteFile(rundir+"/hc", []byte(proj+" "+task+" "+addr+"\n"), 0o666); err != nil { // the newest record is the one `hw done` reads
		a.err.Write([]byte("hw: " + rundir + "/hc: " + reviveErrno(err) + "\n"))
		panic(exitNow{1})
	}
	// the address and the core that holds it travel as a pair (dispatch writes both): a revive in another core re-pairs them
	if id := hcCoreID(a.getenv); id != "" {
		os.WriteFile(rundir+"/hc-core", []byte(id+"\n"), 0o666)
	} else {
		os.Remove(rundir + "/hc-core")
	}
	a.receiptInto(rundir, "revived_pane", addr, "hc run --json address")
	// the app opens a tab for the terminal and calls it by its number («2»): a dispatch names it after its task, and so does a revive
	hcNameTab(herdrAppOut, lSleep, addr, task)
	// proved by the terminal's own `hc get`, as dispatch proves a brief: live, and the session claude wrote for its pid
	var got string
	live := false
	budget, _ := strconv.Atoi(orStr(a.getenv("HW_HC_PROOF_MS"), "30000"))
	for waited := 0; ; waited += 1000 {
		info, read := hcOut(20*time.Second, "get", addr, "--json")
		if m, isMap := func() (jmap, bool) { x, p := bjParse(brTrimNL(info)); mm, k := x.(jmap); return mm, p && k }(); read && isMap && hcStr(m, "state") == "live" {
			live = true
			got = claudeSessionOf(bjRaw(m["pid"]), hcStr(m, "cwd"), a.reviveConfigDir(envShown), a.getenv("CLAUDE_CONFIG_DIR"), a.getenv("HOME")+"/.claude")
			if got != "" {
				break
			}
		}
		if waited >= budget {
			break
		}
		time.Sleep(time.Second)
	}
	if !live {
		a.receiptInto(rundir, "revived_agent", "failed", "hc get "+addr+" --json")
		a.reviveDie(addr + " exists with the recorded environment, but `hc get " + addr + "` does not show it live: read it with `hc read " + addr + "`. It is not closed.")
	}
	a.receiptInto(rundir, "revived_agent", "yes", "hc get "+addr+" --json: state=live")
	switch {
	case sid != "" && got != "" && got != sid:
		a.warn("the agent came up with session " + got + ", not the recorded " + sid + " — reattachment is NOT claimed; read the terminal")
		a.receiptInto(rundir, "revived_session", got+" (recorded: "+sid+") — MISMATCH", "<config>/sessions/<pid>.json of the pid `hc get "+addr+"` names")
	case got != "":
		a.receiptInto(rundir, "revived_session", got, "<config>/sessions/<pid>.json of the pid `hc get "+addr+"` names")
	}
	a.reviveReopen(rundir, marker)
	a.ok("revived " + proj + ":" + task + " in " + addr + " (herdr-core) — " + vendor + " " + st.agentArgs + " in " + cwd)
	a.info("nothing was rebuilt: worktree, ports and database are whatever the directory still holds")
}

// reviveConfigDir is the CLAUDE_CONFIG_DIR the recorded env carries ("" for the default account).
func (a *app) reviveConfigDir(envShown []string) string {
	for _, rec := range envShown {
		if strings.HasPrefix(rec, "CLAUDE_CONFIG_DIR=") {
			return strings.TrimPrefix(rec, "CLAUDE_CONFIG_DIR=")
		}
	}
	return ""
}

// sessionPrograms are the agents a session runs as; shellPrograms what the core puts in a terminal it kept after a loss.
var (
	sessionPrograms = map[string]bool{"claude": true}
	shellPrograms   = map[string]bool{"zsh": true, "bash": true, "sh": true, "fish": true, "nu": true, "dash": true, "ksh": true, "tcsh": true}
)

// hcSessionProcess says whether a session runs in the terminal `hc get` described (m): "live" when its foreground process is
// claude, "stale" when the terminal is finished or holds only a shell, and "unknown" for anything else (no record,
// no foreground process, another program), with what was read. Unknown is never stale: the caller refuses on it.
func hcSessionProcess(m jmap, addr string) (verdict, what string) {
	if m == nil {
		return "unknown", "`hc get` gave no record"
	}
	if st := hcStr(m, "state"); st != "live" {
		if hcRecordGone(m, addr) {
			return "stale", "the terminal is finished"
		}
		return "unknown", "its state is " + orStr(st, "missing")
	}
	fg, _ := m["foreground"].(jmap)
	argv, _ := fg["argv"].([]any)
	if len(argv) == 0 {
		return "unknown", "`hc get` names no foreground process"
	}
	arg0, _ := argv[0].(string)
	prog := filepath.Base(strings.TrimPrefix(arg0, "-"))
	pid := ""
	if p := bjRaw(fg["pid"]); p != "" && p != "null" {
		pid = " pid " + p
	}
	switch {
	case sessionPrograms[prog]:
		return "live", prog + pid + " runs there"
	case shellPrograms[prog]:
		return "stale", "only " + prog + pid + " runs there, no claude"
	}
	return "unknown", "its foreground process is " + orStr(prog, "unnamed") + pid + ", neither claude nor a shell"
}

// herdrAppOut is `herdr-app <args>` as hcNameTab takes it: (stdout, exit status); a call that cannot run is status 1.
func herdrAppOut(args ...string) (string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "herdr-app", args...)
	cmd.WaitDelay = 100 * time.Millisecond
	cmd.Env = childEnv()
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(out), ee.ExitCode()
		}
		return string(out), 1
	}
	return string(out), 0
}

// hcEndedWithin says whether `hc get addr` shows a finished terminal within tries reads, half a second apart.
func hcEndedWithin(addr string, tries int) bool {
	for i := 0; i < tries; i++ {
		if i > 0 {
			lSleep(500 * time.Millisecond)
		}
		if _, live, read := hcAgentRead(addr); read && !live { // only a finished record is ended: unread is not gone
			return true
		}
	}
	return false
}
