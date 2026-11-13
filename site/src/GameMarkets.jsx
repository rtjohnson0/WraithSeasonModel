import { useState } from 'react'
import { visibleQuote, quoteState, bookName, clock } from './quote.js'
import { useNow } from './time.js'
import { explainGame, bestGameQuote } from './explain.js'
import TeamRanks from './TeamRanks.jsx'
import { oddsText } from './format.js'

const pc = (p) => `${(p * 100).toFixed(1)}%`
const signed = (n) => `${n >= 0 ? '+' : ''}${n.toFixed(1)}`
function Market({ game, market, now, evidence }) {
  const quotes = (game.quotes || []).filter((q) => q.market === market && visibleQuote(q))
  const [book, setBook] = useState('')
  const q = quotes.find((q) => q.book === book) || bestGameQuote(game, quotes, now)
  if (!q) return null
  const e = explainGame(game,q)
  const qualifies=e.primary.conditional>=.55 && e.primary.edge>=.01 && (evidence?.[market]||0)>=2
  const far=market==='total'?Math.abs(game.total-game.marketTotal)>7:Math.abs(game.margin-game.marketMargin)>4
  const status=qualifies&&!far?'Selected play':e.primary.edge>0?'Value lean':'Model direction'
  return <details className="market-play">
    <summary><span><small>{market === 'total' ? 'Game total' : market === 'spread' ? 'Spread' : 'Moneyline'}</small><b>{e.choice}</b><small>{status} · {e.shortPrice}</small><span className="expand-hint">View analysis <span aria-hidden="true">⌄</span></span></span><span className="market-prob">{pc(e.primary.conditional)}<small>{oddsText(e.primary.odds)} · {bookName(q.book)}</small></span></summary>
    <div className="market-expanded"><div className="game-margin-chart"><span>{game.away} {game.awayScore.toFixed(1)}</span><div><i style={{width:`${100*game.awayScore/(game.homeScore+game.awayScore)}%`}}/></div><span>{game.home} {game.homeScore.toFixed(1)}</span></div>
      <div className="game-market-heading"><h4>Pick breakdown</h4><select aria-label={`${game.away} at ${game.home} ${market} book`} value={q.book} onChange={(ev) => setBook(ev.target.value)}>{quotes.map((x) => <option key={x.book} value={x.book}>{bookName(x.book)}</option>)}</select></div>
      <p>{e.why}</p><h4>What drives the projection</h4><p className="small">{Number.isFinite(game.ratingsMargin) ? `Team ratings: ${game.ratingsMargin>=0?game.home:game.away} by ${Math.abs(game.ratingsMargin).toFixed(1)}. ` : ""}{Number.isFinite(game.marketMargin) ? `Book consensus: ${game.marketMargin>=0?game.home:game.away} by ${Math.abs(game.marketMargin).toFixed(1)}. ` : ""}The fitted blend projects {game.margin>=0?game.home:game.away} by {Math.abs(game.margin).toFixed(1)} and {game.total.toFixed(1)} total points.</p><p className="small">Ranks below describe past full-game production; game probabilities use team ratings and the market blend. Position matchups are shown with player props. Injuries and starting lineups may change before tip.</p><p>{e.price}</p>{!qualifies&&<p className="small">A selected play needs at least 55% model probability, a 1 percentage point edge over the price and demonstrated market-level evidence. The model direction remains available for analysis when those checks do not clear.</p>}{e.alternative && <p className="small">{e.alternative}</p>}
      {(e.support.length>0||e.against.length>0)&&<div className="reason-columns"><div><h4>What supports this direction</h4>{e.support.length?<ul>{e.support.map((v,i)=><li key={i}>{v}</li>)}</ul>:<p className="small">No strong ranking signal in the available sample.</p>}</div><div><h4>What works against it</h4>{e.against.length?<ul>{e.against.map((v,i)=><li key={i}>{v}</li>)}</ul>:<p className="small">No strong opposing ranking signal. That does not make the pick certain.</p>}</div></div>}{e.rankContext.map((v,i)=><p className="small" key={i}>{v}</p>)}
      {e.context && <p className="small">{e.context}</p>}
      <div className="booktable"><table><thead><tr><th>Side / price</th><th>Model</th><th>Break-even</th><th>Edge</th><th>Expected return</th></tr></thead><tbody>
        {e.sides.map((s) => <tr key={s.label}><td><b>{s.label}{market === 'total' ? ` ${q.line}` : market === 'spread' ? ` ${(s.label===game.home?q.line:-q.line)>=0?'+':''}${s.label===game.home?q.line:-q.line}` : ''}</b> {oddsText(s.odds)}</td><td>{pc(s.conditional)}</td><td>{pc(s.breakEven)}</td><td>{signed(s.edge * 100)} pp</td><td>{signed(s.ev * 100)}%</td></tr>)}
      </tbody></table></div>
      <p className="small">Checked {clock(q.quoteAt)} · {quoteState(q, now).label}. Prices may change.</p>
    </div>
  </details>
}
export default function GameMarkets({ data }) {
 const now=useNow()
 if (!data?.games?.length) return null
 return <section className="game-markets"><div className="game-market-heading"><h2>Moneyline, spreads & totals</h2></div><p className="small">Model picks and score projections · updated {clock(data.at)}. Tap a pick for the numbers behind it.</p>
 {data.games.map((g) => <article className="game-market-card" key={g.id}><header className="matchup-header"><div><span className="card-eyebrow">NBA · Full game</span><h3><span>{g.away}</span><small>AT</small><span>{g.home}</span></h3></div><span className="tip-chip">{clock(g.tip)}</span></header>
 <div className="projection-summary"><span>Projected score<b>{g.away} {g.awayScore.toFixed(1)} · {g.home} {g.homeScore.toFixed(1)}</b></span><span>Projected total<b>{g.total.toFixed(1)}</b></span></div>
 <details className="team-context"><summary>Team comparison & defensive rankings <span aria-hidden="true">⌄</span></summary><TeamRanks teams={[g.awayContext,g.homeContext]}/></details><Market game={g} market="moneyline" now={now} evidence={data.z}/><Market game={g} market="spread" now={now} evidence={data.z}/><Market game={g} market="total" now={now} evidence={data.z}/></article>)}
 </section>
}
