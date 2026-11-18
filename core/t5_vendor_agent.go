package main

// Train T5, area B: the permissions and the vendor's command line.
// bin/project-spaces.sh: hw_permissions_file, hw_permissions_resolve; bin/hw: _build_agent_args.

import (
	"os"
	"strings"
)

// vdSpaceCut is `tr -d '[:space:]'`.
func vdSpaceCut(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\v', '\f', '\r':
			return -1
		}
		return r
	}, s)
}

// permissionsResolve is hw_permissions_resolve: sets permissions/permissionsSrc, false (with permissionsErr) when
// the value that won is neither ask nor skip.
func (d *dsp) permissionsResolve(flag string) bool {
	v, src := flag, ""
	d.permissionsErr = ""
	cfg := d.env("XDG_CONFIG_HOME")
	if cfg == "" { // # MUTATION-ANCHOR: 844-B32
		cfg = d.env("HOME") + "/.config"
	}
	f := cfg + "/hw/permissions" // hw_permissions_file
	switch {
	case v != "":
		src = "chosen (--permissions)"
	case d.env("HW_PERMISSIONS") != "": // # MUTATION-ANCHOR: 844-B33
		v, src = d.env("HW_PERMISSIONS"), "HW_PERMISSIONS"
	case isFile(f):
		data, _ := os.ReadFile(f)
		v, src = vdSpaceCut(string(data)), "machine config "+f // # MUTATION-ANCHOR: 844-B34
	default:
		v, src = "skip", "default (recommended)"
	}
	if v == "ask" || v == "skip" { // # MUTATION-ANCHOR: 844-B35
		d.permissions, d.permissionsSrc = v, src
		return true
	}
	d.permissions, d.permissionsSrc = "skip", src
	d.permissionsErr = "permissions must be ask or skip (got: '" + v + "', from " + src + ")"
	return false
}

// vdValidEffort is the `low|medium|high|xhigh|max` pattern.
func vdValidEffort(e string) bool {
	switch e {
	case "low", "medium", "high", "xhigh", "max":
		return true
	}
	return false
}

// vdEffortAnyVendor is every level --effort takes on ANY vendor: vdValidEffort's five and pi's `off`|`minimal`
// (t6_pi.go piBuildArgs). It exists for one reading: what may follow a `:` in a model
// id. Anything else after the colon is the model's (`:batch`, `:free`, `:8b` — measured 2026-11-08 in projects.json and in
// `pi --list-models`), and refusing it would be a false refusal.
func vdEffortAnyVendor(e string) bool {
	switch e {
	case "off", "minimal":
		return true
	}
	return vdValidEffort(e)
}

// effortDeclaredDefault is the ONE rule that supplies a level nobody asked for, for every vendor: the level and the
// source to name, ok=false when hw declares none and the vendor's own default is what runs. pi is the only vendor hw
// declares one for — the roster's, by the brief's kind (pinned in setup/decisions.md, 2026-11-01) — and a lane that ever
// declares its own belongs here, beside it, not in another arm. Both the level and the words travel: piBuildArgs
// (t6_pi.go) takes the level, effortShownAndMark (t5_manifest.go) prints the source, so what runs and what the dry run
// says are one decision.
func (d *dsp) effortDeclaredDefault() (level, src string, ok bool) {
	if d.agent != "pi" {
		return "", "", false
	}
	kind := d.vdBriefKind()
	if kind == "explore" {
		return "off", "pi roster, kind explore", true
	}
	return "low", "pi roster, kind " + orDefaultStr(kind, "none"), true
}

// effortRefusal is the ONE judgment of a level for a vendor, in --effort's words, and "" when the level is the vendor's
// (or none was given). Both doors call it — the --effort flag in each vendor's arm of buildAgentArgs (and piBuildArgs), and
// the brief's `effort:` (checkBriefContract) — so a level a vendor cannot take is refused in the same words wherever it was
// written (setup/decisions.md 2026-11-08, «un nivel, una puerta»).
func effortRefusal(agent, level string) string {
	switch {
	case level == "":
		return ""
	case agent == "pi":
		switch level {
		case "off", "minimal", "low", "medium", "high", "xhigh", "max":
		default:
			return "pi --effort must be one of: off minimal low medium high xhigh max (got: " + level + ")"
		}
	default:
		if !vdValidEffort(level) { // # MUTATION-ANCHOR: 844-B44
			return "--effort must be one of: low medium high xhigh max (got: " + level + ")"
		}
	}
	return ""
}

// modelEffortSuffix reads a reasoning level written into a model id — `<model>:<level>`, pi's own CLI spelling
// (provider/id:<level>). ok=false when the id carries no colon, or carries something after it that is not a level.
func modelEffortSuffix(model string) (base, level string, ok bool) {
	i := strings.LastIndexByte(model, ':')
	if i <= 0 || i == len(model)-1 {
		return "", "", false
	}
	if !vdEffortAnyVendor(model[i+1:]) {
		return "", "", false
	}
	return model[:i], model[i+1:], true
}

// modelEffortSuffixRefusal is the ONE refusal of a level written into a model id, and "" when there is none to refuse.
// Every door that fixes a model calls it — --model and the lane default (t5.go run), the brief's `model:` (t5_brief.go),
// and the model `hw next` reads off the pane's dispatch (t4_next.go) — so the rule is one, not one per vendor. It names
// the model, the level and the exact flag to use instead (setup/decisions.md 2026-11-08: an explicit request is never
// lost, and a level written where only --effort may write one is refused, never dropped in silence).
func modelEffortSuffixRefusal(model, where string) string {
	base, level, ok := modelEffortSuffix(model)
	if !ok {
		return ""
	}
	return "model id " + model + " (" + where + ") carries the reasoning level " + level + " inside the id, but a level travels only through --effort — on every vendor, never inside a model. Pass the model without it and the level through the flag: --model " + base + " --effort " + level + ". Nothing was built or sent."
}

// buildAgentArgs is _build_agent_args: the vendor's command line, as one string of words.
func (d *dsp) buildAgentArgs() {
	d.agentArgs = ""
	d.modelPinned = ""
	perm := orDefaultStr(d.permissions, "skip")
	switch d.agent {
	case "claude":
		if perm == "skip" { // # MUTATION-ANCHOR: 844-B37
			d.agentArgs = "--dangerously-skip-permissions"
		}
		if d.model != "" {
			if strings.HasPrefix(d.model, "-") || strings.ContainsAny(d.model, " \t\r\n*?[") { // # MUTATION-ANCHOR: 844-B38
				d.die("hw cannot safely pass this Claude model as one literal argument (got: " + d.model + ")")
			}
			switch {
			case d.model == "fable" || d.model == "opus" || d.model == "sonnet" || d.model == "haiku": // # MUTATION-ANCHOR: 844-B39
			case strings.HasPrefix(d.model, "claude-"): // # MUTATION-ANCHOR: 844-B40
			case strings.Contains(d.model, "/"): // # MUTATION-ANCHOR: 844-B41
				d.die("that is a provider/model id, which is pi's form, not claude's (got: " + d.model + "). Use an alias (opus, sonnet, haiku, fable) or a full claude-* name — or launch it with --agent pi.")
			default:
				d.warn("claude model '" + d.model + "' is not recognized by hw; availability is unverified, passing it to the vendor unchanged.") // # MUTATION-ANCHOR: 844-B42
			}
			d.modelPinned = d.modelPinFor(d.model)
			d.agentArgs += " --model " + orDefaultStr(d.modelPinned, d.model) // # MUTATION-ANCHOR: 844-B43
		}
		if d.effort != "" {
			if msg := effortRefusal(d.agent, d.effort); msg != "" {
				d.die(msg)
			}
			d.agentArgs += " --effort " + d.effort // # MUTATION-ANCHOR: 844-B45
		}
	case "pi":
		d.piBuildArgs(perm) // t6_pi.go
	}
	d.agentArgs = strings.TrimPrefix(d.agentArgs, " ") // # MUTATION-ANCHOR: 844-B67
	if d.agentArgs != "" {
		d.info("agent args: " + d.agentArgs)
	}
	d.explorerResolve()
}
