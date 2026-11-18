package main

// Area B2 — starting the agent (bin/hw: _agent_start,
// _claude_startup_ready, _claude_start_failure_screen, _agent_step, _agent_ready_state,
// _dispatch_publish; bin/project-spaces.sh: claude_startup_dialog, claude_dialog_advice). OWNER: the B2 track.

import (
	"strings"
	"time"
)

// lSleep is bash's `sleep`; a package variable so a test can set it to nothing. bash has no scale knob, so the
// contract world runs with real sleeps.
var lSleep = time.Sleep

// lHwRun is ${HW_RUN:-}: the global bash keeps, else the environment it started from.
func (l *lch) lHwRun() string {
	if l.hwRun != "" {
		return l.hwRun
	}
	return l.env("HW_RUN")
}

// lHwWorkdir is ${HW_WORKDIR:-}.
func (l *lch) lHwWorkdir() string {
	if l.hwWorkdir != "" {
		return l.hwWorkdir
	}
	return l.env("HW_WORKDIR")
}

// ── claude_startup_dialog / claude_dialog_advice (bin/project-spaces.sh:801) ────────────────────────────────

// claudeStartupDialog: what a Claude Code pane shows, matched on the dialog's own title; "" when none.
func claudeStartupDialog(screen string) string {
	screen = bashSub(screen)
	switch {
	case strings.Contains(screen, "Try the new fullscreen renderer?"):
		return "fullscreen-offer"
	case strings.Contains(screen, "running in Bypass Permissions mode"):
		return "bypass"
	case strings.Contains(screen, "Is this a project you created or one you trust"):
		return "trust"
	case strings.Contains(screen, "Choose the text style"), strings.Contains(screen, "Select login method"):
		return "onboarding"
	}
	return ""
}

func claudeFirstRunCommand(cfgDir string) string {
	if cfgDir != "" {
		return "CLAUDE_CONFIG_DIR=" + cfgDir + " claude --dangerously-skip-permissions"
	}
	return "claude --dangerously-skip-permissions"
}

func claudeAccountJSON(cfgDir, home string) string {
	if cfgDir != "" {
		return cfgDir + "/.claude.json"
	}
	return home + "/.claude.json"
}

// claudeDialogAdvice: what the person does about each screen, in one line.
func claudeDialogAdvice(dialog, cfgDir, home string) string {
	switch dialog {
	case "bypass":
		return "the Bypass Permissions warning is waiting, and accepting it is yours: run `" + claudeFirstRunCommand(cfgDir) + "` once in a terminal, accept it, /exit, and try again"
	case "onboarding":
		return "Claude Code's welcome (theme, then login) has never been completed in this account: run `" + claudeFirstRunCommand(cfgDir) + "` once in a terminal, finish it and accept the Bypass Permissions warning, /exit, and try again — `claude auth login` alone does not complete it"
	case "trust":
		return "the folder-trust question is waiting: this directory is not marked trusted in " + claudeAccountJSON(cfgDir, home)
	case "fullscreen-offer":
		return "the fullscreen-renderer offer is waiting; \"Not now\" (Esc) keeps the renderer as it is"
	}
	return ""
}

// ── startup readiness ───────────────────────────────────────────────────────────────────────────────────────

// agentRead is `herdr agent read <t> --source detection --lines 80 2>/dev/null || true`.
func (l *lch) agentRead(target string) string {
	out, _ := l.herdr(false, true, "", "agent", "read", target, "--source", "detection", "--lines", "80")
	return bashSub(out)
}

// claudeStartupReady is `_claude_startup_ready`: answers only the renderer offer ("Not now", Esc); names every
// other first-run screen and fails.
func (l *lch) claudeStartupReady(target string) bool {
	dialog := claudeStartupDialog(l.agentRead(target))
	switch dialog {
	case "":
		return true
	case "fullscreen-offer":
		l.info("claude opened on its fullscreen-renderer offer — answering \"Not now\" (Esc), which keeps the renderer as it is")
		if _, rc := l.herdr(true, true, "", "agent", "send-keys", target, "esc"); rc != 0 { // # MUTATION-ANCHOR: 847-B2-04
			return false
		}
		lSleep(2 * time.Second)
		return true
	}
	l.errf("claude is waiting on a first-run screen (%s): %s\n", dialog, claudeDialogAdvice(dialog, l.accountConfigDir, l.env("HOME")))
	return false // # MUTATION-ANCHOR: 847-B2-05
}

// claudeStartFailureScreen is `_claude_start_failure_screen`: what the pane was SHOWING when the start was given up.
func (l *lch) claudeStartFailureScreen(target string) {
	screen := l.agentRead(target)
	if screen == "" {
		return
	}
	l.errf("what the pane was showing when hw gave up (last lines):\n")
	var keep []string
	for _, ln := range strings.Split(screen, "\n") {
		if strings.Trim(ln, " \t\n\v\f\r") == "" { // sed '/^[[:space:]]*$/d'
			continue
		}
		keep = append(keep, ln)
	}
	if len(keep) > 8 { // tail -8
		keep = keep[len(keep)-8:]
	}
	for _, ln := range keep {
		l.errf("    %s\n", ln)
	}
}

// ── agent start ─────────────────────────────────────────────────────────────────────────────────────────────

// jqPath follows keys the way jq's `.a.b.c` does; ok is false when jq would have errored.
func jqPathVal(text string, path ...string) (*jval, bool) {
	vals, err := parseStream([]byte(text))
	if err != nil || len(vals) == 0 {
		return nil, false
	}
	cur := vals[len(vals)-1]
	for _, p := range path {
		next, ok := cur.get(p)
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// startReady is `jq -e '.result.agent.interactive_ready == true'`.
func startReady(out string) bool {
	v, ok := jqPathVal(out, "result", "agent", "interactive_ready")
	return ok && v.kind == vBool && v.b
}

// probeIsAgent is `jq -e --arg pane --arg name --arg kind '.result.agent | .pane_id == $pane and .name == $name and .agent == $kind'`.
func probeIsAgent(probe, pane, name, kind string) bool {
	eq := func(key, want string) bool {
		v, ok := jqPathVal(probe, "result", "agent", key)
		return ok && v.kind == vStr && v.s == want
	}
	return eq("pane_id", pane) && eq("name", name) && eq("agent", kind)
}

// probeReady is `jq -e '.result.agent as $a | $a.interactive_ready == true and (["idle","done"] | index($a.agent_status) != null)'`.
func probeReady(probe string) bool {
	r, ok := jqPathVal(probe, "result", "agent", "interactive_ready")
	if !ok || r.kind != vBool || !r.b {
		return false
	}
	st, ok := jqPathVal(probe, "result", "agent", "agent_status")
	return ok && st.kind == vStr && (st.s == "idle" || st.s == "done")
}

// explorerClaudeArm is `_explorer_claude_arm "${EXPLORER_RUNDIR:-$HW_WORKDIR/.hw/$HW_RUN}"` when EXPLORER_ARM=1: the
// explorer's JSON goes to a file in the run directory and the pane is handed the function that reads it, so the
// launch line stays short. "" (and a warning) when it cannot be armed: the executor then starts without it.
func (l *lch) explorerClaudeArm(pane string) string {
	if !l.explorerArm { // # MUTATION-ANCHOR: 1007-G01
		return ""
	}
	rundir := l.lHwWorkdir() + "/.hw/" + l.lHwRun()
	json := l.explorer().claudeJSON()
	file := explorerClaudeFile(rundir)
	if !explorerRundirOK(rundir) || json == "" || l.fs.mkdirAll(rundir) != nil || l.fs.write(file, []byte(json+"\n"), 0o600) != nil {
		l.warn("the cheap explorer could not be armed for " + pane + " (no run directory, or the role is unreadable): this executor starts without it")
		return ""
	}
	return explorerClaudeFunction(file)
}

// agentStart is `_agent_start "$pane" "$name" "$kind"`: true when the agent is up and ready (bash's status 0).
// Retries while the pane comes up; every call here is inside an `if` in bash, so no status escapes under set -e.
func (l *lch) agentStart(pane, name, kind string) bool {
	if kind == "pi" && (!l.piProjectSkillsPrepare() || !l.piPluginMirrorsPrepare()) { // the project skills' one directory of links, and the plugins' -e mirrors, before anything starts
		return false
	}
	if kind == "claude" {
		l.claudeSessionEnvPrep(pane, l.explorerClaudeArm(pane))
	}
	tries, registered := 0, false
	for tries < 15 { // # MUTATION-ANCHOR: 847-B2-11
		if registered {
			// agent_not_ready keeps the registration. Poll that exact agent; a second creation would collide with our own name.
			out, _ := l.herdr(false, true, "", "agent", "get", pane)
			probe := bashSub(out)
			if !probeIsAgent(probe, pane, name, kind) {
				l.errf("%s\n", probe)
				return false
			}
			if kind == "claude" && !l.claudeStartupReady(pane) {
				return false
			}
			if kind == "pi" && !l.piStartupReady(pane) {
				return false
			}
			if probeReady(probe) { // # MUTATION-ANCHOR: 847-B2-12
				return true
			}
			lSleep(2 * time.Second)
			tries++
			continue
		}
		// --timeout is explicit: herdr's own default (30000ms) is too short for a real worktree's cold start.
		args := []string{"agent", "start", name, "--kind", kind, "--pane", pane, "--timeout", "60000"}
		if l.agentArgs != "" {
			args = append(args, "--")
			args = append(args, strings.Fields(l.agentArgs)...) // unquoted $AGENT_ARGS: word-split
		}
		raw, rc := l.herdrCapture(true, `{"result":{"agent":{"interactive_ready":true}}}`, args...)
		out := bashSub(raw)
		if rc == 0 && startReady(out) { // # MUTATION-ANCHOR: 847-B2-13
			// Herdr can report idle + interactive_ready while a dialog still owns stdin: read the buffer first.
			if kind == "claude" && !l.claudeStartupReady(pane) {
				return false
			}
			// pi's pane reports ready while a startup screen can still own stdin.
			if kind == "pi" && !l.piStartupReady(pane) { // # MUTATION-ANCHOR: 986-G11
				return false
			}
			return true
		}
		switch {
		case strings.Contains(out, "agent_pane_busy"):
		case strings.Contains(out, "agent_not_ready"):
			registered = true // # MUTATION-ANCHOR: 847-B2-14
		case strings.Contains(out, "timed out waiting for agent startup"):
			// A busted BUDGET, not a busted agent: herdr keeps the name, exactly as agent_not_ready does.
			registered = true // # MUTATION-ANCHOR: 847-B2-15
		default:
			l.errf("%s\n", out)
			return false
		}
		lSleep(2 * time.Second)
		tries++
	}
	if kind == "claude" {
		l.claudeStartFailureScreen(pane)
	}
	return false
}

// agentReadyState is `_agent_ready_state`: herdr reports terminal recognition, not vendor runtime readiness.
func (l *lch) agentReadyState(pane string) string {
	j, _ := l.herdr(false, true, "", "agent", "get", pane)
	j = bashSub(j)
	ready := jqRawOr(j, "", "result", "agent", "interactive_ready")
	st := jqRawOr(j, "", "result", "agent", "agent_status")
	if ready+st == "" {
		return "unknown — herdr state unreadable; runtime readiness unestablished"
	}
	switch ready + ":" + st {
	case "true:idle", "true:working", "true:done": // # MUTATION-ANCHOR: 847-B2-16
		return "unknown — runtime readiness unestablished; herdr interactive_ready=" + ready + " agent_status=" + st
	}
	q := func(s string) string {
		if s == "" {
			return "?"
		}
		return s
	}
	return "no — interactive_ready=" + q(ready) + " agent_status=" + q(st)
}

// dispatchPublish is `_dispatch_publish`: the executor is on the brainer's cockpit from its first second. Best effort.
func (l *lch) dispatchPublish(pane string) {
	if pane == "" {
		return
	}
	invoker := l.invokerPane()
	or := func(s string) string {
		if s == "" {
			return "unknown"
		}
		return s
	}
	args := []string{"pane", "report-metadata", pane, "--source", "hw:dispatch", "--ttl-ms", "86400000",
		"--token", "hw_project=" + or(l.proj), "--token", "hw_task=" + or(l.task), "--token", "hw_run=" + or(l.lHwRun())}
	if invoker != "" {
		args = append(args, "--token", "hw_invoker="+invoker)
	}
	l.sh(cmdSpec{name: "herdr", args: args, mut: true, quiet: true})
	if invoker != "" { // # MUTATION-ANCHOR: 847-B2-17
		l.sh(cmdSpec{name: l.binDir() + "/cockpit-state", args: []string{"--kick", "--invoker", invoker}, mut: true, quiet: true})
	}
}

// ── the effort read back ────────────────────────────────────────────────────────────────────────────────────

// ── the step every build path ends with ─────────────────────────────────────────────────────────────────────

// agentStep is `_agent_step`: start the agent in l.pAgent, record, deliver the brief — or die. Ends with the
// workspace focused (on herdr-core there is none: hcAgentStep returns first).
func (l *lch) agentStep() {
	if l.hcRoute() {
		l.hcAgentStep()
		return
	}
	if l.agentStart(l.pAgent, l.task, l.agent) { // # MUTATION-ANCHOR: 847-B2-21
		l.info(l.agent + " registered in " + l.pAgent + "; vendor runtime readiness is unestablished")
		// MEASURED AFTERWARDS, not inferred from the call returning 0.
		l.receipt("agent_ready", l.agentReadyState(l.pAgent), "herdr agent get "+l.pAgent)
		l.receipt("pane", l.pAgent, "layout.apply output")
		if l.modelFloorLine != "" {
			l.receipt("model_floor", l.modelFloorLine, "_check_model_floor against projects.json model_floor of "+l.proj)
		}
		if l.explorerLine != "" {
			l.receipt("explorer", l.explorerLine, "_explorer_resolve (lib/hw/explorer.sh)")
		}
		if l.topTierLine != "" {
			l.receipt("top_tier_why", l.topTierWhy, "--top-tier-why or the brief's top_tier_why (_check_top_tier_ceiling)")
		}
		if l.afterLine != "" {
			l.receipt("after", l.afterLine, "the brief's after: (_check_after), --ignore-after --why when overridden")
		}
		if l.belowFloorWhy != "" {
			l.receipt("below_floor_why", l.belowFloorWhy, "--below-floor-why")
		}
		src := l.requestedBySrc
		if src == "" {
			src = "no citation"
		}
		l.receipt("requested_by", trimNL(l.requestedLine()), "_check_requested: "+src)
		l.dispatchPublish(l.pAgent)
		if rc := l.sendBrief(l.pAgent); rc != 0 { // set -e: a failing _send_brief ends hw with its status
			panic(exitNow{rc}) // # MUTATION-ANCHOR: 847-B2-22
		}
	} else {
		// A launch with no agent is a failed launch. The receipt and its verdict are written first: what hw
		// measured is worth keeping precisely then.
		l.warn(l.agent + " did NOT start in " + l.pAgent + " — the pane is a plain shell; start it by hand")
		if l.brief != "" {
			l.warn("brief NOT delivered (no agent to receive it)")
		}
		l.receipt("agent_ready", "no — _agent_start failed", "hw _agent_start exit status")
		l.receiptVerdict()
		l.die(l.agent + " did NOT start in " + l.pAgent + ", so this task is NOT running: no agent, no brief delivered, and nothing to report back. The pane and its worktree are intact — read the receipt with `hw receipt " + l.proj + " " + l.task + "`, then close it with `hw done " + l.proj + " " + l.task + "` and dispatch again.") // # MUTATION-ANCHOR: 847-B2-23
	}
	l.receiptVerdict()
	if _, rc := l.herdr(true, false, "", "workspace", "focus", l.ws); rc != 0 { // `herdr workspace focus >/dev/null` under set -e
		panic(exitNow{rc}) // # MUTATION-ANCHOR: 847-B2-24
	}
}
