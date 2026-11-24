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

export function formView(p, count=5) {
 const rows=(count===10&&p.last10?.length?p.last10:p.last5)||[]
 const games=rows.slice(-count).filter(g=>Number.isFinite(g.value))
 const hits=games.filter(g=>p.side==='OVER'?g.value>p.line:g.value<p.line).length
 return {...p,last5:games,l5Hits:hits,l5Avg:games.length?games.reduce((a,g)=>a+g.value,0)/games.length:0}
}
export function FormRange({p,value,onChange}) {
 return <div className="form-range" role="group" aria-label="Recent form window">{[5,10].map(n=><button type="button" key={n} aria-pressed={value===n} disabled={n===10&&!p.last10?.length} onClick={()=>onChange(n)}>Last {n}</button>)}</div>
}
