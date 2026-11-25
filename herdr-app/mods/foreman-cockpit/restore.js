// The app's start brings the last brainers back (reboot-restore-brainers).
//
// After a reboot macOS relaunches the app at login and the app starts its core, which gives every kept terminal a shell:
// brain-<lane> is there, its claude is not. After a quit the core and its programs kept running. The app's start is the one
// hook that follows both, so the cockpit's body runs `<brain>/bin/brain restore --on-start` when it starts. brain decides the
// rest: it detaches at once (this exec only starts it), and the detached half restores once per core holder — the process that
// keeps terminals alive across quits — so a quit, a layout switch or a reload of this mod finds nothing to do. Its log is
// <cache>/hw/brain-restore.log. `brain` is named by path: the exec's PATH (the login shell's) does not carry the brain's bin/.
import { binDirOf } from "./actions.js";

// restoreOnStart runs `<bin>/brain restore --on-start`, <bin> beside the cockpit/ of props.verbs (binDirOf). Returns
// { state: "started" | "skipped" | "failed", text }: skipped when the block names no verbs file (no bin/ to find). The outcome
// is logged through log (main.js passes the app's console.log): a start that failed is otherwise invisible, and nothing re-runs it until the
// app starts again.
export async function restoreOnStart(verbsPath, homeDir, exec, log) {
  const r = await outcome(verbsPath, homeDir, exec);
  log(`foreman-cockpit: brain restore --on-start ${r.state}${r.text ? `: ${r.text}` : ""}`);
  return r;
}

async function outcome(verbsPath, homeDir, exec) {
  const path = typeof verbsPath === "string" && verbsPath.startsWith("~/") ? `${homeDir}/${verbsPath.slice(2)}` : verbsPath;
  const bin = binDirOf(path);
  if (!bin) return { state: "skipped", text: "no props.verbs naming <brain>/cockpit/verbs.txt: the brain's bin/ is unknown" };
  try {
    const r = await exec(`${bin}/brain`, ["restore", "--on-start"], { timeout: 30000 });
    const text = String(r.stdout || r.stderr || "").trim();
    return { state: r.code === 0 ? "started" : "failed", text };
  } catch (e) {
    return { state: "failed", text: String(e?.message ?? e) };
  }
}
