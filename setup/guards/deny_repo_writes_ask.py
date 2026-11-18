#!/usr/bin/env python3
"""The read-only guard, asked by the pi executor (`setup/pi/hw-executor.ts`).

THE DECISION IS NOT HERE. It is in `deny_repo_writes.py` beside this file, the one the Claude hooks run. pi has no
settings hooks, so its adapter spawns this per question, with one JSON request on stdin and one JSON answer on stdout:

  {"lane": L, "op": "check"}                          -> {}                       (the lane resolves)
  {"lane": L, "op": "decide", "command": C, "cwd": D} -> {"reason": R | null}     (`decide`: R refuses)
  {"lane": L, "op": "inside", "paths": [P, ...]}      -> {"inside": [bool, ...]}  (each P, absolute, in a protected tree)
  {"lane": L, "op": "product_repos"}                  -> {"repos": [...]}

Nothing here decides anything. FAILS CLOSED by exit code: an unknown lane, a request that is not one of the above, a
crash, all end non-zero with no JSON, and the adapter refuses on anything but exit 0 with a JSON object.
"""
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import deny_repo_writes as guard  # noqa: E402


def answer(req):
    cfg = guard.config(req["lane"])
    op = req["op"]
    if op == "check":
        return {}
    if op == "decide":
        verdict = guard.decide(cfg, req["command"], req["cwd"])
        return {"reason": verdict[1] if verdict else None}
    if op == "inside":
        return {"inside": [bool(guard._inside_protected(cfg, p)) for p in req["paths"]]}
    if op == "product_repos":
        return {"repos": list(guard.PRODUCT_REPOS)}
    raise ValueError("unknown op %r" % (op,))


if __name__ == "__main__":
    print(json.dumps(answer(json.load(sys.stdin))))
