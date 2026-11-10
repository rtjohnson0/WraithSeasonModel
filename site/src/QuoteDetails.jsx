import { useNow } from './time.js'
import { bookName, clock, ageText, quoteState } from './quote.js'
import { oddsText, pct } from './format.js'

export default function QuoteDetails({ p, compact = false }) {
  const now = useNow()
  const status = quoteState(p, now)
  const evidence = p.tier === 'PICK' ? 'Qualifies for posting rules' : p.tier === 'STRONG' ? 'Experimental model · tracked' : p.tier === 'CHECK' ? 'Needs model/status review' : 'Paper tracking · no stake'
  return (
    <div className={`quote-details ${status.key}`}>
      <div className="quote-pills">
        <span className={`quote-status ${status.key}`}>{status.label}</span>
        <span className="model-evidence">{evidence}</span>
      </div>
      <p className="quote-caption">
        {bookName(p.book)} · {p.side === 'OVER' ? 'Over' : 'Under'} <b>{p.line}</b> at <b>{oddsText(p.odds)}</b>
        {p.quoteAt && <> · Captured {ageText(p.quoteAt, now)}</>}
      </p>
      {!compact && <>
        <p className="quote-caption">Model {pct(p.prob, 1)} · Break-even {pct(p.breakEven || implied(p.odds), 1)}. This probability applies only to this line and price.</p>
        <details>
          <summary>Compare book quotes and timestamps</summary>
          <p className="small">Snapshot {clock(p.quoteAt)}. Source price updated {clock(p.sourceUpdatedAt)}. Market median {p.marketMedian ?? '—'} across {p.bookCount || 0} distinct books.</p>
          <div className="scroll"><table className="book-quotes"><thead><tr><th>Book</th><th>Line</th><th>Over</th><th>Under</th><th>Feed</th><th>Captured</th></tr></thead>
            <tbody>{(p.bookQuotes || []).map((q) => <tr key={`${q.book}-${q.source}`} className={q.book === p.book?.replace(/^sgo\//, '') ? 'selected-quote' : ''}>
              <th>{bookName(q.book)}</th><td>{q.line}</td><td>{oddsText(q.overOdds)}</td><td>{oddsText(q.underOdds)}</td><td>{q.source}</td><td>{clock(q.capturedAt)}</td>
            </tr>)}</tbody></table></div>
          <p className="small">These are provider snapshots, not direct sportsbook confirmation. If your book shows a different line or price, this pick does not apply. Reloading the page checks for a newer publication; it does not query the sportsbook.</p>
        </details>
      </>}
    </div>
  )
}
function implied(odds) {return odds > 0 ? 100 / (odds + 100) : odds < 0 ? -odds / (-odds + 100) : 0}
