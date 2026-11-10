import { useState } from 'react'
import { visibleQuote, quoteState, bookName, clock } from './quote.js'
import { useNow } from './time.js'
import { explainGame, bestGameQuote } from './explain.js'
import { oddsText } from './format.js'

const pc = (p) => `${(p * 100).toFixed(1)}%`
const signed = (n) => `${n >= 0 ? '+' : ''}${n.toFixed(1)}`
function Market({ game, market, now }) {
  const quotes = (game.quotes || []).filter((q) => q.market === market && visibleQuote(q))
  const [book, setBook] = useState('')
  const q = quotes.find((q) => q.book === book) || bestGameQuote(game, quotes, now)
  if (!q) return null
  const e = explainGame(game,q)
  return <details className="market-play">
    <summary><span><small>{market === 'total' ? 'Game total' : 'Moneyline'}</small><b>{e.choice}</b><small>{e.shortPrice}</small></span><span className="market-prob">{pc(e.primary.conditional)}<small>{oddsText(e.primary.odds)} · {bookName(q.book)}</small></span></summary>
    <div className="market-expanded">
      <div className="game-market-heading"><h4>Pick breakdown</h4><select aria-label={`${game.away} at ${game.home} ${market} book`} value={q.book} onChange={(ev) => setBook(ev.target.value)}>{quotes.map((x) => <option key={x.book} value={x.book}>{bookName(x.book)}</option>)}</select></div>
      <p>{e.why}</p><p>{e.price}</p>{e.alternative && <p className="small">{e.alternative}</p>}
      {e.context && <p className="small">{e.context}</p>}
      <div className="booktable"><table><thead><tr><th>Side / price</th><th>Model</th><th>Break-even</th><th>Edge</th><th>Expected return</th></tr></thead><tbody>
        {e.sides.map((s) => <tr key={s.label}><td><b>{s.label}{market === 'total' ? ` ${q.line}` : ''}</b> {oddsText(s.odds)}</td><td>{pc(s.conditional)}</td><td>{pc(s.breakEven)}</td><td>{signed(s.edge * 100)} pp</td><td>{signed(s.ev * 100)}%</td></tr>)}
      </tbody></table></div>
      <p className="small">Checked {clock(q.quoteAt)} · {quoteState(q, now).label}. Prices may change.</p>
    </div>
  </details>
}
export default function GameMarkets({ data }) {
 const now=useNow()
 if (!data?.games?.length) return null
 return <section className="game-markets"><div className="game-market-heading"><h2>Moneyline & game totals</h2></div><p className="small">Model picks and score projections · updated {clock(data.at)}. Tap a pick for the numbers behind it.</p>
 {data.games.map((g) => <article className="game-market-card" key={g.id}><h3>{g.away} @ {g.home} <small>{clock(g.tip)}</small></h3>
 <div className="projection-summary"><span>Projected score<b>{g.away} {g.awayScore.toFixed(1)} · {g.home} {g.homeScore.toFixed(1)}</b></span><span>Projected total<b>{g.total.toFixed(1)}</b></span></div>
 <Market game={g} market="moneyline" now={now}/><Market game={g} market="total" now={now}/></article>)}
 </section>
}
