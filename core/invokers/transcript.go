package invokers

import (
	"bufio"
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// TranscriptProof is what an executor's own transcript (and its subagents') says it invoked: the names of its
// Skill tool_uses and the subagent_type of its Agent/Task tool_uses (claude), or the skills it read SKILL.md for and
// the agents it launched with the subagent/Agent tool (pi), from the moment the task's receipt line was written.
type TranscriptProof struct {
	Names  map[string]bool
	Groups [][]string // names launched TOGETHER by one call (pi: one toolCall; claude: one assistant message)
	Vendor string     // the vendor whose session was read; the refusal names that vendor's mechanism
	// JudgeRounds: one entry per call that launched a Judgment Day judge (jd-judge-*), in the order they ran, with how
	// many judge runs the call launched and the text its results carried. JudgmentDayProtocolGate reads the protocol
	// (budget, frozen ledger) off these.
	JudgeRounds []JudgeRound
}

// JudgeRound is one launching call of Judgment Day judges and what came back.
type JudgeRound struct {
	Runs   int    // judges launched by the call (2 for a round, 1 for a coverage relaunch)
	Text   string // every result the call returned, concatenated
	Prompt string // what the call was given (a re-judgment's prompt carries the frozen ledger and the fix delta)
}

// isJudgeName is a Judgment Day judge (jd-judge-a, jd-judge-b).
func isJudgeName(n string) bool { return strings.HasPrefix(n, "jd-judge-") }

// Together is true when ONE recorded call launched every name it is given: the
// two blind judges of a Judgment Day must run in the same parallel call, and two
// calls one after the other cost the same minutes twice (measured 2026-11-04,
// no-notifications: ~13 min). A judge proved by a different call does not count.
func (p *TranscriptProof) Together(names ...string) bool {
	if p == nil || len(names) == 0 {
		return false
	}
	for _, g := range p.Groups {
		have := map[string]bool{}
		for _, n := range g {
			have[n] = true
		}
		all := true
		for _, n := range names {
			if n == "" {
				continue
			}
			if !have[n] {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// Has is true when name ran: a namespaced `plugin:skill` must be equal to one of the names; a bare one must be the
// skill part of a namespaced name (a receipt written before this gate named bare skills). Never a substring:
// `pstack:thermo-nuclear-code-quality-review-subagent` is not `thermo-nuclear-code-quality-review`.
func (p *TranscriptProof) Has(name string) bool {
	if p == nil {
		return false
	}
	if strings.Contains(name, ":") {
		return p.Names[name] // # MUTATION-ANCHOR: 1150-G12
	}
	for n := range p.Names {
		if i := strings.LastIndex(n, ":"); i >= 0 && n[i+1:] == name {
			return true
		}
	}
	return p.Names[name]
}

// transcriptFiles is the executor's jsonl and its subagents', found by session id under every Claude config dir
// the account may have used (CLAUDE_CONFIG_DIR first, then ~/.claude*): a session id is unique, so the encoded
// cwd the path carries is not needed. Read-only.
func (c *Ctx) transcriptFiles(sid string) []string {
	if sid == "" || strings.ContainsAny(sid, "/*?[\\") {
		return nil
	}
	var roots []string
	if d := c.get("CLAUDE_CONFIG_DIR"); d != "" {
		roots = append(roots, d)
	}
	if h := c.get("HOME"); h != "" {
		m, _ := filepath.Glob(filepath.Join(h, ".claude*"))
		sort.Strings(m)
		roots = append(roots, m...)
	}
	for _, r := range roots {
		main, _ := filepath.Glob(filepath.Join(r, "projects", "*", sid+".jsonl"))
		if len(main) == 0 {
			continue
		}
		files := []string{main[0]}
		subs, _ := filepath.Glob(filepath.Join(strings.TrimSuffix(main[0], ".jsonl"), "subagents", "*.jsonl")) // # MUTATION-ANCHOR: 1150-G13
		sort.Strings(subs)
		return append(files, subs...)
	}
	return nil
}

// transcriptExtractor reads one line of one vendor's transcript and adds to p the names that line proves. A vendor
// whose call is answered on a later line keeps that state and drops what was never answered in flush.
type transcriptExtractor interface {
	line(raw []byte, p *TranscriptProof)
	flush(p *TranscriptProof)
}

// vendorTranscript is one vendor's two halves: where its session files live, and how one of its lines is read. What
// counts as a proof belongs to the reader, not here.
type vendorTranscript struct {
	files     func(*Ctx, string) []string
	extractor func() transcriptExtractor
}

var transcriptVendors = map[string]vendorTranscript{
	"claude": {files: (*Ctx).transcriptFiles, extractor: func() transcriptExtractor { return &claudeExtractor{} }},
	"pi": {files: (*Ctx).piTranscriptFiles, extractor: func() transcriptExtractor {
		return &piExtractor{pending: map[string][]string{}}
	}},
}

// ReadTranscriptProof reads the transcripts of the session the receipt names. It returns nil and why when the
// transcript cannot be reached (a vendor with no reader, no session, no file, an unreadable one): the caller says
// so, it never passes silently.
func (c *Ctx) ReadTranscriptProof(receipt string, since time.Time) (*TranscriptProof, string) {
	vendor, sid := receiptValue(receipt, "session_vendor"), receiptValue(receipt, "session_id")
	v, ok := transcriptVendors[vendor]
	if !ok {
		if vendor == "" {
			return nil, "no transcript reader for this executor: the receipt has no session_vendor"
		}
		return nil, "no transcript reader for vendor " + vendor
	}
	if sid == "" || strings.HasPrefix(sid, "none") {
		return nil, "the receipt has no session id"
	}
	files := v.files(c, sid)
	if len(files) == 0 {
		return nil, "no transcript file for session " + sid
	}
	p, err := scanTranscripts(files, since, v.extractor)
	if err != nil {
		return nil, "the transcript of session " + sid + " could not be read: " + err.Error()
	}
	p.Vendor = vendor
	return p, ""
}

// scanTranscripts is the one loop both vendors share: open, buffer, the `since` cut, the reader's own line, and the
// scanner's own error, which is returned rather than read as an empty proof.
func scanTranscripts(files []string, since time.Time, newExtractor func() transcriptExtractor) (*TranscriptProof, error) {
	p := &TranscriptProof{Names: map[string]bool{}}
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			return p, err
		}
		ex := newExtractor()
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 1<<20), 1<<28)
		for sc.Scan() {
			raw := sc.Bytes()
			if !since.IsZero() {
				var env struct {
					Timestamp string `json:"timestamp"`
				}
				if json.Unmarshal(raw, &env) == nil {
					if ts, err := time.Parse(time.RFC3339Nano, env.Timestamp); err == nil && ts.Before(since) { // # MUTATION-ANCHOR: 1150-G15
						continue
					}
				}
			}
			ex.line(raw, p)
		}
		err = sc.Err()
		fh.Close()
		if err != nil {
			return p, err
		}
		ex.flush(p)
	}
	return p, nil
}

// looseBool reads a JSON flag the way Claude Code records it: the string "true" (367 of 367 run_in_background flags in 20 days of
// transcripts) as well as the bool. Anything else is false and never an error, so one odd field cannot drop the whole line.
type looseBool bool

func (b *looseBool) UnmarshalJSON(raw []byte) error {
	*b = string(raw) == "true" || string(raw) == `"true"`
	return nil
}

// backgroundAgent is true for an Agent call that is not a judge and was launched in the background: it answers its tool_use only with
// "Async agent launched", so the launch proves the start and not the run (a review agent such as pstack:comment-sicko launched this way
// was the 2026-11-09 friction). The run is proved when its <task-notification> reports it completed (agentNotification): a background
// agent that never completes proves nothing. The flag is only a hint, since most async launches carry none (632 of 643 in 40
// transcripts): a call without it is taken back when its tool_result says "Async agent launched" (asyncResult), and proved the same way.
// A judge's background round is read from its notification too (judgeNotification).
func backgroundAgent(subagentType string, runInBackground looseBool) bool {
	return bool(runInBackground) && !isJudgeName(subagentType)
}

// asyncLaunched is the text a background Agent call is answered with, in place of the agent's report.
const asyncLaunched = "Async agent launched"

// agentLaunch is a recorded foreground Agent call, kept to take back when its tool_result says it ran async.
type agentLaunch struct {
	name  string
	group int // index in TranscriptProof.Groups
}

// claudeExtractor reads a claude jsonl line: a `Skill` tool_use proves the skill it names, and an `Agent`/`Task`
// tool_use proves its `subagent_type`. A call is recorded on one line, so every assistant tool_use counts.
type claudeExtractor struct {
	lastID    string // message.id of the last assistant line that launched agents
	lastGroup int    // its index in TranscriptProof.Groups

	judgeMsg   string          // message.id of the last assistant message that launched a judge
	judgeRound int             // its index in TranscriptProof.JudgeRounds
	judgeByUse map[string]int  // tool_use id of a judge call -> its round, to put its result in it
	judgeSeen  map[string]bool // task-notification results already added: Claude Code repeats one under several line types

	launches map[string]agentLaunch // tool_use id of a non-judge Agent call still waiting for its result -> what to retract if it says async
	pending  map[string]string      // tool_use id of a background non-judge call -> its agent name, until its notification says completed
}

// flush puts the agent names that survived into Names: Groups holds exactly the foreground calls left after the retractions.
func (e *claudeExtractor) flush(p *TranscriptProof) {
	for _, g := range p.Groups {
		for _, n := range g {
			p.Names[n] = true
		}
	}
}

func (e *claudeExtractor) line(raw []byte, p *TranscriptProof) {
	var line struct {
		Type    string `json:"type"`
		Message struct {
			ID      string          `json:"id"`
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(raw, &line) == nil && line.Type == "user" {
		e.asyncResult(line.Message.Content, p)
		e.judgeResult(raw, p)
		return
	}
	if line.Type == "attachment" {
		e.judgeAttachment(raw, p)
		return
	}
	if json.Unmarshal(raw, &line) != nil || line.Type != "assistant" {
		return
	}
	var blocks []struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Name  string `json:"name"`
		Input struct {
			Skill           string    `json:"skill"`
			SubagentType    string    `json:"subagent_type"`
			Prompt          string    `json:"prompt"`
			RunInBackground looseBool `json:"run_in_background"`
		} `json:"input"`
	}
	if json.Unmarshal(line.Message.Content, &blocks) != nil {
		return
	}
	for _, b := range blocks {
		if b.Type == "tool_use" && b.Name == "Skill" && b.Input.Skill != "" {
			p.Names[b.Input.Skill] = true
		}
	}
	// A judge launch: one assistant message is one round (two judges in a parallel call, or one in a relaunch), and its
	// results arrive on later `user` lines under each tool_use id.
	for _, b := range blocks {
		if b.Type != "tool_use" || (b.Name != "Agent" && b.Name != "Task") || !isJudgeName(b.Input.SubagentType) {
			continue
		}
		if e.judgeByUse == nil {
			e.judgeByUse = map[string]int{}
		}
		if line.Message.ID == "" || line.Message.ID != e.judgeMsg || e.judgeRound >= len(p.JudgeRounds) {
			p.JudgeRounds = append(p.JudgeRounds, JudgeRound{})
			e.judgeMsg, e.judgeRound = line.Message.ID, len(p.JudgeRounds)-1
		}
		p.JudgeRounds[e.judgeRound].Runs++
		p.JudgeRounds[e.judgeRound].Prompt += "\n" + b.Input.Prompt
		if b.ID != "" {
			e.judgeByUse[b.ID] = e.judgeRound
		}
	}
	// One assistant message with two Agent/Task tool_uses is ONE parallel call: that is the shape a
	// Judgment Day needs, and the reason the group is kept (1241's sibling, retro-fixes-rc3 item 7).
	// Claude Code writes each tool_use block of ONE parallel assistant message as its own jsonl line, all sharing
	// message.id (measured: two Agent calls of one message on lines 388 and 390 of a real transcript), so the
	// group is the message, not the line. A line with no id is its own group.
	gi, merge := len(p.Groups), line.Message.ID != "" && line.Message.ID == e.lastID && e.lastGroup < len(p.Groups)
	if merge {
		gi = e.lastGroup
	}
	var group []string
	for _, b := range blocks {
		if b.Type != "tool_use" || (b.Name != "Agent" && b.Name != "Task") || b.Input.SubagentType == "" {
			continue
		}
		if backgroundAgent(b.Input.SubagentType, b.Input.RunInBackground) {
			e.await(b.ID, b.Input.SubagentType) // proved by its completed notification, not by the launch
			continue
		}
		group = append(group, b.Input.SubagentType)
		if b.ID != "" && !isJudgeName(b.Input.SubagentType) {
			if e.launches == nil {
				e.launches = map[string]agentLaunch{}
			}
			e.launches[b.ID] = agentLaunch{name: b.Input.SubagentType, group: gi}
		}
	}
	if len(group) == 0 {
		return
	}
	if merge {
		p.Groups[gi] = append(p.Groups[gi], group...)
		return
	}
	p.Groups = append(p.Groups, group)
	e.lastID, e.lastGroup = line.Message.ID, gi
}

// asyncResult takes back a foreground-looking Agent call whose tool_result is "Async agent launched": Claude Code runs most agents
// async without the input flag, and such a call proves the start, not the run. A judge is never taken back (judgeNotification).
func (e *claudeExtractor) asyncResult(content json.RawMessage, p *TranscriptProof) {
	if len(e.launches) == 0 {
		return
	}
	var blocks []struct {
		Type      string          `json:"type"`
		ToolUseID string          `json:"tool_use_id"`
		Content   json.RawMessage `json:"content"`
	}
	if json.Unmarshal(content, &blocks) != nil {
		return
	}
	for _, b := range blocks {
		l, ok := e.launches[b.ToolUseID]
		if b.Type != "tool_result" || !ok {
			continue
		}
		delete(e.launches, b.ToolUseID) // answered: only a result that says async takes the call back, and the map drains
		if !strings.HasPrefix(textOfContent(b.Content), asyncLaunched) {
			continue
		}
		if i := slices.Index(p.Groups[l.group], l.name); i >= 0 {
			p.Groups[l.group] = slices.Delete(p.Groups[l.group], i, i+1)
		}
		e.await(b.ToolUseID, l.name)
	}
}

// await keeps a background non-judge call until its notification says it completed.
func (e *claudeExtractor) await(id, name string) {
	if id == "" {
		return
	}
	if e.pending == nil {
		e.pending = map[string]string{}
	}
	e.pending[id] = name
}

// agentNotification proves a background non-judge agent from the <task-notification> that reports it completed: the run, which its
// "Async agent launched" answer did not show. It joins the proof as a group of its own, since it did not run in the foreground
// beside its siblings. Any other status (failed, killed) proves nothing.
func (e *claudeExtractor) agentNotification(text string, p *TranscriptProof) {
	if len(e.pending) == 0 || !strings.Contains(text, "<task-notification>") {
		return
	}
	use := notifyUseRe.FindStringSubmatch(text)
	if use == nil || !strings.Contains(text, "<status>completed</status>") {
		return
	}
	name, ok := e.pending[use[1]]
	if !ok {
		return
	}
	delete(e.pending, use[1]) // Claude Code repeats a notification under several line types
	p.Groups = append(p.Groups, []string{name})
}

// notification reads a <task-notification> for both kinds of background agent: a judge's round and any other agent's proof.
func (e *claudeExtractor) notification(text string, p *TranscriptProof) {
	e.agentNotification(text, p)
	e.judgeNotification(text, p)
}

// judgeResult adds to its round the text of a `user` line's tool_result that answers a judge call.
func (e *claudeExtractor) judgeResult(raw []byte, p *TranscriptProof) {
	if len(e.judgeByUse) == 0 && len(e.pending) == 0 {
		return
	}
	var line struct {
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(raw, &line) != nil {
		return
	}
	var blocks []struct {
		Type      string          `json:"type"`
		ToolUseID string          `json:"tool_use_id"`
		IsError   bool            `json:"is_error"`
		Content   json.RawMessage `json:"content"`
	}
	var notice string
	if json.Unmarshal(line.Message.Content, &notice) == nil { // a background judge: its answer is a task-notification, not a tool_result
		e.notification(notice, p)
		return
	}
	if json.Unmarshal(line.Message.Content, &blocks) != nil {
		return
	}
	for _, b := range blocks {
		round, ok := e.judgeByUse[b.ToolUseID]
		if b.Type != "tool_result" || !ok || b.IsError || round >= len(p.JudgeRounds) {
			continue
		}
		p.JudgeRounds[round].Text += "\n" + textOfContent(b.Content)
	}
}

// judgeAttachment reads the same notification when Claude Code delivers it as a queued_command attachment.
func (e *claudeExtractor) judgeAttachment(raw []byte, p *TranscriptProof) {
	var line struct {
		Attachment struct {
			Type   string `json:"type"`
			Prompt string `json:"prompt"`
		} `json:"attachment"`
	}
	if json.Unmarshal(raw, &line) == nil && line.Attachment.Type == "queued_command" {
		e.notification(line.Attachment.Prompt, p)
	}
}

var (
	notifyUseRe    = regexp.MustCompile(`<tool-use-id>([^<]+)</tool-use-id>`)
	notifyResultRe = regexp.MustCompile(`(?s)<result>(.*)</result>`)
)

// judgeNotification adds to its round the result of a <task-notification> that answers a background judge call: an Agent
// launched with run_in_background answers its tool_use only with "Async agent launched", and its findings arrive later as
// this text (measured: the first `hw review --at` of a product build, 2026-11-12; fixture testdata/transcripts/claude_bg_judges.jsonl). The result is XML-escaped.
func (e *claudeExtractor) judgeNotification(text string, p *TranscriptProof) {
	if len(e.judgeByUse) == 0 || !strings.Contains(text, "<task-notification>") { // # MUTATION-ANCHOR: 1250-G04
		return
	}
	use, res := notifyUseRe.FindStringSubmatch(text), notifyResultRe.FindStringSubmatch(text)
	if use == nil || res == nil {
		return
	}
	round, ok := e.judgeByUse[use[1]]
	if !ok || round >= len(p.JudgeRounds) {
		return
	}
	key := use[1] + "\x00" + res[1]
	if e.judgeSeen[key] { // # MUTATION-ANCHOR: 1250-G05
		return
	}
	if e.judgeSeen == nil {
		e.judgeSeen = map[string]bool{}
	}
	e.judgeSeen[key] = true
	p.JudgeRounds[round].Text += "\n" + html.UnescapeString(res[1])
}

// textOfContent is the text of a tool_result's content: a string, or a list of {type:text,text}.
func textOfContent(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var out []string
	for _, p := range parts {
		out = append(out, p.Text)
	}
	return strings.Join(out, "\n")
}

// ProofFromFiles scans claude jsonl transcripts for assistant tool_use blocks named Skill, Agent or Task.
func ProofFromFiles(files []string, since time.Time) *TranscriptProof {
	p, _ := scanTranscripts(files, since, func() transcriptExtractor { return &claudeExtractor{} })
	return p
}

// receiptAt is the `at` of the last receipt line with this key: when the task's closing was written. Zero when absent.
func receiptAt(path, key string) time.Time {
	b, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	var at string
	for _, l := range strings.Split(string(b), "\n") {
		var r struct {
			Key string `json:"key"`
			At  string `json:"at"`
		}
		if json.Unmarshal([]byte(l), &r) == nil && r.Key == key {
			at = r.At
		}
	}
	t, _ := time.Parse(time.RFC3339Nano, at)
	return t
}
