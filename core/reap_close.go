package main

// What `hw reap` DECIDES instead of keeping (task reap-keeps-itself-clean). On 2026-11-20 one lane held 91 worktrees (9.9 GB) that
// reap kept and nobody removed: a lock that outlived its executor, detached trees of gate and cut runs, the dirty or unmerged
// tree of a task long closed. Each is now removed by --apply, and the plan (no --apply) says so. Branches are never deleted
// here; a live pane, a live lease and anything that read `recent` still keep what they kept.
//
// Shared by the plan (reap.go) and the apply (t4_reap_wt.go): the two carry a copy of the verdict each, and these are the
// predicates both ask.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// A detached tree under the cache root (a gate, a cut, a triage base) is kept this long after it was made, for whoever
	// inspects a red run, and removed after it. The age is its `.git` link's: written once, at `git worktree add`.
	detachedTreeTTL = 24 * time.Hour
	// A task that reported less than this ago is not closed yet: its dirty tree is still the brainer's to look at.
	closedTaskGrace = 24 * time.Hour
	// The archive reap writes (dirty trees, task outputs, database dumps) is pruned after this many days.
	archiveKeepDays = 30
)

// hwLockRE is the reason hw gives the lock it puts on a worktree (receiptWorktree): a lock of anyone else's is not ours to lift.
var hwLockRE = regexp.MustCompile(`^hw \S+ run \S+, executor live$`)

// deadHwLock: hw's own lock, read AFTER the occupancy gate said nobody is in the tree (no pane, no hc terminal of its run), so
// the executor it names is dead and the lock is a leftover.
func deadHwLock(reason string) bool { return hwLockRE.MatchString(reason) }

// cacheRoots are the directories hw's own runs make trees in: <cache>/hw and the per-run TMPDIRs (<cache>/hw-run-tmp).
func cacheRoots(getenv func(string) string) []string {
	cache := getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(cache) {
		h := getenv("HOME")
		if !filepath.IsAbs(h) {
			return nil
		}
		cache = h + "/.cache"
	}
	var roots []string
	for _, r := range []string{cache + "/hw", runTmpRoot(getenv)} {
		if p, err := filepath.EvalSymlinks(r); err == nil {
			roots = append(roots, p)
		}
	}
	return roots
}

// expiredCacheTree: wt lies under a cache root and was made more than detachedTreeTTL ago.
func expiredCacheTree(getenv func(string) string, wt string, now time.Time) bool {
	real, err := filepath.EvalSymlinks(wt)
	if err != nil {
		return false
	}
	for _, root := range cacheRoots(getenv) {
		if !strings.HasPrefix(real, root+"/") {
			continue
		}
		st, err := os.Stat(wt + "/.git")
		return err == nil && now.Sub(st.ModTime()) > detachedTreeTTL
	}
	return false
}

// closedTaskAt: dir's task reported (runReported) at least closedTaskGrace ago.
func closedTaskAt(dir string, now time.Time) bool {
	if !runReported(dir) {
		return false
	}
	ids := RunIDs(dir + "/.hw") // .hw may have vanished since runReported read it
	if len(ids) == 0 {
		return false
	}
	st, err := os.Stat(runDoneMarker(dir + "/.hw/" + ids[0]))
	return err == nil && now.Sub(st.ModTime()) > closedTaskGrace
}

// cacheTreeInUse is the plan line of an expired cache tree that is kept: a live process has its cwd in it (or lsof could not say, and
// an unread answer is not an empty one).
func cacheTreeInUse(unread bool) string {
	why := "a live process has its cwd in it"
	if unread {
		why = "lsof could not say whether a process works in it"
	}
	return "detached tree under the cache, made over " + itoa(int(detachedTreeTTL/time.Hour)) + "h ago — kept: " + why
}

// closedWhy is the reason a dirty or unmerged verdict of a closed task becomes `closed`.
func closedWhy(was string) string {
	return was + " — its task reported and nobody is in it: the uncommitted work is archived first, the worktree removed, the branch kept"
}

type expiredArchive struct {
	path string
	age  int64 // whole days since it was last written
}

func (e expiredArchive) why() string {
	return "archive: " + itoa(int(e.age)) + "d > " + itoa(archiveKeepDays) + "d"
}

// archiveExpired lists what lies directly under a lane's archive directory (<archive>/<lane>: a dated directory of closed trees, a task's
// outputs, a database dump) and was last written more than archiveKeepDays days ago.
func archiveExpired(laneArchive string, now time.Time) []expiredArchive {
	ents, err := os.ReadDir(laneArchive)
	if err != nil {
		return nil
	}
	var out []expiredArchive
	for _, e := range ents {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if age := int64(now.Sub(info.ModTime()) / (24 * time.Hour)); age > archiveKeepDays {
			out = append(out, expiredArchive{laneArchive + "/" + e.Name(), age})
		}
	}
	return out
}
