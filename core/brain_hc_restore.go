package main

// `brain restore` — a reboot costs one command (foreman-without-herdr-server), and with --on-start none (reboot-restore-brainers).
//
// After a reboot the app's core brings its terminals back (`hc keep`), but each brainer's claude is gone: the terminal is a
// shell in the lane's directory, still named brain-<lane> and labelled brain. Nothing resumed the lanes, so each one was
// revived by hand (`hc send <t> "claude --resume <id>"`, with CLAUDE_CONFIG_DIR for a personal lane). The same job is
// `brain <lane> --resume <id>` once per lane, which replaces that shell with the brainer (brain_hc_reset.go) — so restore
// only finds the id and calls it: the newest transcript under the lane's project directory, in the account the manifest names.
//
// A lane is skipped, and named, when claude already runs in its terminal (a live brainer is never touched, working or not;
// only its cockpit writer is ensured), when its brainer is not claude, or when it has no transcript to resume.
//
// The setup lane goes LAST: it is the brainer that repairs the others (MEASURED 2026-11-16, after a reboot).
// The terminal restore runs in — or one it dies with (`die_with_parent`) — is not skipped when it holds only a shell: it is
// replaced last, by a DETACHED `brain <lane> --resume` (its own session, brain_unix.go), because a child of the terminal it
// replaces dies with it before the resume is done (2026-11-16: a hand-off script, an hc child of brain-setup, died that way).
//
// --on-start is the hook the herdr app fires on every start (the cockpit mod's init, herdr-app/mods/foreman-cockpit/restore.js).
// The app starts after a reboot (macOS relaunches it at login; MEASURED 2026-11-16: Herdr pid 657 at boot, its core started by
// it) and after a quit; only after the first did the terminals lose their programs. So it is keyed to the core's HOLDER — the
// process that keeps terminals alive across app quits (`hc server status`: holder.instance): restore runs once per holder,
// under a lock, detached from the app's exec, and writes what it did to <cache>/hw/brain-restore.log. Only lanes whose
// terminal the core kept are restored: a lane with no brain-<lane> terminal had no brainer to bring back.

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

var reProjectDirChar = regexp.MustCompile(`[^A-Za-z0-9]`)

// restoreLastLane is restored after every other lane.
const restoreLastLane = "setup"

// restoreDetachedEnv marks the detached half of `brain restore --on-start`.
const restoreDetachedEnv = "BRAIN_RESTORE_DETACHED"

// laneProjectDirs is every claude project directory a lane's brainer may have written: its own directory, then the old ones
// its brain aliases name (a lane renamed keeps its old name as an alias, and its brainer's transcripts stay under
// -brain-<old name>: MEASURED 2026-11-16 for two lanes).
func (b *brainRun) laneProjectDirs(acct, lane string) []string {
	proj := func(dir string) string {
		return filepath.Join(acct, "projects", reProjectDirChar.ReplaceAllString(dir, "-"))
	}
	dirs := []string{proj(pyRealpath(b.brain + "/" + lane))}
	var aliases []string
	for alias, l := range b.tab.aliasB {
		if l == lane {
			aliases = append(aliases, alias)
		}
	}
	sort.Strings(aliases)
	root := pyRealpath(b.brain)
	for _, a := range aliases {
		dirs = append(dirs, proj(root+"/"+a))
	}
	return dirs
}

// newestSession is the id of the newest conversation of the lane's brainer: the most recently written <uuid>.jsonl under
// <account>/projects/<a lane directory, every non-alphanumeric as "-"> (laneProjectDirs), which is how claude names a project directory.
func (b *brainRun) newestSession(lane string) (id string) {
	acct := b.laneAccountDir(lane)
	if acct == "" {
		acct = b.home + "/.claude"
	}
	var files []string
	for _, dir := range b.laneProjectDirs(acct, lane) {
		f, _ := filepath.Glob(brEscapeGlob(dir) + "/*.jsonl")
		files = append(files, f...)
	}
	sort.Strings(files)
	var newest int64
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), ".jsonl")
		if !reUUID.MatchString(base) {
			continue
		}
		st, err := os.Stat(f)
		if err != nil || st.ModTime().UnixNano() < newest {
			continue
		}
		newest, id = st.ModTime().UnixNano(), base
	}
	return id
}

// restoreOrder is lanes with the setup lane moved last, the others in their order.
func restoreOrder(lanes []string) []string {
	out := make([]string, 0, len(lanes))
	last := false
	for _, l := range lanes {
		if l == restoreLastLane {
			last = true
			continue
		}
		out = append(out, l)
	}
	if last {
		out = append(out, restoreLastLane)
	}
	return out
}

// restoreAncestry is the terminal this process runs in and those it dies with: its `die_with_parent` parents, read with
// `hc get`. A record that cannot be read ends the walk (what was read stays known).
func (b *brainRun) restoreAncestry() map[string]bool {
	own := b.getenv("HC_TERMINAL_ID")
	if own == "" {
		return nil
	}
	set := map[string]bool{own: true}
	for addr, i := own, 0; i < 8; i++ {
		m, read := b.hcGet(addr)
		if !read || !pyTruthy(m["die_with_parent"]) {
			break
		}
		p, _ := m["parent"].(string)
		if p == "" || set[p] {
			break
		}
		set[p] = true
		addr = p
	}
	return set
}

// restoreDir is where --on-start keeps its lock, its holder stamp and its log: <XDG_CACHE_HOME or ~/.cache>/hw.
func (b *brainRun) restoreDir() string {
	cache := b.getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(cache) {
		cache = b.home + "/.cache"
	}
	return cache + "/hw"
}

// hcHolder is the instance of the core's holder (`hc server status --json`: holder.instance), "" when it does not say.
func (b *brainRun) hcHolder() string {
	r := b.hc(brainCmd{}, "server", "status", "--json")
	if r.RC != 0 {
		return ""
	}
	v, _ := bjParse(brTrimNL(r.Out))
	m, _ := v.(jmap)
	h, _ := m["holder"].(jmap)
	return hcStr(h, "instance")
}

// appendNew is append that leaves a lane named twice (`restore demo demo`, or a lane and its alias) as the one it is.
func appendNew(lanes []string, l string) []string {
	if slices.Contains(lanes, l) {
		return lanes
	}
	return append(lanes, l)
}

type restoreStep struct{ lane, addr, sid string }

// restore is `brain restore [<lane>...] [--dry-run] [--on-start]`.
func (b *brainRun) restore(args []string) int {
	dry, onStart := false, false
	var lanes []string
	for _, a := range args[1:] {
		switch {
		case a == "--dry-run":
			dry = true
		case a == "--on-start":
			onStart = true
		case a == "-h" || a == "--help":
			b.outf("usage: brain restore [<lane>...] [--dry-run] [--on-start]   (every lane when none is named; --on-start: once per core holder, detached)\n")
			return 0
		case strings.HasPrefix(a, "-"):
			b.errf("brain restore: unknown flag: %s  (try: --dry-run, --on-start)\n", a)
			return 64
		case b.laneKnown(a):
			lanes = appendNew(lanes, a)
		default:
			if al := b.laneAlias(a); al != "" {
				lanes = appendNew(lanes, al)
				continue
			}
			b.errf("brain restore: not a lane: %s  (try: %s)\n", a, strings.Join(b.tab.names, ", "))
			return 64
		}
	}
	if len(lanes) == 0 {
		lanes = b.tab.names
	}
	lanes = restoreOrder(lanes)
	if onStart {
		return b.restoreOnStart(args, lanes, dry)
	}
	if !b.restoreOnTheCore() {
		return 1
	}
	_, rc := b.restoreLanes(lanes, dry, false)
	return rc
}

// restoreOnTheCore is restore's route check: an app terminal, or a shell whose herdr has no server while the core answers.
func (b *brainRun) restoreOnTheCore() bool {
	if b.hcBackend() {
		return true
	}
	if r := b.herdr(brainCmd{KeepErr: true}, "workspace", "list"); b.herdrGone(r) {
		return true
	}
	b.errf("brain restore: this runs on herdr-core (the herdr app), and this shell is neither one of its terminals nor on a machine whose herdr server is off while the core answers. Nothing was sent.\n")
	return false
}

// restoreOnStart is `brain restore --on-start`: the app's start. It detaches (the app's exec cuts a run at 30 s), waits a
// bounded time for the core, and under a lock restores the lanes that have not SETTLED for the core's current holder. The
// stamp <cache>/hw/brain-restore.holder is the holder, then one settled lane per line: a lane whose kept terminal was not
// listed yet, could not be read, or whose resume failed is not settled, so the next start looks at it again; a settled one
// is never looked at again for that holder (a brainer a person ended stays ended until the terminals are lost again).
func (b *brainRun) restoreOnStart(args, lanes []string, dry bool) int {
	cacheDir := b.restoreDir()
	if !dry && b.getenv(restoreDetachedEnv) == "" {
		// the app's exec waits for what it starts and may cut it: the work runs in a session of its own, and says so in its log
		os.MkdirAll(cacheDir, 0o755)
		log := cacheDir + "/brain-restore.log"
		pid, err := b.detach(b.binDir+"/brain", append([]string{"restore"}, args[1:]...), envSet(b.env, restoreDetachedEnv, "1"), log)
		if err != nil || pid == 0 {
			b.errf("brain restore --on-start: the detached restore could not be started (%v); nothing was restored.\n", err)
			return 1
		}
		b.outf("brain restore --on-start: runs detached as pid %d; it writes what it did to %s\n", pid, log)
		return 0
	}
	b.outf("brain restore --on-start: %s\n", b.now().Format(time.RFC3339))
	if !dry { // first: a second start (a quit and a start, a reload) leaves at once instead of waiting beside this one
		lock := cacheDir + "/brain-restore.lock"
		os.MkdirAll(cacheDir, 0o755)
		release, held, err := tryLock(lock)
		if err != nil {
			b.errf("brain restore --on-start: the lock %s: %v. Nothing was sent.\n", lock, err)
			return 1
		}
		if !held {
			b.outf("brain restore --on-start: another restore holds %s; this one does nothing.\n", lock)
			return 0
		}
		defer release()
	}
	// after a reboot the app may start before its core answers: a bounded wait, which costs the app nothing (this is detached)
	wait := atoiDefault(b.getenv("BRAIN_RESTORE_CORE_WAIT"), 60)
	deadline := b.now().Add(time.Duration(wait) * time.Second)
	holder := b.hcHolder()
	for holder == "" && b.now().Before(deadline) {
		b.sleep(2 * time.Second)
		holder = b.hcHolder()
	}
	if holder == "" {
		b.errf("brain restore --on-start: `hc server status --json` named no holder in %ds, so whether the core's terminals were lost is UNKNOWN. Nothing was sent.\n", wait)
		return 1
	}
	if !b.restoreOnTheCore() {
		return 1
	}
	stamp := cacheDir + "/brain-restore.holder"
	settled := map[string]bool{}
	got, _ := os.ReadFile(stamp)
	if rows := strings.Split(strings.TrimSpace(string(got)), "\n"); rows[0] == holder {
		for _, l := range rows[1:] {
			settled[l] = true
		}
	}
	var todo []string
	for _, l := range lanes {
		if !settled[l] {
			todo = append(todo, l)
		}
	}
	if len(todo) == 0 {
		b.outf("brain restore --on-start: already ran for the core's holder %s: every lane settled, and its terminals were not lost since. Nothing to do.\n", holder)
		return 0
	}
	done, rc := b.restoreLanes(todo, dry, true)
	if dry {
		return rc
	}
	rows := []string{holder} // every lane settled for this holder, named in this run or not
	for _, l := range restoreOrder(b.tab.names) {
		if settled[l] || done[l] {
			rows = append(rows, l)
		}
	}
	if err := os.WriteFile(stamp, []byte(strings.Join(rows, "\n")+"\n"), 0o600); err != nil {
		b.errf("brain restore --on-start: %s: %v (the next start looks at every lane again, which skips what runs)\n", stamp, err)
	}
	return rc
}

// restoreLanes restores each lane and returns the lanes it SETTLED: restored, already running claude, not claude, or with
// nothing to resume. keptOnly (the app's start) starts no terminal the core did not keep. rc is 1 when a resume failed or
// the terminals could not be listed.
func (b *brainRun) restoreLanes(lanes []string, dry, keptOnly bool) (settled map[string]bool, rc int) {
	settled = map[string]bool{}
	terms, ok := b.hcTerminals("label=brain")
	if !ok {
		return settled, 1
	}
	mine := b.restoreAncestry()
	var last []restoreStep // the lanes whose terminal this process lives in: replaced after the others, detached
	n := 0
	for _, lane := range lanes {
		laneDir := b.brain + "/" + lane
		if !brFileExists(laneDir + "/CLAUDE.md") {
			continue
		}
		n++
		addr := ""
		for _, t := range terms {
			if sameDir(t.cwd, laneDir) {
				addr = t.addr
				break
			}
		}
		if addr == "" && keptOnly {
			b.outf("skipped %s: the core lists no brain-%s terminal (yet), so there is no brainer to bring back now\n", lane, lane)
			continue
		}
		where := orStr(addr, "no terminal yet")
		if addr != "" {
			info, live, read := hcAgentRead(addr)
			if !read { // an unread terminal is not a terminal without claude: `brain --resume` would end it if it is live
				b.outf("skipped %s (%s): `hc get %s` could not be read, so whether claude runs there is UNKNOWN; nothing was ended or started\n", addr, lane, addr)
				continue
			}
			if live && jqRawOr(info, "", "result", "agent", "agent") == "claude-code" {
				settled[lane] = true
				if mine[addr] {
					b.outf("skipped %s (%s): it is the terminal brain is running in\n", addr, lane)
					continue
				}
				// its writer may have died while the pane lived (the cockpit then reads «NO STATE»): one is started, and the writer's
				// own lock makes a second a no-op. A dry run starts nothing.
				ensured := ""
				if !dry {
					b.proj = lane
					b.cockpitWriter("claude", addr)
					ensured = "; its cockpit writer is ensured"
				}
				b.outf("skipped %s (%s): claude is already running there%s\n", addr, lane, ensured)
				continue
			}
		}
		sid := b.newestSession(lane)
		if sid == "" {
			b.outf("skipped %s (%s): no conversation under %s to resume\n", where, lane, orStr(b.laneAccountDir(lane), b.home+"/.claude"))
			settled[lane] = true
			continue
		}
		if mine[addr] {
			last = append(last, restoreStep{lane, addr, sid})
			continue
		}
		if dry {
			b.outf("would restore %s (%s) resuming %s\n", where, lane, sid)
			continue
		}
		res := b.run(brainCmd{Name: b.binDir + "/brain", Args: []string{lane, "--resume", sid}, Merge: true})
		if res.RC != 0 {
			b.outf("FAILED %s (%s): brain refused, rc=%d: %s\n", where, lane, res.RC, tail2(brTrimNL(res.Out)))
			rc = 1
			continue
		}
		settled[lane] = true
		b.outf("restored %s (%s) resuming %s\n", where, lane, sid)
	}
	for _, s := range last {
		if dry {
			b.outf("would restore %s (%s) resuming %s, last and detached: this restore runs in that terminal\n", s.addr, s.lane, s.sid)
			continue
		}
		// a child of the terminal it replaces dies with it: the resume runs in a session of its own, without this terminal's
		// identity (hcEnd refuses to end the terminal brain runs in, and this one no longer runs in it)
		env := envWithout(b.env, func(n string) bool { return n == "HC_TERMINAL_ID" })
		log := filepath.Join(brOrStr(b.getenv("TMPDIR"), "/tmp"), "brain-restore-"+sanitizePane(s.addr)+".log")
		pid, err := b.detach(b.binDir+"/brain", []string{s.lane, "--resume", s.sid}, env, log)
		if err != nil || pid == 0 {
			b.outf("FAILED %s (%s): the detached resume could not be started (%v)\n", s.addr, s.lane, err)
			rc = 1
			continue
		}
		settled[s.lane] = true
		b.outf("restoring %s (%s) resuming %s, last and detached as pid %d (this restore runs in that terminal); log %s\n", s.addr, s.lane, s.sid, pid, log)
	}
	if n == 0 {
		b.outf("brain restore: no lane matched.\n")
	}
	return settled, rc
}
