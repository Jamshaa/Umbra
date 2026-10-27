# Umbra UI

Pixel-art interface for the Umbra remediation operator: findings, sandbox
diffs, agent tool-call timeline. React 19 + Vite 7, **no component library**,
**no Tailwind** — plain CSS modules-by-file driven by the design tokens in
[`src/styles/base.css`](src/styles/base.css).

## Run

```bash
cd ui
npm install
npm run dev      # http://localhost:5173
npm run build    # static bundle in dist/ (base './' → relocatable)
```

## Data source policy

Everything is fed from [`src/mock.js`](src/mock.js) — **explicitly labeled
placeholder data** (namespaces `demo`, registry `ghcr.io/example`). The mock
layer is the seam: when the operator exposes the `Finding` CRDs and status API:

1. replace `mock.js` imports with React Query queries (poll CRD status),
2. replace the static `TIMELINE` with the native WebSocket timeline feed.

No network detail is hardcoded here — pointing the client at a cluster
endpoint is a one-line config change which **must not** be committed with a
real value (see [SECURITY.md](../SECURITY.md) §4).

## Design contract (do not soften)

- Palette: exactly the 8 tokens in `base.css`. No gradients, no blur, no
  `border-radius`, transitions of 0 ms.
- Hard shadows: `4px 4px 0 var(--hard-shadow)`; pressed state =
  `translate(2px, 2px)` + `2px` shadow.
- 8px spacing grid for margins/padding/gaps (`--sp-*`).
- Fonts: **Press Start 2P** only at 8px multiples (labels/buttons — never
  body copy), **VT323** 20–22px for body, **DM Mono** 12–14px for code/YAML.
- Scanline overlay: 2px lines @ 5% opacity (the one sanctioned transparency
  pattern), blinking cursor via `steps(1)`.

## Layout

Three columns (stack vertically under 900px):

| Column | Content | Source |
|---|---|---|
| FINDINGS | severity-filtered finding cards, SCAN button | CRD list (mocked) |
| DETAIL | live manifest + sandbox diff + approve-to-prod gate (disabled until verification passed) | CRD status (mocked) |
| AGENT TIMELINE | collapsible tool-call feed with status-colored rails | WebSocket (mocked) |

The **Approve → prod** button stays disabled unless the finding is in
`verified` state — that is the human approval gate, rendered at the UI layer
too. It is intentionally *not* clickable in the alpha: approval actions bind
to the operator's approval workflow once it exists.
