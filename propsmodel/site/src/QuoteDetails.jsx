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

      </div>
      <p className="quote-caption">
        {bookName(p.book)} · {p.side === 'OVER' ? 'Over' : 'Under'} <b>{p.line}</b> at <b>{oddsText(p.odds)}</b>
        {p.quoteAt && <> · Checked {ageText(p.quoteAt, now)}</>}
      </p>
      {!compact && <>
        <p className="quote-caption">Model {pct(p.prob, 1)} · Break-even {pct(p.breakEven || implied(p.odds), 1)}. This probability applies only to this line and price.</p>
        {!status.usable && <p className="small">Pregame snapshot. Confirm the current line and price before betting.</p>}
      </>}
    </div>
  )
}
function implied(odds) {return odds > 0 ? 100 / (odds + 100) : odds < 0 ? -odds / (-odds + 100) : 0}
