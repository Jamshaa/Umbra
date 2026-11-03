import { useEffect, useRef, useState } from 'react'
import HeaderBar from './components/HeaderBar.jsx'
import PixelButton from './components/PixelButton.jsx'
import FindingCard from './components/FindingCard.jsx'
import CodePanel, { DiffView } from './components/CodePanel.jsx'
import TimelineEntry from './components/TimelineEntry.jsx'
import Scoreboard from './components/Scoreboard.jsx'
import Sprite from './components/Sprite.jsx'
import { MOCK_FINDINGS, DIFF_CONTEXT, TIMELINE } from './mock.js'
import { loadFindings } from './api.js'
import './App.css'

const SEVERITIES = ['all', 'high', 'medium', 'low']

/**
 * Umbra UI — visual alpha.
 *
 * Data comes from src/mock.js until the operator's Finding CRDs + status API
 * are the source of truth (see docs/PLAN.md Day 5/6); React Query polling and
 * the native WebSocket timeline subscribe at that point. No extra deps until.
 */
export default function App() {
  const [findings, setFindings] = useState([])
  const [selectedId, setSelectedId] = useState(null)
  const [severityFilter, setSeverityFilter] = useState('all')
  const [notice, setNotice] = useState('loading findings…')
  const [source, setSource] = useState('mock')
  const [copied, setCopied] = useState(false)
  // Monotonic scan id: only the latest refresh may write state, so fast
  // repeated clicks can never leave stale results on screen.
  const scanGen = useRef(0)
  const applyLoaded = ({ source: src, findings: list }) => {
    setFindings(list)
    setSelectedId(list[0]?.id ?? null)
    setSource(src)
    setNotice(src === 'live' ? 'LIVE — operator API' : 'mock data — set VITE_UMBRA_API=live for the operator API')
  }
  const applyFallback = () => {
    // Live API unreachable: degrade to mock instead of a dead screen.
    setFindings(MOCK_FINDINGS)
    setSelectedId(MOCK_FINDINGS[0]?.id ?? null)
    setSource('mock')
    setNotice('live API unreachable — showing mock data')
  }
  const refresh = () => {
    setNotice('scanning cluster…')
    const gen = scanGen.current + 1
    scanGen.current = gen
    loadFindings().then(
      (r) => { if (gen === scanGen.current) applyLoaded(r) },
      () => { if (gen === scanGen.current) applyFallback() },
    )
  }

  useEffect(() => {
    let cancelled = false
    loadFindings().then(
      (r) => { if (!cancelled) applyLoaded(r) },
      () => { if (!cancelled) applyFallback() },
    )
    return () => { cancelled = true }
  }, [])

  const selected = findings.find((f) => f.id === selectedId) ?? null
  const visible = findings.filter((f) => severityFilter === 'all' || f.severity === severityFilter)
  const counts = {
    total: findings.length,
    high: findings.filter((f) => f.severity === 'high').length,
    verified: findings.filter((f) => f.state === 'verified').length,
  }
  const countFor = (s) => (s === 'all' ? findings.length : findings.filter((f) => f.severity === s).length)
  const verified = selected?.state === 'verified'
  // The id flows into a shell command the human pastes into a terminal:
  // allow only DNS-safe characters so a hostile API cannot inject flags
  // or chained commands. Invalid ids get no copy button, loudly.
  const approveId = selected && /^[A-Za-z0-9-]+$/.test(selected.id) ? selected.id : null
  const approveCmd = approveId
    ? `kubectl annotate finding ${approveId} umbra.dev/approved=<your-user>`
    : ''
  const copyApprove = async () => {
    try {
      await navigator.clipboard.writeText(approveCmd)
    } catch {
      // Fallback for contexts without the async clipboard API.
      const ta = document.createElement('textarea')
      ta.value = approveCmd
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      ta.remove()
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  useEffect(() => {
    setCopied(false)
  }, [selectedId])

  return (
    <div className="app-shell">
      <HeaderBar source={source} />
      <main className="app-columns">
        {/* ── left column: findings ─────────────────────────────────────── */}
        <section className="column">
          <h2 className="column-title">FINDINGS</h2>
          {notice !== '' && <p className="empty-note">{notice}</p>}
          <div className="summary-row" aria-label="Findings summary">
            <span className="stat"><b>{counts.total}</b> total</span>
            <span className="stat is-high"><b>{counts.high}</b> high</span>
            <span className="stat is-ok"><b>{counts.verified}</b> verified</span>
          </div>
          <div className="scan-row">
            <PixelButton onClick={refresh}>Scan cluster</PixelButton>
          </div>
          <div className="filter-row" role="group" aria-label="Filter findings by severity">
            {SEVERITIES.map((s) => (
              <PixelButton
                key={s}
                size="sm"
                tone={severityFilter === s ? 'ok' : 'default'}
                onClick={() => setSeverityFilter(s)}
              >
                {s} {countFor(s)}
              </PixelButton>
            ))}
          </div>
          {visible.map((f) => (
            <FindingCard
              key={f.id}
              finding={f}
              selected={f.id === selectedId}
              onSelect={(finding) => setSelectedId(finding.id)}
            />
          ))}
          {visible.length === 0 && (
            <div className="empty-block">
              <p className="empty-note">no findings match this filter</p>
              {severityFilter !== 'all' && (
                <PixelButton size="sm" onClick={() => setSeverityFilter('all')}>Show all</PixelButton>
              )}
            </div>
          )}
        </section>

        {/* ── center column: detail + sandbox diff + production gate ────── */}
        <section className="column">
          <h2 className="column-title">DETAIL</h2>
          {selected ? (
            <>
              <p className="subject-line">
                {selected.kind}: <span className="subject-name">{selected.target}</span>
              </p>
              <div className="chip-row" aria-label="Finding summary">
                <span className={`chip sev-${selected.severity}`}>{selected.severity}</span>
                <span className={`chip st-${selected.state}`}>{selected.state}</span>
                <span className="chip">{selected.kind}</span>
                <span className="chip is-muted">
                  {selected.liveYaml ? `${selected.liveYaml.split('\n').length} lines · sanitized` : 'no manifest'}
                </span>
              </div>
              <CodePanel title="LIVE MANIFEST (sanitized copy)" code={selected.liveYaml} />
              <div className="diff-summary" aria-label="Diff summary">
                <span className="chip is-add">+{selected.diffAdded.length} added</span>
                <span className="chip is-del">-{selected.diffRemoved.length} removed</span>
              </div>
              <CodePanel title="PROPOSED FIX — SANDBOX DIFF" className="panel-diff">
                <DiffView context={source === 'live' ? [] : DIFF_CONTEXT} removed={selected.diffRemoved} added={selected.diffAdded} />
              </CodePanel>
              <div className="gate-row">
                {/* Production gate: display-only in the UI. There is no
                    propose endpoint (the API is read-only by design), so
                    approval is a kubectl command the human runs. The copy
                    button only copies text; it never approves anything. */}
                <code className={'gate-note ' + (verified ? 'is-ok' : 'is-pending')}>
                  <Sprite name={verified ? 'check' : 'chev'} scale={1} />{' '}
                  {verified ? 'verification PASSED ✓ — awaiting human approval' : 'verification pending'}
                </code>
              </div>
              <div className="gate-row">
                {approveCmd === '' ? (
                  <code className="gate-note">invalid finding id — approval command withheld</code>
                ) : (
                  <>
                    <code className="gate-cmd" title="Run this to approve as a human">{approveCmd}</code>
                    <PixelButton size="sm" onClick={copyApprove} aria-label="Copy approval command">
                      {copied ? 'Copied!' : 'Copy'}
                    </PixelButton>
                  </>
                )}
              </div>
            </>
          ) : (
            <p className="empty-note">select a finding to inspect its sandbox diff</p>
          )}
        </section>

        {/* ── right column: agent tool-call timeline ────────────────────── */}
        <section className="column">
          <h2 className="column-title">AGENT TIMELINE</h2>
          {source === 'live' && selected?.timeline?.length > 0 ? (
            selected.timeline.map((entry, i) => (
              <TimelineEntry key={i} entry={entry} />
            ))
          ) : (
            <>
              <p className="empty-note">simulation — not a real run</p>
              {TIMELINE.map((entry, i) => (
                <TimelineEntry key={i} entry={entry} />
              ))}
            </>
          )}
          <div className="thinking-row">
            <span className="cursor" aria-hidden="true" />
            <span className="thinking-label">agent thinking…</span>
          </div>
        </section>
      </main>
      <Scoreboard />
    </div>
  )
}
