import './TimelineEntry.css'

const STATUS_TONE = {
  running: 'running',
  success: 'success',
  error: 'error',
}

/**
 * One agent tool call in the right column: pixel glyph + tool name + args
 * (DM Mono) + collapsible result. Border-left color = status channel:
 * blue running / green success / red error.
 */
export default function TimelineEntry({ entry, open: openProp = null }) {
  const tone = STATUS_TONE[entry.status] ?? 'running'
  const open = openProp ?? Boolean(entry.defaultOpen)

  return (
    <details className={`timeline-entry ${tone}`} open={open}>
      <summary className="timeline-summary">
        <span className="timeline-glyph" aria-hidden="true" />
        <span className="timeline-tool">{entry.tool}</span>
        <span className="timeline-chevron" aria-hidden="true">
          {open ? '▾' : '▸'}
        </span>
      </summary>
      <code className="timeline-args">{entry.args}</code>
      <div className="timeline-result">{entry.result}</div>
    </details>
  )
}
