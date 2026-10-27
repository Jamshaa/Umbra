import './PixelButton.css'

const TONES = { default: '', ok: ' ok', warn: ' warn', danger: ' danger' }
const SIZES = { sm: ' sm', md: '', lg: ' lg' }

/**
 * Pixel button: Press Start 2P, hard shadow, translate-on-press.
 * Tones: default (panel/blue) · ok (green) · warn (gold: production gate)
 * · danger (red: destructive intents). Shapes stay square, always.
 */
export default function PixelButton({ children, tone = 'default', size = 'md', disabled = false, onClick, title, ariaLabel }) {
  return (
    <button
      type="button"
      className={`pixel-btn${TONES[tone] ?? ''}${SIZES[size] ?? ''}`}
      disabled={disabled}
      onClick={onClick}
      title={title}
      aria-label={ariaLabel}
    >
      {children}
    </button>
  )
}
