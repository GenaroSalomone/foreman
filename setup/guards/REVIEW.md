## Guard changes: what you check, because this diff touches `setup/guards/`

`hw review` puts this section in the brief only when the diff changes a path under `setup/guards/` (`setup/guards/REVIEW.md`). It lists the
bypass shapes this system has already shipped, each with the `setup/decisions.md` (or `setup/decisions/<quarter>.md`) row that
recorded it: **seek the row, do not trust this summary** (`rg -n '<words of the title>' setup/decisions.md setup/decisions/`).
Walk each class against every guard the diff changes and write what you tried in `review.md`; a class that does not apply to the
changed guard is one line saying why.

1. **Only the first stage of the line is judged.** `docker ps && systemctl stop docker` passed because the guard read `parts[0]`.
   Does the changed guard judge every segment (`;`, `&&`, `||`, `|`, a newline), and refuse what it cannot parse? (2026-09-21,
   «Un guard valida cada etapa de la linea, no la primera palabra»; `setup/tests/162`.)
2. **A wrapper or interpreter hides the command.** `bash -c`, `sh -c`, `env VAR=x cmd`, nested `env`, a heredoc fed to a shell,
   `python3 -c "open(p,'w')"`. Does the guard look inside, with a depth budget that denies when it runs out? Masking text before
   matching is safe for data and unsafe for code. (2026-08-25, «masking text before a guard matches…»; 2026-09-30, the nested-`env`
   guard.)
3. **A quoted or spelled-out path.** The same path in quotes, with `~`, `$HOME`, `..`, a trailing slash, a different case on APFS.
   Is the destination resolved before it is compared, and is the inode compared, not only the text? (2026-09-29, the three guards
   compare the file identity; 2026-09-09, «El guard de escritura decide por el destino».)
4. **A symlink, resolved or dangling.** `rm`, `>` or `cp` through a link that points at a protected root, and a link whose target
   does not exist yet. Does the guard resolve `..` from the link's real parent? A vector that needs the protected roots to exist
   on the machine is not a vector. (2026-09-06, `rm` through a symlink allowed in all eight copies; 2026-09-29, «La guarda JS
   seguía symlinks colgantes».)
5. **Flat form passes, nested form does not (or the reverse).** The same rule in the Python guard, the JS guard and the Codex
   one; the flat command and the one inside a wrapper. Is the new case in `deny-repo-writes-vectors.json` or
   `shell-vectors.json`, and does it run against every implementation? (2026-09-06, «las ocho copias del guard tenían cuatro
   agujeros iguales»; 2026-09-29, «80 vectores corren contra las tres guardas».)
6. **Fail open on the default.** A binary or verb absent from the table meaning «allowed». The default of an allowlist must deny;
   a derived table must not let a name be allowed without a declared shape. (2026-09-21, the `host` guard's `BINARY_SHAPE`.)
7. **A tool the guard does not see.** Edit/Write and the OpenCode `edit`/`write` tools are outside a Bash-only guard; a mention of
   a protected path inside data is not a write to it. Does the change judge by the destination, not by the words present?
   (2026-09-08, «El guard no distingue escribir en un repo de mencionarlo»; 2026-09-28, «su guarda cubre solo Bash».)
8. **A process pattern wider than the target.** `pkill -f <pattern>` with an option after the pattern, or a pattern that matches
   other lanes' processes. (2026-10-01, «la guarda de pkill»; `deny-blind-process-kill.py`.)
9. **A hook that never runs, or runs open.** Committed as `100644`, a `git -c` that leaks into every descendant git, a guard that
   errors and lets the call through. Does an error in the guard end in a refusal, and does a test invoke the hook the way git or
   the harness does, not by `sh hook`? (2026-09-09, 2026-09-16, `setup/tests/114`, `115`, `1281`.)
10. **A test that cannot fail.** A vector whose cwd, environment or machine state keeps it from reaching the position the guard is
    used from. Run the vector with the guard change reverted (`setup/guards/drive-guard --guard <guard.py> <file>`, from a file:
    `setup/tests/1333`) and see it deny or allow differently. (2026-09-09, the redirect vector that ran with cwd outside the lanes.)
