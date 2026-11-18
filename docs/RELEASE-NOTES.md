# foreman 1.0.0-rc.3

foreman 1.0.0-rc.3 is the last candidate before 1.0: fewer moving parts, a brain guard that fails closed, and a short, honest list of what it still cannot do.

## In short

- **Two agents, one guard implementation.** Codex, OpenCode and the Spec Kit flow are gone; pi asks the same Python write guards Claude Code runs, so the two can no longer drift.
- **The brain guard refuses what it cannot read.** A command that names a brain path is judged only when it is flat, and any word it cannot resolve to a literal is refused with the spelling that passes.
- **Known limitations down to three.** The public KNOWN-LIMITATIONS lists only what a user can hit; the rest was fixed or moved to internal notes.
- **Reviews and chaining hold their ground.** `hw review --account` picks the reviewer's account, `--at` keeps the receipt's fork point, and `hw next` no longer loses a delivered reviewer's lease.
- **A release cut fails in seconds, not 40 minutes in,** when its `hw-core` is missing or stale, and the docs are a short tour with the reference moved to REFERENCE.md.

## What changed

102 changes since 1.0.0-rc.2.

### Added
- `hw review <lane> <task> --account <name>` runs the reviewer on that account.
  The flag goes to the reviewer's dispatch as it is, so the same parity check,
  refusal and `account` manifest line apply (`--dry-run` shows it). Without it
  the reviewer keeps the lane's default account.

### Changed
- The guard's verb finder and builtin check read one table of precommand words. hw-core's "brain guard present" checks (dispatch, revive, `hw done`'s leak scan) read one list, which also requires the two modules the guard imports, `setup/guards/deny_brain_writes_shell.py` and `setup/guards/deny_repo_writes.py` (without either it fails closed on every call).
- Fail-closed costs, measured on 5426 product-executor commands (KNOWN-LIMITATIONS L2): 154 refused (the last round 127). Among the 27 more, a brain-naming command that reads an assignment while its text holds a `(` even inside quotes (`MSG="done (x)"; done-invoker "$MSG"`), and a heredoc fed to `python3` in a command that names brain. Write such a value where it is used, and such a script to a file.
- Fail-closed cost, measured on 5426 product-executor commands (KNOWN-LIMITATIONS L2): 162 refused (the last round 154). The 8 more are a `cat > file <<'EOF'` in the same command as a brain program called with a `(` in a quoted argument (`done-invoker "done (x)"`): run the heredoc in a command of its own.
- Fail-closed cost, measured on 5426 product-executor commands (KNOWN-LIMITATIONS L2): 169 refused (the last round 162). The 7 more are a `cat > file <<'EOF'` in a command that also holds a `|` and names brain: run the heredoc in a command of its own.
- An unknown word that is only data is allowed again (round 15 refused it anywhere in a writing command): `make test; echo "rc=$?" >> "$HW_ARTIFACTS/x"`, `cmd | tail -5; echo "exit=$?"`, `ls -la ${TMPDIR:-/tmp}/x.* | head`, `ps > /tmp/ps.$$`. Still refused, fail closed: an unknown word as an operand of a writing command even where it cannot reach brain (`cp "$f" "${f%.txt}.bak"`, `gh … ${PR:-1}`), and an unknown word in a command with a rename, `exec`, a function or a pipe into `tee`. Spell the word literally.
- Docs: the README is a short tour (what foreman is, install, the first dispatch, links); the herdr app, the tools table and the comparison with similar tools moved to REFERENCE.md. The docs no longer describe Codex, OpenCode, the Spec Kit flow or `hw unstick`.
- `hw wait` and `hw next` judge a blocked pane by its screen alone: the "settled" verdict and
  `hw wait` exit 5 came from an OpenCode endpoint and are gone, and the
  `hw unstick` advice with them (`channel-send` keeps its own exit 5, an unconfirmed
  delivery). A blocked/stuck pane whose completion tokens are
  delivered is still re-tasked without `--force`.
- A `projects.json` written by `install.sh --vendor opencode` still loads: `opencode_config_dir` is accepted and
  ignored for one release, and a lane with `vendor: opencode` or `codex` opens a Claude brainer. Dispatching an
  executor of that vendor is refused naming the removal.
- The pi executor now asks the same Python write guards the Claude hooks run (`deny_repo_writes.py` through the new `setup/guards/deny_repo_writes_ask.py`, and `deny_brain_writes.py` directly), one spawn per question (measured 2026-11-17: median 40 ms, p95 76 ms a spawn). A guard that does not run, or answers anything but the shape asked for, refuses that call (and only that one: the next call asks again). Every guard now starts with `python3 -I` (the pi spawns, the Claude shims and the hook commands), so a `usercustomize.py` in the user site can no longer stand in for it.
- `hw help launch` says a lane whose `vendor:` is `codex` or `opencode` is no longer supported, in the words of the
  dispatch refusal, instead of "no flag needed".
- `hw wait` and the `hw next` gate share one witness rule (a moved screen is live, anything else unknown).
- `docs/KNOWN-LIMITATIONS.md` is three entries now (macOS and Linux only; the guards are a tripwire, not a sandbox; foreman needs herdr and Claude Code), each as the limit, why, and the workaround. The version floors in `docs/REFERENCE.md` are stated as the versions foreman was tested on; the Windows, first-run, `--permissions ask` and install notes live in `docs/INSTALL.md`.
- `hw next`: the `t<seq>` run directory is made only once the lease gate has passed, instead of being made first and removed on a refusal (no change in what a refusal leaves behind).
- The pi guard's measured cost in the changelog agrees with `docs/pi-vendor.md` (median 40 ms, p95 76 ms a spawn), and INSTALL
  says the pi adapter needs `/usr/bin/python3`.
- New test 1392: no public doc offers Codex, OpenCode, Spec Kit, `hw unstick` or `--sandbox` outside the files that are history.

### Removed
- The brain guard's simulator of what `${A#…}`, `${A/x/y}` and `${A:2}` give. Such a read keeps its unknown piece and names brain when brain's name or `..` is in its text or in a value it reads, which also refuses `${A:-${B:-../../..}}/brain/…`, `${A/n[^a]/n}` and zsh's `${A:s/…/}` and `${A[2,20]}`.
- `bin/invoker-common.sh` and `bin/state-witness.sh`, the two bash libraries no program read since the invokers,
  `channel-send` and the witness run in Go, with the subjects and mutation guards that only sourced them. The Go witness
  keeps its answers as goldens frozen from the bash's last run, and `hw next`, `hw ruling` (resuming a blocked task) and a dispatch's brief delivery no longer refuse a
  `brain/bin` for lacking `invoker-common.sh`.
- The Spec Kit flow. `--sdd speckit` and a brief's `sdd: speckit` now die with one message, `hw next --sdd speckit` too;
  hw no longer looks for `speckit-*` skills on disk, names an entry command, refuses a sonnet or haiku
  orchestrator, or prints the `framework` and `entry` lines of a dispatch manifest. `--sdd none` and `sdd: none`
  stay accepted, and ignored, for one release. Limitation L14 and the Spec Kit entry of the dependencies table go with it.
- Codex and OpenCode are no longer supported: foreman runs Claude Code and pi only.
  `--agent codex|opencode` is refused with a message naming the two supported agents.
- `brain --opencode`, the `hw unstick` command and `install.sh --vendor` are gone, with
  the OpenCode and Codex routes of `channel-send` (`opencode`, `opencode-tui`, `codex`),
  the OpenCode plugins and turn-end adapter, the Codex guards and hook, `opencode-auto`
  and `codex-channel-session-hook.sh`.
- KNOWN-LIMITATIONS no longer lists Codex or OpenCode; the file holds three limitations, renumbered L1-L3.
- The JavaScript ports of the guards (`setup/guards/deny-repo-writes.js`, `setup/guards/deny-brain-writes.js`) and the JavaScript half of the guard conformance suite and its mutation arm: there is one implementation of each rule now, so the two can no longer drift.
- `--sandbox` and `sandbox: true` in a brief: hw no longer runs an executor under a Seatbelt profile. The flag and `sandbox: true` die
  with one message before anything is built (a dispatch that asked for confinement must not run without it); `sandbox: false` is
  accepted and ignored. `bin/hw-sandbox`, its outbox broker, the arming on `hc` and on `hw revive`, and the `HW_SANDBOX` branch of
  `done-invoker` and `ask-invoker` are gone. The docs that offered it (README, THREAT-MODEL, REFERENCE, INSTALL, BRIEF-TEMPLATE,
  pi-vendor, `hw help flags`) say so no more. Containment is the operating system's job; the guards stay a tripwire.

### Fixed
- The brainer's batch gate under `setup/heavy` (no `HW_TASK`) now waits up to 3600 s for the Go test batch lock instead of dying at 300 s behind an executor's `--bless`; executors keep 300 s.
- The brain guard no longer lets an expanded or quoted value of `env -u`/`-C`/`--unset`, `nice -n` or `timeout` replace the verb after it: `env -u 'HW_TASK' hw reap setup --apply` and `env -u "$X" secret-unlock --auto` are denied again, and a quoted literal (`env -u "HOME" rg x <brain path>`) is a literal. A wrapper word that may split into several (dash-led `-$(…)`, attached `-u$(…)`, an option word or an assignment such as `A=$(…)`) is also judged as the verb, so `env -u -$(echo X tee) cat <brain path>` is denied instead of running `tee` over the file.
- The brain guard no longer tries to recognise which expanded values may split into a verb (the list missed `"${@:1}"`, zsh `"$arr[@]"`, a quote inside `$(…)`, and a 33-candidate cap let a late one through: each truncated a file in real bash/zsh). A wrapper's option, value or assignment (`env`, `nice`, `timeout`, `command`, `setup/heavy`, `hw secret run`) is skipped only when it is provably one plain word: bare `[A-Za-z0-9_.,:/@%+=-]`, `'…'`, or `"…"` with no `$` (other than `$NAME`/`${NAME}`), backtick, backslash or `[`. Any other leaves the verb undetermined, and a command that names a brain path is refused whatever follows, a reader included.
- Over-refusal that comes with it: `env PATH=$PATH rg x <brain path>` and `env -u X\ Y rg x <brain path>` are now refused. The refusal says why and names the spelling that passes: `env PATH="$PATH" rg …`, `env -u 'X Y' …`. A command that names no brain path is unaffected (`env -u $X rg x /tmp/f`).
- The brain guard judges the WHOLE command once its verb is undetermined, not only the words after the verb: `env -u $V <brain>/setup/heavy -- …`, `nice -n $N <brain>/bin/hw secret run …`, `timeout $D <brain>/setup/heavy -- …` and `env PATH=<brain>/bin:$PATH nice -n $N …` were allowed and truncated brain's own files in real bash/zsh. The refusal now also names single quotes (`env -u 'X Y'`).
- Refused beside it, each shape measured (the classes they belong to stayed open until the next entry, `task-brain-guard-env-unset-fix4`), each a way to run a brain program or write brain that the guard allowed: `timeout -s SIG` / `--signal` / `-k` values were read as the verb (`timeout -s KILL 5 hw reap setup --apply`); `exec -a NAME` and GNU `time -f FMT` / `-o FILE` likewise; a word that expands and spells a program of brain's `bin/` (`env -u {X,hw} reap …`, `$(echo hw) reap …`); a verb that expands to nothing (`E=; $E tee <brain>/x`); a reader's writing flag behind an expanded argument (`sort $O <brain>/x`, `rg $X`, `fd $X`, `find … $X`); a pipe from a brain read into an undetermined verb (`… | env -u $V cat`); and a variable changed by `export A=x`, `read A`, `printf -v A`, `A+=x`, `eval`, a nameref (`$A` after one is a guess, so a wrapper word or verb made of it is undetermined).
- Over-refusal that comes with it: `env PATH=<brain>/bin:$PATH rg x /tmp/q` and a reader with a writing flag (`sort`, `rg`, `fd`, `find`, `sed`, `tree`, `uniq`) given an unreadable argument alongside a brain path are refused. `exec -a NAME cat <brain path>` and `timeout -s KILL 5 rg x <brain path>` are now allowed (the option's value is no longer read as the verb).
- The brain guard skips the value of an `env` option (`-u`, `-C`, `--unset`, `--chdir`, …) only when it is a literal word: a shell-expanded one (`{X,tee}`, `$V`, `$(…)`) is judged as the verb, so `env -u {X,tee} cat <brain path>` is denied instead of running `tee` over the file.
- The brain guard's flat rule admits the two shapes executors use to call a brain program: a heredoc whose delimiter is quoted (`cat > "$HW_ARTIFACTS/checks.md" <<'EOF' … EOF`, then `<brain>/bin/done-invoker …`) and an unconditional literal assignment (`MSG="…"; done-invoker "$MSG"`, `B=<brain>/bin; $B/hw secret check x`), which has exactly one value. Measured on 5426 product-executor commands with the brain at today's root: main refuses 58, the round before last 91, the last round 210, this guard 127 (the 42 above the round before last were each a glob, `$(…)`, unquoted heredoc, block, subshell or brace in a command that names brain).
- Refused now, each allowed by the last round and each a truncation of a brain file or a run of `hw reap --apply` in real bash or zsh: `${X:?$(…)}` and `${X?`…`}` with X unset, a brace holding a quoted or escaped blank (`sort {-o,'<brain>/x/a b'} /dev/null`), a subshell after a reserved word (`! (…)`, `if (…); then`), an assignment chain five deep or after 64 earlier values (`A=<agents>; B=$A; … cat > $E/brain/x`), and zsh `noglob`/`nocorrect`/`-` and bash `jobs -x` before `hw reap setup --apply`.
- `S=<path>; $S "$(cat f)"` is no longer refused (S has one value), and when a verb may expand to nothing the refusal names that variable.
- The brain guard's two admissions are textual whitelists that fail closed (`setup/export/overlay/docs/KNOWN-LIMITATIONS.md` L2). A heredoc is data only behind exactly `'WORD'` or `"WORD"` and fed to no interpreter in its command or pipeline (`bash`, `python*`, `xargs` … one table); any other delimiter spelling (`<<\EOF`, `<<'EOF'x`, `<<'-EOF'`) refuses the command whole, since the guard cannot tell where its body ends. An assignment has one value only in a command whose text holds no `(`, backtick, `{` (but `${`), `[[`, background `&` or reserved word, with no `~`/`=` at its value's start or after a `:`.
- Refused now, each allowed by the last round and each a truncation of a brain file or a run of `hw reap setup --apply` in real bash or zsh: a heredoc after `<<\EOF`, `<<E\OF`, `<<'E'OF`, `<<'EOF'x` or `<<'-EOF'`; `A=~/…/brain/x/f` and `A=~/agents; cd $A` (also `export`, after a `:`, zsh `A==hw`); an assignment after `||` holding `$(:;)`; an assignment inside `if …; end; …; fi` or `time -p while`; `jobs -rx`/`-r -x`/`-xr hw reap setup --apply`; `bash <<'EOF'`, `cat <<'EOF' | bash`, `python3 - <<'EOF'` and `xargs rm <<'EOF'` with a relative brain path; `patch <<'EOF'` naming one; `A="-o ../brain/x/f"; sort $A`.
- The brain guard's last denylists are allowlists that fail closed (`setup/export/overlay/docs/KNOWN-LIMITATIONS.md` L2). A quoted heredoc is data only for a data consumer (`cat`, `tee`, `jq`, `git commit`, `gh pr`, `done-invoker`, `ask-invoker`, `hw`) alone in its pipeline, in a command whose text has no `(`, `{`, backtick or descriptor past 2; any other heredoc is code, and the interpreter table no longer decides it. A variable the command assigns is read flat only as `$NAME` or `${NAME}`; any other read of it (`${A:-}`, `${A#…}`, `${!A}`) is not flat and is read with every value it may give, empty and a literal default included. A heredoc body of more than 512 paths names brain instead of being cut at 512.
- Refused now, each allowed by the last round and each a truncation of a brain file or a run of `hw reap setup --apply` in real bash 3.2, zsh 5.9 or bash 5.3: `false && A=x; cp /dev/null ${A:-}../../../brain/x/f` (and `${A#…}`, `"${A-}"`, `sort -o`, `${A:-<brain path>}`); `cat <<'EOF' |` with the interpreter on the line after the body, `| (bash)`, `| { …; bash; }`, `| tee … | bash`, `> >(bash)`, `exec 3<<'EOF' … bash <&3`, `bash -c "$(cat <<'EOF' …)"`, `| awk '{system($0)}'`, `ed <<'EOF'`, a `patch` body padded past 512 paths; a `<<\EOF` heredoc inside a backtick; bash 5.3's `${ cmd; }` and `${| cmd; }`, read now as command lists.
- The brain guard no longer takes a brain program for a heredoc's data consumer: `hw secret run item --as V -- bash <<'EOF'` execs bash on the body, and was allowed. A quoted heredoc is data only for `cat`, `tee`, `jq`, `git commit` or `gh pr <verb>`, written as the bare name or a path in `/bin`, `/usr/bin`, `/usr/local/bin` or `/opt/homebrew/bin`, with no assignment before it and its subcommand as the next word (`git -c alias.zz='!bash -s' zz commit`, `./cat`, `PATH=/tmp/d cat` are code). The command around it may hold no `|`, `&`, reserved word (`for … done | bash`, `case … esac | bash`, `coproc bash`), no descriptor the guard cannot read (`>&p`, `>&"$FD"`), no `alias`, `hash` or other builtin outside the flat ones, and no PATH or array-element assignment (`setup/export/overlay/docs/KNOWN-LIMITATIONS.md` L2).
- A read of a variable that is not a plain `$NAME`/`${NAME}` keeps its unknown piece, so the known start before it still names brain (`../../../${A:+brain}/x/f`, `${A/x/brain}`, `${A%Z}`, `${!B}`), and is read with what its form may give: a default's or alternate's text (`${A:+../../..}`, `${HOME:+…}`, a quoted default), every part of a cut or substring, a replacement.
- An unknown piece of a word may be empty: `cat /dev/null > $UNSET../../../brain/x/f` (a flat command) is refused.
- bash 5.3's `${ cmd; }` inside an unquoted heredoc body, a `${…}` or an arithmetic is read as a command list.
- The brain guard no longer reads a heredoc fed to `git commit` or `gh pr` as data: each runs the editor that `GIT_EDITOR`, `EDITOR`, `core.editor`, `GIT_CONFIG_*` or `GH_EDITOR` names on the body (`export GIT_EDITOR='bash -s'; git commit <<'EOF'` ran the body). A quoted heredoc is data only for `cat`, `tee` or `jq`, in a command that exports nothing to it: no `export`, no assignment of `PATH` or of a name the environment holds (`setup/export/overlay/docs/KNOWN-LIMITATIONS.md` L2).
- A command the guard cannot finish reading is denied: past 5 seconds of its own work it denies naming the timeout (and the kernel ends it past twice that of CPU), and both of the brain guard's Claude Code hooks carry `"onFailure": "block"`, so a hook that fails or times out blocks the call. `echo ${PATH//*a*a*a*a*a*a*a*a*Q/x}; …` kept the last guard busy past Claude Code's 60-second hook timeout, which let the command run.
- `$PWD`, `${PWD}` and `~+` before any `cd` are the payload's cwd again: `cat /dev/null > "$PWD/../../../brain/x/f"` is refused, as on main.
- The brain guard refuses a path it cannot know instead of reading its text: a word, redirect or here-string that starts with a read the guard does not expand (`${A/x/}`, `${A%…}`, `${A:-…}` of an assigned name, `${!A}`, zsh's `${~A}`, `$~A`, `$A[…]`, `$A:s/…/`, `$_`, `$PWD` after a `cd` it cannot follow), or holds two, is refused with "spell the path literally". `A=<agents>/bxrain; cat /dev/null > ${A/x/}/x/f` and `> ${PWD%/work*}/b?ain/x/f` wrote into brain. A reader's argument (`echo ${PWD##*/}`) is still allowed when no pipe, subshell or substitution takes its output on.
- One cwd model: `cd`, `chdir` and `pushd` are followed where they surely run, `&&` runs what follows them in their target, and a `cd` that may fail or not run (`cd x 2>/dev/null; …`, `false && cd /tmp; …`), one in a subshell, a pipeline or a function, `cd -`, `popd` and `eval` leave the cwd unknown. `$PWD`, `${PWD}`, `~+`, `$OLDPWD` and `~-` are that cwd. `cd ../../..; cat /dev/null > $PWD/brain/x/f`, `chdir ../../..; … > brain/x/f` (zsh) and `(cd /tmp); … > ../../../brain/x/f` wrote into brain.
- No cap on the length of the cwd the guard follows: past 256 characters it stopped resolving relative paths, so a symlink to the agents directory under a long cwd reached brain.
- A brace after a `"\""` in the same command is read again: `echo {a"\""; env -u {X,hw} reap setup --apply` ran `hw reap --apply`.
- An array's elements are its values: `A=(../../../brain/x/f); cat /dev/null > $A` is refused, as on main.
- The brain guard is never weaker than main's: main's guard, frozen as `setup/guards/deny_brain_writes_main.py`, judges every call first, in the same process and deadline, and its refusal is the verdict, with its message. `true && cd ../../..; cat /dev/null > brain/x/f`, `cat /dev/null > l/brain/x/f; for i in 1; do cd /tmp; done` (`l` a symlink to the agents directory) and `A=('../../../brain/x/f'); cat /dev/null > $A` wrote into brain; main refused each.
- A word the guard cannot resolve to a literal is unknown, and an unknown word in a path position is refused (an output redirect target, an operand of a command that may write, a `cd` target before a write), and an assignment of one makes each later read of its name unknown: a `${…}` but `${NAME}` (any operator: `:-`, `/`, `%`, `#`, `@`, `^`, `,`, a substring, a length, an indirection), `$'…'`, a positional or special parameter (`$1`, `$@`, `$*`, `${1%x}`; not `$?`, `$#`, `$$`, `$!`, digits only), `$_`, `$PWD` after a `cd` it cannot follow. Ten rounds of resolvers each missed a spelling that wrote into brain: `A=x; cat /dev/null > d/${A:+../../../..}/brain/x/f`, `${PWD%/work*}/b?ain/x/f`, `${A:-l}/b?ain/x/f` (`l` a symlink to the agents directory), `${HW_TASK/t/l}/b?ain/x/f`, `${A:-$'<agents>/\x62rain'}/x/f`, `set -- <agents>/braix; > ${1%x}n/x/f`, `${@:-l}/b?ain/x/f`, `A=braix; B=${A%x}n; cd ../../..; > $B/x/f`. A reader whose output stays on screen may still print one.
- A reader's argument is data only where what it prints cannot come back as a path or code: no rename, `exec`, function definition or `printf -v` (nor an option word that is not a known literal), no pipe into a non-reader, never inside a substitution, and once a reader or a block wrote to a file, no input redirect, heredoc, substitution, interpreter, `xargs`, `parallel`, `source`, `.` or `eval` after it (anywhere in a loop). `f() { echo ${A%x}/x/f; }; f | xargs cp /dev/null`, `echo … | xargs cp /dev/null $(true)`, `echo ${A%x}/x/f > l; xargs cp /dev/null < l`, a bare `exec > l` before the reader, and `printf "-v" P …`, `printf \-v P …`, `F=-v; printf $F P …` wrote into brain.
- A command with any compound word reads every `cd` in it as uncertain again: a `case` pattern on its own line named `done` was counted as the end of a loop, and the `cd /tmp` behind it was taken as certain.
- The two leak scans of a push (main's guard's and this one's) run side by side, each within its own 40 s: a push waits for the slower scan, not for both in turn, and both fit the hook's 60 s.
- A reader's argument is never exempt after a rename in the same command: `hash -p /bin/rm cat; cat ${A%x}/x/f` (bash), `hash cat=/usr/bin/tee; cat …` (zsh) and `alias cat=tee` wrote into or deleted from brain through a "reader".
- A `cd` target is resolved as bash and zsh resolve it, logically (`..` cancels the name before it), and physically only under `cd -P`, `set -P`, `set -o physical` or zsh's `chase_links`; a start that leads toward brain either way counts. `cd s/../../../../brai$(printf n) && cat /dev/null > x/f` (`s` a symlink to `/var/empty`) wrote into brain: the guard resolved the symlink first.
- The brain guard judges a command that names a brain path only when it is flat — simple commands of known words (literal text, quotes, `~`, the executor's environment variables) joined by `;`, `&&`, `||`, `|`, with redirections and here-strings — and refuses any other, naming the construct and the flat spelling that passes (`setup/export/overlay/docs/KNOWN-LIMITATIONS.md` L2). It replaces the assignment tracker of the previous round, whose bookkeeping let six more spellings truncate a brain file in real bash/zsh (`A=tee; false &&⏎export A=cat⏎"$A" <brain>/x`, `"" <brain>/x` after a heredoc `${A:=tee}`, `[[ 1 ]]&&true; cat /dev/null > <brain>/x`, `printf -v A tee; "${A:-cat}" <brain>/x`, `set -o braceccl; env -u {Xt}ee cat <brain>/x`, `sort $(echo D/*) <brain>/x`). Also refused now, each allowed before: zsh `{ :; } always { A=tee; }`, `if [[ … ]] { … }`, `time -p A=tee`, `HOME=<brain>/x; … > ~/f`, `… | dash` fed a brain path, `timeout .5 hw reap setup --apply`, zsh `=hw reap setup --apply`.
- `B=<brain>/bin; $B/hw …` is refused, and the refusal spells the call that passes: `<brain>/bin/hw …`. So are a heredoc, a glob, `$(…)`, a loop or `if` in a command that names a brain path: write them in a command of their own.
- Measured on 5426 product-executor commands: main refuses 5, the previous round 34, this guard 38; of the 12 it newly refuses beside the previous round, 11 name brain and are not flat, 1 (`S=x; $S "$(cat f)"`) names none. No flat call of `hw`, `setup/heavy`, `done-invoker` or another `bin/` program is newly refused. `env PATH=$PATH rg x <brain path>` (blank-free PATH), `env -u X\ Y …` and `env -u "a\b" …` stay allowed.
- The reuse check and manifest, `hw reap --apply`'s report test and `hw-reconcile` pick the newest run through the one `RunIDs` instead of three private copies of it.
- `setup/tests/37-chaining-lease.sh`: the fixture panes and the sweep root follow the move to `~/agents`, so the subject runs past C01 again (C18a, C21 live).
- `hw next` no longer refuses with "has no chaining authority" a reviewer whose report was delivered. A second dispatch of a task whose terminal is already live launches nothing, and its run directory is now moved into the executor's own run (`.hw/<run>/reuse/<this run>`) instead of staying beside it as the newest run. Every reader of "the run" (`hw next`, `hw done`, `hw status`, `hw review`, the cockpit witness, the never-reported lists, the invoker's env fallback, reap, the brief archiver) therefore reads the run that holds the done marker and the lease, with no check of its own; the per-run `TMPDIR` of the reuse dispatch is removed.
- A reuse dispatch trusts the terminal's `hw_run` label only when it is a run id.
- A refused `hw next` ("NOTHING was sent or touched") no longer leaves an empty `t<N>/` directory in the run.
- Upgrade: a run directory already marked `reused-live-run` by the interim builds of 2026-11-16 is an ordinary run directory now; it stays the newest run of its task until that task is closed with `hw done`.
- Test 1241 asked the setup brain guard about the old root's brain, a path that stopped being brain when the agents root moved to `~/agents`; it now derives the `~` spelling from `guards.json`. The guard was right.
- Test 1391 now runs when a lane shim (`<lane>/.claude/hooks/deny-repo-writes.py`) or `.claude/settings.json` changes.
- The reviewer `hw review` dispatches is always a fresh executor (`--fresh`).
  Before, a kept pane of the lane on another account was picked for reuse and
  the dispatch refused with a `--fresh` remedy `hw review` does not accept; the
  one remedy left, dropping `--account`, ran the review on the default account.
- When the reviewer's dispatch fails, the command printed to retry keeps
  `--account <name>`.
- The review `hw done` runs after a delivered build carries the build's own
  account (the dispatch records it in the receipt as `account`), so a build
  on `personal` is reviewed on `personal`.
- The review `hw done` runs after a delivered build ignores a receipt account
  `hw` would refuse (it is not a known account): it says so and the review runs
  on the lane's own account, instead of carrying the value into the command.
- A release cut checks the `hw-core` its gates drive before its first gate: a tree with none, or one older than
  `core/`'s sources, is built with `setup/build-core --native`, and a build that cannot be done stops the cut at once
  naming that command (or the architecture mismatch when the build left it where `bin/hw` does not look), instead of the
  fast gate dying on it ~40 minutes in.
- `setup/heavy` refuses to start a job (exit 75) when the user's process table has less than a quarter of its limit left
  (at most 1000, at least 100; `HW_HEAVY_PROC_HEADROOM=<n>` sets it flat, 0 turns it off; the limit is the smaller of
  `kern.maxprocperuid` and `ulimit -u`), naming the count, the limit and the commands with most processes, orphans
  first, instead of the job dying on `fork: Resource temporarily unavailable` halfway through. The short lane
  (`build-core`, a focused test) is never refused, so `hw-core` can always be rebuilt.
- `setup/go-test-core.sh` says `FORK FAILED` (with the process count and the limit, exit 75) when it cannot start or
  count its partitions, instead of printing `[: : integer expression expected` or reporting a plain red; it waits for the
  partitions it had already started before it exits, so none outlives the batch lock.
- A new brief key whose "Known keys" evidence in the mutant table was not updated is caught in seconds by a unit test
  of the fast gate, not by the release's mutation batch.
- An ask or challenge that the brainer answers with a plain prompt (not `channel-send --reply-hold`) no longer stays «ask unanswered» in the cockpit while the executor works on the answer: the executor's first tool call or turn end after a `human` prompt retires the hold (the witness is the session transcript's `origin.kind`, so a queued ruling, a Stop-hook block or a background job's wake never retires an unanswered ask), and that turn end is then judged as a turn with no hold. The `post-tool` hook no longer returns early while a `pending-reply-N` hold exists.
- `setup/heavy`: a waiter, short or slot, whose own queue ticket has been removed (its queue directory deleted underneath it) now exits 75 at once, naming the ticket path, instead of waiting forever. Ten `--short --tag build-core` waiters slept ~4 h on 2026-11-16. A long queue is not stuck, so there is no time limit. A wait cut short under `--when-idle` no longer leaves the pause clock running.
- `bin/core-rebuild`: a builder that exits 75 (`setup/heavy` refused: no process headroom or memory, or its own ticket was gone) is no longer recorded as `rebuild-failed`, so the stale `hw-core` is retried on the next call instead of being left in place until its sources change.
- `hw <lane> <task> --dry-run` from a plain terminal prints the manifest (its return channel says `UNRESOLVED`) instead of stopping at `HW_INVOKER_PANE is UNRESOLVED`; a dry run builds nothing, so it needs no return channel. A real launch still refuses.
- `hw review --at <sha>` on a commit the lane's base already holds measures it against its first parent, so the review counts the commit's lines and dispatches a reviewer instead of reporting 0 code lines.
- The sleep/poll guard reads `nice --adj 10` / `--adj=10` (a long-option prefix) as a prefix, so `nice --adj 10 sleep 600` is judged as `sleep 600`.
- Setup test 1382: a crashing mutant no longer counts as killed; the b07/b08 rows are in order and `env -u -$(…)` has its row.
- Setup test 1391 selects on `guards.json` and `deny_repo_writes.py`; test 1010 names the agents-root brain path.
- `channel-send` (hc route): when the receiver's transcript watermark (`claude-delivery-receipt.py --mark`) cannot be taken, the transcript proof is skipped and the counters decide; before, the failure became "none" and the scan started at byte 0, so an earlier send of the same bytes could count as PROVEN delivery.
- `brain restore`: a lane named twice (or a lane and its alias) is restored once; the second resume ended the terminal the first had just made.
- `channel-send` (claude route): the same failed-watermark rule: no watermark, nothing is sent and the refusal says so (it used to treat the failure as an empty watermark and could report PROVEN from an earlier send).
- `setup/merge-task` no longer reports NOT CONSISTENT when a dirty tracked path with a newline in its name has a piece that
  equals a merged path, and no longer dies in `tr` on a non-UTF-8 tracked name under a UTF-8 locale (it runs in the C locale).
- `setup/merge-task` no longer reports a case-only rename of a dangling symlink as STILL ON DISK, and a deletion git could not
  apply is no longer also counted and listed as «removed».
- `hw train push` no longer drops the staged change of a path that is both staged and edited again (it is git's refusal, like
  any staged path), and refuses before touching an uncommitted edit whose backup cannot be written instead of losing it.
- `hw train push` with several carried edits: a failed backup on any of them puts every edit back (it used to lose the earlier
  ones), a write-back that fails after the advance exits non-zero naming the copy, and a path the train deletes with only a mode
  edit advances again. `setup/merge-task` no longer forks per pair when reporting case-only renames (300 renames: minutes -> seconds).
- `setup/merge-task` no longer reports NOT CONSISTENT (exit 3) for a merge that renames a directory only by case on a
  case-insensitive volume: the added path that is the same file as a removed one is not counted as untracked.
- Test sandboxes no longer leak processes: a contract world (`setup/tests/contract-hw/world-t3.sh`) now stops every process whose command line holds its path, and their descendants, so the `stall-watch` an `executor-turn-end` case starts (and the `bin/core-rebuild` / `setup/heavy` waiter a stale `hw-core` puts behind it) no longer outlives the case orphaned to pid 1. Test 1352 stops the `setup/heavy` waiters under its killed jobs too, not only their subshells.
- `bin/core-rebuild`: a builder refused by `setup/heavy` (exit 75) is now reported as "refused, not broken", not as a failed build; the next call still tries again.
- Test 164's fixture no longer copies the gitignored `core/dist` (82 MB of release binaries in a main checkout). Test 1387 does not fail on a user whose hard process limit is under 3500; it says it did not exercise those two cases.
- The runner snapshot (`setup/test-hw-snapshot.py`) removes a read-only directory its first capture copied, so the retry after a changed input no longer dies with `File exists`.
- `go-test-core.sh`: FORK FAILED says "most likely" the machine ran out of processes (a partition killed by a signal leaves no result either).
- `hw help launch` named only Codex and OpenCode as unsupported vendors, so a lane with any other vendor (`gemini`) read
  "no flag needed" while the dispatch refused it; both now share one predicate and one sentence.
- `hw next --brief` with `sdd: speckit` is refused like the launch refuses it, instead of delivering the brief.
- The Agents list's drop line for a group moved down now sits under the last row's unfolded sub-rows, not between that row and them.
- The Agents list's drop line no longer draws twice for a pinned sub-agent, and a row dragged down onto a row with sub-rows now puts the line under them.

## Known limits

- macOS and Linux only. Windows, native or under WSL2, is not supported or
  tested (KNOWN-LIMITATIONS, L1).
- The Go core is on by default when the installer builds it; `HW_IMPL=bash`
  goes back. Every limit a user can hit is in
  [`KNOWN-LIMITATIONS.md`](KNOWN-LIMITATIONS.md).

## Upgrading from 1.0.0-rc.2

Run `foreman-setup upgrade --brain DIR`. A brain installed before `upgrade`
existed has no record of its flags yet: run your install command once more,
and `upgrade` works from then on.

Every release's changes are in [`CHANGELOG.md`](CHANGELOG.md).
