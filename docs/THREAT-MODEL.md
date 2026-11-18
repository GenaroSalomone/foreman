# Threat model

What foreman's guards are built to stop, what they are not, and what the
harness assumes about the machine it runs on. The limitations register,
[`KNOWN-LIMITATIONS.md`](KNOWN-LIMITATIONS.md), carries the measured detail
(L2 for the guards).

## In short

- **Assumed:** one operator, one machine, one OS account, and agents that are
  cooperative but fallible, not adversarial.
- **Stopped:** a brainer's write into a protected repository, and an
  executor's reach into the brain, by guards that are a tripwire.
- **Not stopped:** a path computed at run time, an executor's writes outside the
  brain.
- **Not a goal:** containing an agent that works to get around the guards. For a
  hard boundary, see the last section.

## What the harness assumes

- One operator, one machine, one OS account. Every brainer and executor runs
  as that account, with the agent's permission prompts turned off by default
  (`--dangerously-skip-permissions` for Claude Code, pi has no permission prompts).
  That is the recommended setting, not a fixed one: `hw --permissions ask`
  leaves the prompts on, and the guards below do not depend on it.
- The agents are **cooperative but fallible**: they follow instructions, and
  sometimes follow wrong ones. They are not assumed to be adversarial.
- The operator trusts the content of the product repositories, the briefs, and
  the agent runtimes (Claude Code, pi, herdr) and what they send
  to a hook.

## What the guards stop — a tripwire

| Actor | Guard | Stops |
|---|---|---|
| Brainer (Claude Code) | the repo-write hook plus `Edit`/`Write` deny rules | a write or a git mutation inside a protected repository or one of its live worktrees (the ones `guards.json` names), spelled with an absolute or relative path, `~`, `$HOME`, an exported or a shell-local variable, a symlink chain, `..`, `//`, `/./`, a letter-case variant, a glob or a brace expansion, or made after a `cd` into it earlier in the same command; `find -delete`/`-exec` and `fd -x`; a command nested in `sh -c` or `eval` with a literal body |
| Product executor (Claude Code, pi) | the reverse guard | naming the brain except to read it or run its `bin/` tools; a brainer's `hw` verb, or `channel-send` to any pane but its own brainer's; a brain path in an unpushed commit |
| An executor whose lane's repository is the brain itself | none | — |

**Fail-closed, where it is built to.** The guards refuse on a missing or
malformed `guards.json` and a missing shared module. The Python guards (the
Claude Code hooks) refuse a payload that is not an object
or a shell call whose input is not an object. The Claude Code
brainer hook also refuses when it crashes while deciding. The reverse guard, on
Claude Code, refuses when there is no `python3` to run it. Not covered:
input that is not JSON at all names no tool and is allowed.

## What they do not stop

- **Brainer:** a path computed at run time — `eval` or `sh -c` over a
  substitution, `$(…)`, a script, a `while read` loop.
- **Executor:** everything outside the brain: the home directory and its
  dotfiles, the agents' own configuration, `~/.local/bin`, LaunchAgents,
  crontab, `git config --global`, other repositories, `git push --force` or
  `--delete`, `branch -D`, the keychain, the network.
- **Executor → control plane:** the brain's `bin/` tools are allowed to an
  executor so it can report. `hw` refuses it the brainer's verbs (re-task,
  rule, restart, reap, close another task, dispatch), and the reverse guard
  allows only its own `hw` verbs and `channel-send` toward its own brainer;
  the guard reads the command it is shown, so a script gets past it, and
  `hw`'s check reads the executor's environment, so unsetting `HW_TASK` and
  `HW_RUN` gets past that one (the guard refuses that spelling).
- **Content:** a product repository's own versioned agent configuration
  (Claude Code settings and hooks, `CLAUDE.md`) loads in the
  executor with the operator's rights. An executor's report text arrives as a
  prompt in the brainer's session.

## Injection: content that carries instructions

Two routes put text or code in front of an agent that runs without permission
prompts.

**A product repository's own content.** An executor works in a worktree of the
repository, so whatever the repository versions loads with the operator's
rights: Claude Code hooks in `.claude/settings.json`,
`CLAUDE.md` and other instruction files. A change merged into the repository
therefore runs on the operator's machine at the next dispatch, and text in a
file can steer the executor toward anything the executor is allowed to do
(see the executor row above: the account's whole reach outside the brain).
*Mitigation today:* `hw` reads a worktree's versioned hook settings before it
hands over a brief, but only to refuse a hook that names a binary missing from
`PATH`; it does not judge what a hook does, and there is no trust prompt for a
repository's hooks. Merging stays with the operator, who is the only review of
that content.

**A report that returns to the brainer.** The text of a `done-invoker` report
arrives as a prompt in the brainer's session, which also runs without
permission prompts and holds every `hw` verb. An executor that read hostile
content can pass it on inside its report. *Mitigation today:* `done-invoker`'s
report is capped in length and forbidden code fences, but an executor can also
reach the brainer through `channel-send`, which has no such limit, so the limit is friction and not a barrier; and the brainer's own guard still
refuses its writes into protected repositories; nothing filters or marks the
report's text as untrusted, and the brainer can still dispatch, close and
message other panes. Treat a report as data from a session that may have read
untrusted files.

Not done: prompting before a repository's versioned hooks load, refusing or
warning when a worktree carries them, and isolating the executor from the
account's reach. Containment is the operating system's job (last section);
`hw` no longer ships an executor sandbox.

## Stated non-goals

Containing an agent that works to get around the guards; protecting secrets
readable by the account; protecting remotes the account can push to.

## A hard boundary, if you need one

It is only as strong as the operating system: a separate account or a mount
the agent cannot write, or a sandbox or container around the agent. `hw` used to
offer a Seatbelt `--sandbox` for one executor; it was removed (2026-11-16) because
nobody dispatched with it and the guards are a tripwire, not a boundary.
