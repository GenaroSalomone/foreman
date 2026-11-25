package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// fdFind lists a tree for the callers that used to run `fd`, with no outside tool: -H (hidden), -I (do not honour ignore rules;
// without it what git ignores in a repository under the root is left out, as fd did), -t f|d|l, -e ext, -d depth, -E name, -g (the pattern is a glob), --changed-within <n>h|d, then pattern and root.
// A directory prints with a trailing slash, the output is in fdSortedOrder, and a root that cannot be read is status 1.
func fdFind(args ...string) (string, int) {
	var hidden, glob, noIgnore bool
	var types, exts, excl, pos []string
	depth := -1
	var within time.Duration
	for i := 0; i < len(args); i++ {
		a := args[i]
		val := func() string {
			i++
			if i < len(args) {
				return args[i]
			}
			return ""
		}
		switch a {
		case "-H":
			hidden = true
		case "-I":
			noIgnore = true
		case "-g":
			glob = true
		case "-t":
			types = append(types, val())
		case "-e":
			exts = append(exts, val())
		case "-E":
			excl = append(excl, val())
		case "-d":
			depth, _ = strconv.Atoi(val())
		case "--changed-within":
			v := val()
			if n, err := strconv.Atoi(strings.TrimRight(v, "hd")); err == nil {
				within = time.Duration(n) * time.Hour
				if strings.HasSuffix(v, "d") {
					within *= 24
				}
			}
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) != 2 {
		return "", 2
	}
	pat, root := pos[0], pos[1]
	match := func(string) bool { return true }
	if glob {
		match = func(n string) bool { ok, _ := filepath.Match(pat, n); return ok }
	} else if pat != "." {
		if pat == strings.ToLower(pat) { // fd's smart case
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return "", 2
		}
		match = re.MatchString
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return "", 1
	}
	cutoff := time.Now().Add(-within)
	var found []string
	// git names a work tree by its physical path; the walk keeps the path it was given, so ignore rules are looked up by the physical one
	walkRoot := strings.TrimRight(root, "/")
	physRoot, err := filepath.EvalSymlinks(walkRoot)
	if err != nil {
		physRoot = walkRoot
	}
	phys := func(p string) string { return physRoot + strings.TrimPrefix(p, walkRoot) }
	var top *fdRepo
	if !noIgnore {
		top = fdRepoOf(root)
	}
	var walk func(dir string, level int, repo *fdRepo)
	walk = func(dir string, level int, repo *fdRepo) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		if !noIgnore && (repo == nil || repo.top != phys(dir)) {
			if _, err := os.Lstat(dir + "/.git"); err == nil {
				repo = fdRepoOf(dir)
			}
		}
		for _, e := range ents {
			name := e.Name()
			if !hidden && strings.HasPrefix(name, ".") {
				continue
			}
			if fdExcluded(excl, name) {
				continue
			}
			p := dir + "/" + name
			kind := "f"
			switch {
			case e.Type()&os.ModeSymlink != 0:
				kind = "l"
			case e.IsDir():
				kind = "d"
			}
			if repo.ignores(phys(p), kind == "d") {
				continue
			}
			ok := fdWantsType(types, kind) && match(name) && fdWantsExt(exts, name)
			if ok && within > 0 {
				info, err := e.Info()
				ok = err == nil && info.ModTime().After(cutoff)
			}
			if ok {
				if kind == "d" {
					p += "/"
				}
				found = append(found, p)
			}
			if kind == "d" && (depth < 0 || level < depth) {
				walk(strings.TrimSuffix(p, "/"), level+1, repo)
			}
		}
	}
	walk(walkRoot, 1, top)
	if len(found) == 0 {
		return "", 0
	}
	return strings.Join(fdSortedOrder(found), "\n") + "\n", 0
}

func fdExcluded(globs []string, name string) bool {
	for _, g := range globs {
		if ok, _ := filepath.Match(g, name); ok {
			return true
		}
	}
	return false
}

func fdWantsType(types []string, kind string) bool {
	if len(types) == 0 {
		return true
	}
	for _, t := range types {
		if t == kind {
			return true
		}
	}
	return false
}

func fdWantsExt(exts []string, name string) bool {
	if len(exts) == 0 {
		return true
	}
	for _, x := range exts {
		if strings.EqualFold(filepath.Ext(name), "."+x) {
			return true
		}
	}
	return false
}

// fdRepo is a git work tree and what git ignores in it (.gitignore files, info/exclude and the user's excludes file), as the
// paths `git ls-files --others --ignored --exclude-standard --directory` lists: a directory with its trailing slash.
type fdRepo struct {
	top     string
	ignored map[string]bool
}

// fdRepoOf is the work tree that holds dir, or nil when dir is in none (git failing is no ignore rules, as fd outside a repository).
func fdRepoOf(dir string) *fdRepo {
	out, rc := fdGit(dir, "rev-parse", "--show-toplevel")
	top := strings.TrimRight(out, "\n")
	if rc != 0 || top == "" {
		return nil
	}
	listed, _ := fdGit(top, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory", "-z")
	r := &fdRepo{top: top, ignored: map[string]bool{}}
	for _, f := range strings.Split(listed, "\x00") {
		if f != "" {
			r.ignored[f] = true
		}
	}
	return r
}

func fdGit(dir string, args ...string) (string, int) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = childEnv()
	out, err := cmd.Output()
	return string(out), exitStatus(err)
}

// ignores is whether git ignores the path p (a directory when isDir) under this work tree.
func (r *fdRepo) ignores(p string, isDir bool) bool {
	if r == nil || !strings.HasPrefix(p, r.top+"/") {
		return false
	}
	rel := p[len(r.top)+1:]
	if isDir {
		rel += "/"
	}
	return r.ignored[rel]
}
