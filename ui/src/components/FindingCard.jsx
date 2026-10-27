import './FindingCard.css'

const STATE_LABEL = {
  detected: 'DETECTED',
  analyzing: 'ANALYZING',
  proposed: 'FIX PROPOSED',
  verified: 'VERIFIED',
  reverted: 'REVERTED',
}

/**
 * One finding in the left column: severity pixel-dot + ID + target + state.
 * Whole card is the click target (selection drives the DETAIL column).
 * Severity drives the 6px left bar; state drives the state label color.
 */
export default function FindingCard({ finding, selected = false, onSelect }) {
  const { id, severity, state, kind, target } = finding
  const shortName = target.split('/')[1] ?? target

  return (
    <button
      type="button"
      className={`finding-card ${severity} st-${state}${selected ? ' is-selected' : ''}`}
      onClick={() => onSelect?.(finding)}
      aria-pressed={selected}
    >
      <span className="finding-dot" aria-hidden="true" />
      <span className="finding-id">{id}</span>
      <span className="finding-name" title={target}>
        {shortName}
      </span>
      <span className="finding-state">{STATE_LABEL[state] ?? state.toUpperCase()}</span>
      <span className="finding-kind">{kind}</span>
    </button>
  )
}
