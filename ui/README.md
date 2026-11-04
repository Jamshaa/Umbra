# Umbra UI

Pixel-art interface for the Umbra remediation operator: findings, sandbox
diffs, agent tool-call timeline. React 19 + Vite 7, **no component library**,
**no Tailwind** — two plain CSS files: tokens and primitives in
[`src/styles/base.css`](src/styles/base.css), layout in [`src/App.css`](src/App.css).

## Run

```bash
cd ui
npm install
npm run dev      # http://localhost:5173
npm run build    # static bundle in dist/ (base './' → relocatable)
```

## Preview a production build (do NOT open dist/ by double-click)

`dist/index.html` opened as `file://` stays blank: ES modules are blocked
by the browser on `file://`. Serve the directory over HTTP instead —
any static server works, e.g. from the repo root:

```bash
cd ui/dist && python3 -m http.server 8901  # → http://127.0.0.1:8901/
```

or serve it through the operator API (same bundle, plus live data):

```bash
go run ./cmd/umbra-api        # serves ui/dist at http://127.0.0.1:8082/
```

With `VITE_UMBRA_API=live npm run build`, the bundle fetches
`GET /api/findings` from the same origin (see "Data source policy").

## Data source policy

Everything is fed through [`src/api.js`](src/api.js) — **explicitly labeled
placeholder data** by default (namespaces `demo`, registry `ghcr.io/example`
from [`src/mock.js`](src/mock.js)). Set `VITE_UMBRA_API=live` and `api.js`
switches to `GET /api/findings` on the same origin (served by umbra-api);
malformed entries are dropped, and an unreachable API degrades to mock
with a notice instead of a dead screen. The remaining seams, when the
operator backend lands:

1. React Query polling can replace the one-shot `loadFindings()` call,
2. replace the static `TIMELINE` with the native WebSocket timeline feed.

No network detail is hardcoded here — pointing the client at a cluster
endpoint is a one-line config change which **must not** be committed with a
real value (see [SECURITY.md](../SECURITY.md) §4).

## Design contract (do not soften)

- Palette: the 9 tokens in `base.css` (`--bg --panel --ink --muted --sel`
  plus `--error --ok --warn --info` as status colours only), in two themes:
  **paper** (default) and **night**. The OS preference picks one; the `t`
  key / header button pins it (stored in `localStorage`, optional).
- Edges, not boxes: 2px solid or dotted rules separate sections; no
  shadows, no blur, no gradients, no `border-radius`, no transitions.
  Buttons get 2px pixel-notched corners (`clip-path`) and drop 2px when
  pressed.
- Spacing on a 4px grid.
- Fonts: **Press Start 2P** only at 8px (labels, buttons) and 16px (logo),
  always lowercase; **VT323** 20px body, 40px subject; **DM Mono** 12–13px
  for ids, code, YAML and commands.
- Sprites are 1-bit SVG paths (`Sprite.jsx`, `crispEdges`), tinted by
  `currentColor`.

## Layout

Three columns (stack under 1000px), bench table and key legend below:

| Column | Content | Source |
|---|---|---|
| findings | filter (id, target, kind, state), severity filter, sort by severity or id, finding rows | `/api/findings` or mock |
| detail | subject, fix/manifest tabs with copy and save, link to the finding, gate verdict, approval command | finding + its `gate_verdict` timeline entry |
| agent | tool-call timeline (mock is labelled "simulation") | finding timeline or mock |

Keys: `j`/`k` or arrows move, `/` filters, `Esc` clears the filter then
leaves it, `1`/`2` switch tabs, `c` copies the approval command, `s` flips
the sort, `r` rescans, `t` flips the theme. The legend hides on touch
screens. The selected finding lives in the URL hash (`#F03`), so a link
opens on it and survives rescans.

The top bar shows the time of the last scan. In live builds `auto`
rescans every 15s. The tab title carries the high-severity count.

The production gate is display-only: the API is read-only by design, so
approval is a `kubectl annotate` command the human copies and runs,
labelled "human step · not executed by umbra". The verdict, and the
"proposed by <model>" line under it, come only from the newest
`gate_verdict` timeline entry; with no model recorded the line is omitted.
