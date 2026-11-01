import { useEffect, useRef, useState } from 'react'
import HeaderBar from './components/HeaderBar.jsx'
import PixelButton from './components/PixelButton.jsx'
import FindingCard from './components/FindingCard.jsx'
import CodePanel, { DiffView } from './components/CodePanel.jsx'
import TimelineEntry from './components/TimelineEntry.jsx'
import Scoreboard from './components/Scoreboard.jsx'
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
    setNotice(src === 'live' ? '' : 'mock data — set VITE_UMBRA_API=live for the operator API')
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
  const verified = selected?.state === 'verified'
  // The exact kubectl command that records a human approval. The user
  // name stays a placeholder: the UI never knows who you are.
  const approveCmd = selected
    ? `kubectl annotate finding ${selected.id} umbra.dev/approved=<tu-usuario>`
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
      <HeaderBar />
      <main className="app-columns">
        {/* ── left column: findings ─────────────────────────────────────── */}
        <section className="column">
          <h2 className="column-title">FINDINGS</h2>
          {notice !== '' && <p className="empty-note">{notice}</p>}
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
                {s}
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
          {visible.length === 0 && <p className="empty-note">no findings match this filter</p>}
        </section>

        {/* ── center column: detail + sandbox diff + production gate ────── */}
        <section className="column">
          <h2 className="column-title">DETAIL</h2>
          {selected ? (
            <>
              <p className="subject-line">
                {selected.kind}: <span className="subject-name">{selected.target}</span>
              </p>
              <CodePanel title="LIVE MANIFEST (sanitized copy)" code={selected.liveYaml} />
              <CodePanel title="PROPOSED FIX — SANDBOX DIFF" className="panel-diff">
                <DiffView context={DIFF_CONTEXT} removed={selected.diffRemoved} added={selected.diffAdded} />
              </CodePanel>
              <div className="gate-row">
                {/* Production gate: display-only in the UI. There is no
                    propose endpoint (the API is read-only by design), so
                    approval is a kubectl command the human runs. The copy
                    button only copies text; it never approves anything. */}
                <code className={'gate-note ' + (verified ? 'is-ok' : 'is-pending')}>
                  {verified ? 'verification PASSED ✓ — awaiting human approval' : 'verification pending'}
                </code>
              </div>
              <div className="gate-row">
                <code className="gate-cmd" title="Run this to approve as a human">{approveCmd}</code>
                <PixelButton size="sm" onClick={copyApprove} aria-label="Copy approval command">
                  {copied ? '¡Copiado!' : 'Copiar'}
                </PixelButton>
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
              <p className="empty-note">simulación — no es una ejecución real</p>
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
