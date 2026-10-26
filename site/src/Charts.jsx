import { shortDate } from './format.js'

const nice = (v) => (Number.isInteger(v) ? String(v) : v.toFixed(1))

// Bars for a player's recent games with a dashed line for the number to beat. Bars that would have
// won the bet are bright; the rest are dim, so the pattern reads without the numbers.
export function FormBars({ games, line, lineLabel, side, tone = 'stat', height = 150, compact = false }) {
  if (!games?.length) return null
  const w = compact ? 96 : 340
  const pw = compact ? w : w - 62 // plot width; the right margin holds the line's label
  const h = compact ? 44 : height
  const padT = compact ? 4 : 22
  const padB = compact ? 2 : 34
  const top = Math.max(line, ...games.map((g) => g.value)) * 1.2 || 1
  const y = (v) => padT + (h - padT - padB) * (1 - v / top)
  const slot = pw / Math.max(games.length, 5)
  const bw = slot * 0.62

  return (
    <svg
      className={`bars${compact ? ' compact' : ''}`}
      viewBox={`0 0 ${w} ${h}`}
      role="img"
      aria-label={`${games.length} recent games against a line of ${line}`}
      preserveAspectRatio="xMidYMid meet"
    >
      {games.map((g, i) => {
        const x = i * slot + (slot - bw) / 2
        const hit = tone === 'stat' ? (side === 'OVER' ? g.value > line : g.value < line) : null
        const cls = `${hit === null ? 'neutral' : hit ? 'hit' : 'miss'}${g.short || g.prior ? ' short' : ''}`
        return (
          <g key={`${g.date}-${i}`}>
            <rect className={`bar ${cls}`} x={x} y={y(g.value)} width={bw} height={Math.max(1, h - padB - y(g.value))} />
            {!compact && (
              <>
                <text className="val" x={x + bw / 2} y={y(g.value) - 5} textAnchor="middle">
                  {nice(g.value)}
                </text>
                <text className="sub" x={x + bw / 2} y={h - 18} textAnchor="middle">
                  {g.opp ? `${g.home ? 'vs' : '@'} ${g.opp}` : g.home ? 'home' : 'away'}
                </text>
                <text className="sub dim" x={x + bw / 2} y={h - 5} textAnchor="middle">
                  {shortDate(g.date)}
                </text>
              </>
            )}
          </g>
        )
      })}
      <line className="ref" x1="0" x2={pw} y1={y(line)} y2={y(line)} />
      {!compact && lineLabel && (
        <text className="reflabel" x={pw + 6} y={y(line) + 4} textAnchor="start">
          {lineLabel}
        </text>
      )}
    </svg>
  )
}

// Running units over the days graded: staked picks, and a flat 1 unit on every tracked play.
export function UnitsChart({ daily }) {
  if (!daily?.length) return null
  const w = 640
  const h = 220
  const m = { l: 44, r: 52, t: 16, b: 30 }
  const series = [
    { key: 'flatUnits', label: 'Every tracked play, 1 unit each', cls: 'flat' },
    { key: 'stakedUnits', label: 'Max plays, units staked', cls: 'staked' },
  ]
  const all = daily.flatMap((d) => [d.flatUnits, d.stakedUnits, 0])
  const lo = Math.min(...all)
  const hi = Math.max(...all)
  const span = hi - lo || 1
  const x = (i) => m.l + (daily.length === 1 ? (w - m.l - m.r) / 2 : (i * (w - m.l - m.r)) / (daily.length - 1))
  const y = (v) => m.t + (h - m.t - m.b) * (1 - (v - lo) / span)
  // Keep tick labels from piling up when two values are nearly the same.
  const ticks = [lo, 0, hi].filter((v, i, a) => a.indexOf(v) === i).filter((v, i, a) => a.every((u, j) => j >= i || Math.abs(u - v) > span * 0.12 || u === 0))

  return (
    <figure className="units">
      <svg viewBox={`0 0 ${w} ${h}`} role="img" aria-label="Running units by day">
        {ticks.map((t) => (
          <g key={t}>
            <line className={t === 0 ? 'zero' : 'grid'} x1={m.l} x2={w - m.r} y1={y(t)} y2={y(t)} />
            <text className="tick" x={m.l - 8} y={y(t) + 4} textAnchor="end">
              {t > 0 ? '+' : ''}
              {t.toFixed(1)}
            </text>
          </g>
        ))}
        {series.map((s) => (
          <g key={s.key} className={s.cls}>
            <polyline points={daily.map((d, i) => `${x(i)},${y(d[s.key])}`).join(' ')} />
            {daily.map((d, i) => (
              <circle key={d.date} cx={x(i)} cy={y(d[s.key])} r="4" />
            ))}
            <text className="end" x={x(daily.length - 1) + 6} y={y(daily[daily.length - 1][s.key]) + 4}>
              {daily[daily.length - 1][s.key] > 0 ? '+' : ''}
              {daily[daily.length - 1][s.key].toFixed(1)}
            </text>
          </g>
        ))}
        {daily.map((d, i) =>
          i === 0 || i === daily.length - 1 || daily.length <= 6 ? (
            <text key={d.date} className="tick" x={x(i)} y={h - 8} textAnchor="middle">
              {shortDate(d.date)}
            </text>
          ) : null,
        )}
      </svg>
      <figcaption>
        {series.map((s) => (
          <span key={s.key} className={`key ${s.cls}`}>
            {s.label}
          </span>
        ))}
      </figcaption>
    </figure>
  )
}

// A one-line picture of the play: where the line sits, where his last games landed, and where the
// model projects him. Dots are his recent games (bright when the pick would have hit), the tall tick
// is the line and the triangle is the projection.
export function LineStrip({ line, proj, games, side, width = 190, height = 36 }) {
  const vals = [...(games || []).map((g) => g.value), line, proj].filter((v) => Number.isFinite(v))
  if (vals.length < 2) return null
  const lo = Math.min(...vals)
  const hi = Math.max(...vals)
  const pad = Math.max((hi - lo) * 0.12, 0.6)
  const x = (v) => 8 + ((v - (lo - pad)) / (hi - lo + 2 * pad)) * (width - 16)
  const mid = 15
  const hit = (v) => (side === 'OVER' ? v > line : v < line)
  return (
    <svg
      className="strip"
      viewBox={`0 0 ${width} ${height}`}
      role="img"
      aria-label={`Line ${line}, projection ${proj.toFixed(1)}, recent games ${(games || []).map((g) => nice(g.value)).join(', ')}`}
    >
      <line className="axis" x1="4" x2={width - 4} y1={mid} y2={mid} />
      <rect className="mark" x={x(line) - 1} y={3} width="2" height={mid * 2 - 6} />
      {(games || []).map((g, i) => (
        <circle key={`${g.date}-${i}`} className={`dot ${hit(g.value) ? 'hit' : 'miss'}`} cx={x(g.value)} cy={mid} r="4" />
      ))}
      <path className="proj" d={`M${x(proj)} ${mid + 7} l5 8 h-10 z`} />
    </svg>
  )
}
