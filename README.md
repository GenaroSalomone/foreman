# foreman

**Plan with one agent. Build with many. Keep your repository out of the planner's hands.**

[![CI](https://github.com/GenaroSalomone/foreman/actions/workflows/ci.yml/badge.svg)](https://github.com/GenaroSalomone/foreman/actions/workflows/ci.yml)

![foreman architecture: you talk to a brainer that cannot write to your repository; hw dispatches its briefs to Claude Code and pi executors in their own worktrees, reports come back through done-invoker and ask-invoker, opt-in review pieces and hw done close a task, engram keeps the memory, and merged work leaves through hw train](docs/foreman-architecture.png)

foreman runs coding agents in two roles, on top of [herdr](https://herdr.dev)
(the terminal UI or the desktop app):

- a **brainer**, the planner of one **lane** (one of your repositories), that reads your code and writes briefs, but cannot touch it;
- **executors** that each take one brief, work in a git worktree of their own and report back.

You talk to the brainer; the results come back to it. Everything lives in your **brain**, a directory outside every repository (`~/brain`). Not related to Ruby's `foreman`.

## What a prompt looks like

You type one line to the brainer:

```text
Logging in with a capitalised email returns a 500. Fix it.
```

It writes the brief an executor works from, `~/brain/myapp/briefs/fix-login.md`:

~~~md
---
kind: build
---

## Goal
Logging in as `Ann@Example.com` works like `ann@example.com`.

## Done when
- A test that fails on the old code passes on the new.

## Verification
```
npm test -- login
```

## Limits
- Touch only `src/auth/`.
~~~

## Features

- **A planner that cannot write.** A guard refuses the brainer's writes to your repository; the installer never writes inside one and never reads a credential.
- **One worktree and one branch per task**, outside your checkout; **reports that arrive** in the brainer's session through `done-invoker`, with no polling.
- **One agent plans, another builds.** The brief sets the executor's agent, model, effort and limits; Claude Code and pi are the agents foreman runs.
- **A review you did not have to ask for**, **cleanup that is safe** (`hw reap` removes only what is merged and clean), **secrets as Bitwarden items** and **a cockpit** with a card per executor ([REFERENCE.md](docs/REFERENCE.md#the-cockpit)).

## What 1.0 promises

From 1.0.0 a change that breaks a working setup takes a new major version. That
covers `hw`, `brain`, the invokers, the brief format, the install and the herdr
app: its Cockpit mod, and the `hc` backend it runs on. Claude Code and pi are the agents the promise is made for
([REFERENCE.md](docs/REFERENCE.md#what-10-promises)).

## Requirements

- **macOS** (Linux is checked by the test suite only: [L1](docs/KNOWN-LIMITATIONS.md)), with `git` (`user.name` and `user.email` set). On macOS git's Command Line Tools also bring the `python3` foreman uses; on Linux install python3 3.9 or newer. `hw-core` comes prebuilt with a release; Go 1.22+ is needed only to build from source.
- **[herdr](https://herdr.dev)**, running, with `herdr integration install claude`.
- **[Claude Code](https://claude.com/claude-code)**, first run finished: `claude --dangerously-skip-permissions`, welcome and login, then `/exit`.

engram is recommended: without it nothing is saved to memory. `install.sh --check` lists what is missing. Minimum versions and the optional tools (engram, pi, Bitwarden, review plugins): [REFERENCE.md](docs/REFERENCE.md#tools-foreman-works-with), [INSTALL.md](docs/INSTALL.md).

## Quickstart

On macOS, [Homebrew](https://brew.sh) installs foreman, and `foreman-setup init` builds a brain with a `demo` lane and ends with a dry-run dispatch:

```sh
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"   # only if `brew --version` fails
brew install GenaroSalomone/tap/foreman
foreman-setup init
```

On Linux, or to read the installer first, run it from a clone (`--check` writes nothing and lists what is missing). It creates your brain and a first lane:

```sh
git clone https://github.com/GenaroSalomone/foreman && cd foreman
./install.sh --brain ~/brain --check
./install.sh --brain ~/brain --lane myapp --repo ~/code/myapp
export PATH="$HOME/.local/bin:$PATH"   # puts hw and brain on your PATH
```

Options and upgrading: [INSTALL.md](docs/INSTALL.md). The first task, on a toy repository (run `herdr` once first):

```sh
git init ~/code/toy && echo "toy: a throwaway repository for trying foreman." > ~/code/toy/README.md
git -C ~/code/toy add README.md && git -C ~/code/toy commit -m init
./install.sh --brain ~/brain --lane demo --repo ~/code/toy
cp docs/examples/demo/briefs/hello.md ~/brain/demo/briefs/
brain demo   # opens the brainer: type the next two lines in it with the ! prefix, or ask it to run them
hw demo hello --brief ~/brain/demo/briefs/hello.md --dry-run   # shows what it would launch, builds nothing
hw demo hello --brief ~/brain/demo/briefs/hello.md                         # launches it
```

An executor opens in a new herdr tab, writes `HELLO.md`, commits it on branch `task/hello`, and its report arrives in the brainer's session.

## How it works

The brainer writes a brief; `hw` creates a worktree and branch outside your checkout, opens a herdr tab and starts the agent.
The executor commits on its branch, asks with `ask-invoker` if it needs a decision, and ends with `done-invoker`, whose report
reaches the brainer without polling. `hw done` runs the brief's verification and closes the tab; merging is up to you, and
`hw reap` then removes what is left.

## Everyday use

| Command | What it does |
|---|---|
| `brain <lane>` | Open the lane's brainer. |
| `hw <lane> <task> --brief <path>` | Launch an executor. Add `--dry-run` to preview what it would launch (`--dry-run=full` prints the whole manifest). |
| `hw status` | What is running, what reported and what is left over. |
| `hw done <lane> <task>` | Run the brief's verification and close the task's tab. |
| `hw reap [<lane>]` | List merged worktrees and branches that are safe to remove; `--apply` removes them. |
| `hw train add\|push <lane> …` | Merge reported tasks into `train-<lane>`, run the suite and move `main` to it. |

`hw help all` lists every command and flag, `hw help <topic>` shows one part; [REFERENCE.md](docs/REFERENCE.md) has each row in full, the herdr app, the cockpit, the comparison with similar tools and every lane setting.

## Learn more

[`INSTALL.md`](docs/INSTALL.md) (install options, engram, secrets) · [`REFERENCE.md`](docs/REFERENCE.md) (commands, brief fields, herdr app, tools, lane settings) ·
[`BRIEF-TEMPLATE.md`](docs/BRIEF-TEMPLATE.md) · [`THREAT-MODEL.md`](docs/THREAT-MODEL.md) (the guards) ·
[`KNOWN-LIMITATIONS.md`](docs/KNOWN-LIMITATIONS.md) · [`CHANGELOG.md`](CHANGELOG.md) · [`CONTRIBUTING.md`](CONTRIBUTING.md) · [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) · [`SECURITY.md`](SECURITY.md)

## License

MIT. See [`LICENSE`](LICENSE).
