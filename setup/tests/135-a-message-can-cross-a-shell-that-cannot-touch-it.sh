#!/usr/bin/env bash
# MEASURED today: a ruling correcting a contract challenge reached its executor
# with every backticked identifier MISSING. Nothing in ask-invoker,
# done-invoker or channel-send evaluates its own text (no `eval` anywhere in
# the three) — the substitution happened in the BRAINER's own shell, before any
# of these tools ran, because the text sat inside a double-quoted argv word and
# bash expands backticks (and `$…`) inside double quotes regardless of what the
# receiving program does with the string afterward. Quoting harder on the
# sending side cannot fix this; the only fix is a way to hand over the text
# that the sending shell does not interpolate at all.
#
# WHY THIS IS MORE SERIOUS THAN A COSMETIC LOSS. If backticks are substituted,
# a ruling containing `` `something` `` EXECUTES `something` in the brainer's
# own shell. Today it cost words; the same mechanism with different text is
# accidental command execution. A channel the sending shell cannot touch at all
# removes the class, not just today's instance.
#
# THE FIX FOLLOWS AN EXISTING CONVENTION RATHER THAN INVENTING ONE:
# channel-send's own usage already documents `<endpoint|->` — a lone '-' as a
# sentinel in a positional slot. This applies the SAME sentinel to the message
# slot, in all four commands that receive free text: ask-invoker, done-invoker,
# channel-send, and `hw ruling`. A caller who writes the text with a
# single-quoted heredoc (`<<'EOF' ... EOF`, which bash does NOT expand) and
# pipes it in through a lone '-' never lets the sending shell see backticks,
# `$`, embedded quotes, or newlines as anything but bytes.
#
# THREE ERRORS, THREE MESSAGES — a message given both ways (argv text AND '-'),
# an empty stdin read, and (documented, not exercised here because it needs a
# real tty) reading '-' with nothing piped in at all. An empty message
# delivered would be worse than any one of these refusals, so each is checked.
#
# ADDITIVE, NOT REPLACING: every existing argv-words invocation must still work
# exactly as before. This is checked first, for all four commands, before any
# of the new stdin behaviour is exercised.
#
# Run alone while working on this subject:
#     bash setup/tests/135-a-message-can-cross-a-shell-that-cannot-touch-it.sh
. "$(dirname "${BASH_SOURCE[0]}")/_common.sh"
need_invokers   # this subject runs ask-invoker and done-invoker, which are Go (go-060 S2): HW_INVOKERS names the built binary

CS_SRC="$ROOT/bin/channel-send"
AI_SRC="$ROOT/bin/ask-invoker"
DI_SRC="$ROOT/bin/done-invoker"
HW_SRC="${HW_SOURCE:-$ROOT/bin/hw}"

# A payload with every character class the incident actually lost: a
# backtick pair (which bash would run as a command substitution if it ever sat
# inside a double-quoted argv word), a `$` expansion, and an embedded quote.
PAYLOAD='fix touches `deny-repo-writes.py`; cost is $0 either way, says "the brief"'

# ── herdr stub: proves the BYTES, not just that something was accepted ─────
# channel-send's herdr route runs `herdr agent prompt <target> <message>` directly (no shell re-interpretation of its
# own), so logging herdr's argv verbatim is a faithful witness of exactly what channel-send decided the message was —
# the strongest evidence available without a live receiver. (A copy of bin/ beside stub herdr/herdr-rpc, the way
# setup/tests/05 runs channel-send; a budget of one witness slice is ONE wait-agent, answered at once.)
csb="$TMP/csb"; mkdir -p "$csb/bin"
cp "$ROOT/bin/channel-send" "$csb/bin/"
[ ! -f "$ROOT/bin/invoker-common.sh" ] || cp "$ROOT/bin/invoker-common.sh" "$ROOT/bin/runenv" "$csb/bin/"
cat > "$csb/bin/herdr" <<'STUB'
#!/usr/bin/env bash
case "${1:-} ${2:-}" in
  "agent prompt") printf '%s' "$4" > "$PROMPT_LOG"; echo '{"result":{}}' ;;
  "agent get") printf '{"result":{"agent":{"agent_status":"idle","cwd":"","tokens":{}}}}\n' ;;
  *) echo '{"result":{}}' ;;
esac
STUB
cat > "$csb/bin/herdr-rpc" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
chmod +x "$csb/bin/herdr" "$csb/bin/herdr-rpc"
PROMPT_LOG="$TMP/prompt.log"; export PROMPT_LOG
export WITNESS_SLICE_MS=100000000

send_herdr() {
  : > "$PROMPT_LOG"
  PATH="$csb/bin:$PATH" "$csb/bin/channel-send" herdr wT:p1 - "$@" 2>&1
}
message_logged() { cat "$PROMPT_LOG"; }

# ── 1. EXISTING ARGV INVOCATIONS STILL WORK, unchanged, for all four ────────
out="$(send_herdr plain unquoted words)"
[ "$?" = 0 ] || fail "channel-send's ordinary argv path regressed: $out"
[ "$(message_logged)" = "plain unquoted words" ] || fail "argv words were not joined into one message as before: $(message_logged)"
pass "channel-send's existing argv-words message path is unchanged"

out="$(env -u HW_INVOKER_PANE "$AI_SRC" "a plain question" 2>&1 || true)"
case "$out" in *"no HW_INVOKER_PANE"*) pass "ask-invoker's existing argv-words question path still reaches its normal refusal" ;; *) fail "ask-invoker's argv path regressed: $out" ;; esac

out="$(env -u HW_INVOKER_PANE "$DI_SRC" "a plain summary" 2>&1 || true)"
case "$out" in *"no HW_INVOKER_PANE"*) pass "done-invoker's existing argv-words summary path still reaches its normal refusal" ;; *) fail "done-invoker's argv path regressed: $out" ;; esac

out="$(env -u HW_INVOKER_PANE "$HW_SRC" ruling nopane "a plain ruling" 2>&1 || true)"
case "$out" in *"herdr reports no cwd"*) pass "hw ruling's existing argv-words text path still reaches its normal refusal" ;; *) fail "hw ruling's argv path regressed: $out" ;; esac

# ── 2. '-' DELIVERS THE EXACT BYTES, backticks and '$' untouched ────────────
out="$(printf '%s' "$PAYLOAD" | send_herdr -)"
[ "$?" = 0 ] || fail "channel-send's stdin path failed to deliver: $out"
[ "$(message_logged)" = "$PAYLOAD" ] || fail "the message read from stdin does not match what was piped in — got: $(message_logged)"
pass "channel-send delivers a backtick-and-\$-bearing message byte-for-byte when given as '-'"

out="$(printf '%s' "$PAYLOAD" | env -u HW_INVOKER_PANE "$AI_SRC" - 2>&1 || true)"
case "$out" in
  *"no HW_INVOKER_PANE"*) pass "ask-invoker accepts '-' and the backtick/\$ payload survives past its own checks" ;;
  *) fail "ask-invoker's '-' path did not reach the normal refusal (payload may have been mangled or rejected): $out" ;;
esac

out="$(printf '%s' "$PAYLOAD" | env -u HW_INVOKER_PANE "$DI_SRC" - 2>&1 || true)"
case "$out" in
  *"no HW_INVOKER_PANE"*) pass "done-invoker accepts '-' and the backtick/\$ payload survives past its own checks" ;;
  *) fail "done-invoker's '-' path did not reach the normal refusal (payload may have been mangled or rejected): $out" ;;
esac

out="$(printf '%s' "$PAYLOAD" | env -u HW_INVOKER_PANE "$HW_SRC" ruling nopane - 2>&1 || true)"
case "$out" in
  *"herdr reports no cwd"*) pass "hw ruling accepts '-' and the backtick/\$ payload survives past its own checks" ;;
  *) fail "hw ruling's '-' path did not reach the normal refusal (payload may have been mangled or rejected): $out" ;;
esac

# ── 3. THE THREE DISTINCT ERRORS, for all four commands ─────────────────────

# 3a. empty stdin
out="$(printf '' | send_herdr -)" && rc=0 || rc=$?
[ "$rc" != 0 ] || fail "channel-send accepted an empty stdin message"
case "$out" in *"stdin for '-' produced no bytes"*) pass "channel-send names an empty '-' read distinctly" ;; *) fail "channel-send's empty-stdin refusal is missing or generic: $out" ;; esac

out="$(printf '' | env -u HW_INVOKER_PANE "$AI_SRC" - 2>&1 || true)"
case "$out" in *"stdin for '-' produced no bytes"*) pass "ask-invoker names an empty '-' read distinctly" ;; *) fail "ask-invoker's empty-stdin refusal is missing or generic: $out" ;; esac

out="$(printf '' | env -u HW_INVOKER_PANE "$DI_SRC" - 2>&1 || true)"
case "$out" in *"stdin for '-' produced no bytes"*) pass "done-invoker names an empty '-' read distinctly" ;; *) fail "done-invoker's empty-stdin refusal is missing or generic: $out" ;; esac

out="$(printf '' | env -u HW_INVOKER_PANE "$HW_SRC" ruling nopane - 2>&1 || true)"
case "$out" in *"stdin for '-' produced no bytes"*) pass "hw ruling names an empty '-' read distinctly" ;; *) fail "hw ruling's empty-stdin refusal is missing or generic: $out" ;; esac

# 3b. text given BOTH ways — argv words AND '-'
out="$(printf 'x' | send_herdr - "extra words")" && rc=0 || rc=$?
[ "$rc" != 0 ] || fail "channel-send accepted a message given both as argv text and '-'"
case "$out" in *"given twice"*) pass "channel-send refuses a message given both ways, distinctly from the empty case" ;; *) fail "channel-send's double-channel refusal is missing or reuses the empty-stdin wording: $out" ;; esac

out="$(printf 'x' | env -u HW_INVOKER_PANE "$AI_SRC" - "extra words" 2>&1 || true)"
case "$out" in *"given on argv AND as '-'"*) pass "ask-invoker refuses a question given both ways, distinctly from the empty case" ;; *) fail "ask-invoker's double-channel refusal is missing or reuses the empty-stdin wording: $out" ;; esac

out="$(printf 'x' | env -u HW_INVOKER_PANE "$DI_SRC" - "extra words" 2>&1 || true)"
case "$out" in *"given on argv AND as '-'"*) pass "done-invoker refuses a summary given both ways, distinctly from the empty case" ;; *) fail "done-invoker's double-channel refusal is missing or reuses the empty-stdin wording: $out" ;; esac

out="$(printf 'x' | env -u HW_INVOKER_PANE "$HW_SRC" ruling nopane - "extra words" 2>&1 || true)"
case "$out" in *"given on argv AND as '-'"*) pass "hw ruling refuses a correction given both ways, distinctly from the empty case" ;; *) fail "hw ruling's double-channel refusal is missing or reuses the empty-stdin wording: $out" ;; esac

# ── mutants ───────────────────────────────────────────────────────────────────
# THE TWO BASH MUTANTS WENT WITH THE BASH (go-060 S5: bin/channel-send and bin/done-invoker are shims). What each held is held in Go:
#   M01 channel-send's '-' branch dropped, so a lone '-' is sent as the literal word      135-G01, core/channel_send.go (`if args[0] == "-"`),
#       setup/tests/1016 kills it in process, judged by TestChannelSendMessageFromStdin
#   M02 done-invoker's double-channel check dropped, so text given both ways is accepted  135-M02, core/invokers/done.go (`if len(args) != 1`),
#       core/invokers/mutants_test.go, judged by TestDiffDone/stdin_and_argv
# Sections 1-3 above still run the four commands through their shims, byte for byte.
printf 'coverage - 135: the argv, "-", empty and double-channel claims of the four commands through the shims, 0 mutants (the 2 bash mutants are retired; 135-G01 is killed by 1016, 135-M02 by core/invokers)\n'
