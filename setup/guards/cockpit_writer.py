#!/usr/bin/env python3
"""A brainer's SessionStart makes sure its cockpit writer is running.

WHY. The cockpit shows a brainer from a state file that ONE writer per pane keeps fresh
(`bin/cockpit-state --loop 5 --bind-pane`). Only `brain <lane>` started that writer, so a
brainer that came back any other way — `claude --resume <id>` typed into a terminal the core
revived after a reboot, a writer that died — drew «NO STATE — no writer's file is live»
(MEASURED 2026-11-13: the revived brainers, until a person started the writers by hand).
SessionStart fires on every start AND resume, whoever typed the command, so it is the one place
that cannot be skipped.

IDEMPOTENT BY THE WRITER'S OWN LOCK: `cockpit-state --bind-pane` takes `<state>.loop.lock` and a
second writer for the same pane finds it held and leaves. So this starts one unconditionally, and
never counts processes. It starts nothing for an executor (HW_TASK/HW_RUN: the cockpit is the
brainer's), with FOREMAN_COCKPIT=0 (the same switch `brain --no-cockpit` honours), outside a pane
(no HC_TERMINAL_ID and no HERDR_PANE_ID), or where bin/cockpit-state is not there.

Nothing here may cost the session: every failure is silence (the caller wraps it too).
"""
import os
import subprocess


def _root(env):
    return env.get("HW_BRAIN_ROOT") or os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def ensure(lane, env=None):
    """Start the writer for this pane; the pane it was started for, or "" when it started none."""
    env = dict(os.environ if env is None else env)
    if env.get("HW_TASK") or env.get("HW_RUN") or env.get("FOREMAN_COCKPIT") == "0":
        return ""
    pane = env.get("HC_TERMINAL_ID") or env.get("HERDR_PANE_ID") or ""
    root = _root(env)
    exe = os.path.join(root, "bin", "cockpit-state")
    if not pane or not lane or not (os.path.isfile(exe) and os.access(exe, os.X_OK)):
        return ""
    env.setdefault("HW_BRAIN_ROOT", root)
    subprocess.Popen([exe, "--loop", "5", "--bind-pane", "--invoker", pane, "--lane", lane],
                     stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                     env=env, start_new_session=True)
    return pane
