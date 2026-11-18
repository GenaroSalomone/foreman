# foreman 1.0.0-rc.4

foreman 1.0.0-rc.4 makes the consultant a standard: a strong model is spent only where a decision is open, and it answers in one shape.

## In short

- **The consultant is a brief, not a habit.** `kind: design` + `asked: investigate` + `decision:` runs fable at effort low, with no worktree, and its prompt asks for the verdict first, facts with file:line and one recommendation.
- **The top tier needs a reason outside a build.** Opus or fable on explore, audit or design is refused without `decision:` or `--model-why`; when hw picked the model itself, the refusal says so.
- **An investigation still gets the repo it requires.** `asked: investigate` drops the worktree only when the brief does not require `repo` or `database`.
- **A design that builds is a build.** `kind: design` with `asked: build` goes through the build ceiling.

## What changed

5 changes since 1.0.0-rc.3.

### Added
- The consultant is a standard hw holds: `kind: design` + `asked: investigate` +
  `decision:`. It defaults to fable at effort low (`effort:` in the brief raises
  it) and its prompt carries the delivery format (verdict first, facts with
  file:line, one recommendation, steps as one brief each, UNESTABLISHED marked).

### Changed
- `asked: investigate` defaults to `worktree: none` unless `requires:` names `repo` or `database`; `--worktree new` still wins
  and the manifest says which one decided.
- Opus or fable on `kind: explore`, `audit` or `design` is refused unless the
  brief has a `decision:` line or the dispatch has `--model-why` (`review` is
  exempt). A model hw cannot rank is not top tier.
- `kind: design` with `asked: build` goes through the build ceiling: opus needs
  `top_tier_why`. `design` + build keeps opus as its default.
- `fable` is ranked beside `opus`, so the manifest's suggestion note reads it.

## Known limits

- macOS and Linux only. Windows, native or under WSL2, is not supported or
  tested (KNOWN-LIMITATIONS, L1).
- The Go core is on by default when the installer builds it; `HW_IMPL=bash`
  goes back. Every limit a user can hit is in
  [`KNOWN-LIMITATIONS.md`](KNOWN-LIMITATIONS.md).

## Upgrading from 1.0.0-rc.3

Run `foreman-setup upgrade --brain DIR`. A brain installed before `upgrade`
existed has no record of its flags yet: run your install command once more,
and `upgrade` works from then on.

Every release's changes are in [`CHANGELOG.md`](CHANGELOG.md).
