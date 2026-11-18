#!/usr/bin/env bash
# The delivery lock of channel-send must know WHO holds it, and must release only what is still theirs.
#
# WHY. The lock recorded its holder, and then decided whether that holder "is a sender" by testing
# `ps -o command=` for the substring `channel-send`. That answers a different question from the one
# being asked. Three ordinary things remove the string from a real sender — a wrapper script in front
# of it, a symlink under another name, and `ps` truncating its column — and each one makes the check
# BREAK A LIVE SENDER'S LOCK, which lets two senders into one exchange: the exact failure the lock
# exists to prevent, produced by the check meant to protect it.
#
# The invoker run lock this subject also held (bin/invoker-common.sh) went with that file: the invokers
# are Go, and their lock is core/invokers'.
. "$(dirname "${BASH_SOURCE[0]}")/_common.sh"

# ── channel-send's delivery lock ────────────────────────────────────────────
# A COMPLETE COPY, because channel-send resolves its neighbours from its own
# directory. Same pattern as 33-killed-sender.sh.
cs="$TMP/cs"; mkdir -p "$cs/bin" "$cs/tmp"
cp "$ROOT/bin/channel-send" "$cs/bin/"
cat > "$cs/bin/herdr-rpc" <<'STUB'
#!/usr/bin/env bash
sleep "${STUB_WAIT_S:-0}"
exit "${STUB_RPC_RC:-0}"
STUB
cat > "$cs/bin/herdr" <<'STUB'
#!/usr/bin/env bash
echo '{"result":{}}'
STUB
chmod +x "$cs/bin"/*

lock="$cs/tmp/hw-deliver-wT_p1.lock"
cs_probe() {   # -> the probe's output; it must not block for a real budget
  env PATH="$cs/bin:$PATH" TMPDIR="$cs/tmp" STUB_WAIT_S=0 STUB_RPC_RC=3 \
    HW_INVOKER_WAIT_MS=2000 "$cs/bin/channel-send" herdr wT:p1 - "probe" 2>&1 || true
}
# A 2s BUDGET IS A 7s LOCK WAIT (budget + channel-send's fixed 5s). The arms
# that wait on a live holder read what the loop says on its first pass and then
# watch it keep the lock until it gives up; at 20000 that was 25s per arm for
# the same observation.

# 5. A REAL SENDER UNDER A NAME `ps` DOES NOT SPELL `channel-send` MUST NOT LOSE
# ITS LOCK. This is the defect: the holder is a genuine sender invoked through a
# symlink, so its command line carries the LINK's name.
# A SYMLINK UNDER ANOTHER NAME, not a wrapper that `exec`s. `exec` replaces the
# process image, so `ps -o command=` still spells `channel-send` and the old
# check survives it — checked while writing this file. A symlink changes what
# `ps` reports (it shows the path the script was invoked through) without
# changing which program is running, which is precisely the case that makes
# "does the command line contain this string" the wrong question.
ln -sf "$cs/bin/channel-send" "$cs/bin/deliver"
[ -x "$cs/bin/deliver" ] || fail "the symlinked sender is not executable"
rm -rf "$lock"; mkdir -p "$lock"
env PATH="$cs/bin:$PATH" TMPDIR="$cs/tmp" STUB_WAIT_S=60 STUB_RPC_RC=2 \
  HW_INVOKER_WAIT_MS=60000 "$cs/bin/deliver" herdr wT:p9 - "holder" >/dev/null 2>&1 &
holder=$!
# WAIT FOR THE FACT, NOT FOR A DURATION. This was `sleep 1`, and one second is a
# bet on the scheduler, not a property of the code: this repository runs several
# executors and whole test suites at once, and on 2026-09-09 this assertion
# failed three times in a row while two suites and two review subagents were
# running — then passed, unchanged, on a quiet machine. Both channel-send
# versions either side of that day's commit pass in isolation, so what the arm
# had started measuring was the load average.
#
# A test that fails under load and passes idle teaches everyone to re-run it,
# which is how a real regression gets waved through. Poll for the file the
# sender is supposed to write, with a bound, and say which of the two failures
# happened.
holder_start="$cs/tmp/hw-deliver-wT_p9.lock/owner.start"
waited=0
while [ ! -s "$holder_start" ] && [ "$waited" -lt 100 ]; do
  sleep 0.2; waited=$((waited + 1))
done
# Its own lock is on wT:p9; its pid goes into the wT:p1 lock, which is the
# situation the identity check has to read correctly.
printf '%s\n' "$holder" > "$lock/owner.pid"
if [ ! -s "$holder_start" ] && ! kill -0 "$holder" 2>/dev/null; then
  fail "the backgrounded sender exited before recording anything — it never reached the lock, so this arm measured nothing"
fi
cp "$holder_start" "$lock/owner.start" 2>/dev/null \
  || fail "the sender recorded no owner.start after $((waited / 5))s, so identity still rests on its name"
pass "delivery lock: a sender records WHEN it started, not only its pid"
# The recorded start is the wrapper's exec'd process — the same process — so it
# must match, and the name must be irrelevant.
OUT="$(cs_probe)"
kill -TERM "$holder" 2>/dev/null || true
wait "$holder" 2>/dev/null || true
case "$OUT" in
  *"holder is gone"*) fail "the fixture's holder died, so this arm proved nothing: $OUT" ;;
  *"NOT a sender"*) fail "a live sender behind a symlink lost its lock to the command-name check: $OUT" ;;
  *"pid was REUSED"*) fail "a live sender behind a symlink was called a reused pid: $OUT" ;;
  *"another sender holds"*) pass "delivery lock: a real sender invoked through a symlink keeps its lock — identity is the process, not the name" ;;
  *) fail "unexpected behaviour against a symlinked sender's lock: $OUT" ;;
esac
rm -rf "$lock" "$cs/tmp/hw-deliver-wT_p9.lock"

# 6. AND A REUSED PID IS BROKEN AT ONCE, positively rather than by inference
# from a name. Same pid, a start time that is not the one recorded.
rm -rf "$lock"; mkdir -p "$lock"
sleep 120 & impostor=$!
printf '%s\n' "$impostor" > "$lock/owner.pid"
printf 'Thu Jan  1 00:00:00 1970\n' > "$lock/owner.start"
t0="$(date +%s)"
OUT="$(cs_probe)"
elapsed=$(( $(date +%s) - t0 ))
kill "$impostor" 2>/dev/null || true
rm -rf "$lock"
case "$OUT" in
  *"pid was REUSED"*) pass "delivery lock: a pid whose start time is not the recorded one is named as reused" ;;
  *) fail "a reused pid was not identified: $OUT" ;;
esac
case "$OUT" in
  *"1970"*) pass "delivery lock: it prints both start times, so the operator can see why" ;;
  *) fail "the reused-pid message does not show the times it compared: $OUT" ;;
esac
[ "$elapsed" -le $((10 * HW_TEST_SLOW)) ] \
  && pass "delivery lock: it breaks that lock in ${elapsed}s instead of spending the budget" \
  || fail "it spent ${elapsed}s on a lock whose holder pid was reused"

# 7. UNREADABLE IS STILL NOT A NEGATIVE ANSWER — the asymmetry every check in
# this toolchain rests on. `ps` unavailable must leave the lock alone.
cat > "$cs/bin/ps" <<'STUB'
#!/usr/bin/env bash
exit 1
STUB
chmod +x "$cs/bin/ps"
rm -rf "$lock"; mkdir -p "$lock"
sleep 120 & impostor=$!
printf '%s\n' "$impostor" > "$lock/owner.pid"
printf 'Thu Jan  1 00:00:00 1970\n' > "$lock/owner.start"
OUT="$(cs_probe)"
kill "$impostor" 2>/dev/null || true
rm -rf "$lock"; rm -f "$cs/bin/ps"
case "$OUT" in
  *"pid was REUSED"*) fail "it broke a lock on a holder whose start time it could not read: $OUT" ;;
  *"NOT a sender"*) fail "it broke a lock on a holder it could not identify at all: $OUT" ;;
  *"another sender holds"*) pass "delivery lock: a holder ps cannot describe is left alone — unreadable is not a negative answer" ;;
  *) fail "unexpected behaviour when ps is unavailable: $OUT" ;;
esac

# MUTANT M03 (identity by command name only, which is what shipped: the symlinked sender of arm 5 then loses its lock) edited a line of
# bin/channel-send's bash and went with it (go-060 S5: the file is a shim). It is held by the Go anchor 101-G03 (core/channel_send_herdr.go,
# `if startWas != ""`, killed in process by setup/tests/1016 and judged by TestChannelSendLock); arms 5-7 above still run the symlinked
# sender, the reused pid and the unreadable `ps` through the shim.

