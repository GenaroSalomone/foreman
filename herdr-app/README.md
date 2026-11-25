# herdr-app: the foreman cockpit in the Herdr app

This export carries one mod for [the Herdr app](https://herdr.dev): **Cockpit**, `mods/foreman-cockpit/`. It draws the
same state file the Claude Code cockpit reads (`bin/cockpit-state`, contract in [`cockpit/schema.json`](../cockpit/schema.json)).
It reads that file and changes nothing in it. It ships no layout, theme or other panel.

## What it gives

- One section per live brainer, named by its lane, with a card per executor: task, status and how long it has held it,
  vendor, model and effort, context percentage, commits, and what waits on the brainer (an ask, a challenge, a blocked
  report, a queued ruling, no progress).
- A queue above the sections of everything that waits on you, oldest first, and a filter (all, needs you, working, done).
- Buttons on a queue row, run through [`cockpit/verbs.txt`](../cockpit/verbs.txt): **Reply** to an ask, **Rule** on a
  challenge or a blocked report, **Done** on a report (it asks to confirm first).
- A state file older than 15 s is marked `stale`; older than 60 s, unreadable or of another schema, it shows `NO STATE`.

## Install

`install.sh` does not install it. From your brain (the folder you gave `--brain`; `~/brain` below):

```sh
mkdir -p ~/.config/herdr-app/mods
ln -sfn ~/brain/herdr-app/mods/foreman-cockpit ~/.config/herdr-app/mods/foreman-cockpit
herdr-app check ~/.config/herdr-app/mods/foreman-cockpit   # must pass
```

The mod has `"place": "none"`: it appears only where a layout puts it. Copy a layout from `~/.config/herdr-app/layouts/`
and add this block where you want the cockpit:

```json
{ "mod": "foreman-cockpit", "props": { "dir": "~/work/.cockpit", "verbs": "~/brain/cockpit/verbs.txt" } }
```

`dir` is the `.cockpit` folder of your work root (`bin/cockpit-state --print-path` prints one writer's file inside it);
without `verbs` the rows have no buttons. Then `herdr-app do layout.switch '{"name":"<your layout>"}'`, or
`cmd-shift-l` to cycle. If a layout breaks the window, `ctrl-alt-shift-escape` restores `main`.

## Update

`foreman-setup upgrade --brain ~/brain` refreshes the brain; the symlink points into it, so the app shows the new mod
on its own (the app reloads mods live).

## Remove

```sh
rm ~/.config/herdr-app/mods/foreman-cockpit
```

and delete the block from your layout. Nothing else was written outside the brain.
