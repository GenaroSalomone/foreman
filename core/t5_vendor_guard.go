package main

// Train T5, area B: the brain guard, the engram label proxy, the artifact tools.
// bin/hw: _brain_guard_applies/_dir/_claude_plugin/_present/_check/_args, _engram_label,
// _engram_proxy_bin/_mcp_bin/_mcp_claude_file, _engram_proxy_args, _artifacts_verdict, _artifacts_args.

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// vdRoot is bash's $_brain_root: the install root the shim hands down.
func (d *dsp) vdRoot() string { return d.env("HW_CORE_ROOT") }

// vdSpace is `*[[:space:]]*`.
func vdSpace(s string) bool { return strings.ContainsAny(s, " \t\n\v\f\r") }

// vdPhysDir is `$(cd -P "$p" 2>/dev/null && pwd)`.
func vdPhysDir(p string) (string, bool) {
	if !isDir(p) {
		return "", false
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", false
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		abs = r
	}
	return abs, true
}

// vdCommandV is `command -v NAME` for a name that is not a builtin: the first executable file on PATH.
func (d *dsp) vdCommandV(name string) string {
	for _, dir := range strings.Split(d.getenv("PATH"), ":") {
		if dir == "" {
			dir = "."
		}
		p := dir + "/" + name
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() && st.Mode()&0111 != 0 {
			return p
		}
	}
	return ""
}

func (d *dsp) brainGuardApplies() bool { return d.vdLane(d.proj, "brain_guard") != "0" } // # MUTATION-ANCHOR: 844-B68

func (d *dsp) brainGuardDir() string { return d.vdRoot() + "/setup/guards" }

func (d *dsp) brainGuardClaudePlugin() string { return d.brainGuardDir() + "/brain-guard" }

// brainGuardFiles are the files of the brain guard under setup/guards: the plugin, the
// guard and the three modules it imports (without any of them it fails closed on every call). Dispatch, revive and hw done's
// leak scan each require all of them.
var brainGuardFiles = []string{"brain-guard/hooks/hooks.json", // # MUTATION-ANCHOR: 844-B69
	"deny_brain_writes.py", "deny_brain_writes_shell.py", "deny_brain_writes_main.py", "deny_repo_writes.py"}

// brainGuardFilesIn reports whether every file of the brain guard is on disk under dir.
func brainGuardFilesIn(dir string) bool {
	for _, f := range brainGuardFiles {
		if !isFile(dir + "/" + f) {
			return false
		}
	}
	return true
}

// brainGuardPresent is _brain_guard_present: the files of the guard are on disk.
func (d *dsp) brainGuardPresent() bool {
	return brainGuardFilesIn(d.brainGuardDir())
}

// brainGuardCheck is _brain_guard_check: a launch refuses what a dry run warns about.
func (d *dsp) brainGuardCheck() {
	if !d.brainGuardApplies() {
		return
	}
	work := d.c.work
	cdWork := orDefaultStr(work, "/nonexistent")
	bgWork, ok := vdPhysDir(cdWork)
	if !ok {
		bgWork = work // # MUTATION-ANCHOR: 844-B70
	}
	bgBrain, ok := vdPhysDir(d.c.brain)
	if !ok {
		bgBrain = d.c.brain
	}
	if strings.HasPrefix(bgWork+"/", bgBrain+"/") { // # MUTATION-ANCHOR: 844-B71
		if d.dispatchDry { // # MUTATION-ANCHOR: 844-B72
			d.warn("work root " + work + " is inside the brain " + d.c.brain + ": this dry run goes on, and a launch would refuse — the brain guard denies every command a product executor runs in its own worktree there. Set \"work\" in projects.json outside the brain")
		} else {
			d.die("work root " + work + " is inside the brain " + d.c.brain + ": the brain guard denies every command a product executor runs in its own worktree there, so it could execute nothing. Set \"work\" in projects.json to a directory outside the brain (install.sh puts it beside it), or declare the lane \"brain_guard\": false")
		}
	}
	if vdSpace(d.vdRoot()) {
		d.die("the brain guard cannot be passed as one word: the checkout path has whitespace (" + d.vdRoot() + ")")
	}
	if d.brainGuardPresent() { // # MUTATION-ANCHOR: 844-B73
		return
	}
	if !d.dispatchDry {
		d.die("the brain guard is not on disk under " + d.brainGuardDir() + ", and a product executor is not launched without it")
	}
	d.warn("the brain guard is not on disk under " + d.brainGuardDir() + ": this dry run shows no guard, and a launch would refuse")
}

// brainGuardArgs is _brain_guard_args: the Claude half, onto the agent args.
func (d *dsp) brainGuardArgs() {
	if d.agent == "claude" && d.brainGuardApplies() && d.brainGuardPresent() {
		d.agentArgs = vdJoinArg(d.agentArgs, "--plugin-dir "+d.brainGuardClaudePlugin())
	}
}

// vdJoinArg is `${AGENT_ARGS:+$AGENT_ARGS }word`.
func vdJoinArg(args, word string) string {
	if args == "" {
		return word
	}
	return args + " " + word
}

// engramLabel is _engram_label: the label of the lane's memory, `brain` unless the lane names one.
func (d *dsp) engramLabel(proj string) string {
	return orDefaultStr(d.vdLane(proj, "engram"), "brain") // # MUTATION-ANCHOR: 844-B74
}

// engramProxyArgs is _engram_proxy_args: each vendor's way to launch `engram mcp` behind the label proxy. A dry
// run writes nothing (the claude --mcp-config file is the launch's), and Go only ever runs a dry dispatch.
func (d *dsp) engramProxyArgs() {
	d.engramProxyLine = ""
	label := d.engramLabel(d.proj)
	proxy := d.vdRoot() + "/bin/engram-label-proxy"
	eng := d.vdCommandV("engram")
	if eng == "" {
		eng = "engram"
	}
	cache := d.env("XDG_CACHE_HOME")
	if cache == "" {
		cache = d.env("HOME") + "/.cache" // # MUTATION-ANCHOR: 844-B75
	}
	file := cache + "/hw/engram-mcp-" + label + ".json"
	if label == "" || strings.Trim(label, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-") != "" { // # MUTATION-ANCHOR: 844-B76
		d.die("engram label '" + label + "' cannot travel as one word to the engram proxy")
	}
	if vdSpace(proxy + eng + file) {
		d.die("the engram proxy cannot be passed as one word: a path has whitespace (" + proxy + ", " + eng + ", " + file + ")")
	}
	if st, err := os.Stat(proxy); err != nil || st.Mode()&0111 == 0 { // # MUTATION-ANCHOR: 844-B77
		if !d.dispatchDry {
			d.die("the engram label proxy is not on disk at " + proxy + ", and an executor is not launched with an unguarded engram")
		}
		d.warn("the engram label proxy is not on disk at " + proxy + ": this dry run shows no proxy, and a launch would refuse")
		return
	}
	switch d.agent {
	case "claude":
		d.agentArgs = vdJoinArg(d.agentArgs, "--mcp-config "+file)
		d.engramProxyLine = "--mcp-config " + file // # MUTATION-ANCHOR: 844-B78
	case "pi":
		d.engramProxyLine = piEngramProxyLine // t6_pi.go: the extension registers hw_engram, nothing rides the launch line
	}
}

const vdArtifactTools = "Artifact,ArtifactComments,ArtifactData"

// vdArtifactsVerdict is _artifacts_verdict: `allow`, or the reason for deny.
func (d *dsp) vdArtifactsVerdict(lane, account string) string {
	want := d.projectArtifacts(lane)
	if want == "deny" { // # MUTATION-ANCHOR: 844-B81
		return "the lane declares artifacts: deny"
	}
	if want != account { // # MUTATION-ANCHOR: 844-B82
		return "the lane publishes only to '" + want + "', and this executor runs under '" + account + "'"
	}
	return "allow"
}

// artifactsArgs is _artifacts_args: whether the claude.ai Artifact tools stay on the executor.
func (d *dsp) artifactsArgs() {
	if d.agent != "claude" { // # MUTATION-ANCHOR: 844-B83
		d.artifactsLine = "not governed — hw removes the claude.ai Artifact tools from Claude Code only; this executor is " + d.agent
		return
	}
	v := d.vdArtifactsVerdict(d.proj, d.account)
	if v == "allow" {
		d.artifactsLine = "publish to '" + d.account + "' only (the account this executor runs under is the lane's)"
		return
	}
	d.agentArgs += " --disallowedTools " + vdArtifactTools
	d.artifactsLine = "DENIED — --disallowedTools " + vdArtifactTools + " (" + v + ")"
}

// sddRemoved is the one warning for the removed spec-driven flow (setup/decisions.md, 2026-11-16); each surface (flag, brief
// key, re-task flag, re-task brief) names itself in `what`. A warning, never a refusal: the old briefs still carry `sdd:`.
func sddRemoved(what string) string {
	return what + " — spec-driven development was removed from hw, so it is ignored"
}

// retiredBodyTokens is the one table of removed mechanisms a brief BODY may still name (briefs are written by copying older
// ones, so a retired instruction travels). The next removal adds a row here, not code.
var retiredBodyTokens = []struct {
	re   *regexp.Regexp
	what string
}{
	{regexp.MustCompile(`--sdd\b|\bsdd:`), "--sdd"},
}

// retiredBodyWarnings is one warning per brief-body line that names a retired token outside a fenced code block, with its
// line number. The frontmatter is not the body (its `sdd:` has its own warning). A warning, never a refusal.
func retiredBodyWarnings(brief string) []string {
	data, err := os.ReadFile(brief)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}
	var out []string
	fence := false
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		for _, t := range retiredBodyTokens {
			if t.re.MatchString(lines[i]) {
				out = append(out, sddRemoved("brief line "+strconv.Itoa(i+1)+" names "+t.what)+": delete it from the brief")
			}
		}
	}
	return out
}

// sandboxRemoved is the one message for the removed Seatbelt option: a dispatch that asked for confinement must not run without it.
func sandboxRemoved(what string) string {
	return what + " was removed: hw no longer runs an executor under a Seatbelt profile (setup/decisions.md, 2026-11-16). Dispatch without it. NOTHING was built."
}
