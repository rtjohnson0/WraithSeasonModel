// Only contemporaneous, explicitly captured research fields are allowed.
// Never extrapolate Wraith's pregame probability into a halftime probability.
export function liveFresh(report, now=Date.now(), maxAge=8*60000) {
 if (!report || report.schema!==1 || !Array.isArray(report.plays)) return false
 const captured=Date.parse(report.capturedAt)
 return Number.isFinite(captured) && captured<=now+60000 && now-captured<=maxAge
}
export function liveBookConfidence(q) {
 if (!q) return null
 const over=Number(q.overMarketProb),under=Number(q.underMarketProb)
 if (!Number.isFinite(over)||!Number.isFinite(under)||over<=0||under<=0||Math.abs(over+under-1)>.012)return null
 return {over,under}
}
export function liveRows(rows,sort='percentage',filter='all') {
 const allowed=(rows||[]).filter(p=>p && typeof p.player==='string' && p.gameId>0 && (filter!=='half'||/half/i.test(p.gameStatus||'')))
 return [...allowed].sort((a,b)=>{
  if(sort==='remaining')return (a.hasLiveStats?a.remainingToOver:Infinity)-(b.hasLiveStats?b.remainingToOver:Infinity)
  if(sort==='game')return String(a.matchup||'').localeCompare(String(b.matchup||''))
  const odds=x=>{const p=liveBookConfidence(x.live);return p?Math.max(p.over,p.under):-1}
  return odds(b)-odds(a)
 })
}
