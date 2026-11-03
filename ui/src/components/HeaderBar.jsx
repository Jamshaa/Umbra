import './HeaderBar.css'
import Sprite from './Sprite.jsx'

/**
 * Top bar: eclipse mark + UMBRA logo (Press Start 2P @16px) + status
 * chips. No nav tabs: sections without a backend (Policy/Sandbox/About)
 * were dead links. They return with the views that back them.
 */
export default function HeaderBar({ source = 'mock' }) {
  const live = source === 'live'
  return (
    <header className="header-bar">
      <Sprite name="eclipse" scale={2} />
      <h1 className="header-logo">
        UMBRA<span className="header-logo-dim">_</span>
      </h1>

      <div className="header-status">
        <span className="status-led" aria-hidden="true" />
        <span className="status-text">kind-umbra · READY</span>
      </div>
      <div className={`header-source ${live ? 'is-live' : 'is-mock'}`}>
        {live ? 'LIVE' : 'MOCK'}
      </div>
    </header>
  )
}
