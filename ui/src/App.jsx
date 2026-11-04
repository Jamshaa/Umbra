import { useEffect, useRef, useState } from 'react'
import Sprite from './components/Sprite.jsx'
import TimelineEntry from './components/TimelineEntry.jsx'
import Scoreboard from './components/Scoreboard.jsx'
import { DiffView, copyText, diffText } from './components/CodePanel.jsx'
import { MOCK_FINDINGS, DIFF_CONTEXT, TIMELINE } from './mock.js'
import { loadFindings, LIVE } from './api.js'
import './App.css'

const SEVERITIES = ['all', 'high', 'medium', 'low']
const SEV_RANK = { high: 0, medium: 1, low: 2 }
const POLL_MS = 15000
const STATE_LABEL = {
  detected: 'detected',
  analyzing: 'analyzing',
  proposed: 'fix proposed',
  verified: 'verified',
  reverted: 'reverted',
}

function initialTheme() {
  try {
    const t = localStorage.getItem('umbra-theme')
    if (t === 'paper' || t === 'night') return t
  } catch {
    // storage blocked: fall through to the OS preference
  }
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'night' : 'paper'
}

function hashId() {
  try {
    return decodeURIComponent(window.location.hash.slice(1))
  } catch {
    return '' // malformed %-escape in a hand-edited URL
  }
}

/** Hand the browser a text file; the Blob never leaves the page. */
function save(name, text) {
  const url = URL.createObjectURL(new Blob([text], { type: 'text/plain' }))
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}

const clock = (d) => d.toLocaleTimeString([], { hour12: false })

/**
 * Umbra UI. Findings come from api.js (mock unless VITE_UMBRA_API=live).
 * The UI is read-only: approval is a kubectl command a human runs.
 */
export default function App() {
  const [findings, setFindings] = useState([])
  const [selectedId, setSelectedId] = useState(null)
  const [severityFilter, setSeverityFilter] = useState('all')
  const [query, setQuery] = useState('')
  const [tab, setTab] = useState('fix')
  const [notice, setNotice] = useState('loading findings…')
  const [source, setSource] = useState('mock')
  const [copied, setCopied] = useState('')
  const [theme, setTheme] = useState(initialTheme)
  const [sort, setSort] = useState('severity')
  const [auto, setAuto] = useState(false)
  const [scanning, setScanning] = useState(false)
  const [scannedAt, setScannedAt] = useState(null)
  const searchRef = useRef(null)
  const detailRef = useRef(null)
  // Monotonic scan id: only the latest refresh may write state, so fast
  // repeated scans can never leave stale results on screen.
  const scanGen = useRef(0)

  // Keep the current selection across rescans; else the URL hash; else the first.
  const pick = (list) => (prev) => {
    const h = hashId()
    const has = (id) => list.some((f) => f.id === id)
    return has(prev) ? prev : has(h) ? h : list[0]?.id ?? null
  }
  const applyLoaded = ({ source: src, findings: list }) => {
    setFindings(list)
    setSelectedId(pick(list))
    setSource(src)
    setScanning(false)
    setScannedAt(new Date())
    setNotice(src === 'live' ? '' : 'mock data — build with VITE_UMBRA_API=live for the operator API')
  }
  const applyFallback = () => {
    // Live API unreachable: degrade to mock instead of a dead screen.
    setFindings(MOCK_FINDINGS)
    setSelectedId(pick(MOCK_FINDINGS))
    setSource('mock')
    setScanning(false)
    setScannedAt(new Date())
    setNotice('live API unreachable — showing mock data')
  }
  const refresh = () => {
    setScanning(true)
    const gen = ++scanGen.current
    loadFindings().then(
      (r) => { if (gen === scanGen.current) applyLoaded(r) },
      () => { if (gen === scanGen.current) applyFallback() },
    )
  }
  const select = (id, reveal = false) => {
    setSelectedId(id)
    window.history.replaceState(null, '', `#${encodeURIComponent(id)}`)
    // Stacked layout: the detail sits below the list, so bring it into view.
    if (reveal && window.matchMedia('(max-width: 1000px)').matches) detailRef.current?.scrollIntoView()
  }
  const refreshRef = useRef(refresh)
  refreshRef.current = refresh

  useEffect(() => refreshRef.current(), [])

  // Live builds only: optional polling; a failed poll falls back to mock
  // and the next tick tries the API again.
  useEffect(() => {
    if (!auto) return undefined
    const id = setInterval(() => refreshRef.current(), POLL_MS)
    return () => clearInterval(id)
  }, [auto])

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    try {
      localStorage.setItem('umbra-theme', theme)
    } catch {
      // storage blocked: theme just won't persist
    }
  }, [theme])

  useEffect(() => {
    setCopied('')
  }, [selectedId, tab])

  const q = query.trim().toLowerCase()
  const visible = findings
    .filter(
      (f) =>
        (severityFilter === 'all' || f.severity === severityFilter) &&
        (q === '' || `${f.id} ${f.target} ${f.kind} ${f.state}`.toLowerCase().includes(q)),
    )
    .sort((a, b) =>
      sort === 'severity'
        ? (SEV_RANK[a.severity] ?? 3) - (SEV_RANK[b.severity] ?? 3) || a.id.localeCompare(b.id, undefined, { numeric: true })
        : a.id.localeCompare(b.id, undefined, { numeric: true }),
    )
  const countFor = (s) => (s === 'all' ? findings.length : findings.filter((f) => f.severity === s).length)
  const selected = findings.find((f) => f.id === selectedId) ?? null
  const highCount = countFor('high')

  useEffect(() => {
    document.title = highCount > 0 ? `(${highCount}) umbra` : 'umbra'
  }, [highCount])

  // Gate verdict comes ONLY from the newest gate_verdict timeline entry
  // written by diagnose --publish. Never derived from state or diffs.
  const gateEntry = selected?.timeline
    ? [...selected.timeline].reverse().find((e) => e.tool === 'gate_verdict')
    : undefined
  let gateNote = 'gates: not recorded; state is maintained by the in-cluster operator (not wired in this demo)'
  let gateTone = 'muted'
  if (gateEntry) {
    if (gateEntry.result === 'PASS' || gateEntry.result.startsWith('PASS')) {
      gateNote = 'gates: PASSED'
      gateTone = 'st-verified'
    } else if (gateEntry.result.startsWith('FAIL at ')) {
      const rest = gateEntry.result.slice('FAIL at '.length)
      const step = rest.split(':')[0]
      gateNote = `gates: FAILED at ${step} — ${rest.slice(step.length + 1).trim()}`
      gateTone = 'sev-high'
    } else {
      gateNote = `gates: ${gateEntry.result}`
    }
  }
  // The id flows into a shell command the human pastes into a terminal:
  // allow only DNS-safe characters so a hostile API cannot inject flags
  // or chained commands. Invalid ids get no copy button, loudly.
  const approveId = selected && /^[A-Za-z0-9-]+$/.test(selected.id) ? selected.id : null
  const approveCmd = approveId ? `kubectl annotate finding ${approveId} umbra.dev/approved=<your-user>` : ''

  const context = source === 'live' ? [] : DIFF_CONTEXT
  const copy = async (what, text) => {
    await copyText(text)
    setCopied(what)
    setTimeout(() => setCopied((c) => (c === what ? '' : c)), 2000)
  }
  // Keyboard: j/k or arrows move, / searches, r rescans, t theme, 1/2 tabs,
  // c copies the approval command, s flips the sort.
  const keys = useRef(null)
  keys.current = (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return
    if (e.target instanceof HTMLInputElement) {
      if (e.key === 'Escape') {
        if (query !== '') setQuery('')
        else e.target.blur()
      }
      return
    }
    const i = visible.findIndex((f) => f.id === selectedId)
    if ((e.key === 'j' || e.key === 'ArrowDown') && visible.length) select(visible[Math.min(visible.length - 1, i + 1)].id)
    else if ((e.key === 'k' || e.key === 'ArrowUp') && visible.length) select(visible[Math.max(0, i - 1)].id)
    else if (e.key === '/') searchRef.current?.focus()
    else if (e.key === 'r') refresh()
    else if (e.key === 't') setTheme((t) => (t === 'night' ? 'paper' : 'night'))
    else if (e.key === '1' && selected) setTab('fix')
    else if (e.key === '2' && selected) setTab('manifest')
    else if (e.key === 'c' && approveCmd) copy('approve', approveCmd)
    else if (e.key === 's') setSort((v) => (v === 'severity' ? 'id' : 'severity'))
    else return
    e.preventDefault()
  }
  useEffect(() => {
    const h = (e) => keys.current(e)
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [])

  const tabText = !selected
    ? ''
    : tab === 'fix'
      ? diffText(context, selected.diffRemoved, selected.diffAdded)
      : selected.liveYaml
  const liveTimeline = source === 'live' && selected?.timeline?.length > 0

  return (
    <div className="shell">
      <header className="top">
        <Sprite name="eclipse" scale={2} />
        <h1 className="logo">umbra</h1>
        <span className="muted">kind-umbra</span>
        <span className={`px tag ${source === 'live' ? 'st-verified' : 'sev-medium'}`}>{source}</span>
        <span className="grow" />
        {scannedAt && <span className="muted stamp" title="last scan">{clock(scannedAt)}</span>}
        {LIVE && (
          <button
            type="button"
            className="btn ghost"
            aria-pressed={auto}
            onClick={() => setAuto((a) => !a)}
            title={`rescan every ${POLL_MS / 1000}s`}
          >
            auto
          </button>
        )}
        <button type="button" className="btn ghost" onClick={refresh} disabled={scanning} title="rescan (r)">
          {scanning ? 'scanning…' : 'scan'}
        </button>
        <button
          type="button"
          className="btn ghost"
          onClick={() => setTheme((t) => (t === 'night' ? 'paper' : 'night'))}
          title="theme (t)"
        >
          {theme === 'night' ? 'paper' : 'night'}
        </button>
      </header>
      {notice !== '' && <p className="notice">{notice}</p>}

      <main className="cols">
        <section className="col list" aria-labelledby="list-h">
          <header className="sec-head">
            <h2 id="list-h" className="px">findings</h2>
            <span className="grow" />
            <button
              type="button"
              className="btn ghost"
              onClick={() => setSort((v) => (v === 'severity' ? 'id' : 'severity'))}
              title="sort (s)"
            >
              by {sort === 'severity' ? 'sev' : 'id'}
            </button>
            <span className="muted">{visible.length}/{findings.length}</span>
          </header>
          <input
            ref={searchRef}
            className="search"
            type="search"
            placeholder="filter…  /"
            aria-label="Filter findings by id, target, kind or state"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <div className="seg" role="group" aria-label="Filter by severity">
            {SEVERITIES.map((s) => (
              <button
                key={s}
                type="button"
                className="btn ghost"
                aria-pressed={severityFilter === s}
                onClick={() => setSeverityFilter(s)}
              >
                {s} <span className="n">{countFor(s)}</span>
              </button>
            ))}
          </div>
          <ul className="rows">
            {visible.map((f) => (
              <li key={f.id}>
                <button
                  type="button"
                  className="row"
                  aria-current={f.id === selectedId ? 'true' : undefined}
                  onClick={() => select(f.id, true)}
                >
                  <span className={`pip sev-${f.severity}`} title={f.severity} />
                  <span className="row-id">{f.id}</span>
                  <span className="row-name" title={f.target}>{f.target.split('/')[1] ?? f.target}</span>
                  <span className={`px st-${f.state}`}>{STATE_LABEL[f.state] ?? f.state}</span>
                </button>
              </li>
            ))}
          </ul>
          {visible.length === 0 && (
            <p className="muted empty">
              nothing matches.{' '}
              <button type="button" className="link" onClick={() => { setSeverityFilter('all'); setQuery('') }}>
                clear filters
              </button>
            </p>
          )}
        </section>

        <section className="col detail" aria-labelledby="detail-h" ref={detailRef}>
          {selected ? (
            <>
              <div className="eyebrow">
                <p className="px muted">{selected.id} · {selected.kind}</p>
                <button type="button" className="btn ghost" onClick={() => copy('link', window.location.href)} title="copy a link to this finding">
                  {copied === 'link' ? 'copied' : 'link'}
                </button>
              </div>
              <h2 id="detail-h" className="subject">{selected.target}</h2>
              <p className="facts">
                <span className={`sev-${selected.severity}`}>{selected.severity}</span>
                <span className={`st-${selected.state}`}>{STATE_LABEL[selected.state] ?? selected.state}</span>
                <span className="muted">
                  {selected.liveYaml ? `${selected.liveYaml.split('\n').length} lines · sanitized` : 'no manifest'}
                </span>
              </p>

              <div className="tabs" role="tablist" aria-label="Finding detail">
                <button type="button" role="tab" className="tab" aria-selected={tab === 'fix'} onClick={() => setTab('fix')}>
                  fix <span className="add">+{selected.diffAdded.length}</span> <span className="del">−{selected.diffRemoved.length}</span>
                </button>
                <button type="button" role="tab" className="tab" aria-selected={tab === 'manifest'} onClick={() => setTab('manifest')}>
                  manifest
                </button>
                <span className="grow" />
                <button type="button" className="btn ghost" onClick={() => copy(tab, tabText)}>
                  {copied === tab ? 'copied' : 'copy'}
                </button>
                <button
                  type="button"
                  className="btn ghost"
                  onClick={() => save(`${approveId ?? 'finding'}.${tab === 'fix' ? 'diff' : 'yaml'}`, tabText)}
                  title="download"
                >
                  save
                </button>
              </div>
              <div role="tabpanel">
                {tab === 'fix' ? (
                  <DiffView context={context} removed={selected.diffRemoved} added={selected.diffAdded} />
                ) : (
                  <pre className="code">{selected.liveYaml || 'no manifest'}</pre>
                )}
                <p className="px muted caption">
                  {tab === 'fix' ? 'proposed fix · applied in sandbox only' : 'live manifest · sanitized copy'}
                </p>
              </div>

              {/* Production gate: display-only. The API is read-only by
                  design, so approval is a kubectl command the human runs;
                  the copy button only copies text, it never approves. */}
              <div className="gate">
                <p className={gateTone}>
                  <Sprite name={gateTone === 'st-verified' ? 'check' : gateTone === 'sev-high' ? 'x' : 'chev'} /> {gateNote}
                </p>
                {gateEntry?.modelVersion && (
                  <p className="muted">
                    proposed by <span className="code-font">{gateEntry.modelVersion}</span>
                  </p>
                )}
                {approveCmd === '' ? (
                  <p className="sev-high">invalid finding id — approval command withheld</p>
                ) : (
                  <>
                    <p className="px muted">human step · not executed by umbra</p>
                    <div className="cmd">
                      <code title="Run this to approve as a human">{approveCmd}</code>
                      <button type="button" className="btn" onClick={() => copy('approve', approveCmd)} aria-label="Copy approval command">
                        {copied === 'approve' ? 'copied' : 'copy'}
                      </button>
                    </div>
                  </>
                )}
              </div>
            </>
          ) : (
            <p className="muted empty">select a finding to inspect its sandbox fix</p>
          )}
        </section>

        <section className="col trail" aria-labelledby="trail-h">
          <header className="sec-head">
            <h2 id="trail-h" className="px">agent</h2>
            {!liveTimeline && <span className="muted">simulation — not a real run</span>}
          </header>
          <ol className="tls">
            {(liveTimeline ? selected.timeline : TIMELINE).map((entry, i) => (
              <TimelineEntry key={i} entry={entry} />
            ))}
          </ol>
        </section>
      </main>

      <Scoreboard />

      <footer className="keys muted">
        <span><kbd>j</kbd><kbd>k</kbd> move</span>
        <span><kbd>/</kbd> filter</span>
        <span><kbd>r</kbd> scan</span>
        <span><kbd>1</kbd><kbd>2</kbd> tabs</span>
        <span><kbd>c</kbd> copy cmd</span>
        <span><kbd>s</kbd> sort</span>
        <span><kbd>t</kbd> theme</span>
      </footer>
    </div>
  )
}
