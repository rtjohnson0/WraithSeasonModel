import {statLabel} from './format.js'
export default function TeamRanks({teams=[],position=[]}) {
 const available=teams.filter(Boolean)
 if(!available.length)return null
 return <section className="team-ranks"><h4>Team & matchup rankings</h4>
 <div className="team-rank-grid">{available.map(t=><div key={t.name}><b>{t.name} · {t.season}–{String(t.season+1).slice(-2)} season</b>
 {t.scoring&&<p>Scoring: <strong>#{t.scoring.rank} of {t.scoring.of}</strong> · {t.scoring.average.toFixed(1)} points per game ({t.scoring.games} games).</p>}
 <div className="scroll"><table><thead><tr><th>Allowed per game</th><th>Rank*</th><th>Average</th><th>Games</th></tr></thead><tbody>{(t.allowed||[]).map(r=><tr key={r.stat}><td>{statLabel[r.stat]||r.stat}</td><td>#{r.rank} / {r.of}</td><td>{r.average.toFixed(1)}</td><td>{r.games}</td></tr>)}</tbody></table></div></div>)}</div>
 {position.length>0?<div className="scroll"><table><caption>Opponent vs {position[0].position} · full-game totals by position group</caption><thead><tr><th>Stat</th><th>Rank*</th><th>Allowed / game</th><th>Games</th></tr></thead><tbody>{position.map(r=><tr key={r.stat}><td>{statLabel[r.stat]||r.stat}</td><td>#{r.rank} / {r.of}</td><td>{r.average.toFixed(1)}</td><td>{r.games}</td></tr>)}</tbody></table></div>:null}
 <details className="rank-method"><summary>Sample & ranking definitions</summary><p className="small">*Defensive rank #1 allows the fewest; scoring rank #1 scores the most. Current-season final games strictly before {available[0].before}, minimum 3 games. Rankings cover {available[0].scoring?.of||'available'} teams with sufficient data; these are per-game totals, including overtime, without pace adjustment. Position coverage uses the feed’s broad groups, not exact PG/SG/SF/PF assignments. Context does not automatically change the model probability.</p></details>
 </section>
}
