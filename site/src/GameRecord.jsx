import {useState} from 'react'
import {oddsText,shortDate} from './format.js'
import {gameRecordRows,summarizeGameRows} from './gameRecord.js'
const names={moneyline:'Moneyline',spread:'Spread',total:'Over / Under'}
const outcomes={WIN:'Hit',LOSS:'Miss',PUSH:'Push',PENDING:'Pending',REVIEW:'In review',VOID:'Void'}
const pick=r=>r.market==='total'?`${r.side} ${r.line}`:r.market==='spread'?`${r.side} ${(r.side===r.home?r.line:-r.line)>=0?'+':''}${r.side===r.home?r.line:-r.line}`:`${r.side} ML`
export default function GameRecord({rows=[]}) {
 const [selected,setSelected]=useState(false)
 const [date,setDate]=useState('')
 const [limit,setLimit]=useState(12)
 const list=gameRecordRows(rows,selected),summary=summarizeGameRows(list)
 const dates=[...new Set(list.map(r=>r.date))].sort().reverse()
 const completedDates=[...new Set(list.filter(r=>['WIN','LOSS','PUSH'].includes(r.result)).map(r=>r.date))].sort().reverse()
 const activeDate=date==='ALL'?'ALL':dates.includes(date)?date:(completedDates[0]||dates[0])
 const visible=list.filter(r=>activeDate==='ALL'||r.date===activeDate).sort((a,b)=>b.date.localeCompare(a.date)||`${a.away}${a.home}`.localeCompare(`${b.away}${b.home}`)||a.market.localeCompare(b.market))
 return <section className="record game-record" aria-label="Game market results">
  <div className="record-section-head"><div><span className="card-eyebrow">THE GAME MARKETS</span><h2>Beyond the props</h2><p>Moneylines, spreads and totals. Tracked separately.{list.some(r=>r.policy==='game-v2') ? ' Game model v2.' : ''}</p></div></div>
  <div className="record-controls">
   <div className="record-result-filter" role="group" aria-label="Game record scope"><button aria-pressed={!selected} onClick={()=>{setSelected(false);setLimit(12)}}>All model directions</button><button aria-pressed={selected} onClick={()=>{setSelected(true);setLimit(12)}}>Selected plays only</button></div>
  </div>
  <p className="small">Summary across all dates · {selected?'Selected plays only':'Includes paper plays and leans'} · flat 1-unit comparisons, not money staked.</p>
  <div className="game-record-summary">{summary.map(s=><article key={s.market}><span className="card-eyebrow">{names[s.market]}</span><strong>{s.wins}<i>–</i>{s.losses}</strong><div><span>{s.hitRate===null?'—':`${(100*s.hitRate).toFixed(1)}%`} hit rate</span><span className={s.units>0?'up':s.units<0?'down':''}>{s.units>0?'+':''}{s.units.toFixed(2)}u flat</span></div><small>{s.pending} pending / review{s.pushes?` · ${s.pushes} push`:''}</small></article>)}</div>
  <div className="record-section-head game-results-heading"><h3>Game results</h3><label>Date<select value={activeDate||''} onChange={e=>{setDate(e.target.value);setLimit(12)}}>{dates.map(d=><option key={d} value={d}>{shortDate(d)}</option>)}<option value="ALL">All available dates</option></select></label></div>
  {!visible.length&&<p className="notice">{!rows.length?'Game-market tracking starts with the next pregame run.':'No game results match this view.'}</p>}
  <div className="game-result-grid">{visible.slice(0,limit).map(r=><article className={`game-result ${r.result?.toLowerCase()}`} key={`${r.date}/${r.game_id}/${r.market}`}>
    <div className="game-result-top"><span>{r.away} <i>@</i> {r.home}</span><span className="outcome">{outcomes[r.result]||r.result}</span></div>
    <span className="game-result-market">{shortDate(r.date)} · {names[r.market]}</span>
    <h4>{pick(r)}</h4>
    <div className="game-result-meta"><span>{oddsText(r.odds)} · {r.book}</span><span>{r.tier==='PICK'?'Selected':r.tier==='PAPER'?'Paper':r.tier==='CHECK'?'Check lineup':'Lean'}{r.tier!=='PICK'?' · not staked':''}</span></div>
    <div className="game-result-score"><span>{['WIN','LOSS','PUSH'].includes(r.result)?`${r.away} ${r.awayScore} – ${r.home} ${r.homeScore}`:r.result==='REVIEW'?'Awaiting verified result':'Awaiting final score'}</span><small>Model {(100*r.prob).toFixed(1)}%</small></div>
    {r.reason&&<p className="small">{r.reason}</p>}
  </article>)}</div>
  {visible.length>limit&&<button className="record-more" onClick={()=>setLimit(n=>n+12)}>Show more game results ({visible.length-limit} remaining)</button>}
  <details className="record-method"><summary>How game results are counted</summary><p className="small">The first valid pregame forecast per game and market is frozen with its exact book, line and price. Model directions include paper plays and leans; selected plays passed the probability, price and evidence rules at posting. Results require matching final game scores. Earlier forecasts are not reconstructed from finals. Pending and review rows do not count as wins or losses.</p></details>
 </section>
}
