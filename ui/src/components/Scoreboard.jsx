import { useEffect, useState } from 'react'
import './Scoreboard.css'


function pickRows(json) {
  if (!json || typeof json !== 'object' || !Array.isArray(json.rows)) return null
  const rows = []
  for (const r of json.rows) {
    const pass = Number(r?.pass)
    const total = Number(r?.cases)
    if (!Number.isFinite(pass) || !Number.isFinite(total)) return null
    const group = typeof r?.group === 'string' && r.group ? ` · ${r.group}` : ''
    const live = r?.live === true ? ' (vivo)' : ' (estático)'
    rows.push({ key: `${r?.config}|${r?.group}`, config: `${r?.config}${group}${live}`, pass, total })
  }
  return rows.length > 0 ? rows : null
}

/**
 * Bench scoreboard: fetches scoreboard.json from the same origin (no
 * credentials) and renders pass/total per harness config. On fetch failure
 * or invalid JSON it shows the "run the bench" note and nothing else —
 * never invented numbers, never mock data.
 */
export default function Scoreboard() {
  const [rows, setRows] = useState(null)
  useEffect(() => {
    let live = true
    fetch('scoreboard.json', {
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
  }, [])

  return (
    <section className="scoreboard">
      <h2 className="column-title">SCOREBOARD</h2>
      {rows === null ? (
        <p className="empty-note">sin mediciones — ejecuta make bench</p>
      ) : (
        <table className="scoreboard-table">
          <thead>
            <tr>
              <th scope="col">config</th>
              <th scope="col">pass/total</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.key}>
                <td>{r.config}</td>
                <td className="scoreboard-score">{r.pass}/{r.total}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}
