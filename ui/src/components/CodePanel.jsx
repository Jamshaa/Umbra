import './CodePanel.css'

/**
 * Code/YAML panel: 2px solid border (info blue by default, gold with the
 * .warn class), DM Mono @12px, --bg fill. Content can arrive as `code`
 * (string renders inside <pre><code>) or as `children` (e.g. DiffView).
 */
export default function CodePanel({ title, code, className = '', children }) {
  return (
    <figure className={`code-panel ${className}`.trim()}>
      {title && <figcaption className="code-panel-title">{title}</figcaption>}
      {code != null ? (
        <pre>
          <code>{code}</code>
        </pre>
      ) : (
        children
      )}
    </figure>
  )
}

/**
 * Sandbox fix diff — hardcoded hard-cut lines: context neutral, removed red,
 * added green. No syntax highlighter dependency for the alpha UI.
 */
export function DiffView({ context = [], removed = [], added = [] }) {
  return (
    <div className="diff-view">
      {context.map((line, i) => (
        <div key={`c${i}`} className="diff-line">
          {line}
        </div>
      ))}
      {removed.map((line, i) => (
        <div key={`r${i}`} className="diff-line del">
          - {line}
        </div>
      ))}
      {added.map((line, i) => (
        <div key={`a${i}`} className="diff-line add">
          + {line}
        </div>
      ))}
    </div>
  )
}
