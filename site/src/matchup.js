const parts={pra:['points','rebounds','assists'],pr:['points','rebounds'],pa:['points','assists'],ra:['rebounds','assists']}
const labels={points:'points',rebounds:'rebounds',assists:'assists',threes:'threes'}
function ordinal(n){const m=n%100;return `${n}${m>=11&&m<=13?'th':n%10===1?'st':n%10===2?'nd':n%10===3?'rd':'th'}`}
export function matchupSignal(p) {
 const c=p.matchupContext
 if(c){
  const stats=parts[p.stat]||[p.stat]
  const candidates=(c.position||[]).filter(r=>stats.includes(r.stat)&&r.of>=2).map(r=>{
   const fewest=r.rank<=(r.of+1)/2,n=fewest?r.rank:r.of-r.rank+1,helps=p.side==='UNDER'?fewest:!fewest
   return {helps,label:`${c.opponent.name} allows ${ordinal(n)}-${fewest?'fewest':'most'} ${labels[r.stat]||r.stat} to ${r.position}`,rank:r.rank}
  })
  return candidates.find(r=>!r.helps)||candidates[0]||null
 }
 return p.defense?{helps:p.defense.helps,label:p.defense.text}:null
}
export function defenseEffect(why='') {
 const m=why.match(/fitted defense-rate sensitivity ([+-][\d.]+)%/)
 if(m){const percent=Number(m[1]);return {active:true,percent,text:`The fitted opponent-defense inputs ${percent>0?'raise':percent<0?'lower':'leave unchanged'} the projected per-minute stat rate by ${Math.abs(percent).toFixed(1)}% versus league-neutral defense, with other inputs held fixed. This is before minutes adjustments and probability calibration; the matchup rank is not used as a direct multiplier.`}}
 if(why.includes('opponent-defense rate features are disabled'))return {active:false,text:'Opponent-defense rate inputs are disabled in this fitted model. The matchup ranking is context only and has not adjusted this forecast.'}
 return {active:null,text:'A measured defense adjustment is not available for this saved forecast. The matchup ranking alone does not establish how much the model used defense.'}
}
