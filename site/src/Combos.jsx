import { useState } from 'react'
import { FormBars, LineStrip } from './Charts.jsx'
import { Seg } from './Board.jsx'
import { ResultList, Table } from './Record.jsx'
import { marketLabel, oddsText, pct, prettyDate, shortDate, statLabel } from './format.js'
import { tipState, useNow } from './time.js'
import QuoteDetails from './QuoteDetails.jsx'
import PickReason from './PickReason.jsx'
import { bookName, visibleQuote, quoteState } from './quote.js'
import { comboGroups } from './combo.js'

const COMBO_STATS = ['pra', 'pr', 'pa', 'ra']


function ComboCard({ c, now }) {
 const comparison=c.tier==='COMBOWATCH'
 const confidence=comparison?(c.probRaw??c.prob):c.prob
 return <li className="combo-row"><details>
  <summary className="combo-summary"><span><small className="card-eyebrow">{c.matchup} · {comparison?'Research comparison':'Selected combo'}</small><b>{c.player} <small className="team-chip">{c.team}</small></b><span>{c.side==='OVER'?'Over':'Under'} {c.line} {marketLabel(c)}</span><small>{comparison?'Line comparison':'Combo pick'} · {oddsText(c.odds)} · {bookName(c.book)}</small>{c.availability&&<small className="availability-hint">Availability watch · Compare minutes</small>}</span>
   <span className="combo-strip"><LineStrip line={c.line} proj={c.projStat} games={c.last5} side={c.side}/></span><span className="combo-prob">{comparison ? c.projStat.toFixed(1) : pct(confidence,1)}<small>{comparison ? 'Projected total' : 'Model probability'}</small><small className="expand-hint">View analysis <span aria-hidden="true">⌄</span></small></span></summary>
  <div className="combo-expanded"><QuoteDetails p={c} compact/>
   {comparison && <p className="small">Raw simulation: {pct(confidence,1)} for this side. This has not been calibrated or demonstrated as a hit rate; it has not cleared the combo selection rules.</p>}
   <p>Projected <b>{c.projStat.toFixed(1)}</b> in {Math.round(c.projMin)} minutes · {c.matchup}</p>
   {c.last5?.length>0 && <div className="charts"><figure><figcaption>{statLabel[c.stat]} · last {c.last5.length} games</figcaption><FormBars games={c.last5} line={c.line} side={c.side} lineLabel={`line ${c.line}`}/></figure><figure><figcaption>Minutes played</figcaption><FormBars games={c.last5.map(g=>({...g,value:g.minutes}))} line={c.projMin} side={c.side} tone="minutes" lineLabel={`projected ${Math.round(c.projMin)}`}/></figure></div>}
   <PickReason p={comparison?{...c,prob:confidence,probabilityBasis:"estimate"}:c}/>
  </div>
 </details></li>
}

export default function Combos({ site }) {
  const { board } = site
  const rec = site.combos?.rows?.length ? site.combos : site.historicalCombos || {}
  const now = useNow()
  const [stat, setStat] = useState('all')
  const [side, setSide] = useState('ALL')
  const [all, setAll] = useState(false)
  const [view, setView] = useState(()=>comboGroups(board.combos||[]).picks.length?'picks':'comparisons')
  const groups = comboGroups((board.combos || []).filter((p) => visibleQuote(p)))
  const plays = groups[view]
  const list = plays.filter((c) => (stat === 'all' || c.stat === stat) && (side === 'ALL' || c.side === side))
  const shown = all ? list : list.slice(0, 24)
  const rows = (rec.rows || []).map((r) => ({ ...r, name: r.name === 'OVER' ? 'Overs' : r.name === 'UNDER' ? 'Unders' : statLabel[r.name] || r.name }))

  return (
    <div className="combopage">
      <section className="day">
        <h1>Combo picks</h1>
        <p className="small">PRA, points + rebounds, points + assists, and rebounds + assists. Tap a line for its projection, recent games and explanation.</p>


        <Seg label="View" value={view} onChange={setView} options={[
          ['picks', 'Selected plays', groups.picks.length],
          ['comparisons', 'Line comparisons', groups.comparisons.length],
        ]}/>
        {!groups.picks.length && groups.comparisons.length>0 && <p className="small">Showing available projections while the combo selection checks gather evidence.</p>}
        {view === 'comparisons' && <p className="small">Research comparisons, not selected bets. Projections and recent games are available below; raw simulations have not earned a reliable hit-rate estimate.</p>}
        {site.comboHealth && <div className="combo-readiness"><b>Combo readiness</b><p>{site.comboHealth.status}</p><p className="small">{site.comboHealth.eligibleRows} eligible pregame forecasts across {site.comboHealth.eligibleDates||0} dates · {site.comboHealth.trainingRows} for calibration · {site.comboHealth.evaluationRows} on later dates. Each combo type must pass its own evaluation before becoming a selected play.</p></div>}
        {!plays.length && <p className="notice">{view === 'picks' ? 'No combo currently clears the calibrated selection rules. Line comparisons remain available in the next tab.' : 'No combo line comparisons are available for this slate.'}</p>}
      </section>

      {plays.length > 0 && (
        <section className="overs combos" aria-label="Today's combo plays">
          <h2>{view === 'picks' ? 'Selected combo plays' : 'Combo line comparisons'}</h2>
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
          {list.length > 24 && (
            <button type="button" className="morebtn" onClick={() => setAll((v) => !v)}>
              {all ? 'Show fewer' : `Show all ${list.length}`}
            </button>
          )}
        </section>
      )}


      {site.combos?.rows?.length>0 && site.historicalCombos?.rows?.length>0 && <section className="record"><h2>Historical combo results</h2><Table rows={site.historicalCombos.rows}/><ResultList rows={(site.historicalCombos.recent||[]).slice(0,30)}/></section>}
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
