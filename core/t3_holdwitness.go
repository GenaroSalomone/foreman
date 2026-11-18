package main

// A hold a plain prompt answered. `channel-send --reply-hold` removes its hold itself; a prompt that arrives any other way
// (a brainer's `hc send`, a person typing) never did, and the row stayed «ask unanswered» and every later turn end stayed
// a legal wait (measured 2026-11-17). The witness that a prompt arrived is the executor's own session transcript, which
// Claude Code writes with the origin of every user row: `human` for a prompt typed or sent to the pane, `task-notification`
// for a background job's wake, `auto-continuation` for the harness's own. A turn end that a Stop hook blocks (a ruling, a
// handback) continues the SAME turn and writes no user row. The mtime of `turns` is none of these: it moves at every turn
// end, so it retired an unanswered hold after a ruling or a background wake (review of 03e5fc229, R1/R2).

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// holdWitnessWindow is how much of the transcript's tail is read: a prompt after the hold is recent, and a row larger than
// the window is not read at all, which keeps the hold (the side that fails safe).
const holdWitnessWindow = 2 << 20

// humanPromptAfter reports whether the transcript holds a `human` user row stamped after `after`.
func humanPromptAfter(transcript string, after time.Time) bool {
	f, err := os.Open(transcript)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return false
	}
	start := int64(0)
	if st.Size() > holdWitnessWindow {
		start = st.Size() - holdWitnessWindow
	}
	buf := make([]byte, st.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return false
	}
	if start > 0 { // the first line of a tail is cut
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			buf = buf[i+1:]
		}
	}
	lines := bytes.Split(buf, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		// the cheap filter before a parse; an older Claude Code recorded a Stop hook's block as a user row
		if !bytes.Contains(lines[i], []byte(`"type":"user"`)) || bytes.Contains(lines[i], []byte("Stop hook feedback")) {
			continue
		}
		var row struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			IsMeta    bool   `json:"isMeta"`
			Origin    struct {
				Kind string `json:"kind"`
			} `json:"origin"`
		}
		if json.Unmarshal(lines[i], &row) != nil || row.Type != "user" || row.IsMeta || row.Origin.Kind != "human" {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, row.Timestamp)
		if err != nil {
			continue
		}
		return at.After(after) // rows run forward in time: the newest human row decides
	}
	return false
}

// retireAnsweredHolds removes a valid `pending-reply-N` hold that a human prompt followed. `payload` is the hook's input,
// which names the session's transcript. Called at the turn end and at post-tool, the first boundary of the answered turn.
func (t *t3) retireAnsweredHolds(taskDir, payload string) {
	ents, _ := os.ReadDir(taskDir)
	transcript := ""
	for _, e := range ents {
		if !strings.HasPrefix(e.Name(), "pending-reply-") {
			continue
		}
		st, err := e.Info()
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		if transcript == "" {
			if transcript = jqRawOr(payload, "", "transcript_path"); transcript == "" {
				return // no session transcript to witness a prompt: the hold stays
			}
		}
		file := taskDir + "/" + e.Name()
		// the cheap witness first, the validator (a python process) only when it matters
		if !humanPromptAfter(transcript, st.ModTime()) || !t.replyFactsValid(file) {
			continue
		}
		if t.fs.readonly { // a shadow run records its plan and touches nothing else
			t.fs.remove(file)
			continue
		}
		facts := map[string]string{}
		if body, err := os.ReadFile(file); err == nil {
			for _, line := range strings.Split(string(body), "\n") {
				if k, v, ok := strings.Cut(line, "="); ok {
					facts[k] = v
				}
			}
		}
		hcTarget := ""
		if facts["route"] == "hc" {
			hcTarget = facts["target"]
		}
		consumeHold(file, "HOLD-RETIRED", facts["logical_id"], "a prompt reached the pane after the hold was written: the ask or challenge is treated as answered", hcTarget)
	}
}

// consumeHold is the one place a hold ends: removed, recorded in the transcript, and the hc terminal's ask label set to
// answered. The reply route (clearReplyHold) and the retirement above both end here; when the retirement lost the race to
// the reply it records nothing, and the reply records its own text. True when this call removed the file.
func consumeHold(file, kind, envelope, text, hcTarget string) bool {
	removed := os.Remove(file) == nil
	if !removed && kind == "HOLD-RETIRED" {
		return false
	}
	if envelope == "" {
		envelope = "-"
	}
	csTranscriptAppend(filepath.Dir(file), kind, envelope, text)
	if hcTarget != "" {
		hcLabelAsked(hcTarget, "answered")
	}
	return removed
}
