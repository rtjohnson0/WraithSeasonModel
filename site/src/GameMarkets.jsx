import { useState } from 'react'
import { visibleQuote, quoteState, bookName, clock } from './quote.js'
import { useNow } from './time.js'
import { explainGame } from './explain.js'
import { oddsText } from './format.js'

const pc = (p) => `${(p * 100).toFixed(1)}%`
const signed = (n) => `${n >= 0 ? '+' : ''}${n.toFixed(1)}`
function Market({ game, market, now }) {
  const quotes = game.quotes.filter((q) => q.market === market && visibleQuote(q))
  const [book, setBook] = useState('')
  const q = quotes.find((q) => q.book === book) || quotes[0]
  if (!q) return <p className="small">No current {market === 'total' ? 'total' : 'moneyline'} quote passed freshness and book checks.</p>
  const explanation = explainGame(game,q)
  return <div className="game-market">
    <div className="game-market-heading"><h4>{market === 'total' ? `Game total · ${q.line}` : 'Moneyline'}</h4>
      <select aria-label={`${game.away} at ${game.home} ${market} book`} value={q.book} onChange={(e) => setBook(e.target.value)}>{quotes.map((x) => <option key={x.book} value={x.book}>{bookName(x.book)}</option>)}</select>
    </div>
    <p className="market-choice"><b>{explanation.choice}</b></p>
    <p>{explanation.why}</p>{explanation.context && <p className="small">{explanation.context}</p>}<p className="small">{explanation.price}</p>
    <div className="booktable"><table><thead><tr><th>Side / price</th><th>Model</th><th>Break-even</th><th>Book, no vig</th><th>Edge</th><th>Model ROI</th></tr></thead><tbody>
      {q.sides.map((s) => <tr key={s.label}><td><b>{s.label}{market === 'total' ? ` ${q.line}` : ''}</b> {oddsText(s.odds)}</td><td>{pc(s.conditional)}</td><td>{pc(s.breakEven)}</td><td>{pc(s.market)}</td><td>{signed(s.edge * 100)} pp</td><td>{signed(s.ev * 100)}%</td></tr>)}
    </tbody></table></div>
    {q.sides[0].push > 0 && <p className="small">Estimated push chance: {pc(q.sides[0].push)}. Model and edge columns exclude pushes; model ROI includes refunds.</p>}
    <p className="small">{quoteState(q, now).label} · Checked {clock(q.quoteAt)} · source updated {clock(q.sourceUpdatedAt)} · {q.bookCount} books.</p>
  </div>
}
export default function GameMarkets({ data }) {
  const now = useNow()
  if (!data) return <section className="game-markets"><h2>Moneyline & game totals</h2><p>Run the game model to publish score projections and book-specific moneyline / Over–Under comparisons.</p></section>
  const games = (data.games || [])
  return <section className="game-markets">
    <div className="game-market-heading"><h2>Moneyline & game totals</h2><span className="quote-badge">Paper tracking</span></div>
    <p className="small">Game model run: {clock(data.at)}. These estimates blend team ratings with market lines. They do not include confirmed lineups. A positive model edge alone does not establish a profitable strategy.</p>
    {!games.length && <p>No pregame projections available from this run.</p>}
    {games.map((g) => <article className="game-market-card" key={g.id}>
      <h3>{g.away} @ {g.home} <small>{clock(g.tip)}</small></h3>
      <div className="projection-summary"><span>Projected score <b>{g.away} {g.awayScore.toFixed(1)} · {g.home} {g.homeScore.toFixed(1)}</b></span><span>Projected total <b>{g.total.toFixed(1)}</b></span><span>Estimated 80% total range <b>{g.totalLow.toFixed(0)}–{g.totalHigh.toFixed(0)}</b></span></div>
      <p className="small">The range uses historical residual spread and is an approximate model range, not a calibrated guarantee.</p>
      <Market game={g} market="moneyline" now={now} /><Market game={g} market="total" now={now} />
    </article>)}
    <details><summary>How to read these numbers</summary><p className="small">Break-even comes from the exact offered odds. Book, no vig removes the two-sided price margin. Edge is model probability minus break-even, in percentage points. Model ROI is expected profit per unit at that price; it is a forecast, not historical performance. Changing books or the total changes the comparison. API snapshots can lag the sportsbook.</p>
    <p className="small">Backtest evidence (z; roughly 2 or higher suggests an edge in that sample): moneyline {Number.isFinite(data.z?.moneyline) ? data.z.moneyline.toFixed(2) : 'unavailable'} · totals {Number.isFinite(data.z?.total) ? data.z.total.toFixed(2) : 'unavailable'}. These cards remain paper tracking.</p></details>
  </section>
}
