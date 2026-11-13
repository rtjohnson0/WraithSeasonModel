import {useState} from 'react'
import {oddsText,shortDate} from './format.js'
import {gameRecordRows,summarizeGameRows} from './gameRecord.js'
const names={moneyline:'Moneyline',spread:'Spread',total:'Over / Under'}
export default function GameRecord({rows=[]}) {
 const [selected,setSelected]=useState(false)
 const list=gameRecordRows(rows,selected),summary=summarizeGameRows(list)
 const pick=r=>r.market==='total'?`${r.side} ${r.line}`:r.market==='spread'?`${r.side} ${(r.side===r.home?r.line:-r.line)>=0?'+':''}${r.side===r.home?r.line:-r.line}`:`${r.side} ML`
 return <section className="record game-record"><h2>Game-market record</h2>
 <div className="filters"><button className={!selected?'active':''} onClick={()=>setSelected(false)}>All model directions</button><button className={selected?'active':''} onClick={()=>setSelected(true)}>Selected plays only</button></div>
 <p className="small">The first valid pregame forecast per game and market is frozen with its exact book, line and price. Model directions include paper plays and leans; selected plays passed the probability, price and evidence rules at posting. Results use explicit matching final game scores. Units below are a flat 1-unit comparison, not money staked.</p>
 {!rows.length&&<p className="notice">Game-market tracking starts with the next pregame run. Earlier forecasts are not reconstructed from final results.</p>}
 <div className="scroll"><table><thead><tr><th>Market</th><th>Record</th><th>Hit rate</th><th>Flat units</th><th>Pending / review</th></tr></thead><tbody>{summary.map(s=><tr key={s.market}><th>{names[s.market]}</th><td>{s.wins}–{s.losses}{s.pushes?` · ${s.pushes} push`:''}</td><td>{s.hitRate===null?'—':`${(100*s.hitRate).toFixed(1)}%`}</td><td>{s.units.toFixed(2)}</td><td>{s.pending}</td></tr>)}</tbody></table></div>
 {list.length>0&&<div className="scroll"><table><thead><tr><th>Date / game</th><th>Frozen direction</th><th>Type</th><th>Model</th><th>Final score</th><th>Result</th></tr></thead><tbody>{list.slice(0,120).map(r=><tr key={`${r.date}/${r.game_id}/${r.market}`}><td>{shortDate(r.date)} · {r.away} @ {r.home}</td><td>{pick(r)} {oddsText(r.odds)}<small>{r.book}</small></td><td>{r.tier==='PICK'?'Selected':r.tier==='PAPER'?'Paper':r.tier==='CHECK'?'Check lineup':'Lean'}</td><td>{(100*r.prob).toFixed(1)}%</td><td>{['WIN','LOSS','PUSH'].includes(r.result)?`${r.away} ${r.awayScore} · ${r.home} ${r.homeScore}`:'—'}</td><td>{r.result}{r.reason&&<small>{r.reason}</small>}</td></tr>)}</tbody></table></div>}
 </section>
}
