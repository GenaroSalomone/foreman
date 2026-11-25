// The Cockpit's buttons: which verb a queue row offers, the argv that verb runs, and what its outcome says.
// Pure (actions.test.mjs): `exec` is handed in, nothing here imports the app.
//
// The verb table is cockpit/verbs.txt (contract v1) read at run time (`parseVerbs`): there is no second copy of
// it in this file. A verb the file does not list is not callable. The app's exec runs no shell and has no stdin,
// so the text a person types is ONE argv word that replaces the template's last, lone `-` (the contract's header
// comment says so); a word that would be read as a flag is refused, never rewritten.
import { thrown } from "./thrown.js";

// A command that finishes within this is a verb; `hw done` verifies and merges nothing, but channel-send may wait
// ~95 s for a lock and ~90 s for the idle gate (core/channel_send_help.txt): never cut it sooner than the app allows.
export const VERB_TIMEOUT_MS = 600000;

const COLUMNS = 4;

// verbs.txt's text → { verb: { argv: ["hw","done","<project>","<task>"], confirm: bool, text: bool } }, or null
// when the file is not the contract (no verbs, a malformed line): the buttons are then off, never guessed.
export function parseVerbs(text) {
  if (typeof text !== "string") return null;
  const table = {};
  for (const raw of text.split("\n")) {
    const line = raw.trim();
    if (line === "" || line.startsWith("#")) continue;
    const cols = line.split(" | ").map((c) => c.trim());
    if (cols.length !== COLUMNS) return null;
    const [verb, template, confirm, stdin] = cols;
    if (!/^[a-z][a-z-]*$/.test(verb) || template === "" || !/^(hw|channel-send)\s/.test(template) || !/^confirm=(yes|no)$/.test(confirm) || !/^stdin=(no|text)$/.test(stdin)) return null;
    const argv = template.split(/\s+/);
    const text_ = stdin === "stdin=text";
    if (text_ && argv[argv.length - 1] !== "-") return null; // the lone `-` is where the text goes
    table[verb] = { argv, confirm: confirm === "confirm=yes", text: text_ };
  }
  return Object.keys(table).length === 0 ? null : table;
}

// What a row offers: [{ id, label, verb, params, text, confirm }], only verbs the table lists. A reply needs the hold
// file hw wrote for that ask (`ask.pending_reply`, null when none is valid); `ruling` only where hw says it would queue.
export function offers(row, table) {
  if (table === null || table === undefined) return [];
  const out = [];
  const add = (label, verb, params) => {
    const spec = table[verb];
    if (spec) out.push({ id: verb, label, verb, params, text: spec.text, confirm: spec.confirm });
  };
  const ask = row.ask ?? null;
  const hold = ask !== null && typeof ask.pending_reply === "string" && ask.pending_reply !== "" ? ask.pending_reply : null;
  const route = ask?.route === "hc" ? "hc" : "herdr"; // the word channel-send takes first; a writer older than the field says herdr
  if (row.attention === "ask" && ask?.kind === "ask" && hold !== null) add("Reply", "reply", { pane: row.pane, pending_reply: hold, route });
  if (row.attention === "challenge" && ask?.kind === "challenge" && hold !== null) add("Rule", "challenge-reply", { pane: row.pane, pending_reply: hold, route });
  if (row.attention !== "ask" && row.attention !== "challenge" && row.actions?.ruling === true) add("Rule", "ruling", { pane: row.pane });
  if (row.attention === "report" && row.actions?.done === true) add("Done", "done", { project: row.project, task: row.task });
  return out;
}

// The argv a verb runs: the template with every `<name>` replaced by exactly one element of `params`, and for a
// verb that takes text, the text as the ONE last element in place of the lone `-`. Throws on a missing parameter.
export function argvOf(spec, params, text) {
  const argv = spec.argv.map((w) => {
    const m = /^<([a-z_]+)>$/.exec(w);
    if (m === null) return w;
    const v = params[m[1]];
    if (typeof v !== "string" || v === "") throw new Error(`the row has no ${m[1]}`);
    if (v.startsWith("-")) throw new Error(`the row's ${m[1]} begins with “-” and would be read as an option`); // a state file is read, not created here
    return v;
  });
  if (spec.text) argv[argv.length - 1] = text;
  return argv;
}

// Why this text cannot be sent as it is, or null. `hw ruling` reads any word beginning with `-` as an option
// ("unknown option"), channel-send only a lone `-` (its message slot is positional: it means "read stdin", and the
// app has none). Measured against core/t3_ruling.go and core/channel_send.go; the message names what to change.
export function textRefusal(spec, text) {
  if (typeof text !== "string" || text.trim() === "") return "Nothing to send: the text is empty.";
  if (spec.argv[0] === "hw" && text.startsWith("-")) return "Not sent: hw reads a text that begins with “-” as an option. Start it with a word.";
  if (text === "-") return "Not sent: a lone “-” means “read stdin” to channel-send. Write a word.";
  return null;
}

// Where `hw` and `channel-send` are: the `bin/` beside the `cockpit/` folder of verbs.txt (`<brain>/cockpit/verbs.txt` →
// `<brain>/bin`), or null when the path is not that shape. The app's exec resolves a bare name on the PATH of the LOGIN
// shell it starts, and the brain's `bin/` is put on it by `.zshrc` (interactive only): measured 2026-11-12 `zsh -l -c 'command -v hw'`
// prints nothing, so every verb died with «cannot run hw: No such file or directory». The path is the brain's own, never the PATH.
export const binDirOf = (verbsPath) => (typeof verbsPath === "string" && /\/cockpit\/verbs\.txt$/.test(verbsPath) ? verbsPath.replace(/\/cockpit\/verbs\.txt$/, "/bin") : null);

const tail = (r) => String(r.stderr || r.stdout || "").trim();

// Runs one verb. Returns { state, text } and calls exec only when every guard passed:
//   refused  nothing ran (unconfirmed, no text, a flag-like text, a row without what the argv needs);
//   sent     exit 0 — the card is NOT cleared: only the next state read says the change happened (`settled`);
//   unknown  the call was cut off in flight (handoff.js's `thrown`), or channel-send's exit 5: it may have landed;
//   failed   any other exit, shown verbatim; the row stays.
export async function dispatch(table, verb, params, { text = null, confirmed = false, bin = null } = {}, exec) {
  const spec = table?.[verb];
  if (!spec) return { state: "refused", text: `Not sent: “${verb}” is not a verb of cockpit/verbs.txt.` };
  if (spec.confirm && confirmed !== true) return { state: "refused", text: "Not run: it needs a confirmation." };
  if (spec.text) {
    const why = textRefusal(spec, text);
    if (why !== null) return { state: "refused", text: why };
  }
  let argv;
  try {
    argv = argvOf(spec, params, text);
  } catch (e) {
    return { state: "refused", text: `Not run: ${e.message}.` };
  }
  let r;
  try {
    r = await exec(bin ? `${bin}/${argv[0]}` : argv[0], argv.slice(1), { timeout: VERB_TIMEOUT_MS }); // argv[0] stays the bare name in every message
  } catch (e) {
    return thrown(e);
  }
  if (r.code === 0) return { state: "sent", text: "Sent. Waiting for the next state read.", out: String(r.stdout ?? "") };
  const says = tail(r);
  const shown = says === "" ? `exit ${r.code}` : says;
  if (r.code === 5 && argv[0] === "channel-send") return { state: "unknown", text: `Unknown (exit 5): ${shown}` };
  return { state: "failed", text: `${argv[0]} ${argv[1] ?? ""} exited ${r.code}: ${shown}` };
}

// Has the next state read shown the change? `before` is what the row held when the verb ran; a row that is gone, or
// that now waits on something else, settles it. The same ask (same attention and seq) is still unanswered.
export const snapshot = (row) => ({ attention: row.attention, seq: row.ask?.seq ?? null, report: row.report?.at ?? null, queued: row.rulings_pending?.count ?? 0 });
export function settled(before, row) {
  if (row === undefined || row === null) return true;
  const now = snapshot(row);
  return now.attention !== before.attention || now.seq !== before.seq || now.report !== before.report || now.queued !== before.queued;
}

// Whether a row's button state is finished with: its row is gone from a live file; it waits (`sent`) and the state now shows the
// change; or it holds a field or question for a verb the row no longer offers (it moved from blocked to ask, a hold went stale),
// which would draw nothing and no button. The row is checked first: a gone row offers nothing to ask.
export function forgettable(act, row, table) {
  if (row === undefined || row === null) return true;
  if (act.mode === "sent") return settled(act.before, row);
  return (act.mode === "compose" || act.mode === "confirm") && !offers(row, table).some((o) => o.id === act.verb);
}
