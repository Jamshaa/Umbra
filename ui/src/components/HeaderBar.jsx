import './HeaderBar.css'

const NAV = ['Scan', 'Policy', 'Sandbox', 'About']

/**
 * Top bar: UMBRA logo (Press Start 2P @16px) + pixel nav + cluster status
 * chip. Registration lines stay coarse: tabs are not wired in the alpha.
 */
export default function HeaderBar() {
  return (
    <header className="header-bar">
      <h1 className="header-logo">
        UMBRA<span className="header-logo-dim">_</span>
      </h1>

      <nav className="header-nav" aria-label="Umbra sections">
        {NAV.map((item, i) => (
          <a
            key={item}
            className={`header-tab${i === 0 ? ' is-active' : ''}`}
            href={`#/${item.toLowerCase()}`}
          >
            {item}
          </a>
        ))}
      </nav>

      <div className="header-status">
        <span className="status-led" aria-hidden="true" />
        <span className="status-text">kind-umbra · READY</span>
      </div>
    </header>
  )
}
