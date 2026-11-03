import './Sprite.css'

// Pixel sprites used by the UI: 8px grids, existing palette tokens only,
// no motion, no new fonts. Only sprites with a caller live here; add a
// new one only together with its usage.
function eclipseRows() {
  const n = 16
  const rows = []
  for (let y = 0; y < n; y++) {
    let r = ''
    for (let x = 0; x < n; x++) {
      const dy = y - 7.5
      const dx = x - 7.5
      const mx = x - 6.2
      const sun = dx * dx + dy * dy <= 7.6 * 7.6
      const moon = mx * mx + dy * dy <= 6.4 * 6.4
      r += sun && moon ? 'k' : sun ? 'g' : '.'
    }
    rows.push(r)
  }
  return rows
}

const SPRITES = {
  eclipse: { rows: eclipseRows() },
  check: {
    rows: ['........', '.......#', '......##', '#....##.', '##..##..', '.####...', '..##....', '........'],
  },
  x: {
    rows: ['##....##', '.##..##.', '..####..', '...##...', '..####..', '.##..##.', '##....##', '........'],
  },
  chev: {
    rows: ['........', '........', '##....##', '.##..##.', '..####..', '...##...', '........', '........'],
  },
}

// RLE per row, one path per run: static maps only, so innerHTML is never
// needed and nothing user-controlled reaches the DOM.
function pathsFor(rows) {
  const w = Math.max(...rows.map((r) => r.length))
  const out = []
  rows.forEach((row, y) => {
    const line = row.padEnd(w, '.')
    let x = 0
    while (x < line.length) {
      const c = line[x]
      if (c === '.') {
        x++
        continue
      }
      let e = x
      while (e < line.length && line[e] === c) e++
      out.push(
        <path
          key={`${y}-${x}`}
          className={`k-${c === '#' ? 'cur' : c}`}
          d={`M${x} ${y}h${e - x}v1h-${e - x}z`}
        />,
      )
      x = e
    }
  })
  return { paths: out, w, h: rows.length }
}

const ECLIPSE = pathsFor(SPRITES.eclipse.rows)
const GLYPHS = {
  check: pathsFor(SPRITES.check.rows),
  x: pathsFor(SPRITES.x.rows),
  chev: pathsFor(SPRITES.chev.rows),
}

/**
 * Decorative pixel sprite (an adjacent label always carries the meaning).
 * Unknown names render nothing; scale is clamped to whole pixels 1-4.
 */
export default function Sprite({ name, scale = 1 }) {
  if (name === 'eclipse') {
    const s = Math.min(4, Math.max(1, Math.floor(Number(scale)) || 1))
    return (
      <span className="sprite" aria-hidden="true">
        <svg
          className="spr"
          width={ECLIPSE.w * s}
          height={ECLIPSE.h * s}
          viewBox={`0 0 ${ECLIPSE.w} ${ECLIPSE.h}`}
          shapeRendering="crispEdges"
          focusable="false"
          aria-hidden="true"
        >
          {ECLIPSE.paths}
        </svg>
      </span>
    )
  }
  const g = GLYPHS[name]
  if (!g) return null
  const s = Math.min(4, Math.max(1, Math.floor(Number(scale)) || 1))
  return (
    <span className="sprite" aria-hidden="true">
      <svg
        className="spr"
        width={g.w * s}
        height={g.h * s}
        viewBox={`0 0 ${g.w} ${g.h}`}
        shapeRendering="crispEdges"
        focusable="false"
        aria-hidden="true"
      >
        {g.paths}
      </svg>
    </span>
  )
}
