// The send could not return an exit code. A timeout or a kill leaves it in flight (channel-send itself says exit 5
// there): unknown. Anything else (it never started) is not delivered.
export function thrown(e) {
  const s = String(e && e.message ? e.message : e);
  return /time(d)?[ -]?out|killed|signal/i.test(s)
    ? { state: "unknown", text: "Unknown: the send was cut off in flight; the brainer may have it. Look in its pane before sending again." }
    : { state: "failed", text: `Not delivered: ${s.slice(0, 120)}` };
}
