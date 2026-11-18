"""The shell, read as far as the brain guard (deny_brain_writes.py) needs it, and whether a command is FLAT.

A command that names brain is judged only when it is flat: simple commands of words the guard knows (literal text,
quotes, `~`, the executor's environment, a variable the command gave ONE literal value, the cwd it follows) joined by
`;`, `&&`, `||`, `|` or a newline, with plain redirections, here-strings, heredocs fed data, and FLAT_BUILTINS. Anything
else (`Shell.odd`) is refused, naming the construct and the flat spelling (docs/internal-limitations.md L36). THE TWO ADMISSIONS
ARE TEXTUAL WHITELISTS THAT FAIL CLOSED: a heredoc is data only behind exactly `'WORD'`/`"WORD"`, fed to one of
DATA_CONSUMERS (`_consumes`) in a command that exports nothing to it and whose text has no `_FENCE` and nothing
`_HANDS_ON`; an assignment has one value only in a command whose text is `simple`, read later only as `$NAME`/`${NAME}`.
A read the guard does not expand is recorded (`Seg.forms`), never simulated. A command that names no brain is read only
to find whether it does (deny_brain_writes `_touches`).
"""
import functools
import glob
import os
import re

MARK = "\x00"  # a part of a word the guard cannot know before the shell runs
FORM = MARK + "\x06"  # the value of a name assigned a word the guard cannot resolve: each read of it is one too
_CHASE = functools.lru_cache(maxsize=None)(re.compile(r"\bset\b.*(-\w*P|physical)|chase_").search)  # `set -P`, zsh
UNSURE = "\x05"  # ends the RAW text of a word that may be several: an unquoted expansion, a glob, a brace, `"$@"`
ASSIGN = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*(\[[^\]]*\])?\+?=")
READS = re.compile(r"\$\{?([A-Za-z_]\w*)")  # the names a raw word reads (`$A`, `${A…}`)
_SIMPLE_VAR = re.compile(r"\$(?:([A-Za-z_]\w*)|\{([A-Za-z_]\w*)\})")  # a plain read: `$NAME`, `${NAME}`
# What groups, nests or runs commands in a command's text: `(` (`$(…)`, `<(…)`, a subshell, `((`, an array), a backtick,
# a `{` but `${` before a name or a special parameter (bash 5.3 runs `${ cmd; }` and `${| cmd; }` in this shell).
_FENCE = r"[(`]|(?<!\$)\{|\$\{(?![\w#!@*?$-])"
_FUNSUB = re.compile(r"\$\{[\s|]")  # bash 5.3 `${ cmd; }`, `${| cmd; }`: a command list, run in this shell
_NAME = re.compile(r"[A-Za-z_]\w*\+?")  # a word so far that an `=` makes an assignment
_BRACKET = re.compile(r"\[[^\s\]]*")  # a bracket glob up to its `]` (`[ab]`), when a `]` ends it in the same word
# THE cwd model (`Seg.cwd`, `_move`): `$PWD`, `${PWD}`, `~+` are the directory a command runs in, `$OLDPWD`, `~-` the
# one before; where the guard cannot follow a move they are unknown, and so is `$_`: reads it does not expand.
_MOVES = {"cd", "pushd", "popd", "chdir"}
_CWD = {"PWD": "cwd", "OLDPWD": "oldcwd", "_": None}
# Variables that change how LATER words are read (a split, a `~`, a `cd`): set on their own, never flat.
_READS_LATER = {"IFS", "CDPATH", "HOME", "cdpath"}
# Every builtin and reserved word of bash 5.3 (`compgen -bk`) and zsh 5.9 (`${(k)builtins} ${(k)reswords}`), with zsh's
# `always`.
SHELL_WORDS = set("""
. : [ [[ ]] ! { } - alias always autoload bg bind bindkey break builtin bye caller case cd chdir command compadd
comparguments compcall compctl compdescribe compfiles compgen complete compgroups compopt compquote compset comptags
comptry compvalues continue coproc declare dirs disable disown do done echo echotc echoti elif else emulate enable end
esac eval exec exit export false fc fg fi float for foreach function functions getln getopts hash help history if in
integer jobs kill let limit local log logout mapfile nocorrect noglob popd print printf private pushd pushln pwd r read
readarray readonly rehash repeat return sched select set setopt shift shopt source suspend test then time times trap
true ttyctl type typeset ulimit umask unalias unfunction unhash unlimit unset unsetopt until vared wait whence where
which while zcompile zformat zle zmodload zparseopts zregexparse zstyle""".split())
# The reserved words that open, continue or close a compound command: the first word of a simple command that is one of
# them (unquoted) is a block, a loop, a test or a function.
_COMPOUND = {"if", "then", "else", "elif", "fi", "do", "done", "while", "until", "for", "select", "case", "esac",
             "{", "}", "[[", "]]", "function", "coproc", "repeat", "foreach", "end", "always"}
# The builtins a flat command may run: none assigns a variable, defines a name or changes how a word is read. `export`
# (literal names, no option), `set` (`_set_flat`) and `print` (no -v/-s/-z) are checked in `_builtin`.
FLAT_BUILTINS = {":", "true", "false", "echo", "printf", "print", "test", "[", "pwd", "cd", "chdir", "pushd", "popd",
                 "dirs", "exit", "logout", "bye", "return", "wait", "kill", "jobs", "disown", "bg", "fg", "suspend",
                 "type", "which", "whence", "where", "help", "umask", "ulimit", "limit", "unlimit", "times", "export",
                 "set", "command", "builtin", "exec", "time", "noglob", "nocorrect", "-", "!", "caller", "log"}
# THE precommand table, for the verb finder (deny_brain_writes._unwrap) and `_builtin`: a word that runs the word after
# it, as `name: (short options taking the next word, long ones)`. Those in SHELL_WORDS may run a builtin; `env`, `nice`,
# `nohup` run a program (`_unwrap` reads `env`/`nice`/`timeout`/`setup/heavy`/`hw secret run` values). bash's `jobs`
# runs the words after an option cluster holding `x`: a precommand whatever its options, so no option hides anything.
PRECOMMANDS = {"command": ("", ()), "builtin": ("", ()), "exec": ("a", ()), "time": ("fo", ("--format", "--output")),
               "noglob": ("", ()), "nocorrect": ("", ()), "-": ("", ()), "!": ("", ()), "jobs": ("", ()),
               "nohup": ("", ()), "env": ("", ()), "nice": ("", ())}
# A word that, anywhere in a command, keeps every assignment of it from having one value (`parse`).
_RESERVED = _COMPOUND | {"time", "!"}
# The one exact heredoc delimiter the guard reads: `'WORD'`, `"WORD"` (data), or a bare WORD (expanded).
_DELIM = re.compile(r"""(['"]?)([A-Za-z_]\w*)\1""")
# THE data consumers: a quoted heredoc is data for them, each EXECUTING NOTHING (corpus2: cat 708). Not `git commit`
# or `gh pr` (they run a configured editor on the body), nor a brain program (`hw secret run … -- bash`).
DATA_CONSUMERS = {"cat", "tee", "jq"}
# The directories a consumer's verb may be spelled in besides its bare name (a `./cat` or `/tmp/d/cat` is any program).
_SYSTEM_BIN = {"/bin", "/usr/bin", "/usr/local/bin", "/opt/homebrew/bin"}
# What in a command's text may take a heredoc's body to another reader: a descriptor past stderr (`3<<'EOF'`, `<&3`,
# `>&p`, `>&$FD`), a pipe (`|`, `||`, `|&`, textual: `done | bash`), a background job.
_BG_AMP = r"(?<![&>|<])&(?![&>])"  # a background `&` (not `&&`, `&>`, `>&`, `|&`)
_HANDS_ON = re.compile(r"(?<![\w$])(?:[3-9]|\d\d+)[<>]|[<>]&\s*(?![012](?!\d)|-)|\||" + _BG_AMP)
# The words after which a `(` or `((` opens a subshell or an arithmetic command, not part of a word (`! (…)`, `if (…)`).
_OPENS = {"!", "time", "-p", "if", "elif", "then", "else", "while", "until", "do", "{", "coproc"}
# `set` options that change nothing the guard reads: errexit, nounset, xtrace, verbose, errtrace, functrace, noclobber,
# pipefail.
_SET_LETTERS = "euxvETC"
_SET_NAMES = {"errexit", "nounset", "xtrace", "verbose", "errtrace", "functrace", "noclobber", "pipefail"}


def known(value):
    """True when the guard knows this value exactly (no unknowable part)."""
    return MARK not in value


def _read(name, value):
    """The read of an unknown `$NAME` (`_expand`): plain, but unexpanded for `$_`, `$PWD`, `$OLDPWD` (the cwd model)."""
    return (name, "$" if name in _CWD or FORM in value else None) if value == MARK or FORM in (value or "") else None


def takes_next(word, letters, longs=()):
    """1 when the option `word` takes its value from the NEXT word: a short one among `letters` that ends the cluster
    (`-s`, `-ks`; `-sKILL` carries it), or a long one (any getopt_long prefix) without `=`. 0 otherwise."""
    if word.startswith("--"):
        return int("=" not in word and len(word) > 2 and any(name.startswith(word) for name in longs))
    for k, letter in enumerate(word[1:], 1):
        if letter in letters:
            return int(k == len(word) - 1)
    return 0


class Seg:
    """One simple command: its words, its redirects, its heredoc data."""
    def __init__(self):
        self.words = []       # (value, raw) — value has MARK where unknowable
        self.redirects = []   # (op, value, raw)
        self.data = []        # heredoc / here-string bodies fed to it
        self.code = []        # the heredoc bodies of them that are not data (`Shell.parse`): code
        self.after_pipe = False  # its stdin is the previous segment's stdout
        self.sep = None  # what ends it: `;` `&&` `||` `|` `&`, or None (a newline is `;`)
        self.head = (0, "", [])  # deny_brain_writes._unwrap: (index of the verb, verb, the words the wrappers skipped)
        self.reads = {}  # (value, raw) of a word or redirect → the name its one MARK reads plainly (`_expand`)
        # (value, raw) of a word or redirect holding a read the guard does not expand (`${A#…}`, `$'…'`, `$1`, `$~A`,
        # an unknown `$PWD` …): one it cannot resolve to a literal, in the order written (the refusal names the first)
        self.forms = {}
        self.cwd = None  # the directory it runs in (the cwd model), None when unknown
        self.nested = False  # in a substitution (`$(…)`, a backtick, `<(…)`): its output becomes a word
        self.tilde = set()  # (value, raw) of a word holding a `~` or `=` where an assignment value starts or after `:`


def _quote_end(t, j):
    """The index after the quote that closes the one at t[j] (`\\` escapes inside `"…"` and `$'…'`), or None."""
    k, escapes = j + 1, t[j] == '"' or t[j - 1:j] == "$"
    while k < len(t) and t[k] != t[j]:
        k += 2 if escapes and t[k] == "\\" else 1
    return k + 1 if k < len(t) else None


def _close(t, i, opening, closing):
    """The index of the `closing` that matches the `opening` at t[i] (quotes skipped), or len(t)."""
    depth, j = 0, i
    while j < len(t):
        c = t[j]
        if c == "\\":
            j += 2; continue
        if c in "'\"":
            j = _quote_end(t, j) or len(t); continue
        if c == opening:
            depth += 1
        elif c == closing:
            depth -= 1
            if depth == 0:
                return j
        j += 1
    return len(t)


def _brace_span(t, i):
    """((start, end) of the first brace with something inside (`{a,b}`, `{1..3}`, `{-o,'a b'}`, zsh's `{ab}`) in the word
    from the unquoted `{` at t[i], quotes and escapes skipped as the shell does, or None; the index the scan stopped at:
    a quote it cannot pair stops it there, and the main loop reads on)."""
    opens, j = [], i
    while j < len(t):
        c = t[j]
        if c == "\\":
            j += 2; continue
        if c in "'\"":
            k = _quote_end(t, j)
            if k is None:
                return None, j
            j = k; continue
        if c.isspace() or c in ";&|<>()":
            return None, j
        if c == "{":
            opens.append(j)
        elif c == "}" and opens:
            start = opens.pop()
            if j > start + 1:
                return (start, j), j
        j += 1
    return None, j


class Shell:
    """One read of a command, in order. A variable the command assigns has ONE value from a flat assignment (`known`,
    only while `flat`) until any other assignment makes it unknown (MARK); `taint` (the second read of a command that
    loops or defines a function, `parse`) reads every name the command assigns anywhere as unknown."""
    def __init__(self, text, env, taint=(), flat=True, cwd=None):
        self.text, self.env, self.i = text, env, 0
        # the cwd model: where the shell stands, where it stood before its last move, where it stands once the and-or
        # list ends (`_move`)
        self.cwd, self.oldcwd, self.settled = cwd, None, cwd
        self.taint = set(taint)
        self.flat = flat  # an assignment may be flat: the first read, until `parse` finds the text is not `simple`
        self.loops = False  # a block, loop or function: a word may run after one written later
        self.andor = ""  # the next command follows `&&` (or an `||`, "||") in its list, so it may not run
        self.unreadable = None  # (construct, spelling) of a heredoc whose body's end the guard cannot tell
        self.heredocs = []  # (seg, body, (start, end) in the text, delimiter quoted): `parse` decides each (`_heredoc`)
        self.segs = []
        self.values = {}   # name → [(value, raw)] every value an assignment of this command gives it
        self.known = {}    # name → its one value, from the last flat assignment (any other leaves it in `unflat` only)
        self.unflat = set()   # names a later read of which makes the command not flat (all but a literal assignment)
        self.odd = []      # (construct, the flat spelling that passes): what makes this command not flat
        self.reads = []    # names of `unflat` or `taint` a word reads (`$NAME`, `~` for HOME)
        self.renames = False  # a builtin outside the flat ones or an array element assigned: a name may run another program
        self.moved = False  # a command so far may have changed the directory (`_move`)
        self.nested = False  # reading a subshell or a substitution (`Seg.nested`)

    def _code(self, spans):
        """The command's text without these heredoc bodies."""
        code, at = [], 0
        for start, end in sorted(spans):
            code.append(self.text[at:start]); at = end
        return "".join(code) + self.text[at:]

    def parse(self):
        self._list(end=None)
        # A quoted heredoc is data only for a consumer (`_consumes`), in a command that exports nothing to it (no
        # `export`, no assignment of PATH or of a name the environment holds: `LD_PRELOAD`) and whose text (heredoc
        # bodies aside) has no `_FENCE`, nothing `_HANDS_ON`, no reserved word and nothing that renames a program: no
        # group, loop, pipe, subshell, substitution, fd or alias takes the body to another reader. Any other is code:
        # each path in it is read as the start of one the code completes.
        data = []  # (start, end) of each body that is data: no part of the command's code text
        reserved = any(r in _RESERVED for s in self.segs for _, r in s.words)
        assigns = any(n in self.env or n in ("PATH", "path") for n in self.values) or any(
            r == "export" for s in self.segs for _, r in s.words)
        fenced = self.heredocs and (self.renames or assigns or reserved or _HANDS_ON.search(code := self._code(
            s for _, _, s, _ in self.heredocs if s)) or re.search(_FENCE, code))
        for seg, body, span, quoted in self.heredocs:
            if quoted and span and not fenced and _consumes(seg):
                data.append(span)
                continue
            seg.code.append(body)
            if quoted:
                self._odd("a heredoc that is not data (only %s, by its bare name, in a command with no `export`, "
                          "assignment of PATH or of an environment variable, `(`, `{`, `|`, `&`, backtick, reserved word, "
                          "other builtin or descriptor past 2, is fed one)" % "/".join(sorted(DATA_CONSUMERS)),
                          "run `cat > file <<'EOF'` in a command of its own, or write the file with the Write tool, "
                          "and give the program that file")
        if self.reads and not self.odd:  # only the first construct is named, and the read is enough to be not flat
            self._unflat_read()
        return not (reserved or re.search(_FENCE + r"|\[\[|" + _BG_AMP, self._code(data)))  # the text is `simple`

    def _subshell(self, word=True):
        """The list up to the next `)`: a move in it stays there; a substitution's output is a word (`Seg.nested`)."""
        at, self.nested = (self.cwd, self.oldcwd, self.settled, self.nested), self.nested or word
        self._list(end=")")
        self.cwd, self.oldcwd, self.settled, self.nested = at

    def _move(self, seg, words, i):
        """The cwd after a command whose verb is words[i]. `cd`/`chdir`/`pushd` run once in this shell (no pipeline,
        background, block, loop or function before) to a target the guard reads: `&&` runs what follows there, and after
        the and-or list the shell stands there if it surely ran to a directory that exists. Any other move: unknown."""
        verb = words[i][0] if i < len(words) else ""
        if known(verb) and verb not in _MOVES | {"eval", "source", "."} and not (
                verb and self.renames and verb not in FLAT_BUILTINS):  # after an `alias`, any name may be a `cd`
            return
        self.moved, to = True, None
        args, mode = [v for v, _ in words[i + 1:]], ""
        while args[:1] and re.fullmatch(r"-[LPe@]+", args[0]):
            mode += args.pop(0)
        args = args[1:] if args[:1] == ["--"] else args
        alone = not (self.loops or seg.after_pipe or seg.sep in ("|", "&"))
        if alone and verb in ("cd", "chdir", "pushd") and len(args) <= 1:
            target = args[0] if args else self._value("HOME", read=False)
            target = self.oldcwd if target == "-" else target
            if target and known(target) and not re.fullmatch(r"[+-]\d+", target) and (
                    target.startswith(("/", ".")) or not self.env.get("CDPATH")):
                p = target if target.startswith("/") else self.cwd and os.path.join(self.cwd, target)
                to = p and (os.path.realpath(p) if re.search(r"P[^L]*$", mode) or _CHASE(self.text) else os.path.normpath(p))  # logical by default
        self.settled = to if to and not self.andor and os.path.isdir(to) else None
        self.oldcwd = self.cwd if self.settled else None
        self.cwd = to if self.andor != "||" else None
        for name in _CWD:  # the shell sets $PWD and $OLDPWD again
            self.known.pop(name, None)

    def _heredoc(self, seg, body, span, quoted):
        """A heredoc body fed to `seg`; span is its (start, end) in the text, None inside a substitution (code)."""
        seg.data.append(body)
        self.heredocs.append((seg, body, span, quoted))

    def _odd(self, what, how):
        self.odd.append((what, how))

    def _rebind(self, what, how):
        """A construct that may also change what a name runs (`alias`, `hash`, `printf -v PATH`, `functions[cat]=`):
        no heredoc of this command is data."""
        self.renames = True
        self._odd(what, how)

    def _substitution(self, spelled):
        self._odd("a command substitution `%s`" % spelled, "run the command first, then write its output literally")

    def _funsub(self, text, i):
        """The `${ cmd; }` or `${| cmd; }` opening at text[i], read as a command list: the index of its `}`."""
        j = _close(text, i + 1, "{", "}")
        self._substitution("${ …; }")
        self._nested(text[i + 2:j].lstrip("|"))
        self.cwd = self.oldcwd = self.settled = None  # it runs in this shell: a move in it is not followed
        return j

    def _backtick(self, text, i):
        """The backtick command list opening at text[i]: (its segments, the index after it)."""
        j = text.find("`", i + 1)
        j = len(text) if j < 0 else j
        self._substitution("` `…` `")
        return self._nested(text[i + 1:j]), j + 1

    def _set(self, name, value=MARK, raw=MARK):
        """NAME may take `value`, or keep the one it had: from here on the guard cannot know which."""
        self.values.setdefault(name, []).append((value, raw))
        self.known.pop(name, None)
        self.unflat.add(name)

    def _value(self, name, read=True):
        """NAME's value here: None when it is unset (never assigned, not in the environment), MARK when unknown."""
        if read and (name in self.unflat or name in self.taint):
            self.reads.append(name)
        if name in self.taint or (name in self.unflat and name not in self.known):
            return FORM if any(FORM in v for v, _ in self.values.get(name, ())) else MARK
        if name in self.known:
            return self.known[name]
        if name in _CWD:
            return _CWD[name] and getattr(self, _CWD[name]) or MARK
        return self.env.get(name)

    def var(self, name):
        v = self._value(name)
        return MARK if v is None else v

    def _list(self, end):
        seg, word, raw, have = Seg(), [], [], False
        reads = []  # per expansion of this word with an unknown part: the read (`_expand`), or None
        quoted = unsure = tilde = False
        eq = None  # the word's raw pieces up to its first `=` when a name precedes it (a value starts there), else False
        braced = bracketed = 0  # the text up to here was scanned for a brace (`_brace_span`), a bracket glob (`_BRACKET`)
        pending_redirect = None
        heredocs = []
        piped = False
        pattern = False  # a `case` pattern comes next (after `;;`): its words, up to `)`, are no command
        t = self.text

        def flush_word():
            nonlocal word, raw, have, pending_redirect, quoted, unsure, reads, tilde, eq
            if not have:
                return
            value, r = "".join(word), "".join(raw) + (UNSURE if unsure else "")
            empty = not quoted and value == ""  # an unquoted expansion to nothing is no word at all
            if len(reads) == 1 and reads[0] and reads[0][1] is None and value.count(MARK) == 1:
                seg.reads[(value, r)] = reads[0][0]
            if any(read and read[1] is not None for read in reads):  # a read the guard does not expand
                seg.forms[(value, r)] = True
            if tilde:
                seg.tilde.add((value, r))
            word, raw, have, quoted, unsure, reads, tilde, eq = [], [], False, False, False, [], False, None
            if pending_redirect is not None:
                if pending_redirect.startswith("<<") and pending_redirect != "<<<":
                    op, w = pending_redirect, r.rstrip(UNSURE)
                    m = _DELIM.fullmatch(w)
                    if not m:  # the command is refused whole (`decide`): no body to read
                        self.unreadable = ("a heredoc `%s%s` (a delimiter that is not exactly 'WORD' or \"WORD\")"
                                           % (op, w), "spell it `%s'EOF'`" % op)
                    else:
                        if not m.group(1):  # its body is expanded: `$(…)`, `${A:=x}`, `$((…))` run
                            self._odd("a heredoc `%s%s` (its delimiter unquoted)" % (op, w),
                                      "quote its delimiter (`%s'%s'`), or write the text to a file with the Write tool"
                                      % (op, w))
                        heredocs.append((seg, m.group(2), op == "<<-", bool(m.group(1))))
                elif pending_redirect == "<<<":
                    seg.data.append(value)
                else:
                    seg.redirects.append((pending_redirect, value, r))
                pending_redirect = None
            elif not empty:
                head = seg.words[0][1] if seg.words else None
                if r == "{" and head in ("function", "coproc", "repeat"):
                    flush_seg()  # `function f {`, `coproc NAME {`, `repeat 3 {`: the body is commands of its own
                header = len(seg.words) == 1 and head in ("function", "repeat")
                seg.words.append((value, r))
                if header:
                    flush_seg()  # `function f`, `repeat 3`: the header ends here, the command after it is the verb

        def flush_seg(sep=None):
            nonlocal seg, piped
            flush_word()
            full = bool(seg.words or seg.redirects or seg.data)
            if full:
                # An empty flush (`|&`, `|` then a newline) keeps the pipe pending for the command that follows it.
                seg.after_pipe, piped, seg.sep, seg.cwd, seg.nested = piped, False, sep, self.cwd, self.nested
                self.segs.append(seg)
                self._command(seg)
            # `&&`/`||` make the next command conditional until `;`, `&` or a newline ends the list (a newline after
            # `&&`, `||` or `|` continues it).
            if sep in ("&&", "||"):
                self.andor = "||" if sep == "||" else self.andor or sep
            elif sep in (";", "&") or (sep == "\n" and full):
                self.andor = ""
            if not (sep in ("&&", "|") or sep == "\n" and not full):
                self.cwd = self.settled  # the and-or list ends here (`_move`)
            seg = Seg()

        while self.i < len(t):
            c = t[self.i]
            if c == ")" and (pattern or ([r for _, r in seg.words[:3:2]] == ["case", "in"])):
                flush_word()  # `case W in a)`, `;; *)`: the pattern ends here
                seg.words = seg.words[:3] if not pattern else []
                pattern = False
                flush_seg(); self.i += 1; continue
            if end and c == end:
                flush_seg(); self.i += 1; break
            if c == "\\" and self.i + 1 < len(t):
                if t[self.i + 1] == "\n":
                    self.i += 2; continue
                word.append(t[self.i + 1]); raw.append(t[self.i:self.i + 2]); have = quoted = True
                self.i += 2; continue
            if c == "'":
                j = t.find("'", self.i + 1)
                j = len(t) if j < 0 else j
                word.append(t[self.i + 1:j]); raw.append(t[self.i:j + 1]); have = quoted = True
                self.i = j + 1; continue
            if c == '"' or t.startswith('$"', self.i):
                if c == "$":  # `$"…"`, a translated string: as `"…"`
                    raw.append("$"); self.i += 1
                self.i += 1
                start = self.i
                while self.i < len(t) and t[self.i] != '"':
                    if t[self.i] == "\\" and self.i + 1 < len(t):
                        word.append(t[self.i + 1]); self.i += 2; continue
                    if t[self.i] == "$" or t[self.i] == "`":
                        s = self.i
                        v, read = self._expand()
                        word.append(v)
                        if MARK in v:
                            reads.append(read)
                        # `"$@"`, `"${arr[@]}"`, `"${(@s: :)X}"`, zsh `"$arr[@]"`: quoted, and still several words
                        if re.match(r"\$(?:@|\{[^}]*[@(=])", t[s:self.i]) or t[self.i:self.i + 1] == "[":
                            unsure = True
                        continue
                    word.append(t[self.i]); self.i += 1
                raw.append(t[start - 1:self.i + 1]); have = quoted = True
                self.i += 1; continue
            if t.startswith("$'", self.i):  # ANSI-C quoting: unknown, a form (`$'\x62'`: escapes the guard does not decode)
                j = (_quote_end(t, self.i + 1) or len(t) + 1) - 1
                word.append(MARK); raw.append(t[self.i:j + 1]); have = quoted = True
                reads.append(("", "$'"))
                self.i = j + 1; continue
            if c == "$" or c == "`":
                s = self.i
                v, read = self._expand()
                piece = t[s:self.i]
                word.append(v); raw.append(piece); have = True
                if MARK in v:
                    reads.append(read)
                # Unquoted, a value is split on IFS and globbed: one word only when it is a plain `$NAME` whose known
                # value holds no blank and no glob character.
                if not (_SIMPLE_VAR.fullmatch(piece) and known(v) and not re.search(r"[\s*?\[]", v)):
                    unsure = True
                continue
            # bash and zsh expand `~` (zsh `=name` too) at a word's start, an assignment value's start or after its `:`:
            # the word records it (`_command`: such a value is not flat)
            at_value = c in "~=" and (not have or bool(eq) and (len(raw) == eq or raw[-1].endswith(":")))
            if c == "=" and eq is None:
                eq = len(raw) + 1 if _NAME.fullmatch("".join(raw)) else False
            tilde = tilde or at_value
            if c == "~" and at_value:
                m = re.compile(r"~([+-]?\d+|[+-]|[A-Za-z0-9_.-]*)(?=/|$|\s|[;&|)%s])" % (":" if have else "")).match(t, self.i)
                if m:
                    key = m.group(1)  # `~+` is $PWD, `~-` $OLDPWD, zsh's `~N` a directory of its stack
                    home = (self.var("PWD" if key == "+" else "OLDPWD") if key in "+-" and key else MARK if
                            re.fullmatch(r"[+-]?\d+", key) else os.path.expanduser("~" + key) if key else self._value("HOME"))
                    word.append(home or os.path.expanduser("~"))
                    if home == MARK:
                        reads.append((key, "~"))
                    raw.append(m.group(0)); have = True; self.i += len(m.group(0)); continue
            if c == "=" and at_value and re.compile(r"=[A-Za-z_./]").match(t, self.i):
                self._odd("zsh's `=name` (it expands to the path of a program)", "write the program's name or path")
                word.append(MARK); raw.append(c); have = unsure = True; self.i += 1; continue
            if c in " \t":
                flush_word(); self.i += 1; continue
            if c == "#" and not have:
                nl = t.find("\n", self.i)
                self.i = len(t) if nl < 0 else nl
                continue
            if c == "\n":
                flush_seg("\n"); self.i += 1
                for hseg, delim, dash, quoted_delim in heredocs:
                    body, start = [], self.i
                    while self.i < len(t):
                        nl = t.find("\n", self.i)
                        nl = len(t) if nl < 0 else nl
                        line = t[self.i:nl]; self.i = nl + 1
                        if (line.lstrip("\t") if dash else line) == delim:
                            break
                        body.append(line)
                    body = "\n".join(body)
                    self._heredoc(hseg, body, (start, self.i), quoted_delim)
                    if not quoted_delim:
                        self._substitutions(body)  # a `$(…)` in it runs: read like any other
                heredocs = []
                continue
            if c == "[" and not have and re.compile(r"\[\[\s").match(t, self.i) and all(r == "!" for _, r in seg.words):
                # `[[ … ]]` up to its own `]]` (a `]]` then a blank, an operator or the end): its `<`, `&&`, `(` are the
                # test's: `[[` and its body as one word (read for a brain path); a `$(…)` in it runs.
                m = re.compile(r"\]\](?=[\s;&|)]|$)").search(t, self.i + 2)
                j = m.end() if m else len(t)
                self._substitutions(t[self.i:j])
                seg.words += [("[[", "[["), (t[self.i + 2:j], t[self.i + 2:j] + UNSURE)]
                self.i = j; continue
            if c in "<>" and t[self.i + 1:self.i + 2] == "(":  # process substitution: a list, and a /dev/fd word
                s = self.i
                self._odd("a process substitution `%s(…)`" % c, "write its output to a file in $HW_ARTIFACTS first")
                self.i += 2
                self._subshell()
                word.append(MARK); raw.append(t[s:self.i]); have = unsure = True
                reads.append(None)
                continue
            if c == "(" and t.startswith("((", self.i) and not have and all(r in _OPENS for _, r in seg.words):
                j = _close(t, self.i, "(", ")")
                self._odd("an arithmetic command `((…))`", "use `[ … ]` or `test`")
                self._substitutions(t[self.i + 2:j])
                for name in re.findall(r"[A-Za-z_]\w*", t[self.i + 2:j]):
                    self._set(name)
                self.i = j + 1; continue
            if c == "(" and (have or not all(r in _OPENS for _, r in seg.words)):
                # `(` inside a word or after one that is not a reserved word: an array (`a=(…)`), a function (`f() …`),
                # a zsh glob group or qualifier (`(A|tee)`, `X(N)`). bash refuses most of these; zsh runs them. Read as
                # part of the word. After `!`, `if`, `while`, `{` … it opens a subshell (below).
                j = _close(t, self.i, "(", ")")
                piece = t[self.i:j + 1]
                so_far = "".join(raw)
                if re.fullmatch(r"\(\s*\)", piece) and not seg.words:
                    self._odd("a function definition `%s()`" % so_far, "run the commands themselves")
                    self.loops = True
                    word.append("()"); raw.append("()"); have = True
                    self.i = j + 1
                    flush_seg()
                    continue
                if re.fullmatch(r"[A-Za-z_]\w*\+?=", so_far):
                    for element in piece[1:-1].split() or [""]:  # `$A` is its first element (bash), or all (zsh)
                        self._set(so_far.rstrip("+="), element, element)
                    self._odd("an array assignment `%s(…)`" % so_far, "assign one value: `NAME=value`")
                else:
                    self._odd("`(` inside a word (`%s%s`: a zsh glob group or qualifier)" % (so_far, piece),
                              "write the word without `(`")
                word.append(piece); raw.append(piece); have = unsure = True
                self.i = j + 1; continue
            if c == "(":
                flush_seg()
                self._odd("a subshell `( … )`", "write its commands without the parentheses")
                self.i += 1
                self._subshell(word=False)
                continue
            if c == ")":  # one more `)` than `(`: read on
                flush_seg(); self.i += 1; continue
            if c == "|" and t[self.i + 1:self.i + 2] == "|":
                flush_seg("||"); self.i += 2; continue
            if c == "&" and t[self.i + 1:self.i + 2] == "&":
                flush_seg("&&"); self.i += 2; continue
            if c == ";":
                flush_seg(";")
                if t[self.i + 1:self.i + 2] in (";", "&"):  # `;;`, `;&`, `;;&`: the next `case` pattern
                    self.i += 3 if t.startswith(";;&", self.i) else 2
                    pattern = True
                else:
                    self.i += 1
                continue
            if c == "&" and not t.startswith("&>", self.i):
                flush_seg("&")
                self._odd("a background job `&`", "run it in the foreground, or in a command that names no brain")
                self.i += 1; continue
            if c == "|":
                flush_seg("|")
                piped = True
                self.i += 2 if t[self.i + 1:self.i + 2] == "&" else 1  # `|&` pipes stderr too
                continue
            m = c in "<>&0123456789" and re.compile(r"(\d*)(>>|>\||&>>|&>|<>|>&|<<<|<<-|<<|<&|>|<)").match(t, self.i)
            if m:
                if not m.group(1) and re.fullmatch(r"\{[A-Za-z_]\w*\}", "".join(raw)):
                    name = "".join(raw)[1:-1]  # `{fd}>file` stores a descriptor number in fd
                    self._set(name)  # (not flat: its brace)
                    word, raw, have, unsure = [], [], False, False
                flush_word()
                op = m.group(2)
                self.i += len(m.group(0))
                if op in (">&", "<&"):
                    # `2>&1` names a descriptor, not a file.
                    mm = re.compile(r"\s*(\d+|-)").match(t, self.i)
                    if mm:
                        self.i += len(mm.group(0)); continue
                    op = ">"
                while self.i < len(t) and t[self.i] in " \t":
                    self.i += 1
                pending_redirect = op
                continue
            if c == "[" and self.i >= bracketed:
                bracketed = _BRACKET.match(t, self.i).end()
                is_glob = t[bracketed:bracketed + 1] == "]" and bracketed > self.i + 1
            else:
                is_glob = c in "*?"
            if is_glob:
                unsure = True  # a glob: several words, or none
                self._odd("a glob `%s`" % c, "name the files, or run the glob in a command that names no brain path")
            elif c == "{" and self.i >= braced:
                b, braced = _brace_span(t, self.i)
                if b:
                    unsure = True  # a brace with content may expand (`{a,b}`, `{1..3}`, zsh's braceccl `{ab}`)
                    self._odd("a brace `%s`" % t[b[0]:b[1] + 1], "write each word out")
            word.append(c); raw.append(c); have = True; self.i += 1
        flush_seg("\n" if end is None else None)
        for hseg, _, _, quoted_delim in heredocs:  # a heredoc with no body line yet
            self._heredoc(hseg, t[self.i:], (self.i, len(t)), quoted_delim)

    def _command(self, seg):
        """What makes this simple command not flat, and the variables it assigns."""
        raws = [r for _, r in seg.words]
        lead = 0
        while lead < len(raws) and raws[lead] in ("!", "time"):
            lead += 1
        if lead < len(raws) and raws[lead] in _COMPOUND:
            self.loops = True
            self._odd("`%s` (a block, a loop, a test or a function)" % raws[lead],
                      "write the commands one after another, joined by ; && || or |")
        if len(raws) >= 2 and raws[0] in ("for", "select", "foreach"):
            name = raws[1]
            ins = raws.index("in") if "in" in raws else len(raws)
            for v, r in seg.words[ins + 1:] or [(MARK, MARK)]:
                self._set(name, v, r)
        alone = all(ASSIGN.match(r) for r in raws)
        # FLAT: an assignment alone (or `export`'s literal NAME=value) in a command whose text is `simple` (`parse`: no
        # subshell, substitution, group or reserved word anywhere, so it is at the top and runs in this shell), not
        # after `&&`/`||` in its list, not in a pipeline or the background, with no redirect. Any other `NAME=` word (a
        # prefix `A=x cmd`, one after `&&` …) is read as an assignment that may or may not run: more than the shell
        # assigns, never less.
        top = self.flat and not (self.andor or seg.after_pipe or seg.sep in ("|", "&") or seg.redirects or seg.data)
        export = raws[:1] == ["export"] and all(re.match(r"^[A-Za-z_]\w*(=|$)", r) for r in raws[1:])
        here = {}  # assigned by an earlier word of this command: `A=x B=$A` reads the x
        for k, (v, r) in enumerate(seg.words):
            m = ASSIGN.match(r)
            if not m:
                continue
            name = r.split("=", 1)[0].split("[", 1)[0].rstrip("+")
            value, text = (FORM,) * 2 if (v, r) in seg.forms else (v.split("=", 1)[1], r.rstrip(UNSURE).split("=", 1)[1])
            # the names its value reads (single-quoted text reads none)
            reads = set(READS.findall(re.sub(r"'[^']*'", "", text)))
            plus = "+=" in m.group(0)
            if plus:  # `A+=x`: the value A had, then x
                old = self._value(name, read=False)
                value, text = (MARK if old is None else old) + value, "$" + name + text
            # bash and zsh expand `~` (and zsh `=name`) at the value's start and after each `:`: not the text as written
            if top and (alone or (export and k)) and not m.group(1) and not reads & here.keys() and (v, r) not in seg.tilde:
                self.values.setdefault(name, []).append((value, text))
                self.known[name] = value
                if plus or not known(value) or not reads.isdisjoint(self.values):
                    self.unflat.add(name)  # `B=$A`, `A+=x`, `A=$(…)`, `x && A=y`: one value, but not a flat one
            else:
                self._set(name, value if not reads & here.keys() else FORM if any(FORM in here[n] for n in reads & here.keys()) else MARK, text)
            here[name] = value
            if name in _CWD:  # bash reads `cd ..` from $PWD
                self.cwd = self.settled = None
            if m.group(1):  # zsh's `functions[cat]=…`, `commands[cat]=…`, `aliases[…]`
                self._rebind("an array element assignment `%s`" % m.group(0), "assign one value: `NAME=value`")
            if name in _READS_LATER and (alone or (k and raws[0] == "export")):
                self._odd("`%s` set for the rest of the command (it changes how later words are read)" % m.group(0),
                          "set it on the one command that needs it: `%s… cmd`" % m.group(0))
        self._builtin(seg)

    def _builtin(self, seg):
        """A builtin outside FLAT_BUILTINS, or one of them with an option that assigns or changes how words are read;
        and the cwd model after it (`_move`)."""
        words, i = seg.words, 0
        while i < len(words) and words[i][1] in _RESERVED:
            i += 1
        while i < len(words) and ASSIGN.match(words[i][1]):
            i += 1
        while i < len(words) and known(words[i][0]) and words[i][0] in SHELL_WORDS and words[i][0] in PRECOMMANDS:
            letters, longs = PRECOMMANDS[words[i][0]]
            i += 1
            while i < len(words) and words[i][0].startswith("-") and len(words[i][0]) > 1:
                i += 1 + takes_next(words[i][0], letters, longs)
        self._move(seg, words, i)
        if i >= len(words) or not known(words[i][0]):
            return
        verb, args = words[i][0], [v for v, _ in words[i + 1:]]
        if "/" in verb or verb not in SHELL_WORDS or verb in _COMPOUND:
            return  # a program (or a reserved word, already named)
        if verb not in FLAT_BUILTINS:
            for a in args:
                if re.fullmatch(r"[A-Za-z_]\w*", a):
                    self._set(a)  # `read A`, `unset A`, `local A` …: A may change
            self._rebind("the builtin `%s` (it may assign a variable, define a name or run code the guard does not read)"
                      % verb, "run `%s` in a command of its own, not in one that names brain" % verb)
        elif verb == "export" and not all(re.match(r"^[A-Za-z_]\w*(=|$)", r) for _, r in words[i + 1:]):
            self._rebind("`export` with an option or a computed name", "export literal names: `export NAME=value`")
        elif verb in ("printf", "print") and any(re.match(r"^-[A-Za-z]*[%s]" % ("v" if verb == "printf" else "vsz"), a)
                                                 for a in args):
            for k, a in enumerate(args):  # `printf -v A x`, `-vA`: A may change
                n = re.fullmatch(r"-[A-Za-z]*v([A-Za-z_]\w*)?", a)
                name = n and (n.group(1) or (args[k + 1] if k + 1 < len(args) else ""))
                if name and re.fullmatch(r"[A-Za-z_]\w*", name):
                    self._set(name)
            self._rebind("`%s -v` (it assigns a variable)" % verb, "print to a file in $HW_ARTIFACTS instead")
        elif verb == "set" and not _set_flat(args):
            self._rebind("`set %s` (an option that changes how words are read, or the positional parameters)"
                      % " ".join(args), "use only `set -euxo pipefail`, or none")

    def _expand(self):
        """One `$…` or backtick expansion at self.i: (its value, the read when that value is unknown, else None). A read
        is (name, None) when plain (`$NAME`, `${NAME}`), else a form (`Seg.forms`): (name or "", what marks it)."""
        t = self.text
        if t[self.i] == "`":
            segs, self.i = self._backtick(t, self.i)
            return _output(segs), None
        if t.startswith("$((", self.i):
            j = _close(t, self.i + 1, "(", ")")
            self._arith(t[self.i + 3:j - 1])
            self.i = j + 1
            return MARK, None
        if t.startswith("$(", self.i):
            self._substitution("$(…)")
            start = len(self.segs)
            self.i += 2
            self._subshell()
            return _output(self.segs[start:]), None
        if _FUNSUB.match(t, self.i):
            self.i = self._funsub(t, self.i) + 1
            return MARK, None
        if t.startswith("${", self.i):
            j = _close(t, self.i + 1, "{", "}")
            body, self.i = t[self.i + 2:j], j + 1
            return self._param(body)
        if t.startswith("$[", self.i):  # the old arithmetic form
            j = _close(t, self.i + 1, "[", "]")
            self._arith(t[self.i + 2:j])
            self.i = j + 1
            return MARK, None
        m = re.compile(r"\$([=~^+#]+)(?:[A-Za-z_]\w*)?").match(t, self.i)
        if m and m.group(0) != "$#":  # zsh `$=A` (split), `$~A` (glob), `$^A`, `$+A`, `$#A`: not the value as written
            self._odd("zsh's `%s`" % m.group(0), "write `$%s` or its value" % m.group(0)[2:])
            self.i += len(m.group(0))
            return MARK, (m.group(0).lstrip("$=~^+#"), m.group(1))
        m = re.compile(r"\$([A-Za-z_][A-Za-z0-9_]*)").match(t, self.i)
        if m:
            self.i += len(m.group(0))
            v = self.var(m.group(1))
            if re.match(r"\[|:[aAcefFghlpPqQrsStuwWxX&]", t[self.i:self.i + 2]):
                # zsh reads `$A[2,3]` as a subscript and `$A:t` as a modifier (bash as literal text)
                self._odd("zsh's `%s%s…` (a subscript or a modifier)" % (m.group(0), t[self.i]),
                          "write `${%s}` and the text after it in quotes" % m.group(1))
                return MARK, (m.group(1), t[self.i])
            return v, _read(m.group(1), v)
        if re.compile(r"\$[0-9@*#?$!-]").match(t, self.i):  # a positional or special parameter: a form, but `$?`,
            c, self.i = t[self.i + 1], self.i + 2  # `$#`, `$$`, `$!`, which expand to digits only and cannot name a path
            return MARK, None if c in "?#$!" else ("", c)
        self.i += 1
        return "$", None

    def _nested(self, text):
        """A backtick's command list, read on its own (in a subshell: nothing it assigns has one value here): its
        findings are this command's."""
        sub = Shell(text, self.env, self.taint, flat=self.flat, cwd=self.cwd)
        sub.known, sub.unflat, sub.moved, sub.oldcwd = dict(self.known), set(self.unflat), self.moved, self.oldcwd
        sub.nested = True
        sub._list(end=None)
        self.moved = sub.moved
        self.segs += sub.segs
        self.odd += sub.odd
        self.reads += sub.reads
        self.unreadable = self.unreadable or sub.unreadable
        self.heredocs += [(s, body, None, quoted) for s, body, _, quoted in sub.heredocs]  # code: in a substitution
        self.loops = self.loops or sub.loops
        for name, vals in sub.values.items():
            for v, r in vals:
                self._set(name, v, r)
        return sub.segs

    def _substitutions(self, text):
        """Every `$(…)`, `${ cmd; }`, `${| cmd; }` and backtick inside `text` (an arithmetic, `${…}` or heredoc body)
        runs: each is read as a command list, as this command's."""
        i = 0
        while i < len(text):
            if text.startswith("$(", i) and not text.startswith("$((", i):
                j = _close(text, i + 1, "(", ")")
                self._substitution("$(…)")
                self._nested(text[i + 2:j])
                i = j + 1
            elif _FUNSUB.match(text, i):
                i = self._funsub(text, i) + 1
            elif text[i] == "`":
                _, i = self._backtick(text, i)
            else:
                i += 1

    def _arith(self, body):
        """`$((…))`: not flat; any name in it may be assigned (`$((A=1))`, `A++`)."""
        self._odd("an arithmetic expansion", "write the number")
        self._substitutions(body)
        for name in re.findall(r"[A-Za-z_]\w*", body):
            self._set(name)

    def _param(self, body):
        """`${NAME}`: plain. Any other form (an operator, a subscript, a length, an indirection) is not flat, and the
        word holding it is one the guard cannot resolve to a literal; one that may assign records its name. (value,
        read) as `_expand`."""
        if re.fullmatch(r"[A-Za-z_]\w*", body):
            v = self.var(body)
            return v, _read(body, v)
        self._odd("`${%s}`" % body, "write `${NAME}` or its value")
        self._substitutions(body)
        n = re.match(r"[#!]?([A-Za-z_]\w*)", body)
        if n:
            self._value(n.group(1))  # a read of a name the command assigns (before `${A:=x}` assigns it)
        a = re.match(r"(?:\([^)]*\))?([A-Za-z_]\w*)(::?=|=)(.*)", body, re.S)
        if a:
            self._set(a.group(1), a.group(3), a.group(3))
        # not plain: its unknown piece stays (MARK), a read the guard does not expand (deny_brain_writes `_unexpanded`)
        return MARK, ("", body)

    def _unflat_read(self):
        """Name the first read of a variable whose assignment is not flat, with the literal spelling that passes."""
        name = self.reads[0]
        last = self.known.get(name, MARK)  # the one value it has, else what its last assignment wrote
        last = last if known(last) else self.values.get(name, [(MARK, MARK)])[-1][1]
        ref = re.compile(r"\$\{?%s\}?(?![\w])" % re.escape(name) if name != "HOME" else
                         r"\$\{?HOME\}?(?![\w])|(?<![^\s=:])~(?=/|$)")
        # the first simple command (as written) that reads it, with the value in its place
        spelled = next((ref.sub(lambda _: last, piece.strip()) for piece in re.split(r"[;\n]|&&|\|\|", self.text)
                        if ref.search(piece) and not re.match(r"\s*%s\+?=" % re.escape(name), piece)), None)
        self._odd("`%s`, which it assigns other than once and literally" % ("~` or `$HOME" if name == "HOME" else "$" + name),
                  "write its value where it is used%s" % (": `%s`" % spelled if spelled else ""))


def _consumes(seg):
    """True when this simple command is one of DATA_CONSUMERS as written: its first word (no assignment before it),
    literal, is the consumer's bare name or its path in a system directory."""
    verb, raw = (seg.words or [(MARK, MARK)])[0]
    literal = known(verb) and UNSURE not in raw
    spelled = os.path.basename(verb) in DATA_CONSUMERS and os.path.dirname(verb) in _SYSTEM_BIN | {""}
    return literal and spelled


def _set_flat(args):
    """`set` with only options that change nothing the guard reads (`-euxo pipefail`)."""
    i = 0
    while i < len(args):
        a = args[i]
        if not re.fullmatch(r"[-+][%so]+" % _SET_LETTERS, a):
            return False
        if a.endswith("o"):
            if i + 1 >= len(args) or args[i + 1] not in _SET_NAMES:
                return False
            i += 1
        i += 1
    return True


def _output(segs):
    """What a `$(…)` prints, when the guard can tell: `echo` of known words (no option, no backslash: zsh's echo reads
    escapes), an absolute glob among them listed as the shell would when it matches, `:`/`true` nothing. MARK
    otherwise. Read only to find whether a command that is not flat names brain: a flat command has no `$(…)`."""
    if len(segs) != 1:
        return MARK
    s = segs[0]
    if s.redirects or s.data or not s.words or s.words[0][1] not in ("echo", ":", "true"):
        return MARK
    if s.words[0][1] != "echo":
        return "" if len(s.words) == 1 else MARK
    out = []
    for v, r in s.words[1:]:
        if not known(v) or "\\" in v or v.startswith("-"):
            return MARK
        if UNSURE not in r:
            out.append(v)
        elif re.fullmatch(r"~?[A-Za-z0-9_.,:/@%+=*?-]+", r.rstrip(UNSURE)) and v.startswith("/") and (hits := glob.glob(v)):
            out += sorted(hits)[:64]
        else:
            return MARK
    return " ".join(out)


def parse(text, env, cwd=None):
    """The command, read in order from `cwd`; one whose text is not `simple` again with no flat assignment, and one that
    loops or defines a function with every name it assigns (and the cwd, if it moves) unknown wherever it is read."""
    sh = Shell(text, env, cwd=cwd)
    if sh.parse() or not (sh.values or sh.loops and sh.moved):  # no assignment: a second read would be the same
        return sh
    again = Shell(text, env, taint=set(sh.values) if sh.loops else (), flat=False,
                  cwd=None if sh.loops and sh.moved else cwd)
    again.parse()
    return again
