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
      timeline: normalizeTimeline(f.timeline),
    })
  }
  return out
}

// Backend tool-call feed per finding: [{tool, args, result, modelVersion, ts}].
// String-or-string-lines coercion, same spirit as diffRemoved/diffAdded.
// Missing or invalid → [] (the UI then shows the labeled mock fallback).
function normalizeTimeline(timeline) {
  if (!Array.isArray(timeline)) return []
  const out = []
  for (const e of timeline) {
    if (!e || typeof e !== 'object') continue
    const tool = textField(e.tool)
    if (tool === '') continue
    out.push({
      tool,
      args: textField(e.args),
      result: textField(e.result),
      modelVersion: textField(e.modelVersion),
      ts: textField(e.ts),
      // Live entries carry no status: a recorded call completed.
      status: 'success',
      defaultOpen: false,
    })
  }
  return out
}

function textField(v) {
  if (typeof v === 'string') return v
  if (Array.isArray(v)) return v.filter((l) => typeof l === 'string').join('\n')
  return ''
}

export async function loadFindings() {
  if (!LIVE) return { source: 'mock', findings: MOCK_FINDINGS }
  const res = await fetch('/api/findings', { headers: { Accept: 'application/json' } })
  if (!res.ok) throw new Error(`GET /api/findings -> ${res.status}`)
  return { source: 'live', findings: normalize(await res.json()) }
}

export { MOCK_FINDINGS }
