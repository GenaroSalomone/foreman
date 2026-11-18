package main

// THE EXECUTOR IN THE HERDR APP (herdr-app-dispatch, step 2 of «foreman en la herdr app»).
//
// When the dispatching shell's nearest multiplexer ancestor is herdr-core (core/internal/mux, design §3),
// `hw` launches the executor as ONE `hc run` terminal instead of a herdr workspace/tab/pane. It is a second
// branch at the two existing seams — `buildSpace` (nothing to build) and `agentStep` (the agent start and the
// brief) — and no new flag: the environment already says where the process runs. A TUI dispatch takes the
// path it always took, byte for byte: `hcRoute` is false with no HC_TERMINAL_ID.
//
// Every vendor launches as one `hc run` terminal with the brief as its launch task (hcRunArgs: the vendor's own binary and
// brief flag). --here splits a herdr pane: hcRefusal says so, once, for the dry run and for
// the launch. A lane's dev servers are `hc run` terminals of their own (hcDevStart), carrying the task's
// labels so `hw done` and reap end them with the executor.

import (
	"strconv"
	"strings"
	"time"
)

// hcRoute is the one question this file asks (cached by dsp.onHC: the answer walks `ps`).
func (l *lch) hcRoute() bool { return l.onHC() }

// hcLayoutDoc stands where `layout.apply` answered: the launch asks it for the labelled agent and shell panes.
const hcLayoutDoc = `{"layout":{"panes":[{"type":"pane","label":"agent","pane_id":"hc"},{"type":"pane","label":"shell","pane_id":"hc"}]}}`

// hcRefusal is what the herdr-core route does not launch, "" when it launches the dispatch. It is ONE function, asked by
// the dry run (before the manifest's ✓) and again by the launch (hcLaneGate), so the dry run prints every refusal the real
// dispatch would make — a refusal only the launch knew was a ✓ that died (a brainer in the app, 2026-11-11).
func (d *dsp) hcRefusal() string {
	if !d.onHC() {
		return ""
	}
	switch {
	case d.here:
		// the reason, not "not yet": on the app hw starts an executor as its own `hc run` terminal, and it has no pane of the caller's to split
		return "--here splits a herdr pane, and this shell runs under herdr-core (the herdr app), where hw starts every executor as its own `hc run` terminal: there is no pane of the caller's for it to split (the app can place a terminal beside another, `herdr-app do terminal.keep` with `place`, but hw does not drive that and it is unmeasured). Dispatch without --here (`--worktree none --no-db` is the same no-worktree run, in a tab of its own), or from a herdr TUI pane. NOTHING was built."
	}
	return ""
}

// hcLaneGate is the launch's second ask of hcRefusal, BEFORE anything is built: a refusal at buildSpace would come after the
// worktree and the database exist.
func (l *lch) hcLaneGate() {
	if msg := l.hcRefusal(); msg != "" {
		l.die(msg)
	}
}

// hcSpace is buildSpace on herdr-core: there is no workspace, tab or layout to build. The run env file is
// still written, because `hw status` is judged against it.
func (l *lch) hcSpace() {
	l.writeRunEnv(l.workdir(), "{}", l.invokerPane())
	l.layoutOut = hcLayoutDoc
	l.ok("herdr-core: no herdr space, tab or layout is built — the executor is one `hc run` terminal")
}

// hcLive is the address of the live terminal that already carries this task, "" when none. It keys on
// hw_project+hw_task, which are stable: hw_run is new on every invocation, so a second `hw` never matched it.
// A failed `hc ls` is not "none": reading it so launches a duplicate. The task's dev-server terminals carry the same
// two labels (so `hw done` ends them) and are not its executor: hcNotDev leaves them out.
func (l *lch) hcLive() (addr string, ok bool) {
	raw, rc := l.sh(cmdSpec{name: "hc", args: []string{"ls", "--json", "--label", "hw_project=" + l.proj, "--label", "hw_task=" + l.task}, quiet: true, synth: `{"terminals":[]}`})
	v, parsed := bjParse(raw)
	list, isList := bjGet(v, "terminals").([]any)
	if rc != 0 || !parsed || !isList {
		return "", false
	}
	var addrs []string
	for _, e := range list {
		if m, isMap := e.(jmap); isMap && hcStr(m, "address") != "" {
			addrs = append(addrs, hcStr(m, "address"))
		}
	}
	runOf := map[string]string{}
	agents, ok := hcNotDev(addrs, func(a string) (string, bool) {
		out, rc := l.sh(cmdSpec{name: "hc", args: []string{"get", a, "--json"}, quiet: true, synth: `{"labels":{}}`})
		if v, _ := bjParse(brTrimNL(out)); v != nil {
			labels, _ := bjGet(v, "labels").(jmap) // absent labels read as no run, never a panic
			runOf[a] = hcStr(labels, "hw_run")
		}
		return out, rc == 0
	})
	if !ok || len(agents) == 0 {
		return "", ok
	}
	l.hcLiveRun = runOf[agents[0]]
	return agents[0], true
}

// hcRecord is the run's mark as an hc run, <rundir>/hc: what `hw done`, reap and status key their hc calls on.
func (l *lch) hcRecord(addr string) {
	if wd, run := l.workdir(), l.hwRun; wd != "" && run != "" {
		l.fs.mkdirAll(wd + "/.hw/" + run)
		l.fs.write(wd+"/.hw/"+run+"/hc", []byte(l.proj+" "+l.task+" "+addr+"\n"), 0o666)
		// the core this address means: t2 is only t2 inside one core (a QA instance, a restarted core), so the run says whose it is
		if id := hcCoreID(l.env); id != "" {
			l.fs.write(wd+"/.hw/"+run+"/hc-core", []byte(id+"\n"), 0o666)
		}
	}
}

// hcSetAsideReuseRun: a dispatch that reuses a live terminal launches nothing, so the terminal's own run (the hw_run label it was
// launched with, its env's HW_RUN) is the one its done-invoker writes the done marker and the chaining lease into. This
// dispatch's run dir is a copy with neither and, being newer, every reader of "the run" would take it (`hw next` said "no
// chaining authority" after a delivered report). It moves into the terminal's own run, to .hw/<that run>/reuse/<this run>,
// where no listing of `.hw/` sees it (the listers that take every directory there included), and its per-run TMPDIR (empty:
// nothing launched) goes. Only when the terminal's run is on disk here: a terminal with no such run keeps this one as its record.
func (l *lch) hcSetAsideReuseRun() {
	wd, run := l.workdir(), l.hcLiveRun
	if wd == "" || l.hwRun == "" || !RunIDPattern.MatchString(run) || run == l.hwRun || !isDir(wd+"/.hw/"+run) {
		return
	}
	if l.fs.mkdirAll(wd+"/.hw/"+run+"/reuse") != nil || l.fs.rename(wd+"/.hw/"+l.hwRun, wd+"/.hw/"+run+"/reuse/"+l.hwRun) != nil {
		l.warn("run " + l.hwRun + " reused a live terminal but could not be set aside: `hw next` may read it instead of run " + run)
		return
	}
	l.fs.remove(runTmpDir(l.getenv, l.hwRun)) // rmdir: only an empty directory goes
	l.info("run " + l.hwRun + " is not a run of its own: the live terminal belongs to run " + run + ", which stays the one `hw next` and `hw done` read (this one is kept in .hw/" + run + "/reuse/)")
}

// hcEnvWords is hwEnvArgs as the `K=V` words `env` takes in an hc argv (`--env K=V` is herdr's grammar).
func (l *lch) hcEnvWords(cwd string) []string {
	var words []string
	f := strings.Fields(strings.Join(l.hwEnvArgs(cwd), " "))
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "--env" {
			words = append(words, f[i+1])
			i++
		}
	}
	return words
}

// hcRunArgs is the whole `hc run` argv, exactly design §6b. Each `-u` is its own word: the P12 accident was an
// unset list passed as one string, and it let the launcher's HERDR_PANE_ID into a terminal that wrote onto it.
func (l *lch) hcRunArgs(cwd string) []string {
	args := []string{"run", "--bg", "--json", "--no-parent", "--no-notify",
		"--name", l.task, "--cwd", cwd,
		"--label", "hw_project=" + l.proj, "--label", "hw_task=" + l.task, "--label", "hw_run=" + l.hwRun,
		"--label", "herdr.space=" + l.c.t.get(l.proj, "space"),
		"--", "env",
		"-u", "HERDR_ENV", "-u", "HERDR_PANE_ID", "-u", "HERDR_SOCKET_PATH",
		"-u", "HERDR_TAB_ID", "-u", "HERDR_WORKSPACE_ID", "-u", "HW_INVOKER_PANE",
		"-u", "CLAUDE_CODE_CHILD_SESSION"}
	// the core's environment is the app's launcher's: with no account redirect an inherited CLAUDE_CONFIG_DIR must not reach claude
	// (the TUI route clears it in exportHwEnv). env reads options only before its first NAME=VALUE word.
	if !(l.accountApplies && l.accountConfigDir != "") {
		args = append(args, "-u", "CLAUDE_CONFIG_DIR")
	}
	if l.agent == "claude" {
		args = append(args, "CLAUDE_CODE_FORCE_SESSION_PERSISTENCE=1", "CLAUDE_CODE_BASH_EDIT_DIFF=0")
	}
	args = append(args, l.hcEnvWords(cwd)...)
	args = append(args, hcPathWord(l.binDir(), l.env("PATH")))
	args = append(args, l.agent)
	if l.agent == "claude" && l.explorerArm { // an hc terminal has no shell to type a function into: the JSON is a word of the argv
		if json := l.explorer().claudeJSON(); json != "" {
			args = append(args, "--agents", json)
		}
	}
	args = append(args, strings.Fields(l.agentArgs)...)
	return append(args, hcBriefWords(l.briefText)...)
}

// hcBriefWords is how a vendor takes its first message on the command line: the brief is the launch task, no keystrokes to
// race. claude and pi read it as the positional after `--` (claude's --mcp-config is variadic and would otherwise read
// the brief as one more config file).
func hcBriefWords(brief string) []string {
	if brief == "" {
		return nil
	}
	return []string{"--", brief}
}

// hcAgentStep is agentStep on herdr-core: reuse the task's live terminal, or launch one with the brief, and
// prove the brief arrived by the terminal's own `status`/`turn`.
func (l *lch) hcAgentStep() {
	if addr, ok := l.hcLive(); !ok {
		l.die("`hc ls` could not answer, so whether " + l.proj + ":" + l.task + " is already live is UNKNOWN — launching now could start a second executor. NO executor was launched" + hcDevLeft(l.hcDevAddrs, l.proj, l.task) + "; the worktree is intact. Dispatch again when `hc ls` answers.")
	} else if addr != "" {
		l.hcRecord(addr)                                                      // this run is the newest, until hcSetAsideReuseRun moves a reuse dispatch's away
		l.receipt("pane", addr, "hc ls --label hw_project= --label hw_task=") // the in-use guard reads the pane from it
		l.hcSetAsideReuseRun()                                                // after the receipts: a later write would make the dir again
		l.ok(l.proj + ":" + l.task + " is already live on herdr-core as " + addr + " — reused, nothing launched")
		return
	}
	if l.agent == "pi" && (!l.piProjectSkillsPrepare() || !l.piPluginMirrorsPrepare()) { // as agentStart: the skills' links and the plugins' -e mirrors exist before pi starts
		l.die("pi's project skills or plugin mirrors could not be prepared (see the warning above), so pi would start without them. NOTHING was launched; the worktree is intact.")
	}
	l.engramRegisterSession()
	l.ledgerCommitBrief()
	if l.brief != "" {
		l.buildBriefText()
		nbytes := strconv.Itoa(len(l.briefText))
		l.receipt("brief_sha256", l.dlSha256(l.briefText), "shasum -a 256 of the "+nbytes+" bytes passed to `hc run` as the launch task")
		l.receipt("brief_bytes", nbytes, "wc -c of BRIEF_TEXT at launch")
	}
	if l.explorerArm && l.explorer().claudeJSON() == "" {
		l.warn("the cheap explorer could not be armed for " + l.label + " (the role is unreadable): this executor starts without it")
	}
	space := l.c.t.get(l.proj, "space")
	app := l.hcApp()
	spaceNew := hcSpaceIsNew(app, space)
	raw, rc := l.sh(cmdSpec{name: "hc", args: l.hcRunArgs(l.workdir()), mut: true, merge: true, synth: `{"address":"shadow:t1"}`})
	v, _ := bjParse(brTrimNL(raw))
	addr := ""
	if m, isMap := v.(jmap); isMap {
		addr = hcStr(m, "address")
	}
	if rc != 0 || addr == "" {
		l.receipt("agent_ready", "no — `hc run` did not start the executor", "hc run exit status")
		l.receiptVerdict()
		running := "NOTHING is running."
		if len(l.hcDevAddrs) > 0 {
			running = "NO executor is running, but the dev-server terminal(s) " + strings.Join(l.hcDevAddrs, " ") + " are: `hw done " + l.proj + " " + l.task + "` ends them."
		}
		l.die("`hc run` did not start " + l.agent + " for " + l.label + " (exit " + strconv.Itoa(rc) + "): " + head3(raw) + " — " + running + " The worktree is intact; dispatch again.")
	}
	if spaceNew {
		hcNameSpace(app, lSleep, space)
	}
	if !l.readonly { // a dry run's address is `shadow:t1`: there is no tab to find, and the retries would only wait
		hcNameTab(app, lSleep, addr, l.task) // the executor's tab, never a dev terminal's (hcDevStart names none)
	}
	l.info(l.agent + " started in " + addr + " (herdr-core); vendor runtime readiness is unestablished")
	l.receipt("pane", addr, "hc run --json address")
	l.hcRecord(addr)
	l.receipt("agent_ready", "unknown — hc run answered with "+addr+"; runtime readiness unestablished", "hc run --json")
	l.dispatchPublishHC(addr)
	if l.brief != "" && !l.hcBriefProved(addr) {
		l.receipt("brief_admitted", "no — "+addr+" showed neither status=working nor a turn within the wait", "hc get "+addr+" --json")
		l.ledgerDispatch(addr, 1)
		l.receiptVerdict()
		l.die("the brief was passed to " + addr + " as its launch task, but `hc get " + addr + "` shows neither status=working nor a turn after the wait: it may be holding a dialog. Read it with `hc read " + addr + "`. Do NOT re-send the brief.")
	}
	if l.brief != "" {
		l.receipt("agent_ready", "yes", "hc get "+addr+" --json: status=working or a turn past 0 (the brief's turn)") // the terminal answered a turn: readiness is measured
		l.receipt("brief_admitted", "yes — "+addr+" reports status=working or a turn past 0", "hc get "+addr+" --json")
		l.ok("brief IN THE CONVERSATION on " + addr + " (" + strconv.Itoa(len(l.briefText)) + " bytes, from " + l.brief + ") — the launch task, proved by hc get")
	}
	l.hcSession(addr)
	l.receiptModel(addr, 0) // MUTATION-ANCHOR: hc-model-running — sendBrief writes model_running on the TUI route; this route skipped it
	l.ledgerDispatch(addr, 0)
	l.receiptVerdict()
}

// hcApp runs `herdr-app <args>`: (stdout, exit status).
func (l *lch) hcApp() func(args ...string) (string, int) {
	return func(a ...string) (string, int) {
		return l.sh(cmdSpec{name: "herdr-app", args: a, mut: len(a) > 1, quiet: true, synth: `{"spaces":[]}`})
	}
}

// hcSession is receiptSession on herdr-core: herdr publishes no agent_session there, but claude writes
// <config dir>/sessions/<pid>.json (pid, sessionId, cwd) and `hc get` names the terminal's pid, so the id `hw revive` needs
// (session_resume) is read from it, accepted only when that file's cwd is the terminal's own.
func (l *lch) hcSession(addr string) {
	if l.wt == "" || l.hwRun == "" {
		return
	}
	if l.agent != "claude" { // only claude writes sessions/<pid>.json: another vendor's is not read on this route
		l.receipt("session_id", "none — "+l.agent+" publishes no session id the herdr-core route reads (only claude's sessions/<pid>.json)", "hc get "+addr+" --json")
		l.warn("NO SESSION ID: " + l.agent + " on herdr-core publishes none this route reads, so this conversation is not addressable after the fact and `hw revive` cannot resume it.")
		return
	}
	budget := lrcMillis(l.env("HW_SESSION_WAIT_MS"), 15000)
	home := l.env("HOME")
	sid := ""
	var waited int64
	for {
		out, _ := l.sh(cmdSpec{name: "hc", args: []string{"get", addr, "--json"}, quiet: true, synth: `{"state":"live"}`})
		if v, ok := bjParse(brTrimNL(out)); ok {
			m, _ := v.(jmap)
			sid = claudeSessionOf(bjRaw(m["pid"]), hcStr(m, "cwd"), l.accountConfigDir, l.env("CLAUDE_CONFIG_DIR"), home+"/.claude")
		}
		if sid != "" || waited >= budget {
			break
		}
		l.lrcSleep()
		waited += 1000
	}
	src := "<config>/sessions/<pid>.json of the pid `hc get " + addr + "` names"
	none := "claude wrote no sessions/<pid>.json for " + addr
	if sid == "" {
		l.receipt("session_id", "none — "+none+" within "+strconv.FormatInt(budget/1000, 10)+"s", src)
		l.warn("NO SESSION ID: this conversation is not addressable after the fact, and `hw revive` cannot resume it.")
		return
	}
	l.receipt("session_id", sid, src+", "+strconv.FormatInt(waited, 10)+"ms after the brief")
	l.receipt("session_vendor", l.agent, "hc run: the vendor is "+l.agent)
	l.receiptResume(sid, l.agent)
}

// dispatchPublishHC is dispatchPublish's twin: the labels `hc run` already set are the executor's identity on
// the cockpit; the brainer to report to is the one fact left to write.
func (l *lch) dispatchPublishHC(addr string) {
	if inv := l.invokerPane(); inv != "" {
		l.sh(cmdSpec{name: "hc", args: []string{"label", addr, "hw_invoker=" + inv}, mut: true, quiet: true})
	}
}

// hcBriefProved polls `hc get` until the terminal is working or its turn left 0. false is "not proved".
func (l *lch) hcBriefProved(addr string) bool {
	budget := lrcMillis(l.env("HW_HC_PROOF_MS"), 30000)
	for waited := int64(0); ; waited += 1000 {
		out, rc := l.sh(cmdSpec{name: "hc", args: []string{"get", addr, "--json"}, quiet: true, synth: `{"state":"live","turn":1}`})
		v, ok := bjParse(brTrimNL(out))
		if m, isMap := v.(jmap); rc == 0 && ok && isMap {
			turn := pyStr(m["turn"])
			if hcStr(m, "state") == "live" && (bjRaw(bjGet(m, "status", "state")) == "working" || (turn != "" && turn != "None" && turn != "0")) {
				return true
			}
		}
		if waited >= budget {
			return false
		}
		lSleep(time.Second)
	}
}

// The app names a space it makes after the folder or the terminal that opened it ("brain", then "qa-d"), so every
// Agents row read "qa-d · …". `herdr.space=<lane>` gives the space its ID only; a space is named by renaming it.
//
// hcSpaceIsNew asks BEFORE the `hc run` that may create the space; hcNameSpace renames it AFTER, and only a space this
// run made — a name a person gave an existing space is theirs. app runs `herdr-app <args>`: (stdout, exit status).
func hcSpaceIsNew(app func(args ...string) (string, int), space string) bool {
	out, rc := app("ls")
	v, ok := bjParse(brTrimNL(out))
	spaces, isList := bjGet(v, "spaces").([]any)
	if rc != 0 || !ok || !isList { // no answer is not "no such space": renaming on it would overwrite a person's name
		return false
	}
	for _, s := range spaces {
		if m, isMap := s.(jmap); isMap && hcStr(m, "id") == space {
			return false
		}
	}
	return true
}

// hcNameSpace retries for a few seconds: measured 2026-11-09, `space.rename` answers `no space "<id>"` for the first
// moments after `hc run` returns. Best effort — a space left with the app's default name is cosmetic.
func hcNameSpace(app func(args ...string) (string, int), sleep func(time.Duration), space string) bool {
	for i := 0; i < 5; i++ {
		if i > 0 {
			sleep(time.Second)
		}
		if _, rc := app("do", "space.rename", "--id", space, "--name", space); rc == 0 {
			return true
		}
	}
	return false
}

// The app names a tab it makes for a terminal by its number («1», «2»), so two lanes' tabs read alike. `herdr.space=<lane>`
// opens a tab for the terminal and says nothing of its name: a tab is named by renaming it, as a space is.
//
// hcNameTab finds the tab holding terminal `term` (by the terminal, never by position: MEASURED 2026-11-11 on an isolated
// app, tab «2» held the third-started terminal and «3» the second) and renames it `name` — only while it still carries the number the
// app gave it, so a name a person chose is theirs. A retry, as hcNameSpace: the tab is not always there the instant
// `hc run` returns. Best effort — a tab left with its number is cosmetic.
func hcNameTab(app func(args ...string) (string, int), sleep func(time.Duration), term, name string) bool {
	for i := 0; i < 5; i++ {
		if i > 0 {
			sleep(time.Second)
		}
		out, rc := app("ls")
		v, ok := bjParse(brTrimNL(out))
		spaces, isList := bjGet(v, "spaces").([]any)
		if rc != 0 || !ok || !isList {
			return false // an app that cannot answer is not a tab that is late: waiting would only slow the dispatch
		}
		id, cur := hcTabOf(spaces, term)
		if id == "" {
			continue
		}
		if !allDigits(cur) {
			return false // already named: not this run's to rename
		}
		if _, rc := app("do", "tab.rename", "--id", id, "--name", name); rc == 0 {
			return true
		}
	}
	return false
}

// hcTabOf is the id and name of the tab that holds terminal `term` in an `herdr-app ls` spaces list, "" when none does.
func hcTabOf(spaces []any, term string) (id, name string) {
	for _, s := range spaces {
		sm, _ := s.(jmap)
		tabs, _ := sm["tabs"].([]any)
		for _, t := range tabs {
			tm, _ := t.(jmap)
			terms, _ := tm["terminals"].([]any)
			for _, e := range terms {
				if em, isMap := e.(jmap); isMap && hcStr(em, "terminal") == term {
					return hcStr(tm, "id"), hcStr(tm, "name")
				}
			}
		}
	}
	return "", ""
}

// hcNotDev is the terminals of a task that are its executor: those whose own labels (`hc ls` carries none, `hc get` does)
// name no hw_role. A terminal `hc get` cannot read is not "no role": ok=false, as for an unreadable list.
func hcNotDev(addrs []string, get func(addr string) (string, bool)) (agents []string, ok bool) {
	for _, a := range addrs {
		raw, got := get(a)
		v, parsed := bjParse(brTrimNL(raw))
		m, isMap := v.(jmap)
		if !got || !parsed || !isMap {
			return nil, false
		}
		if labels, _ := m["labels"].(jmap); hcStr(labels, "hw_role") == "" {
			agents = append(agents, a)
		}
	}
	return agents, true
}

// hcDevName is the terminal's name: 1-32 of a-z 0-9 - _, unique among live terminals, so the task is cut to leave room for the role.
func hcDevName(task, role string) string {
	if room := 32 - 1 - len(role); len(task) > room {
		task = task[:room]
	}
	return task + "-" + role
}

// hcDevStart is `herdr pane run <pane> <cmd>` on herdr-core: a lane's dev server as an `hc run` terminal of its own. The
// pane the layout names by <role> is its spec — cwd and env, rendered as herdr would have rendered them (PORT, CORS_ORIGINS,
// the executor's HW_*) — so the TUI route and this one run the server with the same ground. The terminal carries the task's
// hw_project/hw_task (`hw done` and reap end every terminal of the task by them) and hw_role=<role>, which keeps it from being
// taken for the executor. A terminal of this role already live for the task is kept, never doubled.
func (l *lch) hcDevStart(tpl, role, cmd string) {
	if l.env("HW_TAB") == "" { // the layout names ${HW_TAB}; there is no tab here, and a placeholder with no value is an error
		l.setEnv("HW_TAB", "")
	}
	doc, rc := l.renderLayout(tpl)
	if rc != 0 {
		l.die("cannot render " + tpl + " to find the " + role + " terminal's directory and environment")
	}
	var cwd string
	var env []string
	vals, _ := parseStream([]byte(doc))
	for _, v := range vals {
		if p := spFirstPane(v, role); p != nil {
			if c, _ := p.get("cwd"); c.kind == vStr {
				cwd = c.s
			}
			if e, _ := p.get("env"); e.kind == vObj {
				for _, m := range e.obj {
					if m.v.kind == vStr {
						env = append(env, m.k+"="+m.v.s)
					}
				}
			}
		}
	}
	if cwd == "" {
		l.die("layout " + tpl + " names no " + role + " pane with a cwd: nothing to start the " + role + " terminal in")
	}
	labels := []string{"--label", "hw_project=" + l.proj, "--label", "hw_task=" + l.task, "--label", "hw_role=" + role}
	raw, rc := l.sh(cmdSpec{name: "hc", args: append([]string{"ls", "--json"}, labels...), quiet: true, synth: `{"terminals":[]}`})
	v, parsed := bjParse(raw)
	list, isList := bjGet(v, "terminals").([]any)
	if rc != 0 || !parsed || !isList {
		l.die("`hc ls` could not answer, so whether the " + role + " terminal of " + l.label + " is already live is UNKNOWN — starting it now could double it. Dispatch again when `hc ls` answers.")
	}
	for _, e := range list {
		if m, isMap := e.(jmap); isMap && hcStr(m, "address") != "" {
			l.hcDevAddrs = append(l.hcDevAddrs, hcStr(m, "address"))
			l.ok(role + " terminal already live as " + hcStr(m, "address") + " — kept, not started again")
			l.warn("the " + role + " server keeps the port it was started with: the port this run picked above is NOT the one it listens on — read it from `hc read " + hcStr(m, "address") + "`")
			return
		}
	}
	args := []string{"run", "--bg", "--json", "--no-parent", "--no-notify",
		"--name", hcDevName(l.task, role), "--cwd", cwd,
		"--label", "hw_project=" + l.proj, "--label", "hw_task=" + l.task, "--label", "hw_run=" + l.hwRun, "--label", "hw_role=" + role,
		"--label", "herdr.space=" + l.c.t.get(l.proj, "space"),
		"--", "env",
		"-u", "HERDR_ENV", "-u", "HERDR_PANE_ID", "-u", "HERDR_SOCKET_PATH",
		"-u", "HERDR_TAB_ID", "-u", "HERDR_WORKSPACE_ID", "-u", "HW_INVOKER_PANE", "-u", "CLAUDE_CODE_CHILD_SESSION"}
	args = append(args, env...)
	args = append(args, hcPathWord(l.binDir(), l.env("PATH")), "sh", "-c", cmd)
	space := l.c.t.get(l.proj, "space")
	app := l.hcApp()
	spaceNew := hcSpaceIsNew(app, space) // the dev terminal may be the first of the lane: it is then what makes the space, and what names it
	out, rc := l.sh(cmdSpec{name: "hc", args: args, mut: true, merge: true, synth: `{"address":"shadow:d1"}`})
	rv, _ := bjParse(brTrimNL(out))
	addr := ""
	if m, isMap := rv.(jmap); isMap {
		addr = hcStr(m, "address")
	}
	if rc != 0 || addr == "" {
		l.die("`hc run` did not start the " + role + " terminal for " + l.label + " (exit " + strconv.Itoa(rc) + "): " + head3(out) + " — the worktree is intact" + hcDevLeft(l.hcDevAddrs, l.proj, l.task) + "; dispatch again.")
	}
	if spaceNew {
		hcNameSpace(app, lSleep, space)
	}
	l.hcDevAddrs = append(l.hcDevAddrs, addr)
	l.hcRecord(addr) // the run is an hc run from here: `hw done` finds the task's terminals by it even if the executor never starts
	l.ok(role + " terminal " + addr + " (herdr-core): " + cmd)
}

// hcDevLeft names the dev terminals a failed launch leaves running, for the die's last words.
func hcDevLeft(addrs []string, proj, task string) string {
	if len(addrs) == 0 {
		return ""
	}
	return ", but the terminal(s) " + strings.Join(addrs, " ") + " are running: `hw done " + proj + " " + task + "` ends them"
}
