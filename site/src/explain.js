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
 if (p.defense) (p.defense.helps ? support : against).push(`${p.defense.text}. ${p.defense.helps ? 'This supports' : 'This works against'} the ${side}. A ranking is context, not a direct multiplier on his projection.`)
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
 return { decision: `${side} ${p.line} is the shown side at ${(100*p.prob).toFixed(1)}%. The final probability also reflects the book/calibration; the projection alone does not determine the bet.`, support, against, sensitivity, fitted }
}
export function explainGame(game, quote) {
 const winner = game.margin >= 0 ? game.home : game.away
 const byProbability = quote.sides.reduce((a,b) => b.conditional > a.conditional ? b : a)
 const byValue = quote.sides.reduce((a,b) => b.ev > a.ev ? b : a)
 const choice = byValue.ev > 0 && byValue.edge > 0 ? `${byValue.label}${quote.market === 'total' ? ` ${quote.line}` : ' ML'} — paper value lean` : 'Pass at these prices'
 const why = quote.market === 'total'
  ? `Projected total ${num(game.total)} versus ${quote.line}: the central projection leans ${game.total > quote.line ? 'Over' : game.total < quote.line ? 'Under' : 'neither side'}. The distribution gives ${byProbability.label} ${(byProbability.conditional*100).toFixed(1)}% excluding pushes.`
  : `Projected winner ${winner}, by ${num(Math.abs(game.margin))} points. ${byProbability.label} has the higher model win probability (${(byProbability.conditional*100).toFixed(1)}%). The more likely winner is not automatically the better price.`
 const context = quote.market === 'total'
  ? Number.isFinite(game.ratingsTotal) ? `Team ratings alone project ${num(game.ratingsTotal)} total points; the book consensus is ${num(game.marketTotal)}. Their fitted blend produces ${num(game.total)}, before comparing this book's exact ${quote.line} total.` : ''
  : Number.isFinite(game.ratingsMargin) ? `Team ratings alone project a home margin of ${num(game.ratingsMargin)}; the market consensus implies ${num(game.marketMargin)}. The fitted blend gives ${num(game.margin)}. Positive margins favor ${game.home}.` : ''
 return {choice,why,context,price:`The best model return at this book is ${byValue.label}: ${(100*byValue.conditional).toFixed(1)}% versus ${(100*byValue.breakEven).toFixed(1)}% needed at the quoted price. ${byValue.ev > 0 ? 'A positive forecast still needs validation.' : 'Neither side has positive forecast value, so there is no price-based pick.'}`}
}
