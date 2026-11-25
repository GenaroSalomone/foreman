package main

// `hw-core json <op>`: the few things the shell side of foreman asked jq for, so jq is not a dependency (deps-to-three
// B3). It reads on jsonjq.go, so a number or a string comes back the way jq 1.7 printed it. There is no expression
// language here: each op is one question a caller asked, and a caller that needs another adds an op.
//
//	get [-f FILE] PATH...        the first PATH (`.a.b`, or `.a[]` for every element) that is not null, as `jq -r` prints it;
//	                             exit 1 and nothing when none is. stdin when there is no -f.
//	entries [-f FILE] PATH [F]   the keys of the object at PATH, one per line; with F only the members whose `.F` is
//	                             truthy, as `key<TAB>F`.
//	set [-c] [-f FILE] P=STR...  the document with the string STR written at each dotted path P (objects are made on the
//	                             way), pretty-printed like jq, or on one line with -c.
//	usage-tee ACCOUNT            statusline-usage-tee's two answers in one call, one line each: the record of the account's
//	                             rate limits (empty when the status-line JSON on stdin has no rate_limits), and its
//	                             transcript_path (empty when it has none).
//	ctx                          statusline-seg's `pct<TAB>tokens<TAB>size` of the context window, or nothing.
//	lane-env BRAIN               the lane table as shell assignments for project-spaces.sh's lane_config_load to eval.
//
// A document that does not parse is exit 2 with the reason on stderr (jq's own).

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func runJSON(args []string, stdout, stderr io.Writer, getenv func(string) string, stdin io.Reader) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "hw-core json: no op (get, entries, set, usage-tee, ctx, lane-env)")
		return 64
	}
	op, rest := args[0], args[1:]
	compact, file := false, ""
	for len(rest) > 0 && (rest[0] == "-c" || rest[0] == "-f") {
		if rest[0] == "-c" {
			compact, rest = true, rest[1:]
		} else if len(rest) > 1 {
			file, rest = rest[1], rest[2:]
		} else {
			fmt.Fprintln(stderr, "hw-core json: -f needs a file")
			return 64
		}
	}
	if op == "lane-env" {
		if len(rest) != 1 {
			fmt.Fprintln(stderr, "usage: hw-core json lane-env BRAIN")
			return 64
		}
		return jsonLaneEnv(rest[0], stdout, stderr, getenv)
	}
	var data []byte
	var err error
	if file != "" {
		data, err = os.ReadFile(file)
	} else {
		data, err = io.ReadAll(stdin)
	}
	if err != nil {
		fmt.Fprintf(stderr, "hw-core json: %v\n", err)
		return 2
	}
	vals, perr := parseStream(data)
	if perr != nil {
		fmt.Fprintf(stderr, "jq: error: %v\n", perr)
		return 2
	}
	doc := jnull
	if len(vals) > 0 {
		doc = vals[0]
	}
	switch op {
	case "get":
		return jsonGet(doc, rest, stdout)
	case "entries":
		return jsonEntries(doc, rest, stdout)
	case "set":
		return jsonSet(doc, rest, compact, stdout, stderr)
	case "usage-tee":
		if len(rest) != 1 {
			fmt.Fprintln(stderr, "usage: hw-core json usage-tee ACCOUNT")
			return 64
		}
		return jsonUsageTee(doc, rest[0], stdout, time.Now())
	case "ctx":
		return jsonCtx(doc, stdout)
	}
	fmt.Fprintf(stderr, "hw-core json: %s is not an op (get, entries, set, usage-tee, ctx, lane-env)\n", op)
	return 64
}

// walk is `.a.b.c` on v: a missing member, or a null on the way, is null; indexing anything but an object is jq's error (nil).
func walk(v *jval, path string) *jval {
	for _, k := range strings.Split(strings.TrimPrefix(path, "."), ".") {
		if k == "" {
			continue
		}
		var ok bool
		if v, ok = v.get(k); !ok {
			return nil
		}
	}
	return v
}

func jsonGet(doc *jval, paths []string, out io.Writer) int {
	for _, p := range paths {
		each := strings.HasSuffix(p, "[]")
		v := walk(doc, strings.TrimSuffix(p, "[]"))
		if v == nil || v.kind == vNull {
			continue
		}
		if each && v.kind == vArr {
			for _, e := range v.arr {
				fmt.Fprintln(out, e.raw())
			}
		} else {
			fmt.Fprintln(out, v.raw())
		}
		return 0
	}
	return 1
}

func jsonEntries(doc *jval, args []string, out io.Writer) int {
	if len(args) == 0 {
		return 64
	}
	o := walk(doc, args[0])
	if o == nil || o.kind != vObj {
		return 1
	}
	for _, m := range o.obj {
		if len(args) == 1 {
			fmt.Fprintln(out, m.k)
		} else if f := field(m.v, args[1]); f != nil && f.truthy() {
			fmt.Fprintf(out, "%s\t%s\n", m.k, f.raw())
		}
	}
	return 0
}

func jsonSet(doc *jval, args []string, compact bool, out, errw io.Writer) int {
	for _, a := range args {
		path, val, ok := strings.Cut(a, "=")
		if !ok {
			fmt.Fprintf(errw, "hw-core json set: %q is not PATH=VALUE\n", a)
			return 64
		}
		keys := strings.Split(strings.Trim(path, "."), ".")
		cur := doc
		for i, k := range keys {
			if cur.kind != vObj {
				fmt.Fprintf(errw, "jq: error: Cannot index %s with \"%s\"\n", kindName(cur), k)
				return 5
			}
			var m *member
			for j := range cur.obj {
				if cur.obj[j].k == k {
					m = &cur.obj[j]
				}
			}
			if m == nil {
				cur.obj = append(cur.obj, member{k: k, v: jnull})
				m = &cur.obj[len(cur.obj)-1]
			}
			if i == len(keys)-1 {
				m.v = jstr(val)
			} else if m.v.kind == vNull {
				m.v = &jval{kind: vObj}
			}
			cur = m.v
		}
	}
	if compact {
		fmt.Fprintln(out, doc.toJSON())
	} else {
		var sb strings.Builder
		doc.writePretty(&sb, 0)
		fmt.Fprintln(out, sb.String())
	}
	return 0
}

// writePretty is jq's default layout: two spaces, `"k": v`, and `[]` / `{}` for an empty one.
func (v *jval) writePretty(sb *strings.Builder, depth int) {
	pad := strings.Repeat("  ", depth+1)
	switch {
	case v.kind == vArr && len(v.arr) > 0:
		sb.WriteString("[\n")
		for i, e := range v.arr {
			sb.WriteString(pad)
			e.writePretty(sb, depth+1)
			if i < len(v.arr)-1 {
				sb.WriteByte(',')
			}
			sb.WriteByte('\n')
		}
		sb.WriteString(strings.Repeat("  ", depth) + "]")
	case v.kind == vObj && len(v.obj) > 0:
		sb.WriteString("{\n")
		for i, m := range v.obj {
			sb.WriteString(pad)
			writeJSONString(sb, m.k)
			sb.WriteString(": ")
			m.v.writePretty(sb, depth+1)
			if i < len(v.obj)-1 {
				sb.WriteByte(',')
			}
			sb.WriteByte('\n')
		}
		sb.WriteString(strings.Repeat("  ", depth) + "}")
	default:
		v.write(sb)
	}
}

// jsonUsageTee is statusline-usage-tee's `select(.rate_limits != null) | {account, written_at, rate_limits, model, cwd}`, then the
// transcript_path: both in one call, because every status-line refresh pays this process's start-up once.
func jsonUsageTee(doc *jval, acct string, out io.Writer, now time.Time) int {
	rec := ""
	if rl := walk(doc, ".rate_limits"); rl != nil && rl.kind != vNull {
		first := func(paths ...string) *jval {
			for _, p := range paths {
				if v := walk(doc, p); v != nil && v.truthy() {
					return v
				}
			}
			return jnull
		}
		rec = (&jval{kind: vObj, obj: []member{
			{"account", jstr(acct)},
			{"written_at", &jval{kind: vNum, s: strconv.FormatInt(now.Unix(), 10)}},
			{"rate_limits", rl},
			{"model", first(".model.id", ".model.display_name")},
			{"cwd", first(".workspace.current_dir", ".cwd")},
		}}).toJSON()
	}
	tp := ""
	if v := walk(doc, ".transcript_path"); v != nil && v.kind != vNull {
		tp = v.raw()
	}
	fmt.Fprintf(out, "%s\n%s\n", rec, tp)
	return 0
}

// jsonCtx is statusline-seg ctx's jq program: the context window as a percentage, the tokens in it and its size.
func jsonCtx(doc *jval, out io.Writer) int {
	w := orObj(walk(doc, ".context_window"))
	num := func(v *jval) (float64, bool) {
		if v == nil || v.kind != vNum {
			return 0, false
		}
		f, err := strconv.ParseFloat(v.s, 64)
		return f, err == nil
	}
	size := 0.0
	if s := walk(w, ".context_window_size"); s != nil && s.truthy() {
		f, ok := num(s)
		if !ok {
			return 1
		}
		size = f
	}
	used, haveUsed := num(walk(w, ".used_percentage"))
	u := walk(w, ".current_usage")
	var tok float64
	haveTok := true
	switch {
	case u != nil && u.kind == vObj:
		for _, k := range []string{"input_tokens", "cache_creation_input_tokens", "cache_read_input_tokens"} {
			if f, ok := num(walk(u, "."+k)); ok {
				tok += f
			}
		}
	case u != nil && u.kind == vNum:
		tok, _ = num(u)
	case haveUsed && size > 0:
		tok = used * size / 100
	default:
		haveTok = false
	}
	var pct float64
	switch {
	case haveUsed:
		pct = used
	case haveTok && size > 0:
		pct = tok / size * 100
	default:
		return 1
	}
	if size <= 0 {
		return 1
	}
	f := func(x float64) string { return strconv.FormatFloat(math.Floor(x), 'f', -1, 64) }
	fmt.Fprintf(out, "%s\t%s\t%s\n", f(pct), f(tok), f(size))
	return 0
}

func orObj(v *jval) *jval {
	if v == nil || !v.truthy() {
		return &jval{kind: vObj}
	}
	return v
}

// ── lane-env ────────────────────────────────────────────────────────────────

// shq is jq's @sh on a string.
func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func laneKey(s string) string { return strings.ReplaceAll(s, "-", "_") }

func jsonLaneEnv(brain string, out, errw io.Writer, getenv func(string) string) int {
	file := getenv("HW_PROJECTS_JSON")
	if file == "" {
		file = filepath.Join(brain, "projects.json")
	}
	home := getenv("HOME")
	t, errs := loadTable(file, home)
	if t == nil {
		for _, l := range errs {
			fmt.Fprintln(errw, l)
		}
		return 1
	}
	data, _ := os.ReadFile(file)
	vals, _ := parseStream(data)
	lanes := field(vals[0], "lanes")
	var sb strings.Builder
	put := func(name, v string) { sb.WriteString(name + "=" + shq(v) + "\n") }
	put("HW_LANES", strings.Join(t.names, " "))
	put("HW_LANES_SURVEY", strings.Join(t.survey, "\n"))
	put("_LC_WORK_DEFAULT", t.work)
	put("_LC_RETENTION_ARTIFACT_DAYS", t.retArtifact)
	put("_LC_RETENTION_BACKUP_DAYS", t.retBackup)
	put("_LC_RETENTION_BUILD_DAYS", t.retBuild)
	put("_LC_OPERATOR_DEFAULT", t.operator)
	for _, m := range lanes.obj {
		v, l := m.v, laneKey(m.k)
		for _, kind := range []string{"hw", "brain"} {
			for _, a := range strList(field(v, kind+"_aliases")) {
				put("_LA_"+kind+"__"+laneKey(a), m.k)
			}
		}
		str := func(o *jval, k, def string) string { return orDefault(field(o, k), def) }
		path := func(o *jval, k string) string { return expandHome(str(o, k, ""), home) }
		flag := func(o *jval, k string) string {
			if f := field(o, k); f != nil && f.kind == vBool && f.b {
				return "1"
			}
			return ""
		}
		on := func(k string) string { // sweep, brain_guard: "1" unless the table says false
			if f := field(v, k); f != nil && f.kind == vBool && !f.b {
				return "0"
			}
			return "1"
		}
		pairs := func(o *jval, f func(m member) string) string {
			var ps []string
			if o != nil && o.kind == vObj {
				for _, e := range o.obj {
					ps = append(ps, f(e))
				}
			}
			return strings.Join(ps, " ")
		}
		dep, db, dev, reap, floor := field(v, "deps"), field(v, "db"), field(v, "devserver"), field(v, "reap"), field(v, "model_floor")
		for _, e := range [][2]string{
			{"space", str(v, "space", "")}, {"engram", str(v, "engram", "brain")}, {"org", str(v, "org", "")},
			{"vendor", str(v, "vendor", "claude")}, {"model", str(v, "model", "")},
			{"model_floor", str(floor, "tier", "")}, {"model_floor_accepts", joinStrs(field(floor, "accepts"), " ")},
			{"model_pins", pairs(field(v, "model_pins"), func(e member) string { return e.k + "=" + e.v.tostring() })},
			{"subagent_model", str(v, "subagent_model", "inherit")},
			{"pi_subagent_models", pairs(field(v, "pi_subagent_models"), func(e member) string {
				return e.k + "=" + str(e.v, "model", "null") + "|" + str(e.v, "thinking", "inherit")
			})},
			{"requested_by", str(v, "requested_by", "")}, {"suite_lock", str(v, "suite_lock", "")},
			{"review_default", joinStrs(field(v, "review_default"), ",")}, {"review_post", str(v, "review_post", "none")},
			{"ui_paths", joinStrs(field(v, "ui_paths"), ",")},
			{"account", str(v, "account", "default")},
			{"artifacts", orDefault(field(v, "artifacts"), str(v, "account", "default"))},
			{"repoless", flag(v, "repoless")}, {"product_repo", flag(v, "product_repo")},
			{"checkout", path(v, "checkout")}, {"checkout_var", str(v, "checkout_var", "")},
			{"base", str(v, "base", "")}, {"base_ref_prefix", str(v, "base_ref_prefix", "origin/")},
			{"branch", str(v, "branch", "task/{task}")},
			{"worktree_root", path(v, "worktree_root")}, {"worktree", path(v, "worktree")},
			{"hint_aliases", joinStrs(field(v, "hint_aliases"), "|")}, {"build", str(v, "build", "")},
			{"sweep", on("sweep")}, {"brain_guard", on("brain_guard")},
			{"agents_from", str(v, "agents_from", "")}, {"brief_note", str(v, "brief_note", "")},
			{"qa_target", flag(v, "qa_target")},
			{"deps_line", str(dep, "line", "")}, {"deps_contention", flag(dep, "contention")},
			{"db_line", str(db, "line", "")}, {"db_provisioned", flag(db, "provisioned")},
			{"db_no_worktree", str(db, "no_worktree", "")}, {"db_no_db_inert", flag(db, "no_db_inert")},
			{"devserver", str(dev, "start", "")}, {"devserver_line", str(dev, "line", "")}, {"devserver_off", str(dev, "off", "")},
			{"reap_copies", joinStrs(field(reap, "copies"), "\n")}, {"reap_copies_from", str(reap, "copies_from", "")},
			{"reap_max_worktrees", str(reap, "max_worktrees", "")}, {"reap_max_disk_pct", str(reap, "max_disk_pct", "")},
			{"reap_max_build_gb", str(reap, "max_build_gb", "")},
		} {
			put("_LC_"+l+"__"+e[0], e[1])
		}
		if p := field(v, "ports"); p != nil && p.kind == vObj {
			for _, e := range p.obj {
				put("_LC_"+l+"__port_"+e.k, e.v.tostring())
			}
		}
	}
	io.WriteString(out, sb.String())
	return 0
}
