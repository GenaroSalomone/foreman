# Known limitations

What a user of foreman can hit, each as: the limit, why, what to do about it. A limitation is not a promise to fix it.
How the guards are meant to hold, and where they stop, is in [`THREAT-MODEL.md`](THREAT-MODEL.md).

### L1. macOS and Linux only
- **Limit:** foreman runs on macOS and Linux. Windows (native, Git Bash) is not supported; WSL2 is a Linux and may work, untested.
- **Why:** macOS is where it is developed and tested. Linux is checked in a Debian container (the suite, green) but not
  with a live herdr and a live agent, and it has no keychain backend: `hw handle` exits 2 and `secret-unlock --setup|--auto` is macOS only.
- **Workaround:** on Linux, unlock the Bitwarden vault by hand before a dispatch; elsewhere use a macOS or Linux machine.

### L2. The guards are a tripwire, not a boundary
- **Limit:** every agent runs as your user, so nothing enforces a boundary between agents. The write guards (a brainer into its
  repository, an executor into the brain or another product repository), the Bitwarden guard and the Keychain guard read the
  tool call they are shown and refuse the careless one. They do not contain a process that is trying to get around them: a path
  computed at run time, a script, or a secret already in a child's environment gets past them.
  - The brain guard judges a command that names the brain only when it is plain (simple commands, quotes, literal variables,
    data heredocs) and refuses the rest, naming the construct and the spelling that passes. It can refuse a harmless command.
  - The Bitwarden session token (`~/.cache/hw/bw-session`) opens the whole vault, not only the folder `agents`, and the master
    password sits in your login Keychain: any process of yours can read them.
- **Why:** a guard that runs as the account it watches cannot be stronger than that account.
- **Workaround:** keep in the folder `agents` only what an agent may be trusted with and lock the vault when done
  (`secret-unlock --lock`); for a hard boundary keep protected repositories on an account or mount the agent user cannot write.

### L3. foreman needs herdr and Claude Code, and inherits their behaviour
- **Limit:** every brainer and executor is an herdr pane (or herdr-app terminal) running Claude Code; pi is optional. Without
  herdr running and its Claude integration installed, `hw` dispatches nothing. What those tools do (their first-run screens,
  their prompts, their updates) is theirs: foreman does not answer or mask it.
- **Why:** `hw` reads an agent's state, and delivers reports, through herdr; the agents are the vendors' own CLIs.
- **Workaround:** run Claude Code once and accept its screens before the first dispatch, then `herdr integration install claude`
  (setup steps and versions: [`INSTALL.md`](INSTALL.md), [`REFERENCE.md`](REFERENCE.md)).
