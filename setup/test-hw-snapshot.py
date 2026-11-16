#!/usr/bin/env python3
"""Run test-hw on a private, measured copy, never on changing checkout bytes.

This is optimistic drift detection, NOT an atomic filesystem snapshot. No lock
can exclude writers that do not participate. The before/copy/after manifests
must agree before tests start; another writer can then change the original
without changing the measured candidate. HOME, operator state under the real
brain directory, and external symlink targets remain shared/live dependencies,
as do the explicitly live checks in subject 69. This narrows concurrent-source
contamination; it does NOT close concurrent-live-state contamination.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time


# An hw executor's run state, at the root of its worktree: `.hw/` (written by
# its own Stop hook at every turn end) and `.artifacts/` ($HW_ARTIFACTS, where
# a release cut writes its evidence while its gates run). Neither is source,
# and a write there during the capture aborted a cut's test-hw gate. So is
# `.hw-reap-<lane>` (gitignored), the survey cache `hw reap` writes at the brain
# root: since 2026-10-01 `hw done` reaps too, and a subject's `hw done` wrote it.
RUN_STATE = {'.hw', '.artifacts'}
# and hw's survey caches and db ledger (`.hw-reap-<lane>`, `.hw-db-ledger-<lane>`)
RUN_STATE_PREFIXES = ('.hw-reap-', '.hw-db-ledger-')


def _run_state(n):
    return n in RUN_STATE or n.startswith(RUN_STATE_PREFIXES)


def copy_ignore(root):
    """shutil.copytree's ignore: .git anywhere, run state only at the root."""
    root = os.path.realpath(root)
    def ignore(parent, names):
        skip = {'.git'}
        if os.path.realpath(parent) == root:
            skip |= {n for n in names if _run_state(n)}
        return [n for n in names if n in skip]
    return ignore


def inventory(root):
    """Include ignored files, new subjects, dependencies and symlink objects.

    Git metadata and an executor's run state at the root (RUN_STATE) are the
    only omissions: git metadata must never refer to the original
    index/worktree. Symlink targets outside the checkout are not followed.
    """
    result = {}
    root_real = os.path.realpath(root)
    for parent, dirs, files in os.walk(root, followlinks=False):
        at_root = os.path.realpath(parent) == root_real
        dirs[:] = sorted(d for d in dirs if d != '.git' and not (at_root and _run_state(d))
                         and os.path.join(parent, d) != os.path.join(root, 'core', 'dist'))  # gitignored build output: gtc_fresh_core rebuilds it mid-run
        if at_root:
            files = [f for f in files if not _run_state(f)]
        for name in sorted(dirs + [f for f in files if f != '.git']):
            path = Path(parent) / name
            rel = str(path.relative_to(root))
            info = path.lstat()
            mode = stat.S_IMODE(info.st_mode)
            if path.is_symlink():
                # A link is its target: its own mode is never consulted, and a
                # copy under Git Bash reports another one (511 vs 438).
                value = ['link', os.readlink(path)]
            elif stat.S_ISDIR(info.st_mode):
                value = ['dir', mode]
            elif stat.S_ISREG(info.st_mode):
                with path.open('rb') as source:
                    digest = hashlib.file_digest(source, 'sha256').hexdigest() if hasattr(hashlib, 'file_digest') else hashlib.sha256(source.read()).hexdigest()
                after = path.stat()
                # Reading may update atime; it is not a source-byte change.
                stable = ('st_dev', 'st_ino', 'st_mode', 'st_size', 'st_mtime_ns', 'st_ctime_ns')
                if any(getattr(after, key) != getattr(info, key) for key in stable):
                    raise CaptureDrift('source changed while reading: ' + rel)
                value = ['file', mode, digest]
            else:
                raise RuntimeError('unsupported source entry (not copied): ' + rel)
            result[rel] = value
    return result


def tracking(root):
    try:
        proc = subprocess.run(['git', '-C', str(root), 'ls-files', '-z'],
                              stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    except FileNotFoundError:
        return {'usable': False, 'tracked': []}
    return {'usable': proc.returncode == 0,
            'tracked': sorted(os.fsdecode(p) for p in proc.stdout.split(b'\0') if p) if proc.returncode == 0 else []}


# The checkout is shared with the lane brainers, who write their own `<lane>/decisions.md`, briefs
# and archives all day, so a quiescent checkout never exists (measured 2026-11-13: a batch gate
# aborted because a lane brainer edited its decisions.md during the capture). Two answers,
# neither of which weakens what the verdict certifies (the private candidate, hashed as copied):
#   * a change confined to LANE DOCUMENTATION THE SUITE DOES NOT READ is tolerated — the rule is
#     setup/hooks/suite-trigger-pattern.sh `docs_only_path`, the one pre-push already accepts a
#     tree by (a lane's briefs/, decisions/, and decisions.md, never setup's decisions.md). A
#     decisions.md counts only while its header (subject 113 reads it) is the one the candidate
#     holds, and only when it was neither created nor deleted;
#   * anything else is retried, CAPTURE_ATTEMPTS times with a short backoff, then refused naming
#     the paths that kept moving.
CAPTURE_ATTEMPTS = 3  # MUTATION-ANCHOR: 1384-M01
CAPTURE_BACKOFF = 0.5  # seconds, times the attempt number


class CaptureDrift(RuntimeError):
    """The checkout moved under the capture: worth another attempt."""


def lane_docs(root, paths):
    """Which of `paths` the pre-push cover calls lane documentation (one bash call, one rule)."""
    hook = Path(root) / 'setup/hooks/suite-trigger-pattern.sh'
    if not paths or not hook.is_file():
        return set()  # fail closed: nothing is documentation
    script = '. "$1"; docs_only_init "$2"; while IFS= read -r p; do docs_only_path "$p" && printf "%s\\n" "$p"; done; true'
    try:
        proc = subprocess.run(['bash', '-c', script, '_', str(hook), str(root)],
                              input='\n'.join(paths) + '\n', stdout=subprocess.PIPE,
                              stderr=subprocess.DEVNULL, universal_newlines=True)
    except (OSError, UnicodeError):
        return set()
    return set(proc.stdout.split('\n')) & set(paths) if proc.returncode == 0 else set()


def decisions_header(path):
    """Everything before the first `---` line after the first (decisions_header in the hook)."""
    lines = Path(path).read_bytes().split(b'\n')
    for i, line in enumerate(lines):
        if i and line == b'---':
            return lines[:i]
    return lines


def capture(root, candidate):
    """One attempt. Returns (frozen, tracked, drift, tolerated); `drift` maps each path the suite could read
    that moved to its first difference, `tolerated` lists the lane documentation that moved and may not matter.
    `tracked` is the manifest taken BEFORE the copy: a tolerated doc that became (un)tracked meanwhile is the
    one place it and the candidate's file set can disagree, and only for a path the suite does not read."""
    before = inventory(root)
    tracked = tracking(root)
    shutil.copytree(root, candidate, symlinks=True, ignore=copy_ignore(root))
    frozen = inventory(candidate)
    after = inventory(root)
    now = tracking(root)
    moved = {k for k in set(before) | set(frozen) | set(after) if not before.get(k) == frozen.get(k) == after.get(k)}
    structural = {k for k in moved if None in (before.get(k), frozen.get(k), after.get(k))}  # created or deleted
    if tracked['usable'] != now['usable']:
        moved.add('(git tracking)')
    else:
        flipped = set(tracked['tracked']) ^ set(now['tracked'])
        moved |= flipped; structural |= flipped
    docs = lane_docs(root, sorted(moved))  # MUTATION-ANCHOR: 1384-M02
    tolerated = set()
    for k in docs:
        if k.endswith('/decisions.md'):
            try:
                if k in structural or decisions_header(candidate / k) != decisions_header(Path(root) / k):  # MUTATION-ANCHOR: 1384-M03
                    continue
            except OSError:
                continue
        tolerated.add(k)
    real = sorted(moved - tolerated)  # MUTATION-ANCHOR: 1384-M04
    diff = {}
    for k in real:
        other = frozen if before.get(k) != frozen.get(k) else after
        diff[k] = '%s %r != %r' % (k, before.get(k), other.get(k))
    return frozen, tracked, diff, sorted(tolerated)


def settle_fast_verdict(handoff, valid):
    """Cache the fast gate's staged verdict only for a valid run; drop it otherwise.

    setup/test-hw writes `<tree>.fast.tmp.<pid>` and names it in the handoff
    file (staged path, final path, tree). It used to rename it into place
    itself, which happened BEFORE the candidate was re-inventoried, so a run
    this file then refused still left a verdict pre-push would accept.
    """
    try:
        staged, final, tree = handoff.read_text().split('\n')[:3]
    except (OSError, ValueError):
        return
    if not valid:
        try:
            os.unlink(staged)
        except OSError:
            pass
        print('runner: no fast-gate verdict cached — the run was not green or its snapshot did not hold',
              file=sys.stderr, flush=True)
        return
    try:
        os.replace(staged, final)
    except OSError:
        print('runner: could not write the fast-gate verdict at %s — a task/* push will ask for one' % final,
              file=sys.stderr, flush=True)
        return
    print('runner: fast-gate verdict cached for tree %s (gate=fast) — pre-push accepts it for '
          'refs/heads/task/* only, never main' % tree, file=sys.stderr, flush=True)


def main():
    root = Path(sys.argv[1]).resolve()
    with tempfile.TemporaryDirectory(prefix='hw-source-') as temporary:
        base = Path(temporary).resolve()
        if base == root or root in base.parents:
            raise RuntimeError('temporary snapshot directory is inside the source checkout; set TMPDIR outside it')
        candidate = base / 'brain'
        persistent = None
        for attempt in range(1, CAPTURE_ATTEMPTS + 1):
            shutil.rmtree(candidate, ignore_errors=True)
            failure = None
            try:
                frozen, tracked, diff, tolerated = capture(root, candidate)
            except (CaptureDrift, FileNotFoundError, shutil.Error) as error:  # a file that vanished mid-copy is drift too
                failure, diff = error, {}
            if failure is None and not diff:
                break
            if failure is None:
                persistent = set(diff) if persistent is None else persistent & set(diff)
            if attempt == CAPTURE_ATTEMPTS:
                if failure is not None:
                    raise failure
                raise RuntimeError('source or tracking changed during capture; retry from a quiescent checkout'
                                   + ' (%d attempts; kept changing: %s; first differences: %s)' % (
                                       attempt, ', '.join(sorted(persistent)) if persistent else ', '.join(sorted(diff)[:3]),
                                       ', '.join(diff[k] for k in sorted(diff)[:3])))
            time.sleep(CAPTURE_BACKOFF * attempt)
        if tolerated:
            print('runner: the checkout changed during capture only in lane documentation the suite does not read (%s); '
                  'the candidate holds the bytes copied' % ', '.join(tolerated[:5]), file=sys.stderr, flush=True)
        manifest = base / 'tracking.json'
        manifest.write_text(json.dumps(tracked))
        identity = hashlib.sha256(json.dumps(frozen, sort_keys=True).encode()).hexdigest()
        print('# SOURCE SNAPSHOT sha256=' + identity + ' (checkout bytes; explicit live checks remain live)', flush=True)
        # The fast gate's verdict is staged by the run and cached HERE, after the
        # candidate is re-inventoried: see settle_fast_verdict.
        handoff = base / 'fast-verdict.handoff'
        env = dict(os.environ, TEST_HW_SNAPSHOT_ROOT=str(candidate),
                   TEST_HW_TRACKING_MANIFEST=str(manifest), TEST_HW_LIVE_ROOT=str(root),
                   TEST_HW_GIT_USABLE='1' if tracked['usable'] else '0',
                   TEST_HW_FAST_VERDICT_HANDOFF=str(handoff),
                   PYTHONDONTWRITEBYTECODE='1')
        # Copies retain their modes: subjects copy these bytes into writable
        # mutant fixtures. Isolation is a private copy, not a claim of a
        # hostile-process sandbox. Verify it again before releasing a headline.
        # A TERM OR HUP THAT REACHES THIS WRAPPER REACHES THE SUITE TOO. The runner below holds a
        # machine-wide queue ticket, slots and a pile of subject processes, and it already cleans
        # all of it up on INT/TERM (setup/test-hw traps both) — but only if somebody tells it. A
        # `kill`/`pkill`/`timeout` aimed at this process used to end it alone: the runner, no longer
        # anyone's child, went on with parent 1 for hours (measured 2026-10-08: seven of them, one
        # for 3h37m, left by tests/731, each in a `sleep 1` queue loop). The wrapper does not exit
        # on the signal: the runner ends (rc 143), its output is drained, the verdict is settled as
        # not green and the candidate is checked, as for any other failed run.
        child = []
        def forward(signum, _frame):
            if child and child[0].poll() is None:
                try:
                    child[0].send_signal(signum)
                except OSError:
                    pass
        # Windows Python has no SIGHUP: install the handlers for the signals this platform has.
        for signum in [getattr(signal, n) for n in ('SIGTERM', 'SIGHUP') if hasattr(signal, n)]:  # MUTATION-ANCHOR: 881-M01
            signal.signal(signum, forward)  # MUTATION-ANCHOR-END: 881-M01
        proc = subprocess.Popen(['bash', str(candidate / 'setup/test-hw')],
                                cwd=candidate, env=env, stdout=subprocess.PIPE)
        child.append(proc)
        pending = []
        # BYTES, NOT TEXT. A subject may print bytes that are not UTF-8; reading the runner as text
        # raised UnicodeDecodeError here, the wrapper left its TemporaryDirectory (the snapshot) and
        # the runner went on over a deleted tree: every later bash subject read «does not parse».
        out = sys.stdout.buffer
        for line in proc.stdout:  # MUTATION-ANCHOR: 1367-M01
            if pending or re.match(rb'^(?:[0-9]+ tests passed|NO TRACKED NUMBER)', line):
                pending.append(line)
            else:
                out.write(line); out.flush()  # MUTATION-ANCHOR-END: 1367-M01
        rc = proc.wait()
        ended = inventory(candidate)
        intact = frozen == ended
        settle_fast_verdict(handoff, intact and rc == 0)
        if not intact:
            diff = sorted(k for k in set(frozen) | set(ended) if frozen.get(k) != ended.get(k))[:5]
            raise RuntimeError('private candidate changed during execution; no summary is valid'
                               + ' (first differences: %s)' % ', '.join(
                                   '%s %r != %r' % (k, frozen.get(k), ended.get(k)) for k in diff))
        if rc == 0:
            out.write(b''.join(pending)); out.flush()
        return rc


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, RuntimeError, shutil.Error) as error:
        print('not ok - runner snapshot: ' + str(error), file=sys.stderr)
        if isinstance(error, OSError):
            # Not one of the refusals above: say where it came from.
            import traceback
            traceback.print_exc(file=sys.stderr)
        print('\nRUN ABORTED — no number for this run. See stderr for the failure.')
        sys.exit(1)
