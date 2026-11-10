import { useState } from 'react'
import { Seg } from './Board.jsx'
import { ResultList, Table } from './Record.jsx'
import { marketLabel, oddsText, pct, prettyDate, shortDate, statLabel } from './format.js'
import { tipState, useNow } from './time.js'
import QuoteDetails from './QuoteDetails.jsx'
import PickReason from './PickReason.jsx'
import { visibleQuote, quoteState } from './quote.js'

const COMBO_STATS = ['pra', 'pr', 'pa', 'ra']


function ComboCard({ c, now }) {
  const ts = tipState(c.tip, now)
  const started = ts.state === 'live' || ts.state === 'done'
  return (
    <li className={started ? 'started' : ''}>
      <div className="ov-head">
        <b>{c.player}</b>
        <span className="ov-bet">
          {c.side === 'OVER' ? 'Over' : 'Under'} {c.line} {marketLabel(c)}
        </span>
        <span className="ov-price">
          {oddsText(c.odds)} <small>{c.book}</small>
        </span>
      </div>
      <div className="ov-nums">
        <span>{c.tier === 'COMBOWATCH' ? 'Comparison only · shown' : 'Calibrated'} {pct(c.prob)}</span>
        <span>Raw component model {pct(c.probRaw)}</span>
        <span className={c.edge >= 0 ? 'pos' : 'neg'}>
          Edge {c.edge >= 0 ? '+' : ''}
          {(100 * c.edge).toFixed(1)}%
        </span>
        <span>
          Projected {c.projStat.toFixed(1)} in {Math.round(c.projMin)} min
        </span>
        {c.last5?.length > 0 && (
          <span>
            Last {c.last5.length}: {c.last5.map((g) => Math.round(g.value)).join(', ')} ({c.l5Hits}/{c.last5.length} on this side)
          </span>
        )}
        <span>
          {c.team} {c.matchup && `(${c.matchup})`} {ts.label}
        </span>
      </div>
      <QuoteDetails p={c} />
      <p className="notice">{c.tier === 'COMBOWATCH' ? 'Available line comparison, not a qualified combo pick. No stake or performance-log entry.' : 'Paper combo pick · tracked separately · no stake.'}</p>
      <PickReason p={c} />
      <p className="ov-why">{c.why}</p>
    </li>
  )
}

export default function Combos({ site }) {
  const { board } = site
  const rec = site.combos || {}
  const now = useNow()
  const [stat, setStat] = useState('all')
  const [side, setSide] = useState('ALL')
  const [all, setAll] = useState(false)
  const plays = (board.combos || []).filter((p) => visibleQuote(p))
  const list = plays.filter((c) => (stat === 'all' || c.stat === stat) && (side === 'ALL' || c.side === side))
  const shown = all ? list : list.slice(0, 12)
  const rows = (rec.rows || []).map((r) => ({ ...r, name: r.name === 'OVER' ? 'Overs' : r.name === 'UNDER' ? 'Unders' : statLabel[r.name] || r.name }))

  return (
    <div className="combopage">
      <section className="day">
        <h1>Combo lines</h1>
        <p className="honest">
          Points + rebounds + assists, points + rebounds, points + assists and rebounds + assists. These are tracked, not staked, and they have their own record
          below, apart from the picks. Combo probabilities require their own time-valid calibration; they do not borrow confidence from single stats. Available comparisons are shown even when no line qualifies. Only qualified tracked combos enter the separate paper log.
        </p>


        {!plays.length && <p className="notice">No combo picks have been published for this slate yet.</p>}
      </section>

      {plays.length > 0 && (
        <section className="overs combos" aria-label="Today's combo plays">
          <h2>Current combo picks & comparisons</h2>
          <div className="combo-filters">
            <Seg
              label="Combo"
              value={stat}
              onChange={setStat}
              options={[['all', 'All', plays.length], ...COMBO_STATS.map((s) => [s, statLabel[s], plays.filter((c) => c.stat === s).length])]}
            />
            <Seg
              label="Side"
              value={side}
              onChange={setSide}
              options={[['ALL', 'Both'], ['OVER', 'Over', plays.filter((c) => c.side === 'OVER').length], ['UNDER', 'Under', plays.filter((c) => c.side === 'UNDER').length]]}
            />
          </div>
          {list.length === 0 && <p className="notice">No combo plays match these filters.</p>}
          <ul>
            {shown.map((c) => (
              <ComboCard key={`${c.player}|${c.stat}|${c.side}`} c={c} now={now} />
            ))}
          </ul>
          {list.length > 12 && (
            <button type="button" className="morebtn" onClick={() => setAll((v) => !v)}>
              {all ? 'Show fewer' : `Show all ${list.length}`}
            </button>
          )}
        </section>
      )}


      <section className="record" aria-label="Combo record">
        <h2>Combo record</h2>
        {rows.length === 0 ? (
          <p className="notice">No combo plays have been graded yet. Results appear here the morning after games finish.</p>
        ) : (
          <>
            <p className="counts">
              {rec.since === rec.until ? shortDate(rec.since) : `${shortDate(rec.since)} to ${shortDate(rec.until)}`}, {rec.days} {rec.days === 1 ? 'day' : 'days'} of results
            </p>
            <p className="small">
              Every graded combo play, wins and losses. One play per player, combo and side per day. Units are a flat 1 per play for comparison, since combos are never
              staked. Treat anything under about 100 graded plays as noise.
            </p>
            <Table rows={rows} />
            {rec.buckets?.length > 0 && (
              <div className="buckets">
                {rec.buckets.map((b) => (
                  <div key={b.label} className="bucket">
                    <span className="lbl">{b.label}</span>
                    <span className="pair">
                      <i className="pred" style={{ width: `${b.predicted * 100}%` }} />
                      <i className="act" style={{ width: `${b.actual * 100}%` }} />
                    </span>
                    <span className="nums">
                      said {pct(b.predicted)}, hit {pct(b.actual)} over {b.picks}
                    </span>
                  </div>
                ))}
              </div>
            )}
            <h3>Recent combo results</h3>
            <section className="resultday">
              <ResultList rows={(rec.recent || []).slice(0, 30)} />
            </section>
          </>
        )}
      </section>
    </div>
  )
}
