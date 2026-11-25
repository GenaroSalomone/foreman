package main

// THE herdr-core READER OF THE INVOKER RESOLVER (herdr-app-brain, step 2).
//
// The TUI resolver reads `herdr pane list` and matches on pane_id, label and cwd.
// An app terminal carries the same three facts under other names — its address,
// the `label` key of its hc labels, and its cwd — so this reader NORMALISES
// `hc ls`/`hc get` into that shape and the resolver stays one program: the same
// arms, the same refusals, the same words. `hc ls` answers with no labels, so the
// label of a NAMED terminal costs one `hc get`; the brainers come from a label
// query, which needs none.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// hcOut is herdrOut's hc twin: what `hc <args>` printed, and whether it succeeded.
func hcOut(timeout time.Duration, args ...string) (string, bool) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "hc", args...)
	// a child that outlives the deadline still holds the stdout pipe (herdrOut's reason)
	cmd.WaitDelay = 100 * time.Millisecond
	cmd.Env = childEnv()
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err == nil
}

// herdrNoServer says whether text (a herdr reply or its stderr) is herdr's "no server is running" answer: the one failure
// that lets the app's core stand in for it (brain, status and occupancy each ask it).
func herdrNoServer(text string) bool { return strings.Contains(text, `"server_not_running"`) }

// hcLsTerminals is the terminals array of an `hc ls --json` reply; ok is false when it is not a reply that holds one
// ("not answered", never "none").
func hcLsTerminals(raw string) (terms []any, ok bool) {
	v, parsed := bjParse(brTrimNL(raw))
	terms, isList := bjGet(v, "terminals").([]any)
	return terms, parsed && isList
}

// hcPaneListJSON is this call's terminal list in herdr's shape,
// {"result":{"panes":[{"pane_id","label","cwd","agent"}]}}, or "" when `hc ls`
// could not answer — the same "" the herdr path reads as unreachable.
//
// named are the terminals this call names (HW_INVOKER_PANE, HC_TERMINAL_ID): they
// are read with `hc get` so their own labels are visible, which is what lets the
// resolver refuse an executor or accept a brainer it would otherwise not know.
func hcPaneListJSON(named ...string) string {
	raw, ok := hcOut(0, "ls", "--json", "--label", "label=brain")
	if !ok {
		return ""
	}
	v, parsed := bjParse(brTrimNL(raw))
	list, isList := bjGet(v, "terminals").([]any)
	if !parsed || !isList { // a reply hw cannot read is not "no brainer": see hcTerminals
		return ""
	}
	panes := []any{}
	seen := map[string]bool{}
	add := func(id, label, cwd string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		panes = append(panes, jmap{"pane_id": id, "label": label, "cwd": cwd, "agent": "hc"})
	}
	for _, e := range list {
		if m, isMap := e.(jmap); isMap {
			add(hcStr(m, "address"), "brain", hcStr(m, "cwd"))
		}
	}
	fetched := map[string]bool{}
	for _, id := range named {
		if id == "" || fetched[id] {
			continue
		}
		fetched[id] = true
		one, got := hcOut(0, "get", id, "--json")
		if !got {
			continue // not a live terminal: the resolver's own liveness refusal says so
		}
		v, parsed := bjParse(brTrimNL(one))
		if !parsed {
			continue
		}
		m, isMap := v.(jmap)
		if !isMap || hcStr(m, "state") != "live" { // a finished terminal is not a pane to report to
			continue
		}
		label := ""
		if labels, isMap := m["labels"].(jmap); isMap {
			label = hcStr(labels, "label")
		}
		add(hcStr(m, "address"), label, hcStr(m, "cwd"))
	}
	out, err := json.Marshal(map[string]any{"result": map[string]any{"panes": panes}})
	if err != nil {
		return ""
	}
	return string(out)
}

// hcAgentGetJSON is `herdr agent get <id>` for a live hc terminal, in herdr's shape
// ({"result":{"agent":{agent_status,cwd,agent,tokens}}}), so the witness helpers
// channel-send and hw ruling already have read it unchanged. The tokens are the
// terminal's labels as published (hw_turn_state), plus each one without its hw_
// prefix (turn_state) where that name is free: the herdr tokens carry no prefix.
// agent_status is the program's own OSC 7501 word, "unknown" while it has said none.
// ok is false for a terminal that is gone, finished, or unreadable.
func hcAgentGetJSON(id string) (string, bool) {
	info, live, _ := hcAgentRead(id)
	return info, live
}

// hcRecordGone is the predicate for "this terminal is gone" over an `hc get <asked>` reply: a record that names the terminal asked
// for (its address, or its name: `get` accepts both) and says `finished`. Gone needs positive evidence; `{}`, an error object,
// `starting`, `lost`, another terminal's record and every other reply is UNKNOWN, and a caller that would start a second terminal
// beside it refuses or waits.
//
// The contract is the predicate's alone: a reader that needs a record that IS live reads "not live" as "not listed", never as gone, and
// bin/cockpit-state hc_gone (Python, the cockpit writer) has its own, wider definition that ends a writer and starts nothing. The
// inventory of readers is in the findings (topic_key setup/hc-gone-one-predicate/findings), not here: a hand-kept list drifts.
func hcRecordGone(m jmap, asked string) bool {
	addr := hcStr(m, "address")
	return addr != "" && asked != "" && (addr == asked || hcStr(m, "name") == asked) && hcStr(m, "state") == "finished"
}

// hcFirstArg is argv[0] of a `foreground` record ("" when it has none).
func hcFirstArg(fg jmap) string {
	if argv, _ := fg["argv"].([]any); len(argv) > 0 {
		s, _ := argv[0].(string)
		return s
	}
	return ""
}

// hcAgentRead is hcAgentGetJSON that also says whether `hc get` ANSWERED: read=false is a call that failed or a reply that is no
// record of a terminal in a state this code knows (`{}`, an error object, `starting`, `lost`: unknown), read=true with live=false
// is a terminal the core says is gone: a record with its address and `finished` (hcRecordGone). A caller that acts on "not live"
// (a restore that replaces the terminal) needs that difference; "unknown" is never "not claude".
func hcAgentRead(id string) (info string, live, read bool) {
	raw, ok := hcOut(20*time.Second, "get", id, "--json")
	if !ok {
		return "", false, false
	}
	v, parsed := bjParse(brTrimNL(raw))
	m, isMap := v.(jmap)
	if !parsed || !isMap {
		return "", false, false
	}
	if hcStr(m, "state") != "live" {
		return "", false, hcRecordGone(m, id)
	}
	tokens := jmap{}
	labels, _ := m["labels"].(jmap)
	for k, val := range labels {
		tokens[k], _ = val.(string)
	}
	for k, val := range labels {
		if bare := strings.TrimPrefix(k, "hw_"); bare != k {
			if _, taken := tokens[bare]; !taken {
				tokens[bare], _ = val.(string)
			}
		}
	}
	status, app := "unknown", ""
	if st, isSt := m["status"].(jmap); isSt {
		app = hcStr(st, "app")
		if s := hcStr(st, "state"); s != "" {
			status = s
		}
	} else if fg, _ := m["foreground"].(jmap); filepath.Base(hcFirstArg(fg)) == "claude" {
		// a claude that never reported a status (its startup probe went unanswered) leaves status=null for good; its own title is
		// the witness hcBriefProved already reads, so the idle gate of `hw next` and channel-send is not blind to it
		status = "idle"
		if hcWorkingTitle(hcStr(m, "title")) {
			status = "working"
		}
	}
	out, err := json.Marshal(map[string]any{"result": map[string]any{"agent": jmap{
		"agent_status": status, "cwd": hcStr(m, "cwd"), "agent": app, "turn": m["turn"], "tokens": tokens}}})
	if err != nil {
		return "", false, false
	}
	return string(out), true, true
}

// hcWaitBoundary waits for a terminal to reach one of the statuses `at`: 0 reached, 2 budget spent, 3 terminal gone,
// 6 blocked on a person. `wait` runs `hc wait` for that many seconds and returns its exit code.
//
// A failed `hc wait` returns at once, so it must not become a hot loop until the budget ends: it backs off
// a second and the next status read decides. shortcut: what each non-zero code of `hc wait` means (timeout vs error) is
// unmeasured, so none is read as an answer; the status read is the only authority.
func hcWaitBoundary(state func() (string, bool), wait func(secs int64) int, pause func(), totalMS int64, at ...string) int {
	deadline := nextEpoch() + (totalMS+999)/1000
	for {
		st, ok := state()
		if !ok {
			return 3
		}
		for _, s := range at {
			if st == s {
				return 0
			}
		}
		if st == "blocked" {
			return 6
		}
		remaining := deadline - nextEpoch()
		if remaining <= 0 {
			return 2
		}
		if remaining > 20 {
			remaining = 20
		}
		if wait(remaining) != 0 {
			pause()
		}
	}
}

// hcRunIdentity is the environment an executor carries to say WHICH run it is. An app started from an executor's shell hands it to
// every terminal it opens (QA5 G2: HW_INVOKER_PANE, HW_RUN, HW_TASK in a plain app terminal), so a brainer started there
// would read itself as that executor. A brainer starts without it.
var hcRunIdentity = []string{"HW_RUN", "HW_TASK", "HW_PROJECT", "HW_WORKDIR", "HW_ARTIFACTS", "HW_EXECUTOR_VENDOR",
	"HW_PORT_WEB", "HW_PORT_API", "HW_INVOKER_SESSION", "HW_INVOKER_VENDOR", "HW_INVOKER_ENDPOINT"}

// hcPathWord is the `PATH=` word that puts brain/bin first in an hc terminal. The core runs a program with ITS
// environment, which lacks hw and brain (MEASURED 2026-11-09, herdr-app live-cycle QA: a brainer and an executor
// in the app could dispatch only by absolute path, and a plain shell was refused as an invoker); the caller's PATH
// is the nearest thing to the one its own shell resolves claude, node and gh with.
func hcPathWord(binDir, path string) string {
	if path == "" {
		return "PATH=" + binDir
	}
	return "PATH=" + binDir + ":" + path
}

// hcRunCore is the core id a run dir recorded in its hc-core file ("" when it recorded none), and whether that core is not
// `mine` (both named, and different): the one "whose core is this run" reading, shared by the outbox and the repoint.
func hcRunCore(runDir, mine string) (core string, other bool) {
	if c, err := os.ReadFile(runDir + "/hc-core"); err == nil {
		core = strings.TrimSpace(string(c))
	}
	return core, core != "" && mine != "" && core != mine
}

// hcCoreID is the id of the herdr-core this process talks to ("" when unreadable): `<state>/herdr-core/default/core.json`, with the
// state root the core itself uses ($XDG_STATE_HOME, else ~/.local/state). An address (t2) is only unique inside one core, so
// the files and runs hw keys on one carry this beside it. shortcut: only the `default` core is read, when a second named core exists.
func hcCoreID(getenv func(string) string) string {
	root := getenv("XDG_STATE_HOME")
	if root == "" {
		root = getenv("HOME") + "/.local/state"
	}
	raw, err := os.ReadFile(root + "/herdr-core/default/core.json")
	if err != nil {
		return ""
	}
	var c struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &c) != nil {
		return ""
	}
	return c.ID
}
