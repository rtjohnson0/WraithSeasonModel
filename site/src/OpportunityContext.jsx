import {statLabel} from './format.js'
import PlayerPortrait from './PlayerPortrait.jsx'
const fixed=(n,d=1)=>Number.isFinite(n)?n.toFixed(d):'—'
const volume={points:'FGA + 0.44 × FTA',rebounds:'Rebound chances',assists:'Potential assists',threes:'3PA'}
export default function OpportunityContext({p}) {
 const r=p.rotation
 if(!r||r.policy!=='rotation-v4')return null
 const signals=r.opportunities||[],game=r.gameContext
 const effect=r.production?.find(x=>x.kind==='opportunity-production')?.effects?.find(e=>e.stat===p.stat)
 return <section className="context-opportunities" aria-label="Opportunity and game context">
  {r.workloadReview?.directionChanges&&<p className="notice">Minutes-sensitive lean: this {p.side==='UNDER'?'Under':'Over'} falls to {fixed(100*r.workloadReview.baselineProbability)}% at the primary forecast of {fixed(r.workloadReview.baselineMinutes)} minutes. The displayed estimate assumes the V2 workload change. Both forecasts retain the same V2 production rates.</p>}
  {r.pregameLineups?.map(l=><div key={`${l.game_id}-${l.team_id}`}>
   <h4>{l.teamAbbr || 'Team'} · {l.status==='confirmed'?'Confirmed starters':'Expected starters'}</h4>
   <p className="small">Pregame report · {l.status==='confirmed'?'Provider-confirmed starting five.':'Expected lineup; it can change before tip-off.'}</p>
   <div className="context-starters">{l.lineup.map(s=><div key={s.player.id}><PlayerPortrait name={`${s.player.first_name} ${s.player.last_name}`} team={l.teamAbbr}/><span><small>{s.position}</small>{s.player.first_name} {s.player.last_name}</span></div>)}</div>
  </div>)}
  {effect?.applied&&<p className="small">Opportunity component used in this forecast: {effect.scale>=1?'+':''}{fixed(100*(effect.scale-1))}% to the per-minute rate. Experimental, fitted jointly with lineup composition.</p>}
  {signals.length>0&&<><h4>Opportunities behind the projection</h4><p className="small">Earlier full-game chances and conversion are measured separately. Missing tracking stays unavailable. These comparisons do not establish a reliable hit rate.</p><div className="availability-table-wrap"><table><thead><tr><th>Stat</th><th>Opportunity volume</th><th>Covered recent games</th><th>Historical conversion</th><th>Latest covered game</th></tr></thead><tbody>{signals.map(s=><tr key={s.stat}><td>{statLabel[s.stat]||s.stat}</td><td>{s.games>0?`${fixed(s.chances/s.games)} ${volume[s.stat]}/game`:'Unavailable'}</td><td>{s.games||0}{!s.available&&<small> · {s.reason||'Incomplete coverage'}</small>}</td><td>{s.available?(s.stat==='points'?`${fixed(s.conversion,2)} points/opportunity`:`${fixed(100*s.conversion)}%`):'Unavailable'}</td><td>{s.through||'Unavailable'}</td></tr>)}</tbody></table></div></>}
  {r.trackingProfile?.length>0&&<><h4>Earlier shooting and defense profile</h4><p className="small">Descriptive covered-game measurements; these percentages are not automatic defensive bonuses.</p><div className="availability-table-wrap"><table><thead><tr><th>Measure</th><th>Value</th><th>Covered games</th></tr></thead><tbody>{r.trackingProfile.map(m=><tr key={m.label}><td>{m.label}</td><td>{fixed(m.value)} {m.unit}</td><td>{m.games}</td></tr>)}</tbody></table></div></>}
  {game&&<><h4>Game competitiveness</h4>{game.applied?<p className="small">Projected workload adjustment: {game.minutesShift>=0?'+':''}{fixed(game.minutesShift)} minutes, based on earlier pregame spreads and player workloads. Experimental; a wide spread does not guarantee an early exit.</p>:<p className="small">{game.reason}.</p>}{game.largeMarginRisk!=null&&<p className="small">Historical estimate of a 15+ point final margin: {fixed(100*game.largeMarginRisk)}% ({game.riskGames} comparable games). This is not the probability that this player sits the fourth quarter.</p>}</>}
  {r.workloadScenarios?.length>0&&<><h4>Minutes sensitivity</h4><p className="small">Lower and higher scenarios use the model’s existing minutes distribution. The primary forecast, when shown, changes only workload while retaining V2 production rates. These are sensitivity checks, not confidence intervals or predictions of a particular game outcome.</p><div className="availability-table-wrap"><table><thead><tr><th>Scenario</th><th>Projected minutes</th><th>Stat projection</th><th>{p.side==='UNDER'?'Under':'Over'} estimate</th></tr></thead><tbody>{r.workloadScenarios.map(s=><tr key={s.player}><td>{s.player}</td><td>{fixed(s.minutes)}</td><td>{fixed(s.projection)}</td><td>{s.probability==null?'Unavailable':`${fixed(100*s.probability)}%`}</td></tr>)}</tbody></table></div></>}
 </section>
}
