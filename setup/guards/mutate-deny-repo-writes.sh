#!/usr/bin/env bash
# Mutate copies only under $HW_ARTIFACTS, then exercise the real hook APIs.
#
# BEFORE 2026-09-06 every arm below named one of four Python or four JavaScript
# lane copies (the JavaScript port is gone, 2026-11-17: pi asks the Python), because that is what existed. There is one of each now, plus
# the lane shims, so every arm names a shared file — and that is a strictly better
# mutation target: a surviving mutant is now a hole in ALL FOUR LANES at once,
# which is exactly the blast radius the unification created and the thing worth
# proving the suite can see.
#
# One arm still names a lane file on purpose (`shim-lane-name`):
# the shims are the only per-lane code left, and an unexercised shim is how a
# lane silently stops being guarded while the shared module stays perfect.
set -euo pipefail

ROOT="$(cd -P "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ART="${HW_ARTIFACTS:-$(mktemp -d "${TMPDIR:-/tmp}/deny-repo-writes-mutants.XXXXXX")}"
BASE="$ART/mutants"
OWN_ART=0
[ -n "${HW_ARTIFACTS:-}" ] || OWN_ART=1
rm -rf "$BASE"
mkdir -p "$BASE"
trap '[ "$OWN_ART" -eq 0 ] || rm -rf "$ART"' EXIT

# lanes_of_guard <python expression of the lane table `LANES`>: what the guard's own table says, printed by python
# (the module path goes through argv, never spliced into the program text).
lanes_of_guard() {
  python3 -I -c '
import sys
sys.path.insert(0, sys.argv[1])
from deny_repo_writes import LANES
print(eval(sys.argv[2])(LANES))' "$ROOT/setup/guards" "$1"
}
copy_tree() {
  local dst="$1" lane
  mkdir -p "$dst/brain/setup/guards"
  cp "$ROOT/setup/guards/deny_repo_writes.py" "$dst/brain/setup/guards/"
  cp "$ROOT/setup/guards/deny_repo_writes_ask.py" "$dst/brain/setup/guards/"
  cp "$ROOT/setup/guards/deny-repo-writes-vectors.json" "$dst/brain/setup/guards/"
  # The batch entry that answers the decision driver's Python vectors
  # in one process; the mutant below breaks only its wiring.
  cp "$ROOT/setup/guards/hook-batch.py" "$dst/brain/setup/guards/"
  # THE POLICY THE MODULES READ, from `<root>/guards.json` beside `setup/`. A
  # copy without it would not load at all, and every arm would read VACUOUS.
  cp "$ROOT/guards.json" "$dst/brain/"
  cp "$ROOT/projects.json" "$dst/brain/"   # `{brain}`/`{work}` in the policy resolve from its `root`
  # DERIVED FROM THE GUARD'S OWN LANE TABLE, NOT A SECOND HARDCODED LIST. This
  # used to be a fixed list of four lanes, and it drifted: a fifth lane was
  # registered in deny_repo_writes.py's LANES table (7a014e1) without this
  # list ever being touched, so every mutation arm crashed on a missing shim
  # before reaching the mutated line — every arm reported VACUOUS, not killed. Measured
  # 2026-09-09, chasing a mandatory pre-push gate that could never turn green.
  # `brain` (the root) is excluded because it is handled as the fifth lane
  # below, at a different path shape.
  for lane in $(lanes_of_guard "lambda L: ' '.join(l for l in L if l != 'brain')"); do
    mkdir -p "$dst/brain/$lane/.claude/hooks"
    cp "$ROOT/$lane/.claude/hooks/deny-repo-writes.py" "$dst/brain/$lane/.claude/hooks/"
  done
  # THE ROOT IS THE FIFTH LANE and its shims live one directory up — see the
  # lane table's `brain` entry. Omitting them here does not make the mutation
  # run weaker, it makes it VACUOUS: the decision driver iterates every lane in
  # the table, so a missing shim crashes the driver before any mutation is exercised
  # and every arm after it reports "the mutated line may never have run".
  mkdir -p "$dst/brain/.claude/hooks"
  cp "$ROOT/.claude/hooks/deny-repo-writes.py" "$dst/brain/.claude/hooks/"
}

_mutate_one() {
  local name="$1" path="$2" old="$3" new="$4" expected="${5:-decision}" dir decision_rc filesystem_rc
  dir="$BASE/$name"
  copy_tree "$dir"
  python3 - "$dir/brain/$path" "$old" "$new" <<'PY'
import pathlib, sys
p = pathlib.Path(sys.argv[1])
s = p.read_text()
old, new = sys.argv[2:]
if s.count(old) != 1:
    raise SystemExit(f"expected exactly one replacement for {old!r}, got {s.count(old)}")
p.write_text(s.replace(old, new, 1))
PY
  decision_rc=0
  filesystem_rc=0
  # ONLY THE DRIVER THIS MUTANT IS JUDGED BY RUNS, AND IT STOPS AT ITS FIRST
  # FAILURE: the verdict below reads `$expected.txt`'s first `not ok -` line and
  # that driver's exit, nothing else. The decision driver was ~5500 python3
  # spawns (~70s of CPU); run in full for all 77 mutants, including the three
  # judged by the filesystem driver, it made this arm ~1949s (2026-09-30). Since
  # 2026-10-20 its Python vectors go through one `hook-batch.py`
  # process and a fixed sample keeps the real spawn (see the driver's header).
  if [ "$expected" = decision ]; then
    DENY_GUARD_FAIL_FAST=1 DENY_GUARD_ROOT="$dir/brain" node "$ROOT/setup/guards/test-deny-repo-writes.mjs" >"$dir/decision.txt" 2>&1 || decision_rc=$?
  fi
  TMPDIR="$dir" DENY_GUARD_ROOT="$dir/brain" node "$ROOT/setup/guards/test-deny-repo-writes-filesystem.mjs" >"$dir/filesystem.txt" 2>&1 || filesystem_rc=$?
  # A NON-ZERO EXIT IS NOT EVIDENCE OF A KILL, and this is the same rule
  # `saw_mutant` enforces in tests/_common.sh (see setup/decisions.md). A driver
  # that crashes on startup — a broken import, a missing fixture, a syntax error
  # the mutation happened to introduce — also exits non-zero, and certifying on
  # that alone passes every mutant that never reached the mutated line. Raised
  # by a Judgment Day judge against this exact function.
  #
  # So the kill must be a `not ok -` line the driver PRINTED: a named assertion
  # that flipped. That is text only a mutant which actually ran can produce.
  local out="$dir/$expected.txt" evidence
  case "$expected" in
    decision)   [ "$decision_rc" -ne 0 ] || { echo "not ok - mutant $name survived"; return 1; } ;;
    filesystem) [ "$filesystem_rc" -ne 0 ] || { echo "not ok - mutant $name survived"; return 1; } ;;
  esac
  evidence="$(grep -m1 '^not ok - ' "$out" 2>/dev/null || true)"
  if [ -z "$evidence" ]; then
    echo "not ok - mutant $name VACUOUS: the $expected driver exited non-zero but printed no failing assertion, so the mutated line may never have run — tail: $(tail -3 "$out" | tr '\n' ' ')"
    return 1
  fi
  echo "ok - mutant $name killed by $expected test ($evidence)"
}

# THE MUTANTS RUN FOUR AT A TIME AND ARE REPORTED IN ORDER. Each one owns
# $BASE/<name>, and both drivers make their own mkdtemp sandbox, so no mutant
# can see another's tree. Run one after another they were this suite's longest
# job: ~380s of wall clock on 2026-09-23 under the runner, most of it node
# start-up, and the guards lane runs nothing else meanwhile. What each mutant
# must show is unchanged; `mutate` only queues it, and `_mutate_report` below
# prints every result in declaration order and stops at the first failure,
# exactly as the serial version did.
MUTATE_JOBS="${MUTATE_JOBS:-4}"
MUTANTS=()
mutate() {
  local name="$1"
  while [ "$(jobs -rp | wc -l | tr -d ' ')" -ge "$MUTATE_JOBS" ]; do sleep 0.1; done
  mkdir -p "$BASE/$name"
  ( _rc=0; _mutate_one "$@" > "$BASE/$name.result" 2>&1 || _rc=$?
    printf '%s' "$_rc" > "$BASE/$name.rc" ) &
  MUTANTS+=("$name")
}
_mutate_report() {
  local name
  wait
  for name in "${MUTANTS[@]}"; do
    cat "$BASE/$name.result"
    [ "$(cat "$BASE/$name.rc" 2>/dev/null || echo missing)" = 0 ] || return 1
  done
}

GUARD_PY="setup/guards/deny_repo_writes.py"

# THE ARMS KILLED BY THE LAST VECTORS COME FIRST. A mutant stops at the first
# failing assertion, so one killed by a vector near the END of the decision
# driver costs the whole ~70s run. Declared last, they started when three of
# the four workers were idle and the job ended on its stragglers (measured
# 2026-10-01 alone: the last four finished at 278, 320, 365 and 369s of 370).
# Started first they overlap everything else. Which mutants run is unchanged;
# only the order they are queued and reported in.
# ── A payload of the wrong shape refuses (2026-09-29, audit F12) ────────────
mutate python-malformed-tool-input "$GUARD_PY" \
  "    if not isinstance(tool_input, dict):" "    if False:"

# ── The batch entry's wiring (2026-10-20) ───────────────────────────────────
# The decision driver answers every Python vector through
# `hook-batch.py`, one process for all of them, and keeps a fixed sample on the
# real `python3 <hook>` spawn whose answer must equal the batch's. A batch that
# answers "allow" for everything breaks nothing in the guard and everything in
# the claim: only the sample can name it, and it runs before the vectors do.
mutate batch-answers-allow "setup/guards/hook-batch.py" \
  '    return {"status": status, "stdout": out.getvalue(), "stderr": err.getvalue()}' \
  '    return {"status": 0, "stdout": "", "stderr": ""}'

# ── The shims, the only per-lane code left ─────────────────────────────────
# A shim that names the wrong lane guards the wrong trees; a shim that cannot
# reach the shared module guards nothing at all. Both must be loud.
# The mutated shim is a lane with a directory of its own (not the root), and
# the name it is swapped to is the next such lane — or, where the table has only
# one besides the root, the root itself. So the mutant names no lane of its own
# and still swaps one real shim for another. The fallback is not hypothetical:
# the exported table carries only `brain` and `setup`, and until 2026-09-29 this
# demanded two lanes besides both, so the export's suite aborted here before
# any mutant ran.
read -r SHIM_LANE SHIM_OTHER <<<"$(lanes_of_guard "lambda L: (lambda own: ' '.join([own[0] if own else '', own[1] if len(own) > 1 else ('brain' if own else '')]))([l for l in L if l != 'brain'])")"
[ -n "$SHIM_OTHER" ] || { echo "shim-lane-name: the lane table has no lane besides brain" >&2; exit 1; }
mutate shim-lane-name "$SHIM_LANE/.claude/hooks/deny-repo-writes.py" \
  "LANE = \"$SHIM_LANE\"" "LANE = \"$SHIM_OTHER\""


# ── The write-detection tables, judged by the filesystem driver (which asks the guard as pi does) or the decision driver ──
mutate copy-write-detection "$GUARD_PY" "rm|mv|cp|rsync|tee" "rm|mv|rsync|tee" filesystem
mutate copy-destination-selection "$GUARD_PY" '    dest = operands[-1]' '    dest = operands[0]' filesystem
mutate mv-both-ways "$GUARD_PY" "rm|mv|cp|rsync|tee" "rm|cp|rsync|tee" filesystem
mutate install-destination "$GUARD_PY" "install|patch|sponge" "patch|sponge"
mutate rsync-destination "$GUARD_PY" "rm|mv|cp|rsync|tee" "rm|mv|cp|tee"

# ── The cwd axis ───────────────────────────────────────────────────────────
mutate python-cwd-boundary "$GUARD_PY" \
  "if cand == root or cand.startswith(root + \"/\"):" "if False:"
# The trigger text became "the command names a protected root (<path>)" on
# 2026-09-07: a lane now protects the other lanes' product repos, so "the repo
# or worktree path" no longer says WHICH tree, and a cross-lane denial that
# cannot name its tree reads to its reader as a misconfiguration.
mutate fired-reason "$GUARD_PY" \
  "the command names a protected root (%s)" "the command path"

# ── Code-vs-data, the escape hatches that keep masking safe ────────────────
mutate shell-c-code "$GUARD_PY" "(?:bash|sh|zsh|dash|ksh|ash)" "(?:sh|zsh|dash|ksh|ash)"

# ── Resolve-then-compare: the gate, both runtimes ──────────────────────────
# These are the arms for the holes measured on 2026-09-06 (a symlink or a `..`
# traversal into a protected tree was ALLOWED by all eight pre-unification
# copies). Turning the resolution off must be visible.
#
# BOTH resolution passes at once, since 2026-09-29: the shell-word pass added
# for relative paths (`_segments_resolve_protected`) also resolves the absolute
# symlink and `..` tokens `_resolves_protected` answers, so turning off only the
# older pass changed no verdict and the arm survived. What must be visible is
# "the gate stopped resolving"; each pass alone has its own arm below.
mutate python-resolution-gate "$GUARD_PY" \
  "        resolved_token = _resolves_protected(cfg, no_heredoc)
    resolved_path = _real(resolved_token) if resolved_token else None
    # And of every word as the shell will see it: relative to the directory
    # its own command runs in, variables and braces expanded, globs matched
    # against the roots (see \`_shell_segments\`).
    if not (names_protected or resolved_token):" \
  "        resolved_token = None
    resolved_path = None
    if False:"
mutate python-realpath "$GUARD_PY" \
  "        real = _real(path)" "        real = path"

# ── Redirect targets go through the fail-closed predicate ──────────────────
#
# BOTH redirect passes at once, since 2026-09-29: the per-command pass
# (`_word_lands` from each command's own directory) resolves every target the
# first pass does, so turning off only the first changed no verdict and the arm
# survived. What must be visible is "no redirect target is resolved any more";
# the per-command pass alone has its own arm (python-segment-redirect).
mutate python-redirect-target "$GUARD_PY" \
  '        if not _lands_outside_protected(cfg, t, cwd):
            resolved = t if _is_abs(t) else _norm(_join(cwd or ".", t))
            if _is_abs(resolved):
                resolved = _real(resolved)
            return t, resolved
    # AND AGAIN FROM WHERE EACH COMMAND REALLY RUNS, with its variables
    # expanded (see `_shell_segments`). Only adds targets: every one above was
    # already asked against the payload'"'"'s cwd.
    for seg in segs:' \
  '        if False:
            return t, t
    for seg in ():'


# ── The masker must honour bash's escapes ──────────────────────────────────
# Found by a Judgment Day judge and MEASURED: without the top-level backslash
# escape, `echo \' && rm -rf <REPO>/x \'` blanks the real `rm` out of the text
# every deny regex reads, and three of four lanes went DENY -> ALLOW.
mutate python-escaped-quote "$GUARD_PY" \
  "        if c == \"\\\\\" and i + 1 < n:" "        if False:"

# An unterminated quote must refuse the masking, not blank the rest of the
# command out of the text every deny regex reads.
mutate python-unbalanced-quote "$GUARD_PY" \
  "    return masked if balanced else text" "    return masked"

# ── The lane-scoped exemption stays scoped ─────────────────────────────────
# Granting the spent-worktree teardown to every lane is the levelling-down this
# unification exists to refuse, so it must not pass unnoticed.
# The anchor moved on 2026-09-07: `roots` is now assembled from the lane's own
# pair PLUS `also_protect`, so the assignment is `cfg["roots"] = tuple(roots)`.
# The mutation itself is unchanged — grant every lane the teardown exemption and
# see whether anything notices.
mutate teardown-stays-lane-scoped "$GUARD_PY" \
  "    cfg[\"roots\"] = tuple(roots)" \
  "    cfg[\"roots\"] = tuple(roots); cfg[\"worktree_roots\"] = cfg[\"worktree_roots\"] or cfg[\"roots\"]"

# ── Destination, not payload: heredoc bodies must not open the gate ────────
# The gate that decides whether to even look at a command must ignore a
# protected root cited only inside a heredoc BODY that is not fed to a shell —
# see the module docstring's 2026-09-09 measurement. Reverting either half to
# scan the raw, unstripped command must turn the new heredoc-citation vectors
# from allow back to deny.
# The gate reads the text in two spellings since 53eacc5 (literal, and with
# backslash escapes removed), BOTH derived from the heredoc-stripped text. The
# mutant moves BOTH to the raw `probe`, so it is still exactly "the gate scans
# heredoc bodies" and nothing else; moving only one would leave the other half
# of the gate stripped and prove less than the pre-53eacc5 arm did.
mutate python-heredoc-gate "$GUARD_PY" \
  'if root in no_heredoc or root in unescaped
                       or root in no_heredoc_c or root in unescaped_c), None)' \
  'if root in probe or root in re.sub(r"\\(.)", r"\1", probe)), None)'

# ── A content-only match must say so ────────────────────────────────────────
# When the only signal is a protected path sitting inside a quoted argument,
# the deny message must admit it could not confirm a destination instead of
# asserting one. Forcing `content_only` permanently false must turn that
# admission back into the ordinary confident wording.
mutate python-content-only-honesty "$GUARD_PY" \
  "        and ((named_root is not None and named_root not in masked" \
  "        and False and ((named_root is not None and named_root not in masked"

# ── Not every heredoc body is data: xargs/patch/ed/ex/interpreters ─────────
# Found adversarially while reviewing the heredoc-gate fix, before it shipped:
# treating EVERY non-shell heredoc target as data would let `xargs -I{} rm {}
# <<EOF` and a `patch <<EOF` whose diff header names a protected path straight
# past the gate. Narrowing this back to shells-only must turn both new vectors
# from deny back to allow.
mutate python-heredoc-body-is-live "$GUARD_PY" \
  '    r"(?:bash|sh|zsh|dash|ksh|ash|xargs|patch|ed|ex|"' \
  '    r"(?:bash|sh|zsh|dash|ksh|ash|"'

# ── The message must name the destination it detected, not just the trigger ─
# Flagged by the brainer's own probe: a $HOME-resolved redirect target was
# correctly still denied, but the message described a literal-string test
# this branch does not run. Losing the resolved-destination statement must be
# visible.
mutate python-redirect-states-destination "$GUARD_PY" \
  '                    "redirect'"'"'s destination resolves to %s (written as "' \
  '                    "redirect target is protected: %s (spelled as "'

# ── The content-only fallback must point at the mechanism that works ───────
# A preservation copy through an interpreter cannot be told apart from a real
# write and stays denied (the safe default), but the message must not leave
# the operator without a way forward: `cp -R`/`rsync -a`/`install` already
# parse source from destination correctly. Losing that pointer must be visible.
mutate python-content-only-preservation-pointer "$GUARD_PY" \
  '                    "as this trigger. If you are instead trying to PRESERVE "' \
  '                    "as this trigger. If you are instead trying to sleep "'

# ── A spelling is not an identity ──────────────────────────────────────────
# The filesystem answer to "is this path the protected tree?" — same inode,
# whatever the case — is what keeps a case-flipped root spelling out on APFS. Dropping
# it leaves every text comparison intact and every lower-case vector green, so
# only the sandbox case vectors can see it. Those vectors assert identity only
# where the filesystem folds case (APFS, NTFS); on ext4 a case-variant spelling
# is another, absent path, nothing can kill these two, and they are skipped by
# name — the same probe the vector driver makes.
mkdir -p "$BASE/case-probe"
if [ -e "$BASE/CASE-PROBE" ]; then
  mutate python-same-tree-identity "$GUARD_PY" \
      '    return any(_same_tree(cfg, cand) for cand in cands)' \
      '    return False'
else
  printf 'skip - mutant python-same-tree-identity: this filesystem is case-sensitive, so no case-variant spelling names the protected tree\n'
fi

# ── Where a word really lands (2026-09-29, audit F5-F9) ─────────────────────
# Each piece of the shell reading must be visible on its own: the gate asking
# relative/expanded words, the redirect asked from its own command's directory,
# a cd that lands inside, local assignments, braces, globs and find/fd.
mutate python-segment-gate "$GUARD_PY" \
  "        seg_hit = _segments_resolve_protected(cfg, segs)" "        seg_hit = None"
mutate python-segment-redirect "$GUARD_PY" \
  '            landed = _word_lands(cfg, value, seg["cwd"], bare=True)' "            landed = None"
mutate python-segment-cd "$GUARD_PY" \
  "        cds_into_protected = any(" "        cds_into_protected = False and any("
mutate python-local-assignment "$GUARD_PY" \
  "                self.local[name] = rhs" "                pass"
mutate python-brace "$GUARD_PY" "    for v in _brace(value):" "    for v in [value]:"
mutate python-glob "$GUARD_PY" \
  " or (_GLOB_CHARS.search(v) and _glob_lands(cfg, _norm(p))):" ":"
mutate python-find-writes "$GUARD_PY" \
  "        for m in FIND_WRITES.finditer(masked)" "        for m in FIND_WRITES.finditer(\"\")"
# Judgment Day, 2026-09-29: cd behind a reserved word or wrapper, `sh -c` and
# `eval` bodies, bare names as arguments, and a redirect only the quoted body has.
mutate python-keyword-skip "$GUARD_PY" \
  "                              or words[k][0] in skip):" \
  "                              or False):"
mutate python-dash-c-nesting "$GUARD_PY" "        if name in _SHELLS:" "        if False:"
mutate python-eval-nesting "$GUARD_PY" '        elif name == "eval":' "        elif False:"
# ── A backslash-newline joins, an eval's quoted words are code ─────────────
# (2026-09-30, Judgment Day of guard-gate-relative-paths)
mutate python-line-continuation "$GUARD_PY" \
  "    copies = _join_continuations(no_heredoc)" "    copies = [no_heredoc]"
mutate python-comment-ends-continuation "$GUARD_PY" \
  '        elif quote is None and c == "#" and (not out or out[-1][-1] in _WORD_START):' "        elif False:"
mutate python-blind-join-too "$GUARD_PY" \
  '    return [aware] if blind == aware else [aware, blind]' "    return [aware]"
# Masking both copies as ONE text lets a comment's quote pair across them
# (Judgment Day round 2 of guarda-eval-y-find-multilinea).
mutate python-mask-each-copy "$GUARD_PY" \
  "    masked = _COPY_BOUNDARY.join(_mask_copy(c) for c in copies)" \
  "    masked = _mask_copy(_COPY_BOUNDARY.join(copies))"
mutate python-eval-body-unmasked "$GUARD_PY" \
  "or (has_eval and _EVAL_ARGS.search(text[:quote_start]))" "or False"
mutate python-bare-word "$GUARD_PY" \
  '                    landed = v and _word_lands(cfg, v, d, bare=True)' \
  '                    landed = v and _word_lands(cfg, v, d)'
mutate python-nested-redirect-gate "$GUARD_PY" \
  '    if REDIRECT.search(masked) or any(seg["redirects"] for seg in segs):' \
  "    if REDIRECT.search(masked):"

# ── env/nohup/exec do not move the shell's cwd; the start cwd is asked too ──
# (2026-09-29, Judgment Day re-judgment)
mutate python-env-cd-moves-cwd "$GUARD_PY" \
  "        ck = _head_index(words)" "        ck = _head_index(words, _SKIP_WORDS)"
mutate python-start-cwd-ask "$GUARD_PY" \
  '        if seg.get("start") and seg["start"] != seg["cwd"]:' "        if False:"

_mutate_report
