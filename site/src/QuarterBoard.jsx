import { useState } from 'react'
import { useNow } from './time.js'
import { visibleQuote, quoteState, bookName, clock } from './quote.js'
import { oddsText } from './format.js'

const pct = (n) => `${(n * 100).toFixed(1)}%`
function QuarterCard({ cards, now }) {
  const [book, setBook] = useState('')
  const p = cards.find((p) => p.book === book) || cards[0]
  const c = p.comparison
  const counts = (p.side === 'UNDER' ? c.under : c.over)
  return <article className="game-market-card quarter-card">
    <div className="game-market-heading"><h3>{p.player} <small>{p.matchup}</small></h3><select aria-label={`${p.player} 1Q book`} value={p.book} onChange={(e) => setBook(e.target.value)}>{cards.map((x) => <option key={x.book} value={x.book}>{bookName(x.book)}</option>)}</select></div>
    <h4>1Q · Points · {p.line}</h4>
    <p>Over {p.line} {oddsText(p.overOdds)} · Under {p.line} {oddsText(p.underOdds)}</p>
    <div className="projection-summary"><span>Recent 1Q average <b>{c.mean == null ? 'Unavailable' : c.mean.toFixed(1)}</b></span><span>Above / below / push <b>{c.over} / {c.under} / {c.push}</b></span><span>Verified 1Q appearances <b>{c.n}</b></span></div>
    {c.missing > 0 && <p className="notice">{c.missing} recent game{c.missing === 1 ? '' : 's'} lack verified Q1 scoring. No full-game values are used to fill the gaps.</p>}
    <p className="small">Last {Math.min(5, c.history.length)} verified 1Q performances, newest first:</p>
    {c.history.length ? <div className="quarter-history">{c.history.slice(0, 5).map((h) => <span key={h.gameId}><b>{h.points} pts</b><small>{h.date} · 1Q</small></span>)}</div> : <p>1Q history unavailable.</p>}
    {p.estimate != null ? <p className="small"><b>Paper estimate: {p.side === 'UNDER' ? 'Under' : 'Over'} {p.line}</b> · smoothed historical rate {pct(p.estimate)} · break-even {pct(p.breakEven)}. Observed: {counts} of {c.n - c.push} non-push 1Q appearances. This is a small-sample history estimate, not the calibrated full-game model.</p> : <p className="small">No probability estimate: at least eight verified 1Q appearances and complete recent-game coverage are required.</p>}
    <p className="small">{quoteState(p, now, '1q').label} · Checked {clock(p.quoteAt)} · price updated {clock(p.sourceUpdatedAt)} · {p.bookCount} books.</p>
  </article>
}
export default function QuarterBoard({ data }) {
  const now = useNow()
  if (!data) return <section className="game-markets"><h2>1Q points</h2><p>First-quarter comparisons appear after the quarter-data refresh. They use actual 1Q scoring and a separate paper record.</p></section>
  const groups = new Map()
  for (const p of data.cards || []) {
    if (p.period !== '1q' || !visibleQuote(p, '1q')) continue
    const k = `${p.gameId}/${p.playerId}`
    if (!groups.has(k)) groups.set(k, [])
    groups.get(k).push(p)
  }
  const r = data.record || {}
  return <section className="game-markets quarter-board">
    <div className="game-market-heading"><h2>1Q points</h2><span className="quote-badge">Separate quarter history · paper only</span></div>
    <p className="small">Quarter refresh: {clock(data.at)}. Recent form uses confirmed first-quarter appearances only, including zero-point quarters. Full-game averages, minutes and projections are never used here.</p>
    {!groups.size && <p>No current 1Q point lines passed the quote checks. Full-game props remain in their own section.</p>}
    {[...groups.entries()].map(([key, cards]) => <QuarterCard key={key} cards={cards} now={now} />)}
    <details><summary>1Q paper record & data coverage</summary>
      <p className="small">Quarter quote outcomes: {r.wins || 0} wins · {r.losses || 0} losses · {r.pushes || 0} pushes · {r.review || 0} need participation review · {r.pending || 0} pending. Multiple books and refreshes can describe the same player performance, so this is quote tracking, not independent bets. It does not enter the full-game record.</p>
      <p className="small">Each quarter is accepted only when attributed scoring reconciles with both official Q1 team totals. Missing data stays unavailable. A zero is retained for confirmed quarter participants. Paper grading requires confirmed Q1 participation; sportsbook void rules can differ.</p>
      {(data.recent || []).slice(0, 10).map((e, i) => <p className="small" key={i}>{e.date} · {e.player} · 1Q points · {e.side} {e.line} · {e.result}{e.actual != null ? ` (${e.actual} Q1 points)` : ''}</p>)}
    </details>
  </section>
}
