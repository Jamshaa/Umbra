import Sprite from './Sprite.jsx'

const ICON = { running: 'chev', success: 'check', error: 'x' }

/** One agent tool call: glyph, tool, time; args and result fold open. */
export default function TimelineEntry({ entry }) {
  const tone = ICON[entry.status] ? entry.status : 'running'
  const ts = Number(entry.ts)
  const time = Number.isFinite(ts) && ts > 0 ? new Date(ts * 1000).toLocaleTimeString([], { hour12: false }) : ''
  return (
    <li className={`tl tl-${tone}`}>
      <details open={Boolean(entry.defaultOpen)}>
        <summary>
          <Sprite name={ICON[tone]} />
          <span className="tl-tool">{entry.tool}</span>
          {time && <span className="muted">{time}</span>}
        </summary>
        {entry.args && <code className="tl-args">{entry.args}</code>}
        <p className="tl-result">{entry.result}</p>
      </details>
    </li>
  )
}
