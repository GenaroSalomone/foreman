package main

// stall-watch in Go (bin/stall-watch): the timer for an executor that ended its turn legally and went quiet. `hw executor-turn-end`
// (t3_stall.go) starts one, detached, at each such turn end. It sleeps HW_STALL_POLL seconds at a time and ends silently when the
// wait is over (a new turn, the task reported, the pane gone); still the same turn after HW_STALL_MIN minutes it writes the
// `stalled` marker, one line in the alerts log and, with HW_INVOKER_PANE set, a `channel-send --report` to that pane. ONE alert
// per stall: the marker names the turn it alerted for. The reasons it exists are the bash header's, kept in bin/stall-watch.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var swLiveRe = regexp.MustCompile(`"state": *"live"`)

// swDigits is `tr -dc '0-9'`: every digit of the file, in order, and nothing else; "" when it cannot be read.
func swDigits(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var d []byte
	for _, c := range b {
		if c >= '0' && c <= '9' {
			d = append(d, c)
		}
	}
	return string(d)
}

// swInt is bash's `$(( x ))` for the settings: a number, or 0 for what is not one (an unset name evaluates to 0).
func swInt(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// swReported is `reported`: `done` in the current task's dir (the run dir, or t<N> from task 2 on) and no `reopened` beside it.
func swReported(state string) bool {
	dir := state
	switch n := swDigits(state + "/task"); n {
	case "", "0", "1":
	default:
		dir = state + "/t" + n
	}
	_, errDone := os.Stat(dir + "/done")
	_, errReopened := os.Stat(dir + "/reopened")
	return errDone == nil && errReopened != nil // MUTATION-ANCHOR: s6-sw-reopened
}

// swPaneAlive is `pane_alive`: herdr's agent get for HERDR_PANE_ID; with only an hc terminal, hc's state is live. No hc to ask is
// not "gone".
func swPaneAlive(w *wenv) bool {
	pane, hcID := w.getenv("HERDR_PANE_ID"), w.getenv("HC_TERMINAL_ID")
	if pane == "" && hcID != "" {
		if _, err := exec.LookPath("hc"); err != nil {
			return true
		}
		out, err := exec.Command("hc", "get", hcID, "--json").Output()
		return err == nil && swLiveRe.Match(out)
	}
	if pane == "" {
		return true
	}
	return exec.Command("herdr", "agent", "get", pane).Run() == nil
}

func swStamp(t time.Time) string { return t.Format("2006-01-02T15:04:05") }

// swRun is bin/stall-watch <run-state-dir> <turn-count> <what-it-waits-on>, or bin/stall-watch --children <run-state-dir> <turn-count>
// <claude-pid> (swChildWatch).
func swRun(w *wenv, args []string) int {
	if len(args) > 0 && args[0] == "--children" {
		return swChildWatch(w, args[1:])
	}
	state, turns, what := swArg(args, 0), swArg(args, 1), swArg(args, 2)
	if what == "" {
		what = "a legal wait"
	}
	if st, err := os.Stat(state); err != nil || !st.IsDir() || turns == "" {
		return 0
	}
	dir := w.getenv("HW_STALL_DIR")
	if dir == "" {
		dir = w.getenv("HOME") + "/.cache/hw"
	}
	alerts := dir + "/stall-alerts.log"
	poll := swPoll(w)
	var limit int64
	if v := w.getenv("HW_STALL_AFTER_SEC"); v != "" {
		limit = swInt(v)
	} else {
		mins := int64(20)
		if m := w.getenv("HW_STALL_MIN"); m != "" {
			mins = swInt(m)
		}
		limit = mins * 60
	}
	bin := w.ownDir()
	os.MkdirAll(dir, 0o755)
	start := time.Now()
	for {
		time.Sleep(time.Duration(poll * float64(time.Second)))
		if swDigits(state+"/turns") != turns { // MUTATION-ANCHOR: s6-sw-turn (a new turn: the wait is over)
			return 0
		}
		if swReported(state) {
			return 0
		}
		now := time.Now()
		if now.Unix()-start.Unix() < limit {
			continue
		}
		if !swPaneAlive(w) {
			return 0
		}
		// the marker is the claim: O_EXCL makes exactly one of several watchers of one stall win it. A marker of another turn is
		// stale, and so is one that cannot be read: it goes and the claim below is made again (one duplicate alert at worst,
		// never a stall left unsaid).
		marker := state + "/stalled"
		if b, err := os.ReadFile(marker); err == nil {
			f := strings.Split(strings.TrimRight(string(b), "\n"), " ")
			if len(f) < 2 || f[1] != turns { // MUTATION-ANCHOR: s6-sw-stale
				os.Remove(marker)
			}
		}
		f, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644) // MUTATION-ANCHOR: s6-sw-claim
		if err != nil {
			return 0 // this stall was alerted
		}
		f.WriteString(strconv.FormatInt(start.Unix(), 10) + " " + turns + "\n")
		f.Close()
		mins := (now.Unix() - start.Unix()) / 60
		pane := w.getenv("HERDR_PANE_ID")
		if pane == "" {
			pane = w.getenv("HC_TERMINAL_ID")
		}
		or := func(v, d string) string {
			if v == "" {
				return d
			}
			return v
		}
		what = swAlertWhat(state, turns, what)
		msg := "stall: " + or(w.getenv("HW_PROJECT"), "?") + ":" + or(w.getenv("HW_TASK"), "?") + " (pane " + or(pane, "?") +
			") ended its turn at " + start.Format("15:04") + ", waiting on " + what + ", and has had no new turn for " +
			strconv.FormatInt(mins, 10) + " min (HW_STALL_MIN=" + strconv.FormatInt(limit/60, 10) +
			"). `hw status` shows it STALLED. It may be legitimate (a long heavy run, `setup/heavy --status`; an ask you have not answered) or stuck (an orphaned lock, a stopped holder): look at the pane, then rule it, or `hw done` it."
		swAppend(alerts, swStamp(now)+" "+msg+"\n")
		invoker := w.getenv("HW_INVOKER_PANE")
		if invoker == "" {
			return 0
		}
		cs := w.getenv("HW_STALL_CHANNEL_SEND")
		if cs == "" {
			cs = filepath.Join(bin, "channel-send")
		}
		if err := exec.Command(cs, "--report", "herdr", invoker, "-", msg).Run(); err != nil { // MUTATION-ANCHOR: s6-sw-sendrc
			rc := 127
			if ee, ok := err.(*exec.ExitError); ok {
				rc = ee.ExitCode()
			}
			swAppend(alerts, swStamp(time.Now())+" stall-watch: the alert above did not reach pane "+invoker+" (channel-send exit "+strconv.Itoa(rc)+")\n")
		}
		return 0
	}
}

func swAppend(path, line string) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	f.WriteString(line)
	f.Close()
}

// swChildGone lowers the claim that a turn awaits a child whose shells are gone: the pane's turn_state is the unreported end of a turn
// and children_running is empty, which is what `channel-send --ruling`, `hw ruling` and the cockpit read as a pane to deliver to. A
// turn that ended since (the count moved) is the truth, and is left alone.
func swChildGone(w *wenv, state, turns string) {
	if swDigits(state+"/turns") != turns {
		return
	}
	n, err := strconv.ParseInt(turns, 10, 64)
	if err != nil {
		return
	}
	getenv := func(k string) string {
		if k == "HW_CORE_BIN_DIR" {
			if v := w.getenv(k); v != "" {
				return v
			}
			return w.ownDir()
		}
		return w.getenv(k)
	}
	t := &t3{app: &app{getenv: getenv}, fs: newFsx(getenv)}
	if t.publish("ended_unreported", n, "children_running", "") { // the mark is written only once the claim is really lowered
		os.WriteFile(state+"/child-gone", []byte(turns+"\n"), 0o644)
	}
}

// swAlertWhat is what the stall alert of this turn says it waits on: <what> as the hook named it, unless the child watch lowered the claim for this
// turn (swChildGone), when that child is gone and naming its shell would send the reader after a process that no longer exists.
func swAlertWhat(state, turns, what string) string {
	if b, err := os.ReadFile(state + "/child-gone"); err == nil && strings.TrimSpace(string(b)) == turns {
		return "nothing: the background child it waited on is gone"
	}
	return what
}

// swArg is argument i, or "" past the end.
func swArg(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}

// swPoll is the seconds between two looks (HW_STALL_POLL, 30).
func swPoll(w *wenv) float64 {
	if v := w.getenv("HW_STALL_POLL"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return 30
}

// swChildWatch is `stall-watch --children <run-state-dir> <turn-count> <claude-pid>`: THE CHILD THE TURN WAITS ON MAY DIE UNSEEN. A
// shell that ends without a notification starts no turn, so nothing publishes again and the pane read awaiting_child for as long as it
// lived (t1c1, 2026-11-18: 20+ min, through an interrupt; `channel-send --ruling` refused it and `hw ruling` queued with no turn end
// coming). The hook names the claude process; when it had background shells at this turn end and has none now, the claim is lowered
// (swChildGone). One that had none to see (a claude that names its shells otherwise) is never lowered. It ends with the wait (a new
// turn, the report) or with claude; it is no alert, so it outlives the stall watcher's and HW_STALL_MIN does not reach it.
func swChildWatch(w *wenv, args []string) int {
	state, turns := swArg(args, 0), swArg(args, 1)
	pid, _ := strconv.Atoi(swArg(args, 2))
	if st, err := os.Stat(state); err != nil || !st.IsDir() || turns == "" || pid <= 1 || t3ShellKids(t3ProcTable(), pid) <= 0 {
		return 0
	}
	poll := swPoll(w)
	for {
		time.Sleep(time.Duration(poll * float64(time.Second)))
		if swDigits(state+"/turns") != turns || swReported(state) {
			return 0
		}
		switch t3ShellKids(t3ProcTable(), pid) {
		case -1: // claude itself is gone: nobody is left to rule, and the stall watcher's alert is its own
			return 0
		case 0: // MUTATION-ANCHOR: s6-sw-childgone
			swChildGone(w, state, turns)
			return 0
		}
	}
}
