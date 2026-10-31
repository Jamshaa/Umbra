import './HeaderBar.css'

/**
 * Top bar: UMBRA logo (Press Start 2P @16px) + cluster status chip.
 * No nav tabs: sections without a backend (Policy/Sandbox/About) were
 * dead links. They return with the views that back them.
 */
export default function HeaderBar() {
  return (
    <header className="header-bar">
      <h1 className="header-logo">
        UMBRA<span className="header-logo-dim">_</span>
      </h1>

      <div className="header-status">
        <span className="status-led" aria-hidden="true" />
        <span className="status-text">kind-umbra · READY</span>
      </div>
    </header>
  )
}
