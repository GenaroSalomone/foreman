package main

// Train T5b: the dispatch's LAUNCH — everything bin/hw does after `_dispatch_manifest` when the dispatch is not a
// dry run: the worktree and its branch, the database and ports, the tab or pane, `herdr agent start`, the brief
// delivered, the run's receipt. One state, `lch`, embeds the dispatch (`dsp`, t5_state.go) and one struct per area,
// each in the file of the area that owns it (lenvState, lspState, llnState, lrcState).
// Fields two areas read are declared HERE, named after the bash global (WS → ws, HW_TAB → hwTab).
//
// Conventions are t5_state.go's (a bash `_foo_bar` is the method `fooBar`; text byte for byte; d.env, never
// os.Getenv). Effects go through the exec layer and the fsx (t5l_exec.go): an exec is `l.sh(...)`, never exec.Command,
// so that HW_CORE_READONLY=1 (shadow's first leg) records instead of doing.

type lch struct {
	*dsp
	hcLiveRun    string // hcLive: the hw_run label of the live executor terminal it found, "" when none
	lenvState           // t5l_env.go     the run's environment, trust, artifacts
	lrcState            // t5l_receipt.go the receipt, the ledger, the engram session, stale-hook refusal
	lspState            // t5l_space.go   workspace/tab/layout, --here
	llnState            // t5l_lanes.go   the lane builders (worktree, ports, database, dependencies)

	fs *fsx // every file write of the launch goes through it

	// the launch's identity (bin/hw "LABEL=… BRANCH=… HW_RUN=…")
	label  string // LABEL  "$PROJ:$TASK"
	branch string // BRANCH
	off    string // OFF    `_offset "$TASK"`

	// where it stands (bash globals set by _build_space / the lane files)
	ws, hwTab, hwWorkdir string // WS, HW_TAB, HW_WORKDIR
	layoutOut, layoutDoc string // LAYOUT_OUT, LAYOUT_DOC
	pAgent, pShell, pDev string // P_AGENT, P_SHELL, P_DEV
	wt                   string // WT  the work directory the lane built
	hcDevAddrs           []string // herdr-core: the dev-server terminals this launch started or found (hcDevStart)

	// the rollback record (bin/hw "LAUNCH_WT=… LAUNCH_BRANCH_CREATED=…")
	launchWT, launchBranch               string
	launchWTCreated, launchBranchCreated bool
}
