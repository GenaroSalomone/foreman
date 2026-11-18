// Cross-lane conformance for the read-only guard.
//
// Python is exercised through its PreToolUse stdin contract, from each lane's
// real registered hook path (the lane shim), so a broken shim fails here rather
// than in a pane. (The pi executor asks the same Python, through
// setup/guards/deny_repo_writes_ask.py: setup/tests/181 and 208 drive that question.)
//
// There are two assertions per vector per lane:
//   <lane>/<runtime>   the verdict is what the vector says
//   cross-lane         every lane agrees with every other     (LOGIC parity)
//
// A vector marked `lane_specific` is exempt from the cross-lane assertion and
// must instead declare `expect_with_worktree_roots` (or, for a difference no
// policy field carries, `expect_by_lane`) — that is how a deliberate per-lane
// difference (the spent-worktree teardown) stays visible as an assertion rather
// than becoming the next silent drift.
//
// A SECOND ARM RUNS ON A SANDBOX POLICY (`"sandbox"` vectors). A vector that
// needs a protected root to exist, or to be absent, cannot lean on the machine
// it runs on: the brain's has every root and the export's has none, and that is
// how a hole in a guard was found by the export and not here. The
// sandbox arm writes its own `guards.json` under a temporary HOME, copies the
// guards beside it, and builds exactly the tree each vector needs.
//
// ONE PROCESS FOR THE VECTORS, A SAMPLE FOR THE PROCESS (2026-10-20). This suite
// spawned `python3 <hook>` once per vector, ~2000 interpreters a run, and the
// mutation arm repeated that for each of 77 mutants. The claim "every vector
// through the real process entry" is RENAMED, not dropped:
//
//   every vector     through the real hook FILE (the lane shim) run in-process
//                    by `hook-batch.py`: same `main`, same `decide`, same
//                    refusal protocol, no second implementation.
//   a fixed sample   (first vector per lane x tool x outcome) through the real
//                    `spawnSync("python3", [hook])`, whose answer must EQUAL the
//                    batch's — status, stdout and stderr — and meet the vector's
//                    own expectation.
//
// What a vector no longer proves on its own is the PROCESS: interpreter start-up,
// the OS's `sys.path[0]`, the exit status the kernel reports. The sample proves
// that, for one vector per outcome rather than every one. The malformed-payload
// shapes and the sandbox arm keep a real spawn each.
import { copyFileSync, existsSync, linkSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { execFileSync, spawn, spawnSync } from "node:child_process";
import { homedir, tmpdir } from "node:os";
import { dirname, basename, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = process.env.DENY_GUARD_ROOT ?? fileURLToPath(new URL("../..", import.meta.url));
const allVectors = JSON.parse(readFileSync(new URL("./deny-repo-writes-vectors.json", import.meta.url)))
  .filter((v) => !v._comment);
const vectors = allVectors.filter((v) => !v.sandbox);
const sandboxVectors = allVectors.filter((v) => v.sandbox);

// THE LANE ROOTS COME FROM THE GUARD, NOT FROM A COPY OF THEM HERE. This file
// used to carry its own table of the four lanes' repo/worktrees pairs, which is
// one more place for the thing being tested and the test to disagree.
const { BRAIN_ROOT, LANES } = (() => {
  const doc = JSON.parse(execFileSync("python3", ["-c",
    "import json, sys; sys.path.insert(0, sys.argv[1]); import deny_repo_writes as g; " +
    "print(json.dumps({'brain': g.BRAIN_ROOT, 'lanes': g.LANES}))", `${root}/setup/guards`], { encoding: "utf8" }));
  const lanes = {};
  for (const [name, l] of Object.entries(doc.lanes)) {
    lanes[name] = { repo: l.repo, worktrees: l.worktrees, worktreeRoots: l.worktree_roots, writeHere: l.write_here };
  }
  return { BRAIN_ROOT: doc.brain, LANES: lanes };
})();
const laneNames = Object.keys(LANES);

const outside = "/tmp/deny-repo-writes-out";
const base = mkdtempSync(join(tmpdir(), "deny-repo-writes-vec-"));

// Per-lane renderings that only a real filesystem can provide. `{{link}}` is a
// symlink INTO the lane's protected root, created outside it — the shape that
// every pre-unification copy walked straight past. `{{dotdot}}` reaches the
// same place through a `..` from a sibling that need not exist. `{{sibling}}` is
// the opposite: a name that merely BEGINS with a protected root.
function laneRender(lane) {
  const { repo, worktrees } = LANES[lane];
  const link = `${base}/${lane}-link`;
  // The guard hands roots back in its canonical spelling, which on Windows is
  // `/c/...`: a link must point at the spelling Windows itself opens.
  const winSpelling = (p) => process.platform === "win32" ? p.replace(/^\/([a-z])(?=\/|$)/, "$1:") : p;
  symlinkSync(winSpelling(repo), link);
  // `{{dangling}}` points at a path INSIDE the repo that does not exist, so the
  // link dangles in every context — the brain's, where the repo exists, and the
  // export's, where it does not. A write through it creates the target in the
  // repo; the guard must follow it the way os.path.realpath does.
  const dangling = `${base}/${lane}-dangling`;
  symlinkSync(winSpelling(`${repo}/.deny-guard-absent-target`), dangling);
  // `{{dangling_rel}}` is a dangling link whose RELATIVE target climbs with
  // `..`, sitting in a directory reached through ANOTHER symlink. Its `..` must
  // count from the directory's real location, as the kernel and os.path.realpath
  // count them: from there it lands inside the repo, while from the link's
  // lexical parent the same climb lands outside, under `${base}`. Built entirely
  // outside the repo; POSIX only (a relative climb to `/` has no Windows twin),
  // so on Windows it renders as `{{dangling}}`, which carries the same verdict.
  let danglingRel = dangling;
  if (process.platform !== "win32") {
    const real = `${base}/${lane}-real`;
    mkdirSync(real);
    const via = `${base}/${lane}-via/a/b/c`;
    mkdirSync(via, { recursive: true });
    symlinkSync(real, `${via}/dir`);
    const climb = "../".repeat(realpathSync(real).split("/").filter(Boolean).length);
    symlinkSync(`${climb}${repo.slice(1)}/.deny-guard-absent-target`, `${real}/dl`);
    danglingRel = `${via}/dir/dl`;
  }
  return {
    repo,
    worktrees,
    link,
    dangling,
    danglingRel,
    dotdot: `${dirname(repo)}/.deny-guard-absent/../${basename(repo)}/sub`,
    sibling: `${repo}-deny-guard-sibling`,
    // The repo spelled through `$HOME`, for a vector about the guard resolving
    // the variable rather than matching the text. A repo outside the home has
    // no such spelling and renders as itself.
    homeRepo: repo.startsWith(`${homedir()}/`) ? `$HOME${repo.slice(homedir().length)}` : repo,
    // The repo reached WITHOUT spelling it (2026-09-29, audit F5/F8): its bare
    // name for a `../<name>` climb from `{{sibling}}`, its parent for a `*`, the
    // name with its last letter turned into `?`, and a brace that expands to it.
    repoName: basename(repo),
    repoParent: dirname(repo),
    repoGlob: `${dirname(repo)}/${basename(repo).slice(0, -1)}?`,
    repoBrace: `${dirname(repo)}/{deny-guard-other,${basename(repo)}}`,
  };
}

const category = (reason) =>
  reason.includes("shell redirect") ? "redirect"
    : reason.includes("in-place") ? "inplace"
      : reason.includes("git ") ? "git" : "mutator";

const triggerMatches = (reason, trigger) => !trigger || ({
  // The trigger text became "the command names a protected root (<path>)" on
  // 2026-09-07: a lane now protects the other lanes' product repos too, so
  // "the repo or worktree path" no longer says WHICH tree, and a cross-lane
  // denial that cannot name its tree reads as a misconfiguration.
  names: /command names (?:a protected root|the repo or worktree path)/,
  cwd: /working directory is inside it|shell's cwd is inside it/,
  cd: /command itself cd's into it/,
  resolved: /a path in the command RESOLVES inside it/,
}[trigger]).test(reason);

const render = (value, r) => value
  .replaceAll("{{repo}}", r.repo)
  .replaceAll("{{worktrees}}", r.worktrees)
  .replaceAll("{{link}}", r.link)
  .replaceAll("{{dangling_rel}}", r.danglingRel)
  .replaceAll("{{dangling}}", r.dangling)
  .replaceAll("{{dotdot}}", r.dotdot)
  .replaceAll("{{sibling}}", r.sibling)
  .replaceAll("{{home_repo}}", r.homeRepo)
  .replaceAll("{{repo_name}}", r.repoName)
  .replaceAll("{{repo_parent}}", r.repoParent)
  .replaceAll("{{repo_glob}}", r.repoGlob)
  .replaceAll("{{repo_brace}}", r.repoBrace)
  .replaceAll("{{outside}}", outside)
  .replaceAll("{{out}}", `${outside}/copy`);

// A hook's raw answer (`{status, stdout, stderr}`, from a real spawn or from
// `hook-batch.py`) read as a verdict. One reader for both, so the batch and the
// sample cannot disagree about what an answer means.
function claudeResult(r) {
  if (r.status !== 0) return { crashed: `${r.status}: ${r.stderr}` };
  const out = r.stdout.trim();
  if (!out) return { allowed: true, reason: "" };
  try { return { allowed: false, reason: JSON.parse(out).hookSpecificOutput.permissionDecisionReason }; }
  catch { return { crashed: `invalid hook output: ${out}` }; }
}

const payloadOf = (command, cwd, toolName = "Bash") =>
  JSON.stringify({ tool_name: toolName, tool_input: { command }, cwd });

// One request of `hook-batch.py`, and the same request on a real interpreter.
function realEntry({ hook, stdin, env, cwd }) {
  const r = spawnSync("python3", [hook], { input: stdin, encoding: "utf8", env: env ?? process.env, cwd: cwd ?? undefined });
  return { status: r.status, stdout: r.stdout, stderr: r.stderr };
}

// Every request in ONE `hook-batch.py` process, answered in order. A process that
// ends before answering leaves the rest as a crash (status null), which every
// reader above turns into a failing assertion rather than a silent allow.
//
// THE ANSWERS ARE READ AS THEY ARRIVE, not after the last one: the process works
// while this one judges, and `answer(i)` resolves as soon as request `i` has
// been answered. `first` are the requests that run first (the real-entry sample's
// twins), so the sample is judged before the rest has been decided; the rest keep
// their order. A fail-fast stop therefore costs the vectors up to the failing one,
// not the whole batch, and `kill()` ends the process behind it.
let killBatch = () => {};
function runBatch(requests, first = [], env) {
  const head = new Set(first);
  const order = [...head, ...Array.from(requests.keys()).filter((i) => !head.has(i))];
  const waiting = requests.map(() => { const w = {}; w.promise = new Promise((resolve) => { w.resolve = resolve; }); return w; });
  const child = spawn("python3", [`${root}/setup/guards/hook-batch.py`], { env: env ?? process.env, stdio: ["pipe", "pipe", "pipe"] });
  let buf = "", err = "", next = 0;
  const crashRest = (why) => {
    for (; next < order.length; next++) {
      waiting[order[next]].resolve({ status: null, stdout: "", stderr: `hook-batch.py ended (${why}) before answering request ${order[next]}: ${err}` });
    }
  };
  child.stdout.setEncoding("utf8");
  child.stderr.setEncoding("utf8");
  child.stdout.on("data", (d) => {
    buf += d;
    for (let nl = buf.indexOf("\n"); nl >= 0; nl = buf.indexOf("\n")) {
      const line = buf.slice(0, nl);
      buf = buf.slice(nl + 1);
      if (line && next < order.length) waiting[order[next++]].resolve(JSON.parse(line));
    }
  });
  child.stderr.on("data", (d) => { err += d; });
  child.stdin.on("error", () => {});
  child.on("error", (e) => crashRest(String(e)));
  child.on("close", (code) => crashRest(code));
  child.stdin.end(order.map((i) => JSON.stringify(requests[i])).join("\n") + "\n");
  killBatch = () => child.kill();
  return { answer: (i) => waiting[i].promise };
}

// The same request on a real interpreter, asynchronously: the sample's ~50
// spawns run side by side instead of one after the other.
function realEntryAsync({ hook, stdin, env, cwd }) {
  return new Promise((resolve) => {
    let stdout = "", stderr = "";
    const child = spawn("python3", [hook], { env: env ?? process.env, cwd: cwd ?? undefined, stdio: ["pipe", "pipe", "pipe"] });
    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stdout.on("data", (d) => { stdout += d; });
    child.stderr.on("data", (d) => { stderr += d; });
    child.stdin.on("error", () => {});
    child.on("error", (e) => resolve({ status: null, stdout, stderr: `${stderr}${e}` }));
    child.on("close", (code) => resolve({ status: code, stdout, stderr }));
    child.stdin.end(stdin);
  });
}

function python(path, command, cwd, env) {
  return claudeResult(realEntry({ hook: path, stdin: payloadOf(command, cwd), env }));
}

let pass = 0, fail = 0;
const ok = (name) => { pass++; console.log(`ok - ${name}`); };
// DENY_GUARD_FAIL_FAST=1 STOPS AT THE FIRST `not ok`, for the mutation arm only
// (setup/guards/mutate-deny-repo-writes.sh). A mutant is killed by a non-zero
// exit AND the first `not ok -` line this driver prints — nothing after that
// line is read — so running the other ~5500 vectors, a python3 spawn each,
// bought nothing: 77 mutants x ~70s of CPU made that arm the longest job of
// the full suite (1949s measured under the runner, 2026-09-30). The line and
// the exit a kill is judged on are the same; a surviving mutant still runs
// every vector. Unset, this driver is unchanged.
const FAIL_FAST = process.env.DENY_GUARD_FAIL_FAST === "1";
const notOk = (name, detail = "") => {
  fail++;
  console.log(`not ok - ${name}${detail ? ` :: ${detail}` : ""}`);
  if (FAIL_FAST) {
    killBatch();
    rmSync(base, { recursive: true, force: true });
    console.log(`\n${pass} passed, ${fail} failed (DENY_GUARD_FAIL_FAST: stopped at the first failure)`);
    process.exit(1);
  }
};

// Per-driver tally, so a driver that quietly ran no assertion shows as a zero.
const driverTally = { python: { pass: 0, fail: 0 } };
const tally = (runtime, good) => { driverTally[runtime][good ? "pass" : "fail"]++; };

// One verdict against one vector: the deny/allow shape, its category and
// trigger, and the two wording assertions a vector may carry. Shared by the
// lane arm and the sandbox arm so a check added to one cannot miss the other.
function judge(label, runtime, vector, expect, result) {
  const good = !result.crashed && result.allowed === (expect === "allow") &&
    (expect === "allow" || (category(result.reason) === vector.category && triggerMatches(result.reason, vector.trigger)));
  tally(runtime, good);
  if (good) ok(label);
  else notOk(label, result.crashed ?? `${result.allowed ? "allowed" : `${category(result.reason)} ${result.reason}`}`);
  // `content_only: true` marks a vector whose ONLY signal is a protected
  // path sitting inside a quoted argument — the guard cannot confirm
  // that is the command's actual destination, and the message must say
  // so rather than assert a destination it never resolved (2026-09-09:
  // "the command names a protected root" used to read as a confirmed
  // target even when the match came from inside `-m "..."` prose).
  if (vector.content_only && expect !== "allow") {
    const g = !result.crashed && !result.allowed && /could not confirm/.test(result.reason);
    tally(runtime, g);
    if (g) ok(`${label} names its own uncertainty`);
    else notOk(`${label} names its own uncertainty`, result.crashed ?? result.reason);
  }
  // `message_contains: [...]` asserts every listed substring appears in
  // the denial's reason — for wording the vector's ALLOW/DENY shape and
  // category/trigger cannot express on their own (e.g. that a redirect
  // denial names the RESOLVED destination, not just that it denied).
  if (vector.message_contains && expect !== "allow") {
    const missing = result.crashed ? vector.message_contains
      : vector.message_contains.filter((s) => !result.reason.includes(s));
    const g = !result.crashed && missing.length === 0;
    tally(runtime, g);
    if (g) ok(`${label} message contains expected text`);
    else notOk(`${label} message contains expected text`,
      result.crashed ?? `missing ${JSON.stringify(missing)} in: ${result.reason}`);
  }
}

const verdictOf = (r) => r.crashed ? "crashed" : r.allowed ? "allow" : category(r.reason);

// shape[vectorId][lane] = "allow" | "deny:<category>" | "crashed"
const shape = Object.create(null);
// expected[vectorId][lane] = the verdict the lane's own policy row asks for.
const expected = Object.create(null);
// voice[lane] = Set of lanes whose `writeHere` this lane's denials named.
const voice = Object.create(null);

try {
  // THE LANE ARM'S ANSWERS COME FROM ONE `hook-batch.py`
  // PROCESS, asked before any assertion so the sample below can run first.
  const lanes = new Map();
  for (const laneName of laneNames) {
    // THE ROOT LANE LIVES ONE DIRECTORY UP. `brain` is `~/brain`
    // itself, not `~/brain/brain` — measured once, a session
    // started at the root loaded no settings and no guard at all, so the root
    // became a lane like the other four and its registered files sit directly
    // under it.
    const laneDir = laneName === "brain" ? root : `${root}/${laneName}`;
    lanes.set(laneName, {
      r: laneRender(laneName),
      laneDir,
      pyPath: `${laneDir}/.claude/hooks/deny-repo-writes.py`,
    });
  }
  // `expect_with_worktree_roots` is the expectation for a lane whose policy
  // declares `worktree_roots` — the property that grants the spent-worktree
  // teardown — so the difference follows the table, not a lane's name.
  const expectFor = (laneName, vector) => vector.expect_by_lane?.[laneName]
    ?? (vector.expect_with_worktree_roots && LANES[laneName].worktreeRoots?.length
      ? vector.expect_with_worktree_roots : vector.expect);
  // The Claude entry's request per lane per vector — the order the assertions
  // below consume them in.
  const requests = [];
  const sampleOf = new Map();   // "<lane>|<runtime>|Bash|<outcome>" -> its first request
  for (const [laneName, L] of lanes) {
    for (const vector of vectors) {
      const command = render(vector.command, L.r);
      const cwd = render(vector.cwd, L.r);
      const stdin = payloadOf(command, cwd);
      const expect = expectFor(laneName, vector);
      const py = { hook: L.pyPath, stdin, env: null, cwd: null };
      // FIXED BY THE VECTOR'S OWN EXPECTATION, never by what the code under test
      // answered: the first vector of each lane x runtime x tool x outcome.
      const outcome = (wanted) => wanted === "allow" ? "allow" : `deny:${vector.category}`;
      const key = `${laneName}|python|Bash|${outcome(expect)}`;
      if (!sampleOf.has(key)) sampleOf.set(key, { request: py, vector, wanted: expect, at: requests.length });
      requests.push(py);
    }
  }
  const batch = runBatch(requests, [...sampleOf.values()].map((s) => s.at));
  const reals = await Promise.all([...sampleOf.values()].map((s) => realEntryAsync(s.request)));

  // ── THE SAMPLE: THE REAL PROCESS ENTRY, FIRST ───────────────────────────
  // Runs before every other assertion on purpose: a batch entry that answers
  // wrongly for every vector (wired to allow, say) is named here, as a batch
  // that disagrees with the real entry, before the vectors blame the guard.
  let sampled = 0;
  for (const [key, s] of sampleOf) {
    const real = reals[sampled++];
    const answered = await batch.answer(s.at);
    const [laneName, runtime] = key.split("|");
    const same = real.status === answered.status && real.stdout === answered.stdout && real.stderr === answered.stderr;
    if (same) ok(`sample ${key} (${s.vector.id}): the batch answer equals the real entry's`);
    else notOk(`sample ${key} (${s.vector.id}): the batch answer equals the real entry's`,
      `real=${JSON.stringify(real).slice(0, 300)} batch=${JSON.stringify(answered).slice(0, 300)}`);
    judge(`sample ${key} (${s.vector.id}) real entry`, runtime, s.vector, s.wanted, claudeResult(real));
  }

  let at = 0;
  for (const laneName of laneNames) {
    const { r } = lanes.get(laneName);
    for (const vector of vectors) {
      const command = render(vector.command, r);
      const cwd = render(vector.cwd, r);
      const expect = expectFor(laneName, vector);
      (expected[vector.id] ??= Object.create(null))[laneName] = expect;
      const py = claudeResult(await batch.answer(at++));
      judge(`${laneName}/python ${vector.id}`, "python", vector, expect, py);
      // A DENIAL MUST BE IN ITS OWN LANE'S VOICE.
      //
      // Until 2026-09-07 a shim naming the wrong lane guarded visibly wrong
      // trees, and the deny/allow shape caught it. Now every lane protects the
      // product repos, so lane B's roots are a SUPERSET of lane A's — a lane-A
      // shim mislabelled as lane B denies everything lane A would and the shape
      // sees nothing. Measured that day: the `shim-lane-name` mutant survived.
      //
      // What still differs is what the denial TELLS the reader. `writeHere` is
      // where it says to write instead, and a brainer sent to another lane's
      // store is a real failure, so that is the discriminator now.
      // Collected per lane and judged once below, because not every deny
      // branch carries the redirect sentence (the copy-in diagnosis ends on the
      // destination operand instead). Asking it of every vector would fail a
      // correct file; asking it of the SET is exactly the property that matters.
      if (!py.crashed && !py.allowed) {
        for (const [other, cfg] of Object.entries(LANES)) {
          if (py.reason.includes(cfg.writeHere)) (voice[laneName] ??= new Set()).add(other);
        }
      }

      (shape[vector.id] ??= Object.create(null))[laneName] =
        py.crashed ? "crashed" : py.allowed ? "allow" : `deny:${category(py.reason)}`;
    }
  }

  // ── EVERY LANE DENIES IN ITS OWN VOICE ─────────────────────────────────
  //
  // Until 2026-09-07 a shim naming the wrong lane guarded visibly wrong trees
  // and the deny/allow SHAPE caught it. Now every lane protects the product
  // repos, so lane B's roots are a SUPERSET of lane A's: a lane-A shim
  // mislabelled as lane B denies everything lane A would, and the shape sees
  // nothing. Measured that day — the `shim-lane-name` mutant survived.
  //
  // What still differs is what the denial TELLS the reader: `writeHere`, the
  // store it sends them to instead. A brainer sent to another lane's store is
  // a real failure, so that is the discriminator the shape no longer provides.
  for (const laneName of laneNames) {
    const named = [...(voice[laneName] ?? [])];
    if (!named.includes(laneName)) {
      notOk(`${laneName} lane voice`,
        `no denial from ${laneName} named its own writeHere (${LANES[laneName].writeHere}); it named ${JSON.stringify(named)}. A shim that names the wrong lane now produces the same deny/allow shape, so this sentence is what is left to catch it`);
    } else if (named.length > 1) {
      notOk(`${laneName} lane voice`,
        `${laneName}'s denials named more than one lane's writeHere: ${JSON.stringify(named)}`);
    } else {
      ok(`${laneName} denies in its own voice (${LANES[laneName].writeHere})`);
    }
  }

  // ── CROSS-LANE LOGIC PARITY ────────────────────────────────────────────
  // Each lane was rendered with its OWN roots, so an identical decision shape
  // across all four is the claim "there is one implementation" — measured,
  // rather than inferred from the fact that the files import the same module.
  for (const vector of vectors) {
    const byLane = shape[vector.id];
    const shapes = [...new Set(Object.values(byLane))];
    // A LANE-SPECIFIC VECTOR DIFFERS WHERE THE TABLE MAKES IT DIFFER, and only
    // there. Until 2026-09-29 this demanded a difference unconditionally, which
    // is a claim about OUR lane table, not about the guard: the exported
    // `guards.json` grants `worktree_roots` to no lane, so every lane correctly
    // denies the teardown and the check failed a correct guard. Each lane's
    // own verdict is still asserted above against its policy row; this asks
    // that the lanes split exactly as far as those rows split.
    if (vector.lane_specific) {
      const wanted = new Set(Object.values(expected[vector.id])).size;
      if (wanted > 1 && shapes.length > 1) ok(`cross-lane ${vector.id} DIFFERS by lane, as declared (${JSON.stringify(byLane)})`);
      else if (wanted === 1 && shapes.length === 1) ok(`cross-lane ${vector.id} (${shapes[0]} in all ${laneNames.length} lanes: no lane's policy grants the difference)`);
      else notOk(`cross-lane ${vector.id}`,
        wanted > 1
          ? `declared lane_specific and the lane table grants the difference, but every lane agreed (${shapes[0]}) — the exemption is gone`
          : `no lane's policy grants a difference, yet the lanes disagree — ${JSON.stringify(byLane)}`);
      continue;
    }
    if (shapes.length === 1) ok(`cross-lane ${vector.id} (${shapes[0]} in all ${laneNames.length} lanes)`);
    else notOk(`cross-lane ${vector.id}`,
      `lanes disagree — ${JSON.stringify(byLane)}. Undeclared drift is what the eight-copy split cost; declare it with lane_specific or fix it`);
  }

  // ── A PAYLOAD OF THE WRONG SHAPE REFUSES; IT DOES NOT CRASH ─────────────
  // Measured 2026-09-29 (audit F12, probes M03-M05, M08): a payload that is not
  // an object, or a Bash payload whose `tool_input` is not one, raised
  // AttributeError OUTSIDE `main`'s try in the Claude hook, and exit 1 is a
  // NON-blocking error there — the command ran. The
  // runtime builds the payload, not the model, so the live risk is a vendor
  // changing its schema; a guard that cannot read one has not seen a safe
  // command. An empty or truncated stdin stays an allow (it names no tool),
  // and a tool that is not a shell stays none of this guard's business.
  {
    const laneDir = `${root}/setup`;
    const raw = (path, input, env, cwd) => spawnSync("python3", [path], { input, encoding: "utf8", env: env ?? process.env, cwd });
    const claudeVerdict = (r) => r.status !== 0 ? `crashed ${r.status}`
      : !r.stdout.trim() ? "allow"
        : /"permissionDecision": "deny"/.test(r.stdout) ? "deny" : `invalid ${r.stdout.trim()}`;
    const shapes = [
      ["a JSON array", "[]", "deny"],
      ["a JSON string", '"Bash"', "deny"],
      ["a Bash tool_input that is a string", JSON.stringify({ tool_name: "Bash", tool_input: "rm -rf x", cwd: outside }), "deny"],
      ["a Bash tool_input that is null", JSON.stringify({ tool_name: "Bash", tool_input: null, cwd: outside }), "deny"],
      ["a Bash tool_input that is a list", JSON.stringify({ tool_name: "Bash", tool_input: ["rm", "-rf", "x"], cwd: outside }), "deny"],
      ["an empty stdin (names no tool)", "", "allow"],
      ["a non-shell tool with a string tool_input", JSON.stringify({ tool_name: "Read", tool_input: "x", cwd: outside }), "allow"],
    ];
    for (const [name, input, want] of shapes) {
      const py = claudeVerdict(raw(`${laneDir}/.claude/hooks/deny-repo-writes.py`, input));
      tally("python", py === want);
      if (py === want) ok(`malformed payload, claude hook: ${name} -> ${want}`);
      else notOk(`malformed payload, claude hook: ${name} -> ${want}`, py);
    }
  }

  // ── THE SANDBOX ARM ────────────────────────────────────────────────────
  // Every tree a `"sandbox"` vector names is built here, under a HOME of its
  // own, from a policy written here: no verdict below depends on which roots
  // the machine running it happens to have. `sandbox: "exist"` builds the roots,
  // `"absent"` leaves them out, `"both"` runs the vector against each.
  //
  // POSIX only. A relative climb to `/` and a case-insensitive spelling of a
  // drive have no Windows twin here; Windows folds every path to one lowercase
  // spelling in the guard itself, and the vector says nothing new there.
  if (process.platform === "win32") {
    ok("sandbox arm: not run on win32 (the guard folds drive paths to one spelling there; the sandbox vectors are POSIX filesystem shapes)");
  } else {
    const lane = laneNames.find((n) => n !== "brain");
    if (!lane) throw new Error("guards.json names no lane besides brain — the sandbox arm has no non-root lane to drive");
    const realPolicy = JSON.parse(readFileSync(`${root}/guards.json`, "utf8"));
    const flip = (name) => name.replace(/[a-z]/gi, (c) => (c === c.toLowerCase() ? c.toUpperCase() : c.toLowerCase()));

    function buildSandbox(rootsExist) {
      const sb = realpathSync(mkdtempSync(join(base, "sb-")));
      const home = `${sb}/home`;
      const wsRoot = `${home}/Workspace`;
      const brainDir = `${wsRoot}/brain`;
      const repoName = basename(LANES.brain.repo);
      const repo = `${wsRoot}/${repoName}`;
      const worktrees = `${wsRoot}/worktrees`;
      const out = `${sb}/out`;
      mkdirSync(home);
      mkdirSync(out);
      if (rootsExist) { mkdirSync(repo, { recursive: true }); mkdirSync(worktrees); mkdirSync(brainDir); }
      const laneRow = (writeHere) => ({ repo, worktrees, write_here: writeHere, where: "the sandbox repo" });
      // THE ONE FILE A PRODUCT REPO OPENS: `<repo>/.claude/settings.local.json`, untracked. The
      // other repos are the shapes that must stay shut (vectors `local-settings-*`): one
      // that tracks the file, one whose `.claude` is a symlink, one whose file is a symlink or
      // a hard link into the tree, and one that is not a git checkout at all.
      const gitIn = (dir, ...args) => spawnSync("git", ["-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", ...args], { stdio: "ignore" });
      const extra = {};
      if (rootsExist) {
        const make = (name, build) => {
          const dir = `${wsRoot}/${name}`;
          mkdirSync(`${dir}/src`, { recursive: true });
          writeFileSync(`${dir}/src/a`, "a\n");
          build(dir);
          return dir;
        };
        const git = (dir) => { gitIn(dir, "init", "-q", "."); gitIn(dir, "add", "-A", "-f"); gitIn(dir, "commit", "-q", "-m", "i"); };
        const claude = (dir) => { mkdirSync(`${dir}/.claude`); writeFileSync(`${dir}/.claude/settings.json`, "{}\n"); writeFileSync(`${dir}/.claude/other.json`, "{}\n"); };
        mkdirSync(`${repo}/src`, { recursive: true });
        writeFileSync(`${repo}/src/a`, "a\n");
        claude(repo); git(repo);
        writeFileSync(`${repo}/.claude/settings.local.json`, "{}\n");
        extra.repo_tracked = make("tracked-repo", (d) => { claude(d); writeFileSync(`${d}/.claude/settings.local.json`, "{}\n"); git(d); });
        extra.repo_link = make("link-repo", (d) => { git(d); symlinkSync("src", `${d}/.claude`); });
        extra.repo_symfile = make("symfile-repo", (d) => { claude(d); git(d); symlinkSync("../src/a", `${d}/.claude/settings.local.json`); });
        extra.repo_hard = make("hard-repo", (d) => { claude(d); git(d); linkSync(`${d}/src/a`, `${d}/.claude/settings.local.json`); });
        extra.repo_nogit = make("nogit-repo", (d) => { claude(d); writeFileSync(`${d}/.claude/settings.local.json`, "{}\n"); });
      }
      writeFileSync(`${sb}/guards.json`, JSON.stringify({
        brain_root: brainDir,
        product_repos: [repo, ...Object.values(extra)],
        defaults: realPolicy.defaults,
        lanes: { brain: laneRow(`${brainDir}/<project>/`), [lane]: laneRow(`${brainDir}/${lane}/`) },
      }, null, 2));
      const place = (rel) => {
        mkdirSync(dirname(`${sb}/${rel}`), { recursive: true });
        copyFileSync(`${root}/${rel}`, `${sb}/${rel}`);
      };
      place("setup/guards/deny_repo_writes.py");
      place(`${lane}/.claude/hooks/deny-repo-writes.py`);
      // What a link to each protected shape looks like, all built OUTSIDE the
      // repo. `dl_*` dangle when the roots are absent and point at a directory
      // (or a not-yet-created child of one) when they exist; the verdict is the
      // same, which is the claim.
      const link = (name, target) => { symlinkSync(target, `${out}/${name}`); return `${out}/${name}`; };
      const mid = link("dl-chain-mid", `${repo}/x`);
      const climb = "../".repeat(out.split("/").filter(Boolean).length);
      // A link whose target spells a protected root in the WRONG CASE.
      const caseRepo = `${wsRoot}/${flip(repoName)}`;
      const r = {
        repo, worktrees, home, ...extra,
        ls: `${repo}/.claude/settings.local.json`,
        dl_root: link("dl-root", repo),
        dl_deep: link("dl-deep", `${repo}/a/b/c`),
        dl_chain: link("dl-chain", mid),
        dl_rel: link("dl-rel", `${climb}${repo.slice(1)}/x`),
        dl_outside: link("dl-outside", `${out}/absent-target`),
        case_link: link("case-link", caseRepo),
        case_repo: caseRepo,
        case_ancestor: `${home}/${flip("Workspace")}/${repoName}`,
        case_sibling: `${wsRoot}/${flip(repoName)}-deny-guard-sibling`,
        sibling: `${repo}-deny-guard-sibling`,
      };
      // Whether THIS filesystem treats two spellings as one. APFS and NTFS do,
      // ext4 does not, and a case-variant path means the protected tree only on
      // the first kind; on the second it is another, nonexistent path.
      mkdirSync(`${sb}/case-probe`);
      const caseInsensitive = existsSync(`${sb}/CASE-PROBE`);
      return { sb, brainDir, r, caseInsensitive };
    }

    const renderSb = (value, t) => value
      .replaceAll("{{repo}}", t.r.repo).replaceAll("{{worktrees}}", t.r.worktrees)
      .replaceAll("{{dl_root}}", t.r.dl_root).replaceAll("{{dl_deep}}", t.r.dl_deep)
      .replaceAll("{{dl_chain}}", t.r.dl_chain).replaceAll("{{dl_rel}}", t.r.dl_rel)
      .replaceAll("{{dl_outside}}", t.r.dl_outside).replaceAll("{{case_link}}", t.r.case_link)
      .replaceAll("{{case_repo}}", t.r.case_repo).replaceAll("{{case_ancestor}}", t.r.case_ancestor)
      .replaceAll("{{case_sibling}}", t.r.case_sibling).replaceAll("{{sibling}}", t.r.sibling)
      .replaceAll("{{ls}}", t.r.ls || "").replaceAll("{{repo_tracked}}", t.r.repo_tracked || "")
      .replaceAll("{{repo_link}}", t.r.repo_link || "").replaceAll("{{repo_symfile}}", t.r.repo_symfile || "")
      .replaceAll("{{repo_hard}}", t.r.repo_hard || "").replaceAll("{{repo_nogit}}", t.r.repo_nogit || "")
      .replaceAll("{{outside}}", outside).replaceAll("{{out}}", `${outside}/copy`);

    const realHome = process.env.HOME;
    try {
      for (const rootsExist of [false, true]) {
        const t = buildSandbox(rootsExist);
        const mode = rootsExist ? "roots exist" : "roots absent";
        const fsNote = t.caseInsensitive ? "case-insensitive fs" : "case-sensitive fs";
        const pyEnv = { ...process.env, HOME: t.r.home };
        process.env.HOME = t.r.home;
        for (const vector of sandboxVectors) {
          if (vector.sandbox !== "both" && vector.sandbox !== (rootsExist ? "exist" : "absent")) continue;
          const command = renderSb(vector.command, t);
          const cwd = renderSb(vector.cwd, t);
          const expect = !t.caseInsensitive && vector.expect_case_sensitive_fs ? vector.expect_case_sensitive_fs : vector.expect;
          const tag = `sandbox[${mode}, ${fsNote}] ${vector.id}`;
          judge(`${tag} (python)`, "python", vector, expect,
            python(`${t.sb}/${lane}/.claude/hooks/deny-repo-writes.py`, command, cwd, pyEnv));
        }
      }
    } finally {
      process.env.HOME = realHome;
    }
  }

  // ── AN UNSUPPORTED PLATFORM REFUSES; IT DOES NOT DECIDE ───────────────────
  //
  // windows-ultimos-070 removed the refusal that used to sit at this module's
  // import, and a Cygwin/MSYS Python — `os.name` posix, `sys.platform`
  // cygwin/msys — read a `C:\…` write payload as a relative path under its cwd
  // and ALLOWED it. MEASURED old/new on 2026-11-01: with the refusal removed
  // this block's python arm reads `allow`; with it restored, `deny`. The
  // forcing variable is no exception: it only folds spellings on a supported host.
  {
    const setupHook = `${root}/setup/.claude/hooks/deny-repo-writes.py`;
    const cygwinCommand = "echo x > 'C:\\work\\product\\HOLE.txt'";
    const payload = payloadOf(cygwinCommand, outside);
    const pyWrapper = `${base}/as-cygwin.py`;
    writeFileSync(pyWrapper, [
      "import os, runpy, sys",
      "hook = os.environ[\"CYGWIN_HOOK\"]",
      "sys.argv = [hook]",
      "sys.platform = \"cygwin\"",
      "runpy.run_path(hook, run_name=\"__main__\")",
      "",
    ].join("\n"));
    const cygwinPy = (extraEnv = {}) => claudeResult(realEntry({
      hook: pyWrapper, stdin: payload, cwd: base,
      env: { ...process.env, CYGWIN_HOOK: setupHook, ...extraEnv },
    }));
    const pyRefused = cygwinPy();
    if (!pyRefused.crashed && !pyRefused.allowed && /macOS and Linux only/.test(pyRefused.reason)) {
      ok("unsupported platform (python, sys.platform=cygwin): a C:\\ write is DENIED at import, naming macOS and Linux");
    } else {
      notOk("unsupported platform (python, sys.platform=cygwin)", `wanted a deny naming macOS/Linux, got ${verdictOf(pyRefused)}`);
    }
    const forced = cygwinPy({ DENY_REPO_WRITES_AS_NATIVE_WINDOWS: "1" });
    if (!forced.crashed && !forced.allowed && /macOS and Linux only/.test(forced.reason)) {
      ok("unsupported platform (python, sys.platform=cygwin) WITH DENY_REPO_WRITES_AS_NATIVE_WINDOWS=1: the import still refuses");
    } else {
      notOk("cygwin with the forcing variable set", `the variable must not lift the refusal, got ${verdictOf(forced)}`);
    }
  }

  for (const [runtime, t] of Object.entries(driverTally)) {
    console.log(`# driver ${runtime}: ${t.pass + t.fail} assertions, ${t.pass} passed, ${t.fail} failed`);
    if (t.pass + t.fail === 0) notOk(`driver ${runtime} ran no assertions`);
  }
} finally {
  rmSync(base, { recursive: true, force: true });
}

console.log(`\n${pass} passed, ${fail} failed`);
process.exitCode = fail ? 1 : 0;
