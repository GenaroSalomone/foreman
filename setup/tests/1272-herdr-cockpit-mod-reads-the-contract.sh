#!/usr/bin/env bash
# The herdr app's cockpit mod (herdr-app/mods/foreman-cockpit) reads the cockpit
# state through its contract, cockpit/schema.json v1, and the contract ships a
# table of what a reader must make of each example file: cockpit/fixtures/index.json
# names the class and the number of drawn rows per fixture, including a missing
# file, a >1 MiB file and a file from an older writer.
#
# The mod's `state.js` is the reader and imports nothing from the app, so it runs
# under plain node: this subject pins the mod to that table. Without it, the mod
# could drift from the writer silently — schema.json is enforced on the writer,
# and a reader that draws a dead file as fresh has no other gate.
#
# Run alone while working on this subject:
#     bash setup/tests/1272-herdr-cockpit-mod-reads-the-contract.sh
. "$(dirname "${BASH_SOURCE[0]}")/_common.sh"

READER="$ROOT/herdr-app/mods/foreman-cockpit/state.js"
[ -f "$READER" ] || fail "the mod's reader is missing: $READER"

command -v node >/dev/null || { pass "node is not installed here: the reader's check is skipped"; exit 0; }
# state.js is an ES module in a .js file, and an .mjs importer only loads it on a
# node with module-syntax detection (22.7+, 20.19+). Older node is a loud skip,
# not a red on a SyntaxError that says nothing about the reader.
node -e 'const [a, b] = process.versions.node.split(".").map(Number); process.exit(a > 22 || (a === 22 && b >= 7) || (a === 20 && b >= 19) ? 0 : 1)' ||
  { pass "node $(node -v) has no module-syntax detection: the reader's check is skipped"; exit 0; }

out="$(node "$ROOT/herdr-app/mods/foreman-cockpit/state.test.mjs" 2>&1)" || fail "the mod disagrees with cockpit/fixtures/index.json: $out"
pass "$out"

# Every other mod's own node test (a mod's test with no runner of its own).
# A mod the export excludes is simply absent there, so the glob skips it.
for t in "$ROOT"/herdr-app/mods/*/*.test.mjs; do
  [ -f "$t" ] && [ "$t" != "$ROOT/herdr-app/mods/foreman-cockpit/state.test.mjs" ] || continue
  out="$(node "$t" 2>&1)" || fail "${t#"$ROOT"/} failed: $out"
  pass "${t#"$ROOT"/}: $out"
done

# What the export ships of the app side must load and must be explained (brain only: the export has no manifest).
# An exported .js that imports a file the manifest does not include fails `herdr-app check` in the export
# (measured 2026-11-24: main.js imported actions.js and restore.js, thrown.js behind it, none of them included),
# and the export's user page (overlay herdr-app/README.md) stays a short page whose relative links resolve there and whose
# `ln -sfn ~/brain/...` source is a path install.sh mirrors into the brain (a dangling link otherwise: review of 2026-11-24).
MANIFEST="$ROOT/setup/export/manifest"
PAGE="$ROOT/setup/export/overlay/herdr-app/README.md"
if [ -f "$MANIFEST" ]; then
  [ -f "$PAGE" ] || fail "the export has no user page for the app side: $PAGE"
  out="$(python3 - "$ROOT" <<'PY'
import os, re, sys
root = sys.argv[1]
inc = {l.split(None, 1)[1].strip() for l in open(root + "/setup/export/manifest") if l.startswith("include ")}
ov = "setup/export/overlay/"
shipped = set(inc) | {os.path.relpath(os.path.join(d, f), root + "/" + ov)
                      for d, _, fs in os.walk(root + "/" + ov) for f in fs if "/changes" not in d}
bad = []
for p in sorted(inc):
    if p.startswith("herdr-app/mods/") and p.endswith(".js"):
        for m in re.findall(r'from "(\./[^"]+)"', open(root + "/" + p).read()):
            q = os.path.normpath(os.path.join(os.path.dirname(p), m))
            if q not in shipped:
                bad.append("%s imports %s, which the export does not ship" % (p, q))
page = open(root + "/" + ov + "herdr-app/README.md").read()
if page.count("\n") > 80:
    bad.append("the user page is %d lines, over 80" % page.count("\n"))
installer = open(root + "/install.sh").read()
for m in re.findall(r"^ln -sfn ~/brain/(\S+)", page, re.M):   # a path the page links from the brain: install.sh must put it there
    if '"$BRAIN/%s"' % m not in installer:
        bad.append("the user page links ~/brain/%s, which install.sh does not mirror into the brain" % m)
for m in re.findall(r"\]\((?!https?:|#)([^)#]+)", page):
    q = os.path.normpath(os.path.join("herdr-app", m))
    if q not in shipped:
        bad.append("the user page links %s, which the export does not ship" % q)
print("\n".join(bad))
PY
)" || fail "the export's app side could not be checked: $out"
  [ -z "$out" ] || fail "the export's app side is broken: $out"
  pass "the exported app mods import only shipped files; the user page is short and its links resolve"
fi
