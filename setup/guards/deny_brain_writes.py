#!/usr/bin/env python3
"""The inverse guard: a PRODUCT executor does not write in brain.

`deny_repo_writes` keeps every brainer read-only over the product repos; this is the other direction. brain
(`brain_root` in `guards.json`) is its operator's own project, and an executor of a product lane (every lane whose table
entry does not say `brain_guard: false`) has no business changing it, working from it, or carrying a path into it back
to a product repo.

IT DENIES BY DEFAULT. It does not ask "is this command a write into brain?" (three rounds of review each found a write
the parser did not know); it asks "does this command NAME brain, and if it does, is that one of the two allowed ways?"

  1. As a program of brain's bin/ run by its path (BIN_TOOLS and setup/suite-data/brain-guard-programs.txt), optionally
     behind an env assignment or a wrapper (`timeout 600 <brain>/bin/ask-invoker …`).
  2. As an operand of a reader from a closed list (READ_VERBS: `cat`, `rg`, `sed` without an in-place flag …), with no
     output redirect into brain.

Anything else that names brain is refused: `cd` into it, a write tool, a relative path, a variable, a glob or a brace
that resolves into it, an interpreter, `git` or `cp` given a brain path. A shell standing in brain is refused whole.
"Names" is measured after the expansions the guard reads (`~`, the executor's environment, `..`, symlinks, the cwd the
command moves to, a value the command assigns); a COMMAND THAT NAMES BRAIN MUST BE FLAT (deny_brain_writes_shell.py,
docs/internal-limitations.md L36), and a path the guard cannot know is refused, not modelled. What it cannot see is code that
computes a path at run time (`$(printf …)`, a script): it stops an executor that follows a wrong instruction, not one
that works to evade it. It is not a sandbox.

LEAKS ARE JUDGED ON WHAT LEAVES: on any `git push` (and on `hw done`, see bin/hw), every object no remote has yet,
messages included, is scanned for a brain path, however the commit was made.

WHO LOADS IT: `hw`, for a product executor only — Claude through `--plugin-dir setup/guards/brain-guard`, pi
through its hw-executor adapter (`setup/pi/hw-executor.ts`, which spawns THIS file). Its own files live in
brain, out of the executor's reach by the same rule. FAILS CLOSED: an import failure, a crash, a guard past its
DEADLINE and a leak scan that cannot finish all deny.
"""
import copy
import functools
import glob
import json
import os
import re
import shlex
import signal
import subprocess
import sys
import time

# THE IMPORT IS GUARDED: a hook that dies with a traceback exits 1, which Claude Code reads as a non-blocking error —
# the command would run.
try:
    sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
    import deny_repo_writes as drw  # noqa: E402
    import deny_brain_writes_shell as sh  # noqa: E402
    import deny_brain_writes_main as main_guard  # noqa: E402 — main's guard, frozen: `judge` asks it first
except Exception as _exc:  # noqa: BLE001 — ANY failure here must deny
    print(json.dumps({"hookSpecificOutput": {"hookEventName": "PreToolUse", "permissionDecision": "deny",
                                              "permissionDecisionReason": (
        "Blocked: the brain guard could not load deny_repo_writes or its policy (%s: %s). It reached no verdict, and a "
        "guard that reached no verdict has not established that this is safe." % (type(_exc).__name__, _exc))}}))
    sys.exit(0)

WRITE_TOOLS = {"Write": ("file_path",), "Edit": ("file_path",), "MultiEdit": ("file_path",),  # Claude Code
               "NotebookEdit": ("notebook_path",), "write": ("filePath",), "edit": ("filePath",),  # lowercase spellings; no loader sends them now
               "multiedit": ("filePath",)}
PATCH_TOOLS = {"patch", "apply_patch"}
BASH_TOOLS = {"Bash", "bash"}

# brain's programs a product executor runs by path, measured on the real calls that named brain (channel-send is judged
# again in _channel_send_ok: only toward the brainer). `hw` is NOT here: brain-guard-programs.txt lists the verbs an
# executor runs (the rest are the brainer's). Not every bin/ program: `decisions archive --apply` writes into brain.
BIN_TOOLS = {"done-invoker", "ask-invoker", "channel-send", "browser-close", "herdr-rpc"}

# Other programs of brain a product executor runs, one per line in `setup/suite-data/brain-guard-programs.txt` (they
# name lanes; this directory names none, tests/195): `<path relative to brain> [via=<interp>[,…|-]] [first=<arg>[,…]]`
# (`-` in `via=` also allows running it by its own path). `first=` is the closed list of allowed first arguments.
PROGRAMS_FILE = os.path.join(os.path.dirname(os.path.dirname(os.path.realpath(__file__))),
                             "suite-data", "brain-guard-programs.txt")


def load_programs(path=None):
    out = {"bin/" + t: (None, None) for t in BIN_TOOLS}
    try:
        lines = open(path or PROGRAMS_FILE).read().splitlines()
    except OSError:
        return out
    for line in lines:
        parts = line.split("#", 1)[0].split()
        if not parts:
            continue
        opts = dict(p.split("=", 1) for p in parts[1:] if "=" in p)
        out[parts[0].strip("/")] = (opts.get("via"), opts.get("first"))
    return out


# The readers. A verb is here only if no flag of it writes, or the flags that do are refused in _read_verb_ok.
READ_VERBS = {"cat", "bat", "rg", "grep", "egrep", "fgrep", "sed", "head", "tail", "wc", "ls", "eza", "fd", "find",
              "test", "[", "stat", "file", "jq", "echo", "printf", "realpath", "readlink", "dirname", "basename", "which",
              "diff", "cmp", "sort", "uniq", "cut", "tr", "tree", "du", "shasum", "md5", "nl", "column"}
# Shell words that open or close a compound command; the verb is after them.
RESERVED = {"if", "then", "else", "elif", "fi", "do", "done", "while", "until", "!", "{", "}", "time", "coproc"}

WHY = ("brain is its operator's own project: a product executor names it only to read (cat, rg, sed -n …) or to run "
       "its bin/ tools by path (done-invoker, ask-invoker, channel-send, hw, browser-close, herdr-rpc)")
HOW = ("Persist what you need in $HW_ARTIFACTS or in engram; to use a brain file, read it "
       "(e.g. `cat <brain>/x > \"$HW_ARTIFACTS/x\"`)")

MARK, UNSURE = sh.MARK, sh.UNSURE
_GLOB, _brace = drw._GLOB_CHARS, drw._brace


def brain_config(root=None, programs=None):
    root = drw._norm(root or drw.BRAIN_ROOT)  # on Windows the one canonical spelling
    # The guard's own files are protected with brain: an executor that could edit them, or the config that loads them,
    # could switch the guard off.
    own = os.path.dirname(os.path.realpath(__file__))
    cfg = {"root": root, "roots": (root, own)}
    cfg["resolved_roots"] = drw._resolve_roots(cfg["roots"])
    cfg["brains"] = drw._resolve_roots((root,))
    # The checkout this guard runs from carries the same bin/: an executor launched from a setup worktree is handed ITS
    # done-invoker.
    cfg["own_bin"] = os.path.join(os.path.dirname(os.path.dirname(own)), "bin")
    cfg["programs"] = load_programs(programs)
    home = drw._policy_home()
    spellings = set(drw._resolve_roots((root,)))
    for s in list(spellings):
        if s.startswith(home + "/"):
            # `~/<x>`, `$HOME/<x>` and a bare `<x>` all carry this tail.
            spellings.add(s[len(home) + 1:])
    # `/+`: `$TMPDIR/x` is as often written with a `//` in it.
    cfg["spell"] = re.compile(r"(?<![\w.-])(?:%s)(?![\w.-])" % "|".join(
        "/+".join(re.escape(p) for p in s.strip("/").split("/")) for s in sorted(spellings, key=len, reverse=True)))
    return cfg


def _refuse(rule, what):
    return (rule, "Blocked (brain guard, rule '%s'): %s. The rule: %s. %s." % (rule, what, WHY, HOW))


def _inside(cfg, path):
    # drw._is_abs, not startswith("/"): on Windows C:\x and C:/x are absolute too, and a test that knew only "/"
    # answered them "outside" — an allow.
    return bool(path) and drw._is_abs(path) and drw._inside_any(cfg, drw._norm(path))


# ── the shell: deny_brain_writes_shell.py reads it, and says whether a command is flat ──────────────────

# The interpreters: a program that runs a script it is handed (`_program`) or the code on its stdin (`_reads_code`).
INTERPRETERS = {"python3", "python", "bash", "sh", "zsh", "dash", "ksh", "mksh", "yash", "fish", "csh", "tcsh", "node",
                "perl", "ruby", "eval", "source", ".", "xargs", "parallel"}


def _interpreter(word):
    """True when this word, run as a program, is one of INTERPRETERS (`python3.12` too)."""
    name = os.path.basename(word)
    return name in INTERPRETERS or (name.startswith("python") and bool(re.fullmatch(r"python[\d.]*", name)))


# Declaration builtins: `export A` hands A's value to what runs next.
_DECLARERS = {"export", "declare", "typeset", "readonly", "local", "integer", "float", "private"}


# ── the rule ────────────────────────────────────────────────────────────────

def _names(cfg, value, cwd):
    """Does this word name brain, however it is spelled?"""
    if not value:
        return False
    literal = value.replace(MARK, "")
    if _spells(cfg, literal):
        return True
    # `FOO=<path>` names <path>, and so does each part of `PATH=<a>:<b>`.
    m = re.match(r"^[A-Za-z_]\w*=(.+)$", value)
    if m and any(_names(cfg, part, cwd) for part in m.group(1).split(":") if part):
        return True
    # An unknown piece (MARK) is unknowable before the shell runs, and may be empty (an unset variable, a `$(…)` that
    # prints nothing): the word names brain through its known part, read with each unknown piece before it gone too
    # (`$X../../../brain` is `../../../brain`); a relative path climbing out is refused below.
    for v in dict.fromkeys(v for b in _brace(value) for v in _known_starts(b)):
        if not v:
            continue
        if not ("/" in v or v in (".", "..") or v.startswith("~") or drw._is_abs(v)) and not cwd:
            continue
        p = v if drw._is_abs(v) else (drw._join(cwd, v) if cwd else None)
        if p is None:
            if v.startswith(".."):
                return True  # relative to a directory the guard cannot know
            continue
        if _inside(cfg, p):
            return True
        if _GLOB.search(v):
            try:
                if any(_inside(cfg, g) for g in glob.glob(_loose(p))[:256]):
                    return True
            except re.error:
                pass
    return False


def _loose(pattern):
    """Each `[` after the last `]` of its component read as `?` (more): fnmatch rescans for each unclosed `[`."""
    return re.sub(r"\[(?=[^\]/]*(?:/|$))", "?", pattern)


def _spells(cfg, text):
    """Does this text spell brain? On Windows the roots are canonical (lower
    case, `/c/`), so the text is read folded as well as as written."""
    return bool(cfg["spell"].search(text) or (drw._WINPATHS and cfg["spell"].search(drw._canon_text(text))))


def _data_names(cfg, text, cwd):
    """A heredoc or here-string body: its spellings, and each path in it (absolute, or from `cwd`). A reader may be fed
    it; any other program may make a path of it (`xargs rm`, `patch`, an interpreter)."""
    text = text.replace(MARK, "")
    return _spells(cfg, text) or _any_path(text, lambda p: (cwd or drw._is_abs(p)) and _inside(
        cfg, p if drw._is_abs(p) else drw._join(cwd, p)))


def _any_path(text, test):
    """Does `test` hold for a word of this body that may be a path (one holding a `/`, or `..`)? Past 512 of them it
    does: a cap that dropped the rest would let a padded body through."""
    paths = dict.fromkeys(p for p in re.findall(r"[^\s'\"`;|&<>(){}\[\]=:,]+", text) if "/" in p or p == "..")
    return len(paths) > 512 or any(test(p) for p in paths)


def _fields(word):
    """A (value, raw) word's value, and when the shell may split it (UNSURE) each of its fields: a word names brain
    also through one of them (`A="-o ../x"; sort $A`)."""
    return [word[0]] + (word[0].split() if UNSURE in word[1] else [])


# `env`'s options that take a value. The value is the option's, not the program (`env -u HW_TASK hw next` runs hw).
# `-S`/`--split-string` is also a whole command line, spliced into the words in place of its value.
_ENV_SHORT_VALUE = "uCPaS"
_ENV_LONG_VALUE = {"--unset": None, "--chdir": None, "--argv0": None, "--split-string": "S"}
# Every long option GNU env takes: getopt_long accepts any prefix that names exactly one (`--uns X` is `--unset X`). An
# ambiguous prefix makes env refuse; it is read as taking no value, which leaves more of the command in view.
_ENV_LONG_ALL = (*_ENV_LONG_VALUE, "--ignore-environment", "--null", "--debug", "--help", "--version",
                 "--list-signal-handling", "--block-signal", "--default-signal", "--ignore-signal")


def _env_long_name(name):
    """The long option `name` spells or abbreviates; `name` itself otherwise."""
    if name in _ENV_LONG_ALL:
        return name
    hits = [n for n in _ENV_LONG_ALL if n.startswith(name)]
    return hits[0] if len(hits) == 1 else name


# What a wrapper may skip (`env -u X`, `nice -n 10`, `timeout 5`): an option, value or assignment only when it is
# PROVABLY one word (an expanded word may be the verb, hide it, or vanish); any other leaves the verb undetermined
# (`_judge`). The parser marks UNSURE every word that may be several (an unquoted expansion, a glob, a brace, `"$@"`,
# `"${arr[@]}"`, zsh `"$arr[@]"`); quotes, `\x`, `$'…'` and a quoted `"$(…)"` are one word.


def _plain(word):
    """True when this (value, raw) word is provably ONE word to the shell."""
    return UNSURE not in word[1]


def _exact(word):
    """True when the guard knows this word: one word, and its value."""
    return _plain(word) and sh.known(word[0])


def _env_option(words, i):
    """Index past the `env` option at words[i] and the separate-word value it takes (an
    attached one, `-uX`, is part of the word). Splices the words of a `-S` string into
    `words` after the option (in place). What may expand is judged by `_judge`."""
    word = words[i][0]
    split = None
    if word.startswith("--"):
        name, eq, value = word.partition("=")
        name = _env_long_name(name) if name != "--" else name
        if name not in _ENV_LONG_VALUE:
            return i + 1
        if _ENV_LONG_VALUE[name]:
            split = (i, value if eq else None)
        j = i + 1 if eq else min(i + 2, len(words))
    elif word.startswith("-") and len(word) > 1:
        j = i + 1
        for k, letter in enumerate(word[1:], 1):
            if letter not in _ENV_SHORT_VALUE:
                continue
            attached = word[k + 1:]
            if letter == "S":
                split = (i, attached if attached else None)
            j = i + 1 if attached else min(i + 2, len(words))
            break
    else:
        return i + 1
    if split is not None:
        at, value = split
        if value is None and at + 1 < len(words):
            value = words[at + 1][0]
        try:
            parts = shlex.split(value) if value is not None else []
        except ValueError:
            parts = []
        if parts and value is not None:
            first = at + 1 if split[1] is None else at
            words[first:first + 1] = [(w, w + (UNSURE if "$" in w else "")) for w in parts]
            return first
    return min(j, len(words))


_NICE_NUMBER, _NICE_LONG = r"[+-]?\d+", ("--adjustment",)


def _wrapper_value(words, j, number):
    """True when words[j] is the value a wrapper's option or duration takes: a literal `number`, or a word the shell
    expands (it may be the number, or hide the verb: `_judge` refuses a command that names brain behind it)."""
    return j < len(words) and (not _exact(words[j]) or bool(re.fullmatch(number, words[j][0])))


def _unwrap(seg):
    """(index of the verb among seg.words, verb, the words the wrappers skipped to reach it). Every word from the
    first wrapper to the verb is skipped: its name, its options, their values, an assignment, a duration. `_judge`
    refuses a command that names brain behind a skipped word that is not `_plain`."""
    i, words = 0, seg.words
    while i < len(words) and words[i][1] in RESERVED:
        i += 1
    while i < len(words) and sh.ASSIGN.match(words[i][1]):
        i += 1
    first = i  # the first wrapper, when there is one: everything from here to the verb is skipped
    while i < len(words):
        name = os.path.basename(words[i][0])
        if name in sh.PRECOMMANDS:
            i += 1
            while i < len(words) and (words[i][0].startswith("-") or re.match(r"^[A-Za-z_]\w*=", words[i][1])):
                if name == "env":
                    i = _env_option(words, i)
                elif name == "nice" and sh.takes_next(words[i][0], "n", _NICE_LONG) and _wrapper_value(words, i + 1, _NICE_NUMBER):
                    i += 2  # `-n 10` / `--adjustment 10`: the number is the option's, not the verb
                elif name == "nice":
                    i += 1
                else:
                    i += 1 + sh.takes_next(words[i][0], *sh.PRECOMMANDS[name])
            continue
        if name == "timeout":
            i += 1
            while i < len(words) and words[i][0].startswith("-"):
                i += 1 + sh.takes_next(words[i][0], "sk", ("--signal", "--kill-after"))
            if i < len(words):
                i += 1  # the duration: timeout's first operand, whatever its spelling (`.5`, `1e3`, `inf`)
            continue
        # setup/heavy [--tag T] [--short] [--max S] -- <cmd>: the slot is not the verb, the command after `--` is, and
        # it is judged like any other.
        if (words[i][0] == "setup/heavy" or words[i][0].endswith("/setup/heavy")) \
                and any(w == "--" for w, _ in words[i + 1:]):
            i = next(j for j in range(i + 1, len(words)) if words[j][0] == "--") + 1
            continue
        # hw secret run <item> --as VAR -- <cmd>: the same shape. The value goes into <cmd>'s environment; <cmd> is the
        # verb, judged on its own.
        if os.path.basename(words[i][0]) == "hw" and [w for w, _ in words[i + 1:i + 3]] == ["secret", "run"] \
                and any(w == "--" for w, _ in words[i + 3:]):
            i = next(j for j in range(i + 3, len(words)) if words[j][0] == "--") + 1
            continue
        break
    return i, (words[i][0] if i < len(words) else ""), words[first:i]


def _looks_up(prefix):
    """True when the wrappers before the verb are `command -v`/`-V`: the verb is
    looked up, the way `H=$(command -v hw); $H …` finds a program to run later."""
    for i, (w, _) in enumerate(prefix):
        if os.path.basename(w) == "command":
            return any(re.match(r"^-[a-zA-Z]*[vV]", f) for f, _ in prefix[i + 1:])
    return False


def _flag(args, letters, longs=()):
    """A short flag among `letters`, alone or in a cluster (`-uo`, `-HX`), or a long one starting with a `longs`."""
    for a in args:
        if a.startswith("--"):
            if longs and a.startswith(tuple(longs)):
                return True
        elif re.match(r"^-[A-Za-z0-9]*[%s]" % letters, a):
            return True
    return False


# sed is a reader only for scripts made of addresses and non-writing commands. A closed grammar, not a search for `w`:
# sed accepts `w<file>`, `gw`, `-e<s>`, `--expression=<s>` and more, and each was a way past a search.
_SED_S = re.compile(r"s(?P<d>[^\\\n\s])(?:\\.|(?!(?P=d)).)*(?P=d)" r"(?:\\.|(?!(?P=d)).)*(?P=d)[gpiIM0-9]*")
_SED_Y = re.compile(r"y(?P<d>[^\\\n\s])(?:\\.|(?!(?P=d)).)*(?P=d)(?:\\.|(?!(?P=d)).)*(?P=d)")
_SED_ADDR = re.compile(r"\\(?P<d>.)(?:\\.|(?!(?P=d)).)*(?P=d)I?|/(?:\\.|[^/])*/I?")
_SED_REST = re.compile(r"^[0-9$,;!{}\s~+pPdDqQnNlgGhHxz=]*$")


def _sed_script_reads(script):
    rest = _SED_ADDR.sub(" ", _SED_Y.sub(" ", _SED_S.sub(" ", script)))
    return bool(_SED_REST.match(rest))


def _sed_reads(args):
    scripts, i, explicit = [], 0, False
    operands = []
    while i < len(args):
        a = args[i]
        if a in ("-e", "--expression"):
            scripts.append(args[i + 1] if i + 1 < len(args) else ""); explicit = True; i += 2; continue
        if a.startswith("--expression="):
            scripts.append(a.split("=", 1)[1]); explicit = True; i += 1; continue
        if a.startswith("--"):
            if a.startswith(("--in-place", "--file")):
                return False
            i += 1; continue
        if a.startswith("-") and len(a) > 1:
            flags = a[1:]
            if re.search(r"[iIf]", flags.split("e", 1)[0]):
                return False  # in place, or a script from a file the guard cannot read
            if "e" in flags:
                attached = flags.split("e", 1)[1]
                if attached:
                    scripts.append(attached)
                else:
                    scripts.append(args[i + 1] if i + 1 < len(args) else ""); i += 1
                explicit = True
            i += 1; continue
        operands.append(a); i += 1
    if not explicit and operands:
        scripts.append(operands[0])
    return all(_sed_script_reads(sc) for sc in scripts)


# The readers that have a flag that writes: (does this argument list write?, what writes). One table: `_read_verb_ok`
# refuses the flag, `_judge_head` refuses an argument it cannot read (it may be the flag).
_WRITES = {
    "sed": (lambda a: not _sed_reads(a), "-i, w"),
    "fd": (lambda a: _flag(a, "xX", ("--exec",)), "-x"),
    "find": (lambda a: any(x in ("-delete", "-exec", "-execdir", "-ok", "-okdir", "-fls") or x.startswith("-fprint")
                           for x in a), "-delete, -exec"),
    "rg": (lambda a: any(x.startswith("--pre") for x in a), "--pre"),
    "sort": (lambda a: _flag(a, "o", ("--output",)), "-o"),
    "tree": (lambda a: _flag(a, "o", ("--output",)), "-o"),
    "uniq": (lambda a: len([x for x in a if not x.startswith("-")]) > 1, "a second operand"),  # `uniq in out`
}


def _reader_args(words):
    """The arguments a reader that has a writing flag receives, or None when the guard cannot tell (one may be the
    flag): an exact word is itself, a known value the shell splits (an environment variable) its fields. A flat
    command has no glob or brace to read (deny_brain_writes_shell.py)."""
    out = []
    for w in words:
        if _exact(w):
            out.append(w[0])
        elif sh.known(w[0]) and not _GLOB.search(w[0]):
            out += w[0].split()
        else:
            return None
    return out


def _read_verb_ok(verb, args):
    """False when a reader is asked for one of its writing flags."""
    return verb not in _WRITES or not _WRITES[verb][0](args)


def _judge(cfg, seg, cwd, local=None, env=None):
    """None, or a refusal, for one simple command."""
    for op, target, _ in seg.redirects:
        if op.startswith("<") and op not in ("<>",):
            if _names(cfg, target, cwd):
                seg.reads_brain = True
            continue
        if _names(cfg, target, cwd):
            return _refuse("write", "this command redirects its output into brain (%s)" % _show_value(target))
    idx, verb, skipped = seg.head
    refusal = _judge_head(cfg, seg, cwd, local, env, idx, verb)
    if refusal:
        return refusal
    # A wrapper word the shell expands may be the verb, or hide it: the verb is UNDETERMINED, and a command that names
    # brain is refused whatever the word after it is (a reader included), anywhere in the command.
    odd = _undetermined(seg.words, idx, skipped)
    if odd is None:
        return None
    if _seg_names(cfg, seg, cwd):
        return _refuse("verb", "`%s` is expanded by the shell, so the verb is undetermined and this command names "
                       "brain: spell it literally (`env -u HW_TASK`) or quote it as one word, in single quotes if it "
                       "holds a backslash (`env PATH=\"$PATH\"`, `env -u 'X Y'`)" % _show(odd))
    # A verb that is one read of a variable the command assigns other than once (`for p in …; do $p`) is each value the
    # command gives it, or the one it had.
    is_verb = odd is seg.words[idx]
    values = _read_values(odd, seg, _each(local, env)) if is_verb else [odd[0]]
    # A verb that may expand to nothing leaves the next word the verb: that command is judged too.
    if is_verb and any(not v.strip() for v in values):
        rest = copy.copy(seg)
        rest.words = seg.words[:idx] + seg.words[idx + 1:]
        rest.head = _unwrap(rest)
        refusal = _judge(cfg, rest, cwd, local, env)
        if refusal:
            return refusal[0], refusal[1].replace("): ", "): `%s` may expand to nothing, which leaves the next word the "
                                                  "verb, and " % _show(odd), 1)
    # ... and, naming none, it may still BE a program of brain's bin/, which PATH finds by its bare name.
    if any(_may_run_bin(cfg, (v, odd[1]), is_verb) for v in values):
        return _refuse("run", "`%s` is expanded by the shell and may be any program, brain's bin/ included (PATH finds "
                       "them by name), so what runs is undetermined: spell the program, or give it a path whose last "
                       "part is literal (`\"$(npm bin)\"/eslint`)" % _show(odd))
    return None


def _read_values(word, seg, each):
    """The values of a word whose one unknown piece is a plain read (`seg.reads`) of a name this command assigns other
    than once, each value `each` (`_each`) gives that name in its place. Any other word: itself."""
    name = seg.reads.get(word)
    if name not in each:
        return [word[0]]
    pre, post = word[0].split(MARK)
    return [pre + v + post for v in each[name]]


def _each(values, env):
    """Each value each name the command assigns may hold: an assignment's, the environment's, or empty."""
    return {n: list(dict.fromkeys([v for v, _ in vs] + [(env or {}).get(n, ""), ""])) for n, vs in (values or {}).items()}


def _show_value(value):
    return value.replace(MARK, "…")


def _show(word):
    return _show_value(word[1].replace(UNSURE, ""))


def _undetermined(words, idx, skipped):
    """The word that leaves this command's verb undetermined, or None: a wrapper word that is not provably one word
    (`skipped` holds every word the wrappers stepped over), or a verb that is not exact."""
    odd = next((w for w in skipped if not _plain(w)), None)
    if odd is None and idx < len(words) and not _exact(words[idx]):
        odd = words[idx]
    return odd


def _path_word(word):
    """True when this word, run as a program, is surely a path (a `/` in the part the guard knows of its first field),
    so neither a builtin nor a name PATH looks up."""
    value = word[0]
    if UNSURE not in word[1]:  # one word, as the parser read it
        return "/" in value.replace(MARK, "")
    return "/" in re.split(f"[\\s{MARK}]", value, 1)[0]


def _first_field(value):
    """The first field of a value the shell splits, when the guard knows it whole (a blank follows it before any
    unknowable part, and it holds no glob or brace), else None."""
    m = re.match(rf"\s*([^\s{MARK}*?\[{{]+)(?:\s|$)", value)
    return m.group(1) if m else None


def _may_run_bin(cfg, word, verb=False):
    """True when this undetermined word may run a program of brain's bin/ by its bare name: its first field is not
    surely a path, and its name is unknowable, a glob, or one of brain's programs."""
    names, value = _bin_names(cfg), word[0]
    if _path_word(word):
        return False
    if verb and not value.strip():
        return False  # the word may vanish: `_judge` judges the next one as the verb
    first = _first_field(value) if verb and UNSURE in word[1] else None
    if first is not None:
        return first in names
    if UNSURE not in word[1]:
        # one word: the program is the whole word, so its known start must start one of brain's programs
        known_start = value.split(MARK, 1)[0]
        return (value in names) if sh.known(value) else any(n.startswith(known_start) for n in names)
    return not sh.known(value) or any(re.search(r"[*?\[(]", f) or f in names
                                    for alt in _brace(value) for f in alt.split() if "/" not in f)


def _bin_dirs(cfg):
    return [os.path.join(r, "bin") for r in cfg["brains"]] + [cfg["own_bin"]]


def _bin_names(cfg):
    """The programs of brain's bin/ a bare name finds: brain's own, and the checkout this guard runs from (listed once per decision)."""
    if "bin_names" not in cfg:
        names = set()
        for d in _bin_dirs(cfg):
            try:
                names.update(os.listdir(d))
            except OSError:
                pass
        cfg["bin_names"] = names
    return cfg["bin_names"]


def _assigned_names(cfg, values, name, cwd):
    """True when a value this command gives NAME names brain (`values`: Shell.values)."""
    return any(_names(cfg, v, cwd) for v, _ in (values or {}).get(name, ()))


def _judge_head(cfg, seg, cwd, local, env, idx, verb):
    """None, or a refusal, for one simple command read with the verb at seg.words[idx]."""
    args = [w for w, _ in seg.words[idx + 1:]]
    prefix = seg.words[:idx]
    names_in_args = any(_names(cfg, f, cwd) for w in seg.words[idx + 1:] for f in _fields(w))
    names_in_prefix = any(_names(cfg, w, cwd) and not _heavy_ok(cfg, w) for w, _ in prefix)
    names_verb = _names(cfg, verb, cwd)
    name = os.path.basename(verb)
    data_names = any(_data_names(cfg, d, cwd) for d in seg.data)
    prog = _program(cfg, verb, name, args, cwd)
    if prog is not None:
        rel, via = prog
        want = cfg["programs"].get(rel)
        rest = args[1:] if via else args
        # A DRY RUN CREATES NOTHING: `hw <lane> <task> --dry-run` rehearses a dispatch. The DISPATCH only: a brainer
        # verb that tolerates the flag still changes state, so BRAINER_FIRSTS never takes the bypass.
        dry_run = (rel == "bin/hw" and via is None and "--dry-run" in rest and rest and rest[0] not in BRAINER_FIRSTS)
        if not dry_run and want and via in _vias(want[0]) and not _first_ok(want[1], rest):
            return _refuse("run", "`%s` takes only these first arguments here; call it "
                           "as: %s" % (rel, _allowed_form(cfg, rel, want)))
        # A lookup (`command -v hw`) is not a call, and the bare call is allowed: the lookup of a `first=` program stays
        # refused, as it was when its bare call was.
        known = bool(want) and via in _vias(want[0])
        if _looks_up(prefix) and (not known or (want[1] and (not rest or rest[0] not in want[1].split(",")))):
            return _refuse("run", "`command -v %s` looks a brain program up to run it through a variable, and the "
                                  "guard cannot know what a variable runs: call `%s` by its name or by its path, not "
                                  "through a variable" % (name, name))
        if not known:
            return _refuse("run", "`%s` is not one of the brain programs a product "
                           "executor runs (%s)" % (rel, _programs_named(cfg)))
        if rel == "bin/channel-send" and via is None:
            why = _channel_send_refusal(rest, env or {})
            if why:
                return _refuse("run", why)
        if rel == "bin/hw" and via is None and rest[:1] == ["done"]:
            pos = [a for a in rest[1:] if not a.startswith("-")]
            own = (env or {}).get("HW_TASK", "")
            if own and len(pos) >= 2 and pos[1] != own:
                return _refuse("run", "`hw done` closes only your own task (%s), not %s" % (own, _show_value(pos[1])))
        # `hw reap` reads (a survey); `--apply` removes. Only the survey, and an argument the guard cannot know before
        # the shell runs is not one.
        if rel == "bin/hw" and via is None and rest[:1] == ["reap"] and any(
                a == "--apply" or not sh.known(a) for a in rest[1:]):
            return _refuse("run", "`hw reap --apply` removes worktrees, which is the "
                           "brainer's; an executor runs the survey, `hw reap <lane>`")
        # `hw secret run|put|check|status|ensure` are an executor's; `unlock` is a person's, in a terminal (bw asks the
        # master password there), as `secret-unlock` already is by not being on the list.
        if rel == "bin/hw" and via is None and rest[:2] == ["secret", "unlock"]:
            return _refuse("run", "`hw secret unlock` opens the vault with the master password, which is a person's in "
                                  "a terminal; an executor runs `hw secret run|put|check|status|ensure`")
        if rel == "bin/hw" and via is None and any(w.startswith("HW_DONE_ALLOW_BRAIN_LEAK=") for w, _ in prefix):
            return _refuse("run", "HW_DONE_ALLOW_BRAIN_LEAK is the operator's exit from "
                           "the leak check on `hw done`, never an executor's")
        names_in_args = names_in_args and via is None
        names_verb = False
        if not names_in_prefix or _prefix_ok(cfg, prefix, cwd):
            return None
    if name in _DECLARERS and local:
        names_in_args = names_in_args or any(_assigned_names(cfg, local, a, cwd) for a in args if "=" not in a)
    if not (names_in_args or names_in_prefix or names_verb or data_names or getattr(seg, "reads_brain", False)):
        return None
    # An assignment alone writes nothing; its value is judged where it is used. A quoted empty word is a verb (B2).
    if idx >= len(seg.words) and not seg.redirects:
        return None
    if name in _DECLARERS:
        named = [a for a in args if "=" not in a and _assigned_names(cfg, local, a, cwd)]
        if not named and all(_path_ok(cfg, a, cwd) for a in args if _names(cfg, a, cwd)):
            return None
        return _refuse("export", "this command exports a variable that points into brain, and whatever it runs next "
                                 "would inherit it (only PATH may carry brain's bin/)")
    if names_in_prefix and not _prefix_ok(cfg, prefix, cwd):
        return _refuse("env", "an environment assignment on this command points into brain (%s)"
                       % _show_value(next(w for w, _ in prefix if _names(cfg, w, cwd))))
    if names_verb:
        return _refuse("run", "`%s` is not one of the brain programs a product executor "
                       "runs (%s)" % (verb, _programs_named(cfg)))
    if name in ("cd", "pushd"):
        return _refuse("cd", "this command cd's into brain; work in your worktree or work dir and read brain by path")
    argv = _reader_args(seg.words[idx + 1:]) if name in _WRITES else args
    if argv is None:
        return _refuse("verb", "`%s` has flags that write (%s) and an argument of this command is expanded by the shell, "
                       "so it may be one: spell the argument literally, in single quotes if it holds a backslash"
                       % (name, _WRITES[name][1]))
    if name in READ_VERBS and _read_verb_ok(name, argv):
        return None
    if name == "git" and not names_in_args and data_names:
        return None  # a heredoc message: judged as a leak on push, not here
    return _refuse("verb", "`%s` is given a brain path, and only a reader may be "
                   "(cat, bat, rg, grep, sed -n, head, tail, ls, eza, fd, find, jq, diff …)"
                   % (name or verb))


def _same_dir(a, b):
    if drw._WINPATHS:
        return drw._real(a) == drw._real(b)
    return os.path.realpath(a) == b


def _rel(cfg, path):
    """`path` relative to brain, or None when it is not inside it."""
    # The roots are canonical on Windows (drw._canon), so the path is too.
    real = drw._real(path) if drw._WINPATHS else os.path.realpath(path)
    for r in cfg["brains"]:
        if real.startswith(r + "/"):
            return real[len(r) + 1:]
    return None


def _vias(spec):
    """The ways a listed program may be run: `via=bash,-` is through bash or by
    its own path (`-`); no `via=` is by path only."""
    if not spec:
        return {None}
    return {None if v == "-" else v for v in spec.split(",")}


# A program with `first=` is also allowed with no arguments and with one of these first: they print the usage and change
# nothing, which is how an agent that does not know the verbs finds them.
HELP_FIRST = ("help", "-h", "--help")

# The verbs hw-core's own executor gate refuses: `--dry-run` is no validation for them (`hw sweep --apply --dry-run`
# ignores it). A rehearsal is a DISPATCH, whose first word is the lane's.
BRAINER_FIRSTS = ("next", "ruling", "unstick", "reap", "sweep", "briefs", "preview", "revive", "train", "done")


def _first_ok(first, rest):
    return not first or not rest or rest[0] in first.split(",") or rest[0] in HELP_FIRST


def _allowed_form(cfg, rel, want):
    """The call the line allows, spelled as it is typed: `bash <path> a|b|c (or --help)`."""
    run = next((v for v in want[0].split(",") if v != "-"), "") if want[0] else ""
    return "%s%s %s (or --help)" % (run + " " if run else "", os.path.join(cfg["root"], rel),
                                    "|".join(want[1].split(",")))


def _program(cfg, verb, name, args, cwd):
    """(path relative to brain, interpreter or None) when this command runs a program that lives in brain — by path, by
    a bare name brain's bin/ holds, or as a script handed to an interpreter — else None."""
    if not verb or not sh.known(verb):
        return None
    if "/" in verb:
        full = verb if verb.startswith("/") else os.path.join(cwd or "/", verb)
        if _same_dir(os.path.dirname(full), cfg["own_bin"]):
            return "bin/" + name, None
        rel = _rel(cfg, full)
        return (rel, None) if rel else None
    if _interpreter(name):
        script = next((a for a in args if not a.startswith("-")), None)
        if script and sh.known(script) and "/" in script:
            rel = _rel(cfg, script if script.startswith("/") else os.path.join(cwd or "/", script))
            if rel:
                return rel, name
        return None
    for d in _bin_dirs(cfg):
        if os.path.isfile(os.path.join(d, verb)):
            return "bin/" + verb, None
    return None


# channel-send's flags that take a value; the rest take none.
_CS_VALUE_FLAGS = {"--require", "--id", "--report-state", "--reply-hold"}


def _channel_send_refusal(args, env):
    """None when this channel-send goes to the executor's own brainer pane (HW_INVOKER_PANE), else why not. Its
    positionals are `<vendor> <target> <endpoint> <message>` after the flags. `--help` / `-h` first sends nothing: it
    is the exit-code table CLAUDE.shared.md tells an executor to read."""
    if args[:1] in (["--help"], ["-h"]):
        return None
    pos, i = [], 0
    while i < len(args):
        a = args[i]
        if a == "--ruling":
            return ("`channel-send --ruling` steers another run; a ruling is the brainer's (hw ruling)")
        if a in _CS_VALUE_FLAGS:
            i += 2
            continue
        if a.startswith("--") and not pos:
            i += 1
            continue
        pos.append(a)
        i += 1
    target, pane = (pos[1] if len(pos) > 1 else ""), env.get("HW_INVOKER_PANE", "")
    if pane and sh.known(target) and target == pane:
        return None
    return ("`channel-send` goes only to your brainer's pane (HW_INVOKER_PANE=%s), "
            "and its target here is %r" % (pane or "unset", _show_value(target)))


def _programs_named(cfg):
    return ", ".join(sorted("bin/" + t for t in BIN_TOOLS)) + " and the ones in setup/suite-data/brain-guard-programs.txt"


def _path_ok(cfg, word, cwd):
    """`PATH=…` whose every part inside brain is brain's own bin/."""
    if not word.startswith("PATH="):
        return False
    for part in word[5:].split(":"):
        real = drw._real(part) if drw._WINPATHS else os.path.realpath(part)
        if _names(cfg, part, cwd) and real not in {
                os.path.join(r, "bin") if not drw._WINPATHS else r + "/bin" for r in cfg["brains"]}:
            return False
    return True


def _heavy_ok(cfg, word):
    """brain's own setup/heavy or bin/hw, the wrapper _head stepped over to reach the verb."""
    real = drw._real(word) if drw._WINPATHS else os.path.realpath(word)
    return any(real in (os.path.join(r, "setup", "heavy"), os.path.join(r, "bin", "hw")) for r in cfg["brains"])


def _prefix_ok(cfg, prefix, cwd):
    return all(_path_ok(cfg, w, cwd) for w, _ in prefix if _names(cfg, w, cwd))


def _touches(cfg, text, shell, cwds, env):
    """Does this command (not flat) name a brain path: its text spells one, a word, redirect or code body names one, or
    a value it may hold leads there (`_toward`: brain, inside it, a directory that holds it, a start the shell completes)?
    Each assigned value; each word with an unknown piece, as each value its one plain read may take; past VALUES_BUDGET
    values, the command. (A brain program by its bare name names none: closed lists; a computed verb: `_judge`.)"""
    segs = shell.segs
    if _spells(cfg, text):
        return True
    each = _each(shell.values, env)
    unknown = {(w, c): s for s, c in zip(segs, cwds) for w in s.words + [(t, r) for _, t, r in s.redirects]
               if MARK in w[0]}
    # Counted before any value is built: the budget bounds the work, not only the verdict.
    if sum(len(each.get(s.reads.get(w), [w])) for (w, _), s in unknown.items()) > VALUES_BUDGET:
        return True
    return (any(_seg_names(cfg, s, c, path_ok=True) for s, c in zip(segs, cwds))
            or any(any(_toward(cfg, part + MARK, cwds[0]) for part in v.split(":") if part)
                   and not (name == "PATH" and _path_ok(cfg, "PATH=" + v, cwds[0]))
                   for name, vals in shell.values.items() for v in dict.fromkeys(v for v, _ in vals))
            or any(_any_path(d, lambda p: _toward(cfg, p + MARK, c)) for s, c in zip(segs, cwds) for d in s.code)
            or any((v is not w[0] and _names(cfg, v, c)) or _toward(cfg, v, c)
                   for (w, c), s in unknown.items() for v in _read_values(w, s, each)))


def _unexpanded(seg, quiet, later):
    """The words the guard cannot resolve (`Seg.forms`) in a PATH POSITION: a redirect target, an operand of a command
    that may write, a `cd` target before one (`later`); not an input redirect, nor a reader's argument when `quiet`."""
    idx, verb, _ = seg.head  # assignments alone are data: a value that is a form makes its name's reads forms
    data = seg.words if idx == len(seg.words) else seg.words[idx + 1:] if quiet and not seg.nested and _reads(seg) or (
        os.path.basename(verb) in sh._MOVES and not any(map(_writes, later))) else []
    skip = {*data, *((v, r) for _, v, r in seg.redirects)} - set(_outs(seg))  # an output target is never data
    return [w for w in seg.forms if w not in skip]


def _reads(seg):  # a reader by its exact verb, without its writing flag (`_WRITES`)
    idx, verb, _ = seg.head
    argv = _reader_args(seg.words[idx + 1:]) if verb in _WRITES else ()
    return idx < len(seg.words) and _exact(seg.words[idx]) and verb in READ_VERBS and argv is not None and _read_verb_ok(verb, argv)


def _writes(seg):  # an output redirect to a file, or a verb that is neither a reader nor a `cd`
    return bool(_outs(seg)) or not (_reads(seg) or os.path.basename(seg.head[1]) in sh._MOVES)


def _quiet(segs, renames, loops):
    """Nothing a reader prints can come back as a path or code: no rename, `exec`, function or `printf -v`, no pipe into
    a non-reader, and once a reader or a block fed a file (in a loop, anywhere) nothing that may read it back."""
    def assigns(s):  # an option word not a known literal counts as `-v` (`"-v"`, `\-v`, `$F`)
        for v, _ in s.words[s.head[0] + 1:] if os.path.basename(s.head[1]) in ("printf", "print") else ():
            if not sh.known(v) or v.startswith("-") and "v" in v:
                return True
            if not v.startswith("-") or v == "--":
                break
        return False

    feeds = lambda s: (not s.head[1] or s.head[1] in READ_VERBS) and bool(_outs(s))  # noqa: E731
    fed = loops and any(map(feeds, segs))
    for s in segs:
        name = os.path.basename(s.head[1])
        if renames or assigns(s) or s.after_pipe and not _reads(s) or s.words[:1] == [("function",) * 2] or any(
                r == "exec" or r.endswith("()") for _, r in s.words[:s.head[0] + 1]) or fed and (
                s.nested or s.data or _interpreter(name) or any(
                    op.startswith("<") and v not in _SCREEN for op, v, _ in s.redirects)):
            return False
        fed = fed or feeds(s)
    return True


def _outs(seg):  # the output redirect targets to a file (`<>` creates one): not an input, the terminal or /dev/null
    return [(v, r) for op, v, r in seg.redirects if (not op.startswith("<") or op == "<>") and v not in _SCREEN]


_SCREEN = ("/dev/null", "/dev/stdout", "/dev/stderr", "/dev/tty")


@functools.lru_cache(maxsize=None)  # one hook process, one filesystem: each start is resolved once
def _completed(k):
    """The absolute start `k` resolved physically and logically (`cd`), its tail kept as the part a form completes."""
    head, tail = k.rsplit("/", 1)
    return {f(k) if tail in (".", "..") else f(head or "/").rstrip("/") + "/" + tail for f in (drw._real, drw._norm)}


def _toward(cfg, value, cwd):
    """True when `value` may lead into brain: its known part before an unknown piece (those before it read as empty),
    as a path from `cwd`, starts brain's path or lies inside it (`<agents>`, `~/ag…`, `../../..`)."""
    return any(_toward_start(cfg, k, cwd) for k in dict.fromkeys(_known_starts(value)[:-1]))


def _known_starts(value):
    """The known text before each unknown piece of `value`, each piece before it read as empty (`$X../..$Y` gives ``,
    `../..`), then the whole text with every piece empty: an unknown piece may be nothing (an unset variable, a `$(…)`
    that prints nothing)."""
    parts = value.split(MARK)
    return ["".join(parts[:k + 1]) for k in range(len(parts))]


def _toward_start(cfg, k, cwd):
    """True when the known start `k` of a path the shell completes may lead into brain (`_toward`)."""
    if not k:
        return False
    if not drw._is_abs(k):
        if not cwd:
            return k.startswith("..")  # relative to a directory the guard cannot know
        k = drw._join(cwd, k)
    # `/` alone tells no more than an unknown path does (`$(…)/x`, the stated limit); either resolution counts
    return any(c.strip("/") and (_inside(cfg, c) or any((b + "/").startswith(c) for b in cfg["brains"])) for c in _completed(k))


def _seg_names(cfg, seg, cwd, path_ok=False):
    """Does a word, redirect or body of this command name brain (`path_ok`: not a `PATH=` whose only brain part is
    brain's bin/, which the executor's own PATH carries)?"""
    return (any(_names(cfg, w, cwd) and not (path_ok and _path_ok(cfg, w, cwd)) for word in seg.words for w in _fields(word))
            or any(_names(cfg, t, cwd) for _, t, _ in seg.redirects)
            or any(_data_names(cfg, d, cwd) for d in seg.data))


def _reads_code(name, args):
    """True for an interpreter that runs the code on its stdin: no `-c`/`-e` code, no script operand (`sh`, `bash -s`,
    `python3 -`)."""
    return _interpreter(name) and ("-s" in args or not any(
        re.match(r"^-[A-Za-z]*[ceE]", a) or not a.startswith("-") for a in args))


# ── leaks: what leaves, not how it was made ─────────────────────────────────

LEAK_BUDGET = 40  # seconds; past it the scan denies rather than guess
# Seconds the guard spends reading and judging a command (the leak scan has its own LEAK_BUDGET): past it, it denies
# naming the timeout, and past twice it of CPU the kernel ends it (SIGPROF), which `|| exit 2` in hooks.json blocks.
DEADLINE = 5
VALUES_BUDGET = 4096  # the values `_touches` reads for the words with an unknown piece: past it, the command names brain


class GuardTimeout(BaseException):  # not an Exception, nor an OSError: no `except` on the way swallows it
    pass


def _expired(*_):
    raise GuardTimeout("the guard did not reach a verdict within %s s" % DEADLINE)


def _bound(seconds):
    """Bound the guard's own work to `seconds` (0: unbound), where there is a setitimer."""
    if hasattr(signal, "setitimer"):
        signal.signal(signal.SIGALRM, _expired)
        signal.setitimer(signal.ITIMER_REAL, seconds)
        signal.setitimer(signal.ITIMER_PROF, 2 * seconds)


def _git(top, args, left):
    r = subprocess.run(["git", "-C", top] + args, capture_output=True, text=True,
                       errors="replace", timeout=max(1, left))
    if r.returncode:
        raise RuntimeError("git %s: %s" % (args[0], (r.stderr or r.stdout).strip()[:200]))
    return r.stdout


def scan_unpushed(cfg, top, refs, deadline=None):
    """(ref, commit, line) for the first brain path in the objects reachable from `refs` that no remote has — diffs
    (merges against their first parent) and messages — or None. Raises when git fails or runs out of time: a scan
    that did not finish has not found the objects clean."""
    deadline = deadline or time.time() + LEAK_BUDGET
    for ref in refs:
        out = _git(top, ["log", "-p", "--diff-merges=first-parent", "--no-color",
                         "--no-ext-diff", "--format=@@commit %H%n%B", ref, "--not",
                         "--remotes"], deadline - time.time())
        commit, in_diff = "?", False
        for line in out.splitlines():
            if line.startswith("@@commit "):
                commit, in_diff = line[9:21], False
                continue
            if line.startswith("diff --git "):
                in_diff = True
                continue
            if in_diff and not (line.startswith("+") and not line.startswith("+++")):
                continue
            if _spells(cfg, line):
                return ref, commit, line.strip()[:160]
    return None


def _pushes(seg):
    words = [os.path.basename(w) if i == 0 else w for i, (w, _) in enumerate(seg.words)]
    if "git" in words and "push" in words[words.index("git") + 1:]:
        return "git"
    if "gh" in words and "pr" in words:
        return "gh"
    return None


def _repo_dirs(seg, cwd):
    dirs = [cwd] if cwd else []
    words = [w for w, _ in seg.words]
    for i, w in enumerate(words):
        if w == "-C" and i + 1 < len(words) and sh.known(words[i + 1]):
            d = words[i + 1]
            dirs.append(d if d.startswith("/") else os.path.join(cwd or "/", d))
    return dirs


def _pushed_refs(seg, top, left):
    """What this push sends: the local refs `git push --dry-run --porcelain` lists with the same arguments (without
    running the repo's own pre-push hook), what a pre-push hook reads on stdin. A push the guard cannot replay (an
    argument it cannot know) is scanned as HEAD plus every local branch, the widest thing it could send."""
    words = [w for w, _ in seg.words]
    i = words.index("push") if "push" in words else -1
    if i < 0 or any(not sh.known(w) for w in words[i + 1:]):
        return ["HEAD", "--branches"]
    out = _git(top, ["push", "--dry-run", "--porcelain", "--no-verify"] + words[i + 1:], left)
    refs = []
    for line in out.splitlines():
        parts = line.split("\t")
        if len(parts) >= 2 and ":" in parts[1] and parts[0].strip() != "-":
            src = parts[1].split(":", 1)[0]
            if src:
                refs.append(src)
    return refs


def _leak(cfg, segs):
    deadline = time.time() + LEAK_BUDGET
    seen = set()
    for seg in segs:
        kind = _pushes(seg)
        if not kind:
            continue
        for d in _repo_dirs(seg, seg.cwd):
            if not os.path.isdir(d):
                continue
            r = subprocess.run(["git", "-C", d, "rev-parse", "--show-toplevel"],
                               capture_output=True, text=True, timeout=10)
            top = r.stdout.strip()
            if r.returncode or not top or top in seen or _inside(cfg, top):
                continue
            seen.add(top)
            try:
                refs = (_pushed_refs(seg, top, deadline - time.time()) if kind == "git" else ["HEAD"])
                hit = scan_unpushed(cfg, top, refs, deadline)
            except (subprocess.TimeoutExpired, RuntimeError, OSError) as exc:
                return _refuse("leak", "what this push would send, in %s, could not be "
                               "scanned (%s), so it is not known to be clean" % (top, exc))
            if hit:
                return _refuse("leak", "this push sends %s, and its commit %s, which no remote has, names brain (%s). "
                                       "Rewrite that commit before pushing" % hit)
    return None


# ── the decision ────────────────────────────────────────────────────────────

def _patch_files(text):
    return re.findall(r"^\*\*\* (?:Add|Update|Delete) File: (.+)$|^\*\*\* Move to: (.+)$", text or "", re.M)


def decide(cfg, tool, tool_input, cwd, env=None):
    """None to allow, or (rule, reason) to refuse: the call (`judge`), then what a push in it sends (`_scanned`)."""
    return _scanned(judge(cfg, tool, tool_input, cwd, env))


def _scanned(verdict):
    """A refusal or None as it is; of the leak scans `judge` returns, the first refusal (main's scan first). The scans
    run side by side, each within its own LEAK_BUDGET: a push waits for the slower one, not for both in turn, and
    both fit the hook's 60 s."""
    if not isinstance(verdict, list):
        return verdict
    if len(verdict) == 1:
        return verdict[0]()
    import concurrent.futures  # a push's cost, not every call's
    with concurrent.futures.ThreadPoolExecutor(len(verdict)) as pool:
        results = list(pool.map(lambda scan: scan(), verdict))
    return next(filter(None, results), None)


# NEVER WEAKER THAN MAIN, BY CONSTRUCTION: main's guard (deny_brain_writes_main.py, frozen) judges every call first, and
# its refusal is the verdict. Its `decide` ends in its leak scan, which has its own LEAK_BUDGET and must not run inside
# DEADLINE: during the call it returns the scan, deferred, over the segments and cwds main read (`_scanned` runs it).
def _main_first(cfg, tool, tool_input, cwd, env):
    leak = main_guard._leak
    main_guard._leak = functools.partial(functools.partial, leak)
    try:
        return main_guard.decide(cfg, tool, tool_input, cwd, env)
    finally:
        main_guard._leak = leak


def judge(cfg, tool, tool_input, cwd, env=None):
    """None to allow, (rule, reason) to refuse, or the leak scans (calls of no argument) of a Bash command main's guard
    and this one allow, which `_scanned` runs (unbounded by DEADLINE: each has its own LEAK_BUDGET, side by side).
    Main's guard first."""
    env = dict(os.environ if env is None else env)
    first = _main_first(cfg, tool, tool_input, cwd, env)
    if isinstance(first, tuple):
        return first
    verdict = _judge_new(cfg, tool, tool_input, cwd, env)
    if isinstance(verdict, tuple):
        return verdict
    return [s for s in (first, verdict and functools.partial(_leak, cfg, verdict)) if s] or None


def _judge_new(cfg, tool, tool_input, cwd, env):
    """This guard's own verdict: None, (rule, reason), or the segments of a Bash command it allows."""
    tool_input = tool_input or {}
    # On Windows the caller may spell its cwd as a drive path with forward slashes (a caller may pass its session
    # directory that way). Every relative word is joined onto it, and a join that does not start with "/" is not a path
    # to _inside: a relative redirect out of the worktree into brain would pass. One spelling first.
    if drw._WINPATHS and cwd:
        cwd = drw._norm(cwd)
    if tool in WRITE_TOOLS or tool in PATCH_TOOLS:
        paths = [tool_input.get(k) for k in WRITE_TOOLS.get(tool, ())]
        if tool in PATCH_TOOLS:
            paths = [next(p for p in pair if p).strip() for pair in
                     _patch_files(tool_input.get("patchText") or tool_input.get("patch") or "")]
        for p in paths:
            if isinstance(p, str) and p:
                full = os.path.expanduser(p)
                full = full if drw._is_abs(full) else drw._join(cwd or "/", full)
                if _inside(cfg, drw._norm(full)):
                    return _refuse("write", "%s targets %s, inside brain" % (tool, p))
        return None
    if tool not in BASH_TOOLS:
        return None
    command = tool_input.get("command") or ""
    if not isinstance(command, str) or not command:
        return None
    shell = sh.parse(command, env, cwd or None)
    segs = shell.segs
    for seg in segs:
        seg.head = _unwrap(seg)
    cwds = [seg.cwd for seg in segs] + [shell.cwd]  # the cwd model (deny_brain_writes_shell.py)
    if cwd and _inside(cfg, cwd):
        # The one way out: a command that only cd's somewhere outside brain. A redirect opens its file HERE, before the
        # cd runs, so it may carry none.
        if segs and cwds[-1] and not _inside(cfg, cwds[-1]) and all(
                os.path.basename(g.head[1]) in ("cd", "pushd") and not g.redirects
                and not g.data for g in segs):
            return None
        return _refuse("cwd", "the shell's cwd is inside brain (%s); leave it with a "
                       "command that is only `cd <your worktree or work dir>`" % cwd)
    # A command that names brain is judged only when it is flat (deny_brain_writes_shell.py).
    if shell.unreadable:
        return _refuse("shell", "this command holds %s: %s. The guard cannot tell where its body ends, so it cannot "
                       "read the rest of the command or know whether it names brain" % shell.unreadable)
    quiet = any(seg.forms for seg in segs) and _quiet(segs, shell.renames, shell.loops)
    word = next((w for i, seg in enumerate(segs) if seg.forms for w in _unexpanded(seg, quiet, segs[i + 1:])), None)
    if word:
        return _refuse("shell", "`%s` is unknown: the guard cannot resolve it to a literal (a `${…}` with an "
                       "operator, `$'…'`, a positional or special parameter, `$_`, `$PWD` after a `cd` it cannot "
                       "follow …) where it may name a path this command writes or moves to. Spell it literally" % _show(word))
    if shell.odd and _touches(cfg, command, shell, cwds, env):
        what, how = shell.odd[0]
        return _refuse("shell", "this command names brain and holds %s: %s. The guard judges a command that names "
                       "brain only when it is flat (simple commands of known words joined by ; && || |)" % (what, how))
    fed = False  # an earlier stage of this pipeline named brain
    for i, seg in enumerate(segs):
        here = cwds[i]
        refusal = _judge(cfg, seg, here, shell.values, env)
        if refusal:
            return refusal
        if cwds[i + 1] and _inside(cfg, cwds[i + 1]):
            return _refuse("cd", "this command cd's into brain (%s); work in your worktree "
                           "or work dir and read brain by path" % cwds[i + 1])
        idx, verb, skipped = seg.head
        name = os.path.basename(verb)
        if seg.after_pipe and fed and (name in ("xargs", "parallel") or _undetermined(seg.words, idx, skipped)
                                       or _reads_code(name, [w for w, _ in seg.words[idx + 1:]])):
            return _refuse("pipe", "`%s` is fed brain paths by the command before it, and "
                           "only a reader may be given them" % (name or "this command"))
        # Anything earlier in the same pipeline, not just the stage before.
        fed = (fed and seg.after_pipe) or _seg_names(cfg, seg, here)
    return segs


def main(stream=None, root=None):
    try:
        payload = json.load(stream or sys.stdin)
    except Exception:
        sys.exit(0)
    # A payload of the wrong shape refuses (audit F12, 2026-09-29): a Bash `tool_input` that was null read as an empty
    # command and was allowed.
    if not isinstance(payload, dict):
        drw.deny("Blocked: the brain guard was handed a payload that is not a JSON object (%s), so it cannot tell "
                 "which tool is running. Refused rather than guessed." % type(payload).__name__)
        return
    tool = payload.get("tool_name", "")
    if tool in BASH_TOOLS | set(WRITE_TOOLS) | PATCH_TOOLS \
            and not isinstance(payload.get("tool_input"), dict):
        drw.deny("Blocked: the brain guard was handed a %s call whose tool_input is not an object (%s), so it cannot "
                 "read what it does. Refused rather than guessed." % (tool, type(payload.get("tool_input")).__name__))
        return
    try:
        cfg = brain_config(root)
        try:  # the guard's own work is bounded; the leak scan has its own LEAK_BUDGET
            _bound(DEADLINE)
            verdict = judge(cfg, tool, payload.get("tool_input", {}), payload.get("cwd", "") or "")
        finally:
            _bound(0)
        verdict = _scanned(verdict)
    except GuardTimeout as exc:
        drw.deny("Blocked: %s, and a guard that reached no verdict has not established that this is safe. Split the "
                 "command, or write what it builds to a file with the Write tool." % exc)
        return
    except Exception as exc:  # noqa: BLE001 — ANY failure here must deny
        drw.deny("Blocked: the brain guard CRASHED while deciding (%s: %s). It reached no verdict, and a guard that "
                 "reached no verdict has not established that this is safe." % (type(exc).__name__, exc))
        return
    if verdict:
        drw.deny(verdict[1])
    sys.exit(0)


def scan_main(argv, root=None):
    """`deny_brain_writes.py --scan <dir> [<branch>]`: hw done's leak check over what the task can carry out — HEAD and
    its branch. Exit 0 clean, 1 leak (named on stdout), 2 could not tell."""
    cfg = brain_config(root)
    try:
        top = subprocess.run(["git", "-C", argv[0], "rev-parse", "--show-toplevel"],
                             capture_output=True, text=True, timeout=10).stdout.strip()
        if not top:
            return 0
        refs = ["HEAD"]
        if len(argv) > 1 and argv[1] and subprocess.run(
                ["git", "-C", top, "rev-parse", "--verify", "-q", "refs/heads/" + argv[1]],
                capture_output=True).returncode == 0:
            refs.append("refs/heads/" + argv[1])
        hit = scan_unpushed(cfg, top, refs)
    except Exception as exc:  # noqa: BLE001
        print("could not scan %s: %s: %s" % (argv[0], type(exc).__name__, exc))
        return 2
    if hit:
        print("%s carries commit %s, which names brain: %s" % hit)
        return 1
    return 0


if __name__ == "__main__":
    root = os.environ.get("HW_BRAIN_GUARD_ROOT") or None
    if sys.argv[1:2] == ["--scan"]:
        sys.exit(scan_main(sys.argv[2:], root))
    main(root=root)
