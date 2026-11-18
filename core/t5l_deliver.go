package main

// Area C — the brief as the executor receives it and its delivery (bin/hw: _send_brief, _deliver_brief,
// _turn_opened, _brief_undelivered). The prompt text is t5l_deliver_prompt.go's.
// OWNER: the C track.

import (
	"strconv"
	"strings"
)

// _send_brief "$pane": the status it returns is the delivery's (0 = delivered or no brief).
func (l *lch) sendBrief(pane string) int {
	// Before the brief guard: a brief-less run saves to engram too.
	l.engramRegisterSession()
	l.ledgerCommitBrief()
	if l.brief == "" { // # MUTATION-ANCHOR: 847-C-11
		l.ledgerDispatch(pane, 0)
		return 0
	}
	l.buildBriefText()
	// THE SESSION ID IS RECORDED ON BOTH PATHS, and the delivery status is passed through untouched.
	drc := l.deliverBrief(pane)
	l.ledgerDispatch(pane, drc)
	l.receiptSession(pane, drc)
	l.receiptModel(pane, drc) // || true
	return drc                // # MUTATION-ANCHOR: 847-C-12
}

// buildBriefText is the prompt assembly of _send_brief: BRIEF_TEXT, wrapped in everything the executor is told.
func (l *lch) buildBriefText() {
	f := l.promptFacts()
	f.assetsNote = l.copyBriefAssets()
	l.briefText = renderPrompt(f)
}

func (l *lch) agentGet(pane string) string {
	out, _ := l.herdr(false, true, "", "agent", "get", pane)
	return out
}

// redeliverLine is the dim line every undelivered branch prints for the herdr route.
func (l *lch) redeliverHerdrLine(pane string) {
	l.printf("  %s%s%s\n", l.cDim, "  "+l.binDir()+"/channel-send herdr "+pane+" - \"$(cat "+l.brief+")\"", l.c0)
}

// turnOpened is _turn_opened: did a turn actually open, asked of herdr.
func (l *lch) turnOpened(pane string) bool {
	switch jqRawOr(l.agentGet(pane), "", "result", "agent", "agent_status") {
	case "working", "done":
		return true
	}
	return false
}

// briefUndelivered is _brief_undelivered: the brief is NOT in the executor's context; only re-delivering it helps.
func (l *lch) briefUndelivered(pane string) {
	l.warn("THE BRIEF IS NOT IN " + pane + "'s CONTEXT. Nothing was given to this executor.")
	l.warn("do NOT tell it to start, and do NOT send a generic nudge — it has nothing")
	l.warn("to read, and no way to tell you so. Re-deliver the brief itself:")
	l.redeliverHerdrLine(pane)
	l.warn("if that fails too, close the run with `hw done` and dispatch again —")
	l.warn("an executor holding no brief is not behind, it is empty")
}

// dlHead is `printf '%s\n' "$text" | head -N >&2`.
func (l *lch) dlHead(text string, n int) {
	lines := strings.Split(text+"\n", "\n")
	lines = lines[:len(lines)-1]
	if len(lines) > n {
		lines = lines[:n]
	}
	for _, ln := range lines {
		l.errf("%s\n", ln)
	}
}

// dlSha256 is `printf '%s' "$text" | shasum -a 256 | cut -d" " -f1`.
func (l *lch) dlSha256(text string) string {
	out, _ := l.sh(cmdSpec{name: "shasum", args: []string{"-a", "256"}, stdin: text})
	lines := strings.Split(out, "\n")
	for i, ln := range lines {
		if j := strings.IndexByte(ln, ' '); j >= 0 {
			lines[i] = ln[:j]
		}
	}
	return bashSub(strings.Join(lines, "\n"))
}

// channelSend is `"$HW_BIN_DIR/channel-send" <args…> 2>&1`: the text and the status.
func (l *lch) channelSend(args ...string) (string, int) {
	out, rc := l.sh(cmdSpec{name: l.binDir() + "/channel-send", args: args, mut: true, merge: true})
	return bashSub(out), rc
}

// deliverBrief is _deliver_brief: ONE DELIVERY PATH FOR BRIEFS AND RE-TASKS. 0 = delivered.
func (l *lch) deliverBrief(pane string) int {
	nbytes := strconv.Itoa(len(l.briefText))
	l.receipt("brief_sha256", l.dlSha256(l.briefText), "shasum -a 256 of the "+nbytes+" bytes passed to the delivery route")
	l.receipt("brief_bytes", nbytes, "wc -c of BRIEF_TEXT at delivery")

	// READ THE SCREEN AGAIN, BECAUSE READINESS IS NOT A PROPERTY. The PANE's measured vendor, not $AGENT.
	paneVendor := jqRawOr(l.agentGet(pane), "", "result", "agent", "agent")
	if paneVendor == "" {
		paneVendor = l.agent
		if paneVendor == "" {
			l.warn("could not measure " + pane + "'s vendor and none was requested — vendor-specific delivery checks are SKIPPED for this delivery")
		}
	}

	// pi: a second look, immediately before anything is typed.
	if paneVendor == "pi" && l.piDialogBeforeDelivery(pane) {
		return 1
	}

	// herdr route. --wait --until working turns "the text was typed" into "a turn opened".
	piBase := 0
	if paneVendor == "pi" {
		piBase = l.piInputsCount()
	}
	timeout := orDefault2(l.env("HW_TURN_WAIT_MS"), "45000")
	pout, prc := l.sh(cmdSpec{name: "herdr", args: []string{"agent", "prompt", pane, l.briefText, "--wait", "--until", "working", "--timeout", timeout}, mut: true, merge: true})
	pout = bashSub(pout)
	if prc == 0 {
		if paneVendor == "pi" { // # MUTATION-ANCHOR: 986-G08
			if l.piAdmit(pane, "a turn opened", nbytes, piBase) {
				return 0
			}
			return 1
		}
		l.ok("brief IN THE CONVERSATION on " + pane + " (" + nbytes + " bytes, from " + l.brief + ") — a turn opened")
		return 0
	}
	switch {
	case strings.Contains(pout, "agent_prompt_stalled"):
		l.warn("herdr did not observe a turn after submission (agent_prompt_stalled)")
		l.warn("submission is not proof of admission; replay remains subject to the delivery checks below")
		// ONE EXACT REPLAY, AUTOMATICALLY: exactly once, and exactly the same bytes.
		l.warn("replaying the exact brief once over herdr")
		rOut, rRC := l.channelSend("herdr", pane, "-", l.briefText)
		if paneVendor == "pi" && rRC == 0 && l.turnOpened(pane) {
			if l.piAdmit(pane, "replayed once over herdr", nbytes, piBase) {
				return 0
			}
			return 1
		}
		if rRC == 0 && l.turnOpened(pane) { // # MUTATION-ANCHOR: 847-C-24
			l.receipt("brief_admitted", "yes — replayed once over herdr", "channel-send herdr + herdr agent get "+pane+" showed a turn")
			l.ok("brief IN THE CONVERSATION on " + pane + " (" + nbytes + " bytes, from " + l.brief + ") — replayed once over herdr, turn confirmed")
			return 0
		}
		l.warn("the replay did not produce a turn either (exit " + strconv.Itoa(rRC) + ")")
		if rOut != "" {
			l.dlHead(rOut, 3)
		}
		l.receipt("brief_admitted", "no — first attempt stalled, replay over herdr also failed", "herdr agent prompt --until working, then channel-send herdr")
	case strings.Contains(pout, "agent_blocked"):
		l.warn(pane + " is blocked and refused the submission before any input was sent")
	default:
		first := pout
		if i := strings.IndexByte(first, '\n'); i >= 0 {
			first = first[:i]
		}
		l.warn("herdr could not put the brief into " + pane + ": " + first)
	}
	l.briefUndelivered(pane)
	return 1
}

// psep is `$(printf '%s\n\n---\n\n%s' "$a" "$b")`.
func psep(a, b string) string { return bashSub(a + "\n\n---\n\n" + b) }
