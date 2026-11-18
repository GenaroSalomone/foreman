package main

// The delivery gate of `hw next`: the sender's route (invoker_resolve_sender, bin/invoker-common.sh), the
// state witness (witness_state and witness_wait,
// the bash state witness) and the refusals that name what the gate read (_next_busy_message,
// _next_settled_message, _next_finished_stale_stuck, lib/hw/next.sh). The witness and the invoker library
// are shared with channel-send and hw ruling, which stay bash; these are `hw next`'s copies.

import (
	"bytes"
	"hwcore/internal/mux"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type nextWitness struct{ verdict, detail string }

// nextRPC runs "$HW_BIN_DIR/herdr-rpc" <args>: stdout, and the exit status.
func (a *app) nextRPC(args ...string) (string, int) {
	cmd := exec.Command(a.nextBinDir()+"/herdr-rpc", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	rc := exitStatus(cmd.Run())
	return out.String(), rc
}

// nextHC says the pane is an app terminal (herdr-core): its gate reads `hc get`, its keys are `hc keys`, and the herdr
// socket is never touched for it.
func (a *app) nextHC(pane string) bool { return mux.IsHCAddress(a.getenv, pane) }

// nextAgentGet is `herdr agent get <pane>`, or `hc get` in the same shape for an app terminal ("" when it cannot answer).
func (a *app) nextAgentGet(pane string) string {
	if a.nextHC(pane) {
		info, _ := hcAgentGetJSON(pane)
		return info
	}
	return nextHerdrSub("agent", "get", pane)
}

// nextWaitIdle waits for idle,done: 0 reached; on the TUI `herdr-rpc wait-agent`'s own codes, on the app hcWaitBoundary's
// (2 budget spent, 3 terminal gone, 6 blocked).
func (a *app) nextWaitIdle(pane string, ms int64) int {
	if !a.nextHC(pane) {
		_, rc := a.nextRPC("wait-agent", pane, "idle,done", "--timeout-ms", strconv.FormatInt(ms, 10))
		return rc
	}
	return hcWaitBoundary(
		func() (string, bool) {
			info, ok := hcAgentGetJSON(pane)
			return jqRawOr(info, "", "result", "agent", "agent_status"), ok
		},
		func(secs int64) int {
			if _, ok := hcOut(0, "wait", pane, "--timeout", strconv.FormatInt(secs, 10)); ok {
				return 0
			}
			return 1
		},
		func() { time.Sleep(time.Second) }, ms, "idle", "done")
}

// nextReadText is the visible text of the pane, `lines` rows (`pane.read` / `hc read`).
func (a *app) nextReadText(pane, lines string) string {
	if a.nextHC(pane) {
		out, _ := hcOut(0, "read", pane, "--lines", lines)
		return out
	}
	out, _ := a.nextRPC("call", "pane.read", `{"pane_id":"`+pane+`","source":"visible","lines":`+lines+`}`)
	return jqRawOr(out, "", "read", "text")
}

// nextEnvInt is `: "${VAR:=def}"` read as a number.
func (a *app) nextEnvStr(k, def string) string { return a.nextEnvOr(k, def) }

func nextSleep(s string) {
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 0 {
		time.Sleep(time.Duration(f * float64(time.Second)))
	}
}

// ── the route ───────────────────────────────────────────────────────────────

func nextPath(v *jval, path ...string) (*jval, bool) {
	cur := v
	for _, p := range path {
		var ok bool
		if cur, ok = cur.get(p); !ok {
			return nil, false
		}
	}
	return cur, true
}

// nextResolveRoute is cmd_next's route block: the brainer-side pane is an hc terminal or a herdr pane; neither has a
// native transport, so the delivery is channel-send's `hc` or `herdr` route.
func (a *app) nextResolveRoute(s *nextState) {
	if a.nextHC(s.pane) {
		s.route, s.target, s.endpoint = "hc", s.pane, "-"
		return
	}
	s.route, s.target, s.endpoint = "herdr", s.pane, "-"
}

// ── the reads of one agent snapshot ─────────────────────────────────────────

func nextAgentField(info string, path ...string) string {
	return jqRawOr(info, "", append([]string{"result", "agent"}, path...)...)
}

// nextBusyMessage is _next_busy_message.
func nextBusyMessage(pane string, waitMS int64, info string) string {
	st := nextAgentField(info, "agent_status")
	askState := nextAgentField(info, "tokens", "ask_state")
	doneStatus := nextAgentField(info, "tokens", "done_status")
	doneState := nextAgentField(info, "tokens", "done_state")
	or := func(v, d string) string {
		if v == "" {
			return d
		}
		return v
	}
	switch {
	case st == "blocked" && askState == "delivered" && doneStatus != "done" && doneStatus != "blocked":
		seq := jqRawOr(info, "?", "result", "agent", "tokens", "ask_seq")
		at := jqRawOr(info, "an unknown time", "result", "agent", "tokens", "ask_at")
		text := nextAskText(info)
		return pane + " is blocked on its own ask " + seq + ", delivered " + at + " and still unanswered: \"" + text + "\". Re-tasking cannot reach idle,done from here no matter how large a budget you pass — hw next gates on idle,done, and a pane parked on its own ask never gets there until the ask is answered. Answer it, then retry: herdr agent get " + pane
	case st == "blocked" && askState == "delivered":
		return pane + " already reported done (done_status=" + doneStatus + ", done_state=" + or(doneState, "unknown") + ") — the ask token you see is stale, published before that report, and there is nothing left to answer. Do not answer it: read the actual report with `herdr agent get " + pane + "` and re-task from that, or override with --force if you are deliberately replacing it."
	}
	return pane + " was still busy after " + strconv.FormatInt(waitMS/1000, 10) + "s (agent_status=" + st + "), so nothing was sent and the task counter did not move. A mid-turn prompt is NOT dropped — it is acted on — but task N+1 answered inside task N is answered against the wrong state, with the counter already advanced, which is what this gate is for. Retry, or raise the budget: HW_NEXT_WAIT_MS=540000 hw next …" // # MUTATION-ANCHOR: 862-G11
}

// nextAskText is `jq -r '[.result.agent.tokens | .ask, .ask2, … .ask8] | map(select(. != null)) | join("")'`.
func nextAskText(info string) string {
	vals, _ := parseStream([]byte(info))
	var outs []string
	for _, v := range vals {
		tk, ok := nextPath(v, "result", "agent", "tokens")
		if !ok {
			continue
		}
		var sb strings.Builder
		bad := false
		for _, k := range []string{"ask", "ask2", "ask3", "ask4", "ask5", "ask6", "ask7", "ask8"} {
			if tk.kind == vNull {
				continue
			}
			c, ok := tk.get(k)
			if !ok {
				bad = true
				break
			}
			switch c.kind {
			case vNull:
			case vStr, vNum, vBool:
				sb.WriteString(c.tostring())
			default:
				bad = true
			}
		}
		if !bad {
			outs = append(outs, sb.String())
		}
	}
	return bashSub(strings.Join(outs, "\n"))
}

// ── the witness (screen motion) ──────────────────────────────────────────────

// nextScreenHash is witness_screen_hash: the first 16 hex of sha256 of the visible text, or false.
func (a *app) nextScreenHash(pane string) (string, bool) {
	out, rc := a.nextRPC("call", "pane.read", `{"pane_id":"`+pane+`","source":"visible","lines":`+a.nextEnvStr("WITNESS_READ_LINES", "40")+`}`)
	out = bashSub(out)
	if rc != 0 || out == "" {
		return "", false
	}
	vals, perrE := parseStream([]byte(out))
	perr := perrE != nil
	if perr || len(vals) == 0 {
		return "", false
	}
	v := vals[len(vals)-1]
	if v.kind != vObj {
		return "", false
	}
	rd, ok := v.get("read")
	if !ok || rd.kind != vObj {
		return "", false
	}
	t, ok := rd.get("text")
	if !ok || t.kind != vStr {
		return "", false
	}
	var sb strings.Builder
	for _, x := range vals { // jq -j '.read.text' over every value
		if r, ok := nextPath(x, "read", "text"); ok {
			sb.WriteString(r.raw())
		}
	}
	return nextSha16(sb.String()), true
}

// nextScreen is witness_screen: "moved <hash>" / "static <hash>", or false when any sample failed.
func (a *app) nextScreen(pane string) (string, string, bool) {
	first, ok := a.nextScreenHash(pane)
	if !ok {
		return "", "", false
	}
	samples, _ := strconv.Atoi(a.nextEnvStr("WITNESS_SAMPLES", "3"))
	for i := 1; i < samples; i++ {
		nextSleep(a.nextEnvStr("WITNESS_INTERVAL_S", "2"))
		now, ok := a.nextScreenHash(pane)
		if !ok {
			return "", "", false
		}
		if now != first {
			return "moved", first, true
		}
	}
	return "static", first, true
}

// nextWitnessState is witness_state: sets the verdict (live: its screen moved; unknown: the cross-check could not be
// made or the screen was static) and the detail the refusals quote. Nothing reads whether a prompt is outstanding.
func (a *app) nextWitnessState(pane string, w *nextWitness) {
	info := nextHerdrSub("agent", "get", pane)
	status := nextAgentField(info, "agent_status")
	st := status
	if st == "" {
		st = "unreadable"
	}
	claim := "agent_status=" + st
	if reason := nextAgentField(info, "tokens", "blocked_reason"); reason != "" {
		claim += "/" + reason
		if scope := nextAgentField(info, "tokens", "blocked_scope"); scope != "" {
			claim += ":" + scope
		}
	}
	samples, interval := a.nextEnvStr("WITNESS_SAMPLES", "3"), a.nextEnvStr("WITNESS_INTERVAL_S", "2")
	state, _, ok := a.nextScreen(pane)
	if !ok {
		state = "unreadable"
	}
	w.verdict, w.detail = witnessVerdict(pane, claim, state, samples, interval)
}

// nextFinishedStaleStuck is _next_finished_stale_stuck: blocked/stuck WITH delivered completion tokens.
func nextFinishedStaleStuck(info string) bool {
	vals, perrE := parseStream([]byte(info))
	perr := perrE != nil
	if len(vals) == 0 {
		return false
	}
	st := false
	for _, v := range vals {
		get := func(p ...string) *jval {
			c, ok := nextPath(v, p...)
			if !ok {
				return nil
			}
			return c
		}
		is := func(c *jval, s string) bool { return c != nil && c.kind == vStr && c.s == s }
		ds := get("result", "agent", "tokens", "done_status")
		st = is(get("result", "agent", "agent_status"), "blocked") &&
			is(get("result", "agent", "tokens", "blocked_reason"), "stuck") &&
			(is(ds, "done") || is(ds, "blocked")) &&
			is(get("result", "agent", "tokens", "done_state"), "delivered") // # MUTATION-ANCHOR: 862-G10
	}
	if perr {
		return false
	}
	return st
}

// ── the wait ────────────────────────────────────────────────────────────────

// nextWitnessWait is witness_wait: 0 idle,done; 2 budget spent; 6 an app terminal blocked on a person (hc get says
// blocked and nothing can read the prompt); 3/4 herdr-rpc's own. Between slices it records what the screen says, for the
// refusal that quotes it.
func (a *app) nextWitnessWait(pane string, totalMS int64, w *nextWitness) int {
	w.verdict, w.detail = "", ""
	slice, _ := strconv.ParseInt(a.nextEnvStr("WITNESS_SLICE_MS", "20000"), 10, 64)
	wait := func(ms int64) int { return a.nextWaitIdle(pane, ms) }
	if a.nextHC(pane) {
		// no screen witness for an app terminal: `hc get` is the only authority, and it says blocked or it does not
		rc := wait(totalMS)
		switch rc {
		case 6:
			w.detail = pane + " is blocked on a prompt (hc get reports status blocked). Nothing here can read it, so a person has to look at that terminal."
		case 3:
			w.detail = "hc knows no live terminal " + pane + " (or `hc get` could not answer)."
		}
		return rc
	}
	if totalMS <= slice {
		return wait(totalMS)
	}
	deadline := nextEpoch() + totalMS/1000
	for {
		remaining := (deadline - nextEpoch()) * 1000
		if remaining <= 0 {
			return 2
		}
		ms := slice
		if ms > remaining {
			ms = remaining
		}
		switch rc := wait(ms); rc {
		case 0:
			return 0
		case 2:
		default:
			return rc
		}
		if nextEpoch() >= deadline {
			return 2
		}
		a.nextWitnessState(pane, w)
	}
}
