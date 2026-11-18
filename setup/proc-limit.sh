# proc-limit.sh — what this user's process table allows, in one place. Sourced by setup/heavy (its process-headroom refusal) and setup/go-test-core.sh
# (the FORK FAILED report), so the two never name different limits for one machine. Held by setup/tests/1387.
#
# proc_limit   the number fork(2) enforces: the smaller of kern.maxprocperuid and `ulimit -u`; HW_HEAVY_PROC_LIMIT forces it (a subject's stub). Empty when the
#              machine cannot say (a number that is not one, `unlimited`).
# proc_list    this user's processes, `pid ppid command` per line (a subject stubs `ps` on PATH).
proc_limit() {
  if [ -n "${HW_HEAVY_PROC_LIMIT:-}" ]; then printf '%s' "$HW_HEAVY_PROC_LIMIT"; return; fi
  { sysctl -n kern.maxprocperuid 2>/dev/null; ulimit -u 2>/dev/null; } | awk '/^[0-9]+$/ && (m == "" || $1 + 0 < m + 0) { m = $1 } END { print m }'
}
proc_list() { ps -U "$(id -u)" -o pid=,ppid=,comm= 2>/dev/null; }
