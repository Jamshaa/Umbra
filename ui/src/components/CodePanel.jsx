// Code and diff blocks share one look: DM Mono on --panel, no highlighter.

/** Copy text; falls back to execCommand where the async clipboard is missing. */
export async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    ta.remove()
  }
}

export function diffText(context, removed, added) {
  return [...context.map((l) => `  ${l}`), ...removed.map((l) => `- ${l}`), ...added.map((l) => `+ ${l}`)].join('\n')
}

export function DiffView({ context = [], removed = [], added = [] }) {
  if (removed.length + added.length === 0) return <pre className="code muted">no change proposed</pre>
  return (
    <pre className="code">
      {context.map((l, i) => <div key={`c${i}`} className="ln">{'  '}{l}</div>)}
      {removed.map((l, i) => <div key={`r${i}`} className="ln del">- {l}</div>)}
      {added.map((l, i) => <div key={`a${i}`} className="ln add">+ {l}</div>)}
    </pre>
  )
}
