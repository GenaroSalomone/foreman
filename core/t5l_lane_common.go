package main

// Track D2 — what the maintainer's own product lanes share and D1 may not provide
// (lanes/*.sh, bin/hw `_pick_port`, `_port_free`, `_link_or_copy`). The `d2` prefix marks a helper the integrator
// can unify with D1's.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// d2Must is `set -e` on one simple command: a failing status ends the launch with that status.
func (l *lch) d2Must(c cmdSpec) string {
	out, rc := l.sh(c)
	if rc != 0 { // # MUTATION-ANCHOR: 847-D2-01
		panic(exitNow{rc})
	}
	return out
}

// d2PaneRun is `herdr pane run "$pane" "$cmd" >/dev/null` under set -e.
func (l *lch) d2PaneRun(pane, cmd string) {
	l.d2Must(cmdSpec{name: "herdr", args: []string{"pane", "run", pane, cmd}, mut: true})
}

// d2Unset is bash's `unset VAR` for the dispatch's environment: the variable reads empty from here on.
// (d.envOver cannot express an absent variable; a child sees VAR= instead of no VAR. Integrator: a real
// unset in environ() makes this exact.)
func (l *lch) d2Unset(keys ...string) { l.unsetEnv(keys...) }

// d2Readable is `[ -r path ]`.
func d2Readable(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func d2IsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// d2Exists is `[ -d p ]`.
func d2Exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// d2DBName is `basename "$dir" | tr '\-/' '__'`.
func d2DBName(dir string) string {
	d := strings.TrimRight(dir, "/")
	if d == "" && dir != "" {
		d = "/"
	}
	base := d
	if i := strings.LastIndexByte(d, '/'); i >= 0 && d != "/" {
		base = d[i+1:]
	}
	return strings.Map(func(r rune) rune {
		if r == '-' || r == '/' {
			return '_'
		}
		return r
	}, base)
}

// d2TailLines is `printf '%s\n' "$s" | tail -n`.
func d2TailLines(s string, n int) string {
	lines := strings.Split(s, "\n") // s + "\n" splits into the lines plus a final empty element
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n") + "\n"
}

// d2GrepMatch is `printf '%s' "$text" | grep -q[i] "$pattern"` for a database name used as a basic regular
// expression: the characters BRE takes literally are escaped for Go's syntax.
func d2GrepMatch(text, pattern string, fold bool) bool {
	var b strings.Builder
	b.WriteString("(?m)")
	if fold {
		b.WriteString("(?i)")
	}
	for _, r := range pattern {
		if strings.ContainsRune(`+?|(){}`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(text)
}

// ── ports ────────────────────────────────────────────────────────────────

// d2Which finds an executable on the dispatch's PATH, as `command -v`.
func (l *lch) d2Which(name string) bool {
	for _, dir := range strings.Split(l.env("PATH"), ":") {
		if dir == "" {
			dir = "."
		}
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return true
		}
	}
	return false
}

// d2PortFree is `_port_free`.
func (l *lch) d2PortFree(p int) bool {
	ps := strconv.Itoa(p)
	switch {
	case l.d2Which("lsof"):
		_, rc := l.sh(cmdSpec{name: "lsof", args: []string{"-nP", "-iTCP:" + ps, "-sTCP:LISTEN"}, quiet: true})
		return rc != 0
	case l.d2Which("ss"):
		out, rc := l.sh(cmdSpec{name: "ss", args: []string{"-Hltn", "sport = :" + ps}})
		if rc != 0 { // ss failed: no answer, not "busy"
			return true
		}
		return strings.TrimRight(out, "\n") == ""
	}
	return true
}

// d2PickPort is `_pick_port "$1"`: lane D1's pickPort, as a string.
func (l *lch) d2PickPort(first int) string { return strconv.Itoa(l.pickPort(first)) }

// d2Mkdir is `mkdir "$d" 2>/dev/null`: fails when it exists (the claim); read-only it is recorded and succeeds.
func (l *lch) d2Mkdir(d string) bool {
	if l.fs.readonly {
		l.fs.rec(fsOp{Op: "mkdir", Path: d})
		return true
	}
	return os.Mkdir(d, 0o777) == nil
}

// d2FreshWithin is `find "$d" -mmin -N` printing something; ok=false when find itself would fail.
func d2FreshWithin(d string, age time.Duration) (fresh, ok bool) {
	now := time.Now()
	ok = true
	filepath.WalkDir(d, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			if p == d {
				ok = false
			}
			return nil
		}
		if info, ierr := e.Info(); ierr == nil && now.Sub(info.ModTime()) < age {
			fresh = true
		}
		return nil
	})
	return fresh, ok
}

// d2PortOf is `$(( $(lane_get "$PROJ" port_X) + OFF ))`.
func (l *lch) d2PortOf(field string) int {
	base, _ := strconv.Atoi(strings.TrimSpace(l.c.t.get(l.proj, field)))
	off, _ := strconv.Atoi(strings.TrimSpace(l.off))
	return base + off
}

// ── env files ────────────────────────────────────────────────────────────

// d2LinkOrCopy is `_link_or_copy main wt rel mode` (mode: link | copy).
func (l *lch) d2LinkOrCopy(main, wt, rel, mode string) {
	src, dst := main+"/"+rel, wt+"/"+rel
	if !d2Exists(src) {
		l.warn(rel + " — NOT PRESENT in " + main + "; the dev server may boot against code defaults")
		return
	}
	lst, lerr := os.Lstat(dst)
	isLink := lerr == nil && lst.Mode()&os.ModeSymlink != 0
	if isLink {
		if tgt, err := os.Readlink(dst); err == nil && tgt == src {
			l.info(rel + " — already linked")
			return
		}
	}
	if d2Exists(dst) && !isLink && mode == "copy" { // # MUTATION-ANCHOR: 847-D2-03
		l.info(rel + " — already present, kept")
		return
	}
	if d2Exists(dst) || isLink {
		l.d2Must(cmdSpec{name: "rm", args: []string{"-f", dst}, mut: true})
	}
	switch mode {
	case "link":
		l.d2Must(cmdSpec{name: "ln", args: []string{"-s", src, dst}, mut: true})
		l.ok(rel + " — symlink (shared)")
	case "copy":
		l.d2Must(cmdSpec{name: "cp", args: []string{"-p", src, dst}, mut: true})
		l.ok(rel + " — copy (task-private)")
	}
}

// laneSkillWithheld names the lanes whose worktrees get the main checkout's untracked skills, and the name prefixes
// each withholds. A lane registers itself from its own file (init). The launch and the brief's `skills:` check both
// read it, so what the check promises is what the launch does.
var laneSkillWithheld = map[string][]string{}

// d2SkillDirs are the skill roots a worktree inherits from git alone: whatever git ignores there stays in main.
var d2SkillDirs = []string{".claude/skills", ".agents/skills"}

// d2TrackedSkills is the set of "<root>/<name>" skill entries git tracks in main, from one git call.
func d2TrackedSkills(main string) map[string]bool {
	set := map[string]bool{}
	out, rc := brExec("", "git", append([]string{"-C", main, "ls-files", "-z", "--"}, d2SkillDirs...)...)
	if rc != 0 {
		return set
	}
	for _, f := range strings.Split(out, "\x00") {
		if parts := strings.SplitN(f, "/", 4); len(parts) >= 3 {
			set[strings.Join(parts[:3], "/")] = true
		}
	}
	return set
}

// d2SkillTracked: git tracks something at main/rel, so a worktree checks it out itself.
func d2SkillTracked(main, rel string) bool { return d2TrackedSkills(main)[rel] }

// d2SkillWithheld is the prefix of withheld that name starts with, "" when none.
func d2SkillWithheld(name string, withheld []string) string {
	for _, p := range withheld {
		if strings.HasPrefix(name, p) {
			return p
		}
	}
	return ""
}

// d2ReplicateLocalSkills symlinks every entry under main's skill roots that git does not track and the worktree
// lacks (one source, no drift). A tracked or already-present entry is never touched; a name starting with a
// withheld prefix is not replicated. Lane-neutral: a lane opts in by calling it.
func (l *lch) d2ReplicateLocalSkills(main, wt string, withheld []string) {
	tracked := d2TrackedSkills(main)
	for _, dir := range d2SkillDirs {
		ents, err := os.ReadDir(main + "/" + dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			name := e.Name()
			rel := dir + "/" + name
			if strings.HasPrefix(name, ".") || tracked[rel] {
				continue
			}
			if p := d2SkillWithheld(name, withheld); p != "" {
				l.info(rel + " — NOT replicated: " + p + "* skills drive production, so they stay out of a task's worktree")
				continue
			}
			src, dst := main+"/"+rel, wt+"/"+rel
			if lst, err := os.Lstat(dst); err == nil {
				if tgt, _ := os.Readlink(dst); lst.Mode()&os.ModeSymlink == 0 || tgt != src {
					l.info(rel + " — already present, kept")
					continue
				}
			}
			l.d2Must(cmdSpec{name: "mkdir", args: []string{"-p", wt + "/" + dir}, mut: true})
			l.d2LinkOrCopy(main, wt, rel, "link")
		}
	}
}

// d2IsEnvLocalName is the name side of the shell glob `.env.*.local`.
func d2IsEnvLocalName(n string) bool {
	return len(n) >= len(".env..local") && strings.HasPrefix(n, ".env.") && strings.HasSuffix(n, ".local")
}

// d2EnvLocalGlob is `"$main"/.env.*.local` filtered by `[ -f ]`, in glob order.
func d2EnvLocalGlob(main string) []string {
	ents, err := os.ReadDir(main)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		n := e.Name()
		if d2IsEnvLocalName(n) {
			if st, err := os.Stat(main + "/" + n); err == nil && st.Mode().IsRegular() {
				out = append(out, n)
			}
		}
	}
	sort.Strings(out)
	return out
}

// ── a worktree directory hw itself left half-made ────────────────────────

// reclaimHalfMade clears the way for a fresh `git worktree add` when wt holds what an interrupted launch left: a directory
// with no .git whose every entry is something hw itself makes (hwMade names the lane's replication list; `.hw/`, the run
// state, is always hw's). Without it the lane reads the directory as "worktree already exists, reusing" and builds on a
// tree that has no checkout. An entry hw does not make refuses the launch naming it, and nothing is deleted: hwMade
// judges the kind as well as the name (hw makes symlinks and regular files, never a directory). A directory with a .git
// is never touched, nor is a symlink. If git still lists the path (a killed `worktree add` leaves its admin entry, locked
// when hw made it), `worktree remove -f -f <path>` clears that one entry, so the add can reuse the name. The database is
// not touched: the lane's own provision step keeps its reuse rule, and a real worktree's database is never dropped here.
// It reports whether it removed (or, read-only, planned to remove) the directory.
func (l *lch) reclaimHalfMade(main, wt string, hwMade func(name string, mode fs.FileMode) bool) (removed bool) {
	if st, err := os.Lstat(wt); err != nil || !st.IsDir() || laneExists(wt+"/.git") {
		return false
	}
	ents, err := os.ReadDir(wt)
	if err != nil {
		return false
	}
	var made, foreign []string
	for _, e := range ents {
		if n := e.Name(); n == ".hw" && e.IsDir() || hwMade != nil && hwMade(n, e.Type()) {
			made = append(made, n)
		} else {
			foreign = append(foreign, n)
		}
	}
	if len(foreign) > 0 {
		l.die(wt + " exists, is not a git worktree (no .git) and holds entries hw does not make: " + strings.Join(foreign, ", ") +
			" — nothing was removed. Move them aside and dispatch again, or dispatch under another task name.")
	}
	real, _ := filepath.EvalSymlinks(wt)
	if l.readonly {
		l.fs.rec(fsOp{Op: "remove", Path: wt})
	} else if err := os.RemoveAll(wt); err != nil {
		l.die("could not remove the half-made worktree directory " + wt + ": " + err.Error())
	}
	what := "empty"
	if len(made) > 0 {
		what = "only hw's own: " + strings.Join(made, ", ")
	}
	l.info("worktree directory " + wt + " was a half-made leftover (no .git; " + what + ") — removed, building it fresh")
	listing, _ := l.sh(cmdSpec{name: "git", args: []string{"-C", main, "worktree", "list", "--porcelain"}, quiet: true})
	for _, ref := range parseWorktreeList(listing) {
		if ref.path == real || ref.path == wt {
			// Only this entry: `worktree prune` is repo-wide (it drops other tasks' stale entries) and spares a locked one,
			// which the add then refuses as "missing but locked". -f -f removes a locked entry, and the directory is gone.
			l.sh(cmdSpec{name: "git", args: []string{"-C", main, "worktree", "remove", "-f", "-f", ref.path}, mut: true, merge: true})
			break
		}
	}
	return true
}

// reuseWorktree is the lanes' "does the worktree already exist" check, with the half-made leftover cleared first: true
// only for a directory the lane may reuse. A lane asks this and nothing else, so the order cannot be got wrong.
func (l *lch) reuseWorktree(main, wt string, hwMade func(name string, mode fs.FileMode) bool) bool {
	return !l.reclaimHalfMade(main, wt, hwMade) && d2IsDir(wt)
}
