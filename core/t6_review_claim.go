package main

// reviewLockPath is the lock of one reviewer task (a lane, a build and a head), held while an `hw review` dispatches it.
func (a *app) reviewLockPath(lane, rtask string) string {
	return sharedLockDir(a.getenv) + "/hw-review-" + lane + "." + rtask + ".lock"
}

// reviewClaim says, and returns owned, when another `hw review` of this build at this head already owns the round: it is dispatching
// now (its lock), or the build's receipt holds a review_run for this very brief, whether or not its reviewer's worktree
// is still there (a reaped reviewer still owns its round). Nothing is recorded then: a second review_run counts one round twice. release is never nil; a dry run takes no lock.
func (a *app) reviewClaim(p *reviewPlan, dry bool) (release func(), owned bool) {
	release = func() {}
	if !dry {
		lock := a.reviewLockPath(p.lane, p.rtask)
		rel, held, err := dispatchTryLock(lock)
		switch {
		case err != nil:
			a.warn("UNESTABLISHED review lock: " + err.Error() + " — reviewing without it")
		case !held:
			a.ok(p.lane + ":" + p.rtask + " is being dispatched by " + dispatchLockHolder(lock) + " for " + reviewShort(p.head) + ": this review dispatches and records nothing.")
			return release, true
		default:
			release = rel
		}
	}
	for _, r := range runsNewestFirst(p.wt + "/.hw") {
		dir := p.wt + "/.hw/" + r
		if !has(receiptValues(dir, "review_run"), p.briefPath) {
			continue
		}
		a.ok(p.lane + ":" + reviewerTaskOf(dir) + " already owns the review of " + p.lane + ":" + p.task + " at " + reviewShort(p.head) + " (review_run " + p.briefPath + "): this review dispatches and records nothing.")
		return release, true
	}
	return release, false
}
