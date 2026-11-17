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

// The model's range of outcomes for the stat: one bar per final number, bars that win this play are
// bright, the dashed line is the number to beat. The bar heights are the model's own probabilities.
export function OutcomeCurve({ dist, line, side, width = 560, height = 130 }) {
  if (!dist || dist.length < 3) return null
  const n = dist.length
  const maxP = Math.max(...dist)
  const left = 6
  const bottom = 22
  const top = 8
  const bw = (width - left * 2) / n
  const wins = (k) => (side === 'OVER' ? k > line : k < line)
  const push = (k) => k === line
  const winP = dist.reduce((s, v, k) => (wins(k) ? s + v : s), 0)
  const total = dist.reduce((s, v) => s + v, 0) || 1
  const step = n > 30 ? 5 : n > 14 ? 2 : 1
  const lineX = left + (line + 0.5) * bw
  return (
    <figure className="curve">
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label={`Range of outcomes against a line of ${line}. The model puts ${Math.round((winP / total) * 100)} percent on the winning side.`}>
        {dist.map((v, k) => {
          const h = (v / maxP) * (height - bottom - top)
          return (
            <rect
              key={k}
              x={left + k * bw + 1}
              y={height - bottom - h}
              width={Math.max(bw - 2, 1)}
              height={Math.max(h, 0.5)}
              className={`cbar ${push(k) ? 'push' : wins(k) ? 'win' : 'lose'}`}
            />
          )
        })}
        <line x1={lineX} x2={lineX} y1={top - 2} y2={height - bottom} className="cline" />
        <text x={lineX} y={top + 4} className="clabel" textAnchor={lineX > width - 70 ? 'end' : 'start'} dx={lineX > width - 70 ? -4 : 4}>
          line {line}
        </text>
        {dist.map((_, k) =>
          k % step === 0 ? (
            <text key={k} x={left + k * bw + bw / 2} y={height - 6} className="caxis" textAnchor="middle">
              {k}
            </text>
          ) : null,
        )}
      </svg>
      <figcaption>
        Bright bars are the outcomes that win this play. The model puts {Math.round((winP / total) * 100)}% of its range there before the books' prices are blended in.
      </figcaption>
    </figure>
  )
}

// How the market's median line moved through the day. Against the pick is amber, with it is green.
export function MoveSpark({ moves, side, width = 300, height = 54 }) {
  if (!moves || moves.length < 2) return null
  const lines = moves.map((m) => m.line)
  const lo = Math.min(...lines)
  const hi = Math.max(...lines)
  const span = hi - lo || 1
  const x = (i) => 8 + (i / (moves.length - 1)) * (width - 16)
  const y = (v) => 8 + (1 - (v - lo) / span) * (height - 24)
  const first = moves[0]
  const last = moves[moves.length - 1]
  const delta = last.line - first.line
  // A higher line is worse for an Over and better for an Under.
  const against = (side === 'OVER' && delta > 0) || (side === 'UNDER' && delta < 0)
  const tone = delta === 0 ? 'flat' : against ? 'against' : 'with'
  const t = (iso) => new Date(iso).toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', timeZone: 'America/New_York' })
  return (
    <figure className={`spark ${tone}`}>
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label={`Line moved from ${first.line} to ${last.line}`}>
        <polyline points={moves.map((m, i) => `${x(i)},${y(m.line)}`).join(' ')} />
        {moves.map((m, i) => (
          <circle key={m.at} cx={x(i)} cy={y(m.line)} r={i === moves.length - 1 ? 3.4 : 2.2} />
        ))}
        <text x={8} y={height - 3} className="caxis">{t(first.at)} ET</text>
        <text x={width - 8} y={height - 3} className="caxis" textAnchor="end">{t(last.at)} ET</text>
      </svg>
      <figcaption>
        {delta === 0
          ? `The line has held at ${last.line} all day.`
          : `The line went from ${first.line} to ${last.line}, ${against ? 'against' : 'in favor of'} this pick.`}
      </figcaption>
    </figure>
  )
}

// Hit rate against what the model said, one pair of bars per group. Each row says how many plays are behind it.
export function HitRateBars({ rows }) {
  const list = (rows || []).filter((r) => r.wins + r.losses > 0)
  if (list.length === 0) return null
  return (
    <div className="hitbars">
      {list.map((r) => {
        const n = r.wins + r.losses
        return (
          <div key={r.name} className="hrow">
            <span className="lbl">{r.name}</span>
            <span className="pair" aria-hidden="true">
              <i className="pred" style={{ width: `${r.predicted * 100}%` }} />
              <i className="act" style={{ width: `${r.hitRate * 100}%` }} />
              <b className="fifty" />
            </span>
            <span className="nums">
              {r.wins}-{r.losses}, hit {Math.round(r.hitRate * 100)}% (model said {Math.round(r.predicted * 100)}%)
              <small> over {n}{n < 30 ? ', too few to judge' : ''}</small>
            </span>
          </div>
        )
      })}
    </div>
  )
}

// One tile per day: wins and losses, green when the day went over 50% and red under.
export function DayGrid({ days }) {
  if (!days || days.length === 0) return null
  return (
    <div className="daygrid">
      {days.map((d) => {
        const n = d.w + d.l
        const rate = n ? d.w / n : 0.5
        const tone = n === 0 ? 'none' : rate >= 0.55 ? 'good' : rate <= 0.45 ? 'bad' : 'even'
        return (
          <div key={d.date} className={`day-tile ${tone}`} title={`${d.date}: ${d.w}-${d.l}`}>
            <span className="dd">{d.label}</span>
            <b>
              {d.w}-{d.l}
            </b>
            <small>{n ? `${Math.round(rate * 100)}%` : '-'}</small>
          </div>
        )
      })}
    </div>
  )
}
