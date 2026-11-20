package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// A task is dispatched by one process at a time. `hw review` runs twice for the same build when done-invoker's own review and the
// brainer's both fire (2026-11-19, a product lane): the second saw the worktree the first had just made, then both ran CREATE DATABASE for
// it and one died on pg_database_datname_index. Under the lock the second waits, and what it finds afterwards is what a serial
// second dispatch always found: the worktree, the database and a live pane, reused, nothing launched twice.
var (
	dispatchLockWait = 15 * time.Minute // a cold first dispatch (worktree, install, database, dev server) takes minutes
	dispatchLockPoll = 250 * time.Millisecond
)

// dispatchLockPath is one file per lane and task, in the lock directory every executor and brainer shares (sharedLockDir).
func (d *dsp) dispatchLockPath() string {
	return sharedLockDir(d.getenv) + "/hw-dispatch-" + d.proj + "." + d.task + ".lock"
}

// dispatchTryLock is tryLock for a lock that leaves no file behind: release removes the file before it lets go. A taker that was
// waiting on the file the holder just removed gets a lock on an inode no path names, while a third dispatch may already be making
// the next file, so a lock counts only when the path still names the file that was locked.
func dispatchTryLock(path string) (release func(), held bool, err error) {
	for {
		f, held, err := tryLockFile(path)
		if !held {
			return nil, false, err
		}
		fi, e1 := f.Stat()
		pi, e2 := os.Stat(path)
		if e1 == nil && e2 == nil && os.SameFile(fi, pi) {
			return func() { os.Remove(path); f.Close() }, true, nil
		}
		f.Close()
	}
}

// dispatchLockTake waits, bounded, for another dispatch of this task to finish, then holds the lock until release. A flock dies with
// its process, so no holder can leave a stale one behind, and the file goes with the release (a golden tree, and /tmp, keep nothing
// of a dispatch). A lock that cannot be made at all warns and lets the dispatch run as it did before: the lock is a courtesy to a
// second dispatch, never a new way to refuse one.
func (d *dsp) dispatchLockTake() (release func()) {
	path := d.dispatchLockPath()
	label := d.proj + ":" + d.task
	deadline := time.Now().Add(dispatchLockWait)
	waited := false
	for {
		rel, held, err := dispatchTryLock(path)
		if err != nil {
			d.warn("UNESTABLISHED  dispatch lock " + path + ": " + err.Error() + " — dispatching without it")
			return func() {}
		}
		if held {
			os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600) // for the next one's «being dispatched by pid N»
			if waited {
				d.info("the other dispatch of " + label + " finished: continuing on what it left")
			}
			return rel
		}
		if !waited {
			d.info(label + " is being dispatched by " + dispatchLockHolder(path) + ": waiting for it, then reusing what it built (nothing is launched twice)")
			waited = true
		}
		if time.Now().After(deadline) {
			d.die(label + " is still being dispatched by " + dispatchLockHolder(path) + " after " + dispatchLockWait.String() + ". NOTHING was built by this dispatch. Look at that dispatch (it holds " + path + "), then run this command again.")
		}
		time.Sleep(dispatchLockPoll)
	}
}

// dispatchLockHolder is «pid N» from the lock file, or «another dispatch» while the holder has not written it yet.
func dispatchLockHolder(path string) string {
	b, _ := os.ReadFile(path)
	if pid := strings.TrimSpace(string(b)); pid != "" {
		return "pid " + pid
	}
	return "another dispatch"
}
