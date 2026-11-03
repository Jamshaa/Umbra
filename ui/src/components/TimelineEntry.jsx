import './TimelineEntry.css'
import Sprite from './Sprite.jsx'

const STATUS_TONE = {
  running: 'running',
  success: 'success',
  error: 'error',
}

const STATUS_ICON = {
  running: 'chev',
  success: 'check',
  error: 'x',
}

/**
 * One agent tool call in the right column: pixel glyph + tool name + args
 * (DM Mono) + collapsible result. Border-left color = status channel:
 * blue running / green success / red error.
 */
export default function TimelineEntry({ entry, open: openProp = null }) {
  const tone = STATUS_TONE[entry.status] ?? 'running'
  const open = openProp ?? Boolean(entry.defaultOpen)
  const tsNum = Number(entry.ts)
  const time = Number.isFinite(tsNum) && tsNum > 0
    ? new Date(tsNum * 1000).toLocaleTimeString([], { hour12: false })
    : ''

  return (
    <details className={`timeline-entry ${tone}`} open={open}>
      <summary className="timeline-summary">
        <span className="timeline-glyph" aria-hidden="true">
          <Sprite name={STATUS_ICON[tone]} scale={1} />
        </span>
        <span className="timeline-tool">{entry.tool}</span>
        {time && <span className="timeline-time">{time}</span>}
        <span className="timeline-chevron" aria-hidden="true">
          {open ? '▾' : '▸'}
        </span>
      </summary>
      <code className="timeline-args">{entry.args}</code>
      <div className="timeline-result">{entry.result}</div>
    </details>
  )
}
