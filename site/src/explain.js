import { statLabel } from './format.js'
const num = (n) => Number.isFinite(n) ? n.toFixed(1) : 'unavailable'
export function explainProp(p) {
 const under = p.side === 'UNDER', side = under ? 'Under' : 'Over'
 const support = [], against = []
 const gap = p.projStat - p.line
 const projectedSide = gap < 0 ? 'Under' : 'Over'
 ;(projectedSide === side ? support : against).push(`The ${p.period === '1q' ? '1Q' : 'full-game'} projection is ${num(p.projStat)} ${statLabel[p.stat] || p.stat}, ${num(Math.abs(gap))} ${gap < 0 ? 'below' : 'above'} the ${p.line} line.`)
 const typical = p.minTypical || p.minAvg5
 if (Number.isFinite(typical) && p.projMin > 0) {
  const diff = p.projMin - typical
  if (Math.abs(diff) >= 2) (under === (diff < 0) ? support : against).push(`Minutes are projected at ${num(p.projMin)}, versus ${num(typical)} normally. ${diff < 0 ? 'Less court time reduces opportunities.' : 'More court time adds opportunities.'}`)
 }
 if (!p.matchupContext && p.defense) (p.defense.helps ? support : against).push(`${p.defense.text}. ${p.defense.helps ? 'This supports' : 'This works against'} the ${side}. A ranking is context, not a direct multiplier on his projection.`)
 for (const text of positionReasons(p)) (text.helps ? support : against).push(text.text)
 if (p.last5?.length) {
  const n = p.last5.length, hits = p.l5Hits || 0
  ;(hits / n > .5 ? support : against).push(`This exact side cleared in ${hits} of his last ${n} full games. That small sample is context, not a win-probability estimate.`)
 }
 let sensitivity = ''
 if (Number.isFinite(typical) && p.projMin > 0 && Math.abs(typical-p.projMin) >= 2) {
  const adjusted = p.projStat * typical / p.projMin
  if ((adjusted > p.line) !== (p.projStat > p.line)) sensitivity = `Minutes-sensitive: at ${num(typical)} minutes and the same projected rate, the implied total would be ${num(adjusted)}, on the other side of the line. This is a sensitivity check, not a second forecast.`
 }
 const fitted = (p.why || '').split(';').map((s) => s.trim()).filter((s) => s.startsWith('fitted defense-rate') || s.startsWith('opponent-defense rate') || s.startsWith('largest fitted minutes') || s.startsWith('injury feature policy'))
 return { decision: `${side} ${p.line} is the shown side at ${(100*p.prob).toFixed(1)}%. ${p.probabilityBasis === "estimate" ? "This is an uncalibrated simulation estimate, not a validated win probability." : "The final probability also reflects the book/calibration; the projection alone does not determine the bet."}`, support, against, sensitivity, fitted }
}
// Derive price math from the selected book's exact odds, never a cached label.
export function gameSide(s) {
 const odds=Number(s.odds), p=Number(s.conditional)
 const be=odds>0?100/(100+odds):odds<0?-odds/(100-odds):1
 const profit=odds>0?odds/100:odds<0?100/-odds:0
 return {...s,breakEven:be,edge:p-be,ev:(1-(s.push||0))*(p*profit-(1-p))}
}
function fairPrice(p) { return p >= .5 ? `−${Math.floor(100*p/(1-p))}` : `+${Math.ceil(100*(1-p)/p)}` }
export function explainGame(game, quote) {
 const sides=quote.sides.map(gameSide)
 const primary=sides.reduce((a,b)=>b.conditional>a.conditional?b:a)
 const other=sides.find((s)=>s!==primary)
 const moneyline=quote.market==='moneyline'
 const spread=quote.market==='spread'
 const handicap=primary.label===game.home?quote.line:-quote.line
 const choice=`${primary.label}${moneyline?' ML':spread?` ${handicap>=0?'+':''}${handicap}`:` ${quote.line}`}`
 const value=primary.edge>0 && primary.ev>0
 const shortPrice=value?`Positive model value · +${(primary.edge*100).toFixed(1)} pp`:`Model lean · wait for ${fairPrice(primary.conditional)} or better`
 const why=spread?`Projected ${game.margin>=0?game.home:game.away} by ${num(Math.abs(game.margin))} points. ${choice} has ${(primary.conditional*100).toFixed(1)}% model cover probability excluding pushes.`:moneyline?`Projected winner ${primary.label}, with ${(primary.conditional*100).toFixed(1)}% model win probability. Projected score: ${game.away} ${num(game.awayScore)}–${game.home} ${num(game.homeScore)}.`:`Projected total ${num(game.total)} versus ${quote.line}. The model gives ${primary.label} ${(primary.conditional*100).toFixed(1)}% excluding pushes.`
 const price=`${primary.label} needs ${(primary.breakEven*100).toFixed(1)}% to break even at this book; the model gives ${(primary.conditional*100).toFixed(1)}%. ${value?'That clears the price by '+(primary.edge*100).toFixed(1)+' percentage points.':'The pick stays '+choice+', but this price costs more than the model supports.'}`
 const alternative=other?.ev>0?`${other.label} has a positive payout-based estimate at this price, but a lower chance of winning (${(other.conditional*100).toFixed(1)}%). It is not the main model pick.`:''
 const context=moneyline?Number.isFinite(game.ratingsMargin)?`Team ratings and market context combine to project a ${num(Math.abs(game.margin))}-point ${game.margin>=0?game.home:game.away} win.`:'':Number.isFinite(game.totalLow)?`Estimated total range: ${Math.round(game.totalLow)}–${Math.round(game.totalHigh)} points. This is an approximate model range.`:''
 const factors=gameFactors(game,quote,primary)
 return {choice,why,price,context,alternative,primary,sides,shortPrice,...factors}
}
export function bestGameQuote(game, quotes, now=Date.now()) {
 if(!quotes.length)return null
 const fresh=quotes.filter(q=>Date.parse(q.quoteAt)>now-1800000 && Date.parse(q.tip)>now)
 const pool=fresh.length?fresh:quotes
 // Compare the same likely winner/total direction across all available books.
 // At each book, evaluate that quote's higher-probability direction. Spread/total directions can change with the line.
 return [...pool].sort((a,b)=>{
  const score=q=>{const s=[...q.sides].sort((a,b)=>b.conditional-a.conditional)[0];return s?gameSide(s).ev:-Infinity}
  return score(b)-score(a)||String(a.book).localeCompare(String(b.book))
 })[0]
}

// Rank 1 allows the fewest. Each combo uses its actual component stats.
const components={pra:['points','rebounds','assists'],pr:['points','rebounds'],pa:['points','assists'],ra:['rebounds','assists']}
export function positionReasons(p) {
 const c=p.matchupContext;if(!c)return []
 const stats=components[p.stat]||[p.stat]
 return (c.position||[]).filter(r=>stats.includes(r.stat)).map(r=>{
  const lower=r.rank<=(r.of+1)/2, helps=p.side==='UNDER'?lower:!lower
  return {helps,text:`${c.opponent.name} ranks #${r.rank} of ${r.of} in fewest ${statLabel[r.stat]||r.stat} allowed to ${r.position}: ${r.average.toFixed(1)} per game across ${r.games} games. ${helps?'This supports':'This works against'} the ${p.side==='UNDER'?'Under':'Over'}. This is full-game position-group context, not an individual defender rating.`}
 })
}

export function gameFactors(game,quote,primary) {
 const support=[],against=[],context=[]
 const add=(r,t,label,helps)=>{if(!r)return;const text=`${t.name} ranks #${r.rank} of ${r.of} ${label}: ${r.average.toFixed(1)} per game (${r.games} games).`;if(helps===null)context.push(text);else(helps?support:against).push(text)}
 const offense=t=>t?.scoring, defense=t=>t?.allowed?.find(r=>r.stat==='points')
 const decisive=r=>r.rank<=r.of/3?-1:r.rank>2*r.of/3?1:0
 if(quote.market==='total') {
  const over=primary.label==='Over'
  for(const t of [game.homeContext,game.awayContext].filter(Boolean)) {
   const o=offense(t),d=defense(t)
   if(o)add(o,t,'in scoring',decisive(o)===0?null:over===(decisive(o)<0))
   if(d)add(d,t,'in fewest points allowed',decisive(d)===0?null:over===(decisive(d)>0))
  }
 }else {
  const mine=primary.label===game.home?game.homeContext:game.awayContext
  const opp=primary.label===game.home?game.awayContext:game.homeContext
  if(mine){const o=offense(mine),d=defense(mine);if(o)add(o,mine,'in scoring',decisive(o)===0?null:decisive(o)<0);if(d)add(d,mine,'in fewest points allowed',decisive(d)===0?null:decisive(d)<0)}
  if(opp){const d=defense(opp);if(d)add(d,opp,'in fewest points allowed',decisive(d)===0?null:decisive(d)>0)}
 }
 return {support,against,rankContext:context}
}
