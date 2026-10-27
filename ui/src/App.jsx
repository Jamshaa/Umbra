import { useState } from 'react'
import HeaderBar from './components/HeaderBar.jsx'
import PixelButton from './components/PixelButton.jsx'
import FindingCard from './components/FindingCard.jsx'
import CodePanel, { DiffView } from './components/CodePanel.jsx'
import TimelineEntry from './components/TimelineEntry.jsx'
import { MOCK_FINDINGS, DIFF_CONTEXT, TIMELINE } from './mock.js'
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
  const [findings] = useState(MOCK_FINDINGS)
  const [selectedId, setSelectedId] = useState(MOCK_FINDINGS[0]?.id ?? null)
  const [severityFilter, setSeverityFilter] = useState('all')

  const selected = findings.find((f) => f.id === selectedId) ?? null
  const visible = findings.filter((f) => severityFilter === 'all' || f.severity === severityFilter)
  const verified = selected?.state === 'verified'

  return (
    <div className="app-shell">
      <HeaderBar />
      <main className="app-columns">
        {/* ── left column: findings ─────────────────────────────────────── */}
        <section className="column">
          <h2 className="column-title">FINDINGS</h2>
          <div className="scan-row">
            <PixelButton onClick={() => {}}>Scan cluster</PixelButton>
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
                <PixelButton
                  size="lg"
                  tone="warn"
                  disabled={!verified}
                  title={
                    verified
                      ? 'Propose for production — requires explicit human approval'
                      : 'Sandbox verification must pass before the production gate opens'
                  }
                >
                  Approve → prod
                </PixelButton>
                <code className={'gate-note ' + (verified ? 'is-ok' : 'is-pending')}>
                  {verified ? 'verification PASSED ✓ — awaiting human approval' : 'verification pending'}
                </code>
              </div>
            </>
          ) : (
            <p className="empty-note">select a finding to inspect its sandbox diff</p>
          )}
        </section>

        {/* ── right column: agent tool-call timeline ────────────────────── */}
        <section className="column">
          <h2 className="column-title">AGENT TIMELINE</h2>
          {TIMELINE.map((entry, i) => (
            <TimelineEntry key={i} entry={entry} />
          ))}
          <div className="thinking-row">
            <span className="cursor" aria-hidden="true" />
            <span className="thinking-label">agent thinking…</span>
          </div>
        </section>
      </main>
    </div>
  )
}
