import { pct, statLabel } from './format.js'

export default function Availability({ p }) {
 const c=p.availability
 if (!c || c.version!==1) return null
 const side=p.side==='UNDER'?'Under':'Over'
 return <section className="availability" aria-label="Injury and minutes comparison">
  <h4>Availability &amp; minutes</h4>
  <p className="small">Current projection: <b>{c.baselineMinutes.toFixed(1)} minutes</b>. {c.asOf && <>Injury feed checked {new Date(c.asOf).toLocaleTimeString('en-US',{hour:'numeric',minute:'2-digit',timeZone:'America/New_York'})} ET.</>}</p>
  {c.ownStatus && <p className="availability-alert">{p.player} is listed {c.ownStatus}. This projection assumes he plays; a minutes restriction is not modeled in this comparison. Confirm his availability.</p>}
  {(c.teammates||[]).map(e=>{
   const delta=e.withoutAvg-e.withAvg
   const probabilities=Number.isFinite(e.withProbability)&&Number.isFinite(e.withoutProbability)
   return <article className="availability-person" key={e.playerId}>
    <div className="availability-heading"><b>{e.player}</b><span>{e.status}</span></div>
    {e.reportedAt && <p className="small">BALLDONTLIE injury feed · captured {new Date(e.reportedAt).toLocaleTimeString('en-US',{hour:'numeric',minute:'2-digit',timeZone:'America/New_York'})} ET</p>}
    {!e.confirmedOut && <p className="small">His participation is unresolved. {e.assumedOut?'This run treats him as unavailable; his report is still uncertain.':'A questionable or doubtful listing is not a confirmed absence.'}</p>}
    {!e.supported ? <p className="small">Not enough comparable history to quantify a minutes change: {e.withN} games together and {e.withoutN} games without recorded playing time. No numerical boost or percentage adjustment is assigned.</p> : <>
     <p>{p.player} averaged <b>{e.withoutAvg.toFixed(1)} minutes</b> when {e.player} did not play, compared with <b>{e.withAvg.toFixed(1)}</b> when both played. {Math.abs(delta)>=.5 ? `That is ${Math.abs(delta).toFixed(1)} ${delta>0?'more':'fewer'} minutes in those past games.` : 'Those past averages are similar.'}</p>
     <div className="availability-table-wrap"><table><caption>Minutes-only comparison at {side} {p.line} {statLabel[p.stat]||p.stat}</caption><thead><tr><th>Scenario</th><th>Minutes used</th><th>Stat projection</th>{probabilities&&<th>{side} estimate</th>}</tr></thead><tbody>
      <tr><th>At minutes from games together</th><td>{e.withMinutes.toFixed(1)}</td><td>{e.withProjection.toFixed(1)}</td>{probabilities&&<td>{pct(e.withProbability,1)}</td>}</tr>
      <tr><th>At minutes from games he did not play</th><td>{e.withoutMinutes.toFixed(1)}</td><td>{e.withoutProjection.toFixed(1)}</td>{probabilities&&<td>{pct(e.withoutProbability,1)}</td>}</tr>
     </tbody></table></div>
     {probabilities && Math.abs(e.withoutProbability-e.withProbability)>=.005 && <p className="availability-direction">At those historical minutes, his absence {e.withoutProbability>e.withProbability?'supports':'works against'} this {side.toLowerCase()} by {Math.abs(100*(e.withoutProbability-e.withProbability)).toFixed(1)} percentage points.{e.reverses?' The preferred side changes between these scenarios.':''}</p>}
     <p className="small">History: {e.withN} games together · {e.withoutN} without recorded playing time · through {e.through}. Games without playing time do not establish the reason for an absence.</p>
     {e.samples?.length>0 && <details className="availability-history"><summary>View the games used</summary><div className="availability-table-wrap"><table><thead><tr><th>Date</th><th>Teammate played</th><th>Minutes</th></tr></thead><tbody>{e.samples.map(g=><tr key={g.gameId}><td>{g.date}</td><td>{g.teammatePlayed?'Yes':'No'}</td><td>{g.minutes.toFixed(1)}</td></tr>)}</tbody></table></div></details>}
     <p className="small">These use observed past minutes, holding production rates and today’s line and prices fixed. They are sensitivity estimates, not a validated injury-adjusted forecast. Other absences are not added together.{!probabilities?' A probability comparison is unavailable for this market.':''}</p>
    </>}
   </article>
  })}
 </section>
}
