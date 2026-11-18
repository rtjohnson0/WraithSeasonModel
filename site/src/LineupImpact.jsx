import { statLabel } from './format.js'

export default function LineupImpact({ impacts, scale }) {
  const relevant = (impacts || []).filter(x => x.missing?.length)
  if (!relevant.length) return null
  return <section aria-label="Offense and defense absence impact">
    <h4>Offense and defense impact</h4>
    <p className="small">Earlier absence games inform these research estimates. They are associations, not isolated injury effects. Replacements, opponents and game conditions can offset a missing player&apos;s contribution.</p>
    {relevant.map(x => <div key={`${x.kind}-${x.team}`}>
      <h4>{x.kind === 'teammate-production' ? 'Teammates out · player production' : 'Opponents out · matchup allowances'}</h4>
      {x.missing.map(p => <p className="small" key={p.playerId}><b>{p.player}</b> ({p.position || 'position unavailable'}) · {p.games} earlier appearances: {p.minutes.toFixed(1)} MIN, {p.points.toFixed(1)} PTS, {p.rebounds.toFixed(1)} REB, {p.assists.toFixed(1)} AST, {p.blocks.toFixed(1)} BLK per game.</p>)}
      <p className="small">{x.reason}. These player averages describe recent contributions, not the team&apos;s net loss. Blocks are a defensive indicator, not a measured rim-protection rating; rebounds are total rebounds.</p>
      {x.effects?.length > 0 && <div className="availability-table-wrap"><table>
        <thead><tr><th>Stat rate</th><th>Evidence</th><th>Research change</th></tr></thead>
        <tbody>{x.effects.map(e => <tr key={e.stat}><td>{statLabel[e.stat] || e.stat}</td><td>{e.method === 'pooled-lineup' ? (e.role ? `${e.role.games} league player-games / ${e.role.days} dates` : 'Pooled evidence unavailable') : e.method === 'individual-mixture' ? e.evidence?.map(c => <div key={c.playerId}>{x.missing.find(p => p.playerId === c.playerId)?.player || c.playerId}: {c.without} / {c.with} · {(100*c.weight).toFixed(0)}% weight</div>) : `${e.without} / ${e.with}`}</td><td>{e.applied ? `${e.scale >= 1 ? '+' : ''}${((e.scale - 1)*100).toFixed(1)}%` : 'Unchanged · limited evidence'}</td></tr>)}</tbody>
      </table></div>}
    </div>)}
    {relevant.some(x => x.effects?.some(e => e.method === 'pooled-lineup')) && <p className="small">Lineup composition uses projected replacements and player box-score roles learned across teams. Exact injury combinations and repeated head-to-head games are not required. These are experimental associations; historical training rotations were observed, while this forecast uses projected minutes. Effects are capped at ±5% per component and ±8% combined.</p>}
    {!relevant.some(x => x.effects?.some(e => e.method === 'pooled-lineup')) && <p className="small">Joint estimates need five earlier joint-out and five present games and are capped at ±5%. Individual fallback estimates each need five out and five present games, are averaged without stacking and capped at ±2.5%. Unknown shares stay neutral. All estimates shrink toward no change. Combined production changes are capped at ±8%. Opponent comparisons use the player&apos;s position and normalize historical scorers by their own earlier rates.</p>}
    {scale?.length === 4 && <p className="small">Combined rate change: {['points','rebounds','assists','threes'].map((s,k) => `${statLabel[s] || s} ${scale[k] >= 1 ? '+' : ''}${((scale[k]-1)*100).toFixed(1)}%`).join(' · ')}. Minutes are handled separately above.</p>}
  </section>
}
