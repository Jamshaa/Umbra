import { MOCK_FINDINGS } from './mock.js'

// Data-source seam for the findings column.
//
// Default (VITE_UMBRA_API unset or anything but "live"): the mock
// fixtures from mock.js — explicitly labeled placeholder data.
// Live (VITE_UMBRA_API=live, served by umbra-api on the same origin):
// GET /api/findings must return a JSON array of findings shaped like
// {id, severity, state, kind, target, liveYaml, diffRemoved, diffAdded}.
// Malformed entries are dropped, never rendered. The live call never
// carries credentials or cluster addresses (same-origin relative URL).
const LIVE = import.meta.env.VITE_UMBRA_API === 'live'

export function normalize(list) {
  if (!Array.isArray(list)) return []
  const out = []
  for (const f of list) {
    if (!f || typeof f.id !== 'string' || typeof f.kind !== 'string' || typeof f.target !== 'string') continue
    out.push({
      id: f.id,
      severity: typeof f.severity === 'string' ? f.severity : 'medium',
      state: typeof f.state === 'string' ? f.state : 'detected',
      kind: f.kind,
      target: f.target,
      liveYaml: typeof f.liveYaml === 'string' ? f.liveYaml : '',
      diffRemoved: Array.isArray(f.diffRemoved) ? f.diffRemoved.filter((l) => typeof l === 'string') : [],
      diffAdded: Array.isArray(f.diffAdded) ? f.diffAdded.filter((l) => typeof l === 'string') : [],
    })
  }
  return out
}

export async function loadFindings() {
  if (!LIVE) return { source: 'mock', findings: MOCK_FINDINGS, notice: '' }
  const res = await fetch('/api/findings', { headers: { Accept: 'application/json' } })
  if (!res.ok) throw new Error(`GET /api/findings -> ${res.status}`)
  return { source: 'live', findings: normalize(await res.json()), notice: '' }
}

export { MOCK_FINDINGS }
