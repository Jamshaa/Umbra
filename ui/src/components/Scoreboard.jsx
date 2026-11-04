import { useEffect, useState } from 'react'

function pickRows(json) {
  if (!json || typeof json !== 'object' || !Array.isArray(json.rows)) return null
  const rows = []
  for (const r of json.rows) {
    const pass = Number(r?.pass)
    const total = Number(r?.cases)
    const fail = Number(r?.fail)
    const config = typeof r?.config === 'string' && r.config !== '' ? r.config : null
    const group = typeof r?.group === 'string' && r.group !== '' ? r.group : null
    // Reject malformed rows outright: an unchecked key would let two
    // different rows collide on "undefined|undefined".
    if (!Number.isFinite(pass) || !Number.isFinite(total) || config === null || group === null) return null
    const live = r?.live === true
    const commit = typeof r?.commit === 'string' && r.commit !== '' ? r.commit : 'unknown'
    rows.push({
      key: `${config}|${group}|${live ? 'live' : 'static'}|${commit}`,
      config,
      group,
      live,
      commit,
      pass,
      total,
      fail: Number.isFinite(fail) ? fail : Math.max(0, total - pass),
    })
  }
  return rows.length > 0 ? rows : null
}

function Blocks({ pass, total, fail }) {
  const n = Math.min(24, Math.max(0, total))
  const okN = total > 0 ? Math.round((pass / total) * n) : 0
  const failN = total > 0 ? Math.min(n - okN, Math.round((fail / total) * n)) : 0
  return (
    <span className="blocks" aria-hidden="true">
      {Array.from({ length: okN }, (_, i) => (
        <span key={`p${i}`} className="blk" />
      ))}
      {Array.from({ length: failN }, (_, i) => (
        <span key={`f${i}`} className="blk fail" />
      ))}
      {Array.from({ length: Math.max(0, n - okN - failN) }, (_, i) => (
        <span key={`s${i}`} className="blk skip" />
      ))}
    </span>
  )
}

/**
 * Bench scoreboard: fetches scoreboard.json from the same origin (no
 * credentials) and renders pass/total per harness config. On fetch failure
 * or invalid JSON it shows the "run the bench" note and nothing else —
 * never invented numbers, never mock data.
 */
export default function Scoreboard() {
  const [rows, setRows] = useState(null)
  const [tick, setTick] = useState(0)
  useEffect(() => {
    let live = true
    fetch('/scoreboard.json', {
      credentials: 'omit',
      headers: { Accept: 'application/json' },
    })
      .then((res) => {
        if (!res.ok) throw new Error(`GET scoreboard.json -> ${res.status}`)
        return res.json()
      })
      .then((json) => {
        if (live) setRows(pickRows(json))
      })
      .catch(() => {
        if (live) setRows(null)
      })
    return () => { live = false }
  }, [tick])

  return (
    <section className="board" aria-labelledby="board-h">
      <header className="sec-head">
        <h2 id="board-h" className="px">bench</h2>
        <button type="button" className="btn ghost" onClick={() => setTick((t) => t + 1)}>reload</button>
      </header>
      {rows === null ? (
        <p className="muted">no measurements — run make bench</p>
      ) : (
        <div className="board-scroll">
          <table>
            <thead>
              <tr>
                <th scope="col">config</th>
                <th scope="col">group</th>
                <th scope="col">mode</th>
                <th scope="col">result</th>
                <th scope="col"><span className="sr">blocks</span></th>
                <th scope="col">commit</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.key}>
                  <td>{r.config}</td>
                  <td>{r.group}</td>
                  <td className={r.live ? 'st-verified' : 'muted'}>{r.live ? 'live' : 'static'}</td>
                  <td>{r.pass}/{r.total}</td>
                  <td><Blocks pass={r.pass} total={r.total} fail={r.fail} /></td>
                  <td className="muted code-font">{r.commit}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
