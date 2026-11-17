import { marketLabel, statLabel } from './format.js'

export function PropTicket({ p }) {
  const over = p.side === 'OVER'
  return <span className={`prop-ticket ${over ? 'over' : 'under'}`}>
    <span className="ticket-side"><span aria-hidden="true">{over ? '↗' : '↘'}</span>{over ? 'Over' : 'Under'}</span>
    <span className="ticket-line">{p.line}</span>
    <span className="ticket-market"><b>{statLabel[p.stat] || p.stat}</b><small>{marketLabel(p).split(' · ')[0]}</small></span>
  </span>
}

export function RecentForm({ p }) {
  const games = (p.last5 || []).filter(g => Number.isFinite(g.value))
  if (!games.length) return <span className="form-unavailable">Recent form unavailable</span>
  return <span className="recent-form" aria-label={`Recent results against the current ${p.line} line`}>
    {games.map((g, i) => {
      const push = g.value === p.line
      const hit = p.side === 'OVER' ? g.value > p.line : g.value < p.line
      const outcome = push ? 'push' : hit ? 'hit' : 'miss'
      return <span key={`${g.date}-${i}`} className={`form-result ${outcome}`} title={`${g.date}${g.opp ? ` · ${g.home ? 'vs' : '@'} ${g.opp}` : ''} · ${outcome} at today's line${g.prior ? ' · prior season' : ''}`}>
        <b>{Number.isInteger(g.value) ? g.value : g.value.toFixed(1)}</b><small>{outcome === 'push' ? '—' : hit ? '✓' : '×'}</small>
      </span>
    })}
  </span>
}
