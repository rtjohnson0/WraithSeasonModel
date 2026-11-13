import { useMemo, useState } from 'react'
import { FormBars, MoveSpark, OutcomeCurve } from './Charts.jsx'
import Icon from './Icons.jsx'
import QuoteDetails from './QuoteDetails.jsx'
import RunStatus from './RunStatus.jsx'
import { MatchupDetails } from './GameMarkets.jsx'
import QuarterBoard from './QuarterBoard.jsx'
import PickReason from './PickReason.jsx'
import PlayerPortrait from './PlayerPortrait.jsx'
import { PropTicket, RecentForm } from './PropVisuals.jsx'
import TimedGroups from './TimedGroups.jsx'
import { remainingSlate, groupTimedCards, compareGameTip } from './boardData.js'
import {matchupSignal} from './matchup.js'
import { visibleQuote, quoteState } from './quote.js'
import { DaySummary, ResultList } from './Record.jsx'
import { tipState, useNow } from './time.js'
import { LABELS, comparePlayValue, marketLabel, gradeTone, labelInfo, oddsText, pct, prettyDate, statLabel } from './format.js'

const STATS = ['points', 'rebounds', 'assists', 'threes']

// Plain-language reasons: the model's own notes plus what the recent games show.
function reasons(p) {
  const out = []
  if (p.last5?.length) {
    const beat = p.side === 'UNDER' ? 'stayed under' : 'cleared'
    out.push(`He ${beat} ${p.line} in ${p.l5Hits} of his last ${p.last5.length} games and averaged ${p.l5Avg.toFixed(1)}.`)
    const normal = p.minTypical || p.minAvg5
    const diff = p.projMin - normal
    if (Math.abs(diff) >= 2.5) {
      out.push(`Projected minutes (${Math.round(p.projMin)}) are ${Math.abs(diff).toFixed(0)} ${diff < 0 ? 'below' : 'above'} his normal (${Math.round(normal)}, the middle of his last ${p.last5.length} games).`)
    }
    if (p.last5.some((g) => g.short)) {
      out.push('One recent night was cut short (injury, foul trouble or a blowout), so it is left out of his normal minutes.')
    }
  }
  out.push(`Projection ${p.projStat.toFixed(1)} against a line of ${p.line}.`)
  ;(p.why || '')
    .split(';')
    .map((x) => x.trim())
    .filter(Boolean)
    .filter((x) => !x.startsWith('defense vs '))
    .forEach((x) => out.push(x.charAt(0).toUpperCase() + x.slice(1) + '.'))
  if (p.defense) out.push(`${p.defense.helps ? 'Supports this side: ' : 'Against this side: '}${p.defense.text}, out of ${p.defense.of} teams.`)
  return out
}

const pc = (x) => `${Math.round(x * 100)}%`

// The full argument for a play: the arithmetic, the evidence on both sides, and how far to trust it.
function CaseView({ p }) {
  const c = p.case
  const dir = p.side === 'OVER' ? 'above' : 'below'
  return (
    <div className="case">
      <h4>The numbers</h4>
      <p className="math">
        {Math.round(c.minutes)} projected minutes at {c.perMin.toFixed(2)} per minute is a projection of{' '}
        <b>{c.projected.toFixed(1)}</b> against a line of <b>{c.line}</b>
        {c.gap >= 0.05 ? `, ${c.gap.toFixed(1)} ${dir} the line in our favor.` : '.'}
      </p>
      <div className="probs">
        <span>
          <b>{pc(c.modelProb)}</b>
          <small>model alone</small>
        </span>
        <span>
          <b>{pc(c.bookProb)}</b>
          <small>odds imply</small>
        </span>
        <span>
          <b>{pc(c.finalProb)}</b>
          <small>shown (blend)</small>
        </span>
      </div>
      <p className="tiernote">The shown percentage leans mostly on the book's price on purpose: the model alone has been too confident before.</p>
      {c.for?.length > 0 && (
        <>
          <h4>What supports it</h4>
          <ul className="reasons for">
            {c.for.map((r) => (
              <li key={r}>{r}</li>
            ))}
          </ul>
        </>
      )}
      {c.against?.length > 0 && (
        <>
          <h4>What could go wrong</h4>
          <ul className="reasons against">
            {c.against.map((r) => (
              <li key={r}>{r}</li>
            ))}
          </ul>
        </>
      )}
      <h4>How much to trust it</h4>
      {c.track?.length > 0 && (
        <ul className="reasons">
          {c.track.map((r) => (
            <li key={r}>{r}</li>
          ))}
        </ul>
      )}
      <p className="tiernote">{c.trust}</p>
    </div>
  )
}

// Overs that are not picks: the closest to qualifying, and Overs at books priced a point or more
// below the market. A board with no Overs still shows where they are and why they did not make it.
function OversWatch({ overs, now }) {
  const [all, setAll] = useState(false)
  const open = (overs || []).filter((o) => {
    const s = tipState(o.tip, now).state
    return s !== 'live' && s !== 'done'
  }).sort(compareGameTip)
  if (open.length === 0) return null
  const shown = all ? open : open.slice(0, 6)
  return (
    <section className="overs" aria-label="Overs watch" id="overs-watch">
      <div className="section-heading"><div><span className="card-eyebrow">The other side of the board</span><h2>Overs watch<span className="section-dot">↗</span></h2></div><span className="research-chip">Research · not selected picks</span></div>
      <p className="small">
        None of these are picks. They are the Overs closest to qualifying today, and Overs at books priced a point or more below the market, which is the same
        edge the Unders get from a high line.
      </p>
      <ul>
        {shown.map((o) => {
          const ts = tipState(o.tip, now)
          return (
            <li key={`${o.player}|${o.stat}`} className={o.low_line ? 'lowline' : ''}>
              <div className="ov-head">
                <PlayerPortrait name={o.player} team={o.team}/><b>{o.player}</b>
                <span className="ov-bet">
                  Over {o.line} {marketLabel(o)}
                </span>
                <span className="ov-price">
                  {oddsText(o.odds)} <small>{o.book}</small>
                </span>
              </div>
              <div className="ov-nums">
                <span>Model {pct(o.prob)}</span>
                <span className={o.edge >= 0 ? 'pos' : 'neg'}>Edge {o.edge >= 0 ? '+' : ''}{(100 * o.edge).toFixed(1)}%</span>
                <span>
                  Projected {o.proj.toFixed(1)} in {Math.round(o.proj_min)} min
                </span>
                <span>{o.matchup} · {ts.label || tipText(o.tip)}</span>
              </div>
              <p className="ov-why">
                {o.low_line && <em>Low line. </em>}
                {o.status}
              </p>
            </li>
          )
        })}
      </ul>
      {open.length > 6 && (
        <button type="button" className="morebtn" onClick={() => setAll((v) => !v)}>
          {all ? 'Show fewer' : `Show all ${open.length}`}
        </button>
      )}
    </section>
  )
}

const playId = (p) => `play-${p.player}-${p.stat}-${p.side}`.toLowerCase().replace(/[^a-z0-9]+/g, '-')

function PlayRow({ p, started, game, markets }) {
  const quote = quoteState(p, useNow())
  const [open, setOpen] = useState(false)
  const info = labelInfo[p.label] || labelInfo.Lean
  const signal=matchupSignal(p)
  const conf = Math.max(0, Math.min(1, (p.prob - 0.5) / 0.2))
  return (
    <li id={playId(p)} className={`play quote-${quote.key} ${p.side === 'OVER' ? 'over' : 'under'} ${info.key}${p.multi ? ' multi' : ''}${started ? ' started' : ''}`}>
      {game && <div className="card-matchup"><span>{game.matchup}</span><span className={started?'closed-tag':''}>{started ? 'Pregame closed' : tipText(game.tip) || 'Tip pending'}</span></div>}
      <button type="button" className="playbtn" aria-expanded={open} onClick={() => setOpen(!open)}>
        <span className="portrait-grade"><PlayerPortrait name={p.player} team={p.team}/><span className={`grade ${gradeTone(p.grade)}`} aria-label={`Grade ${p.grade}`}>{p.grade}</span></span>
        <span className="who">
          <span className="name">
            {p.player}
            {p.team && <span className="team">{p.team}</span>}
          </span>
          <PropTicket p={p}/>
          {signal && <span className={`matchup-signal ${signal.helps?'helps':'hurts'}`}>{signal.label}<small>{signal.helps?'Supports':'Works against'} this {p.side==='UNDER'?'Under':'Over'}</small></span>}
          {p.availability && <span className="availability-hint">Availability watch · View minutes comparison</span>}
          {p.rotation && <span className="availability-hint">Rotation forecast · View minutes breakdown</span>}
          {p.multi && (
            <span className="seen" title={`On the board in: ${p.seenIn.join(', ')}`}>
              In {p.runs} runs: {p.seenIn.join(', ')}
            </span>
          )}
          {p.badges?.length > 0 && (
            <span className="badges">
              {p.badges.slice(0, 5).map((b) => (
                <span key={b.key} className={`badge ${b.tone}`} title={b.label}>
                  <Icon name={b.key} />
                </span>
              ))}
            </span>
          )}
        </span>
        <span className="strip-wrap"><span className="form-caption">Recent form<small>At this line</small></span><RecentForm p={p}/></span>
        <span className="meta">
          <span className="prob-caption">Model probability</span><span className="prob">{pct(p.prob, 1)}</span>
          <span className="conf" aria-hidden="true">
            <i style={{ width: `${conf * 100}%` }} />
          </span>
          <span className={`tier ${info.key}`} title={info.note}>
            {p.prob<.55?'Low-confidence lean':p.label}
          </span>
          <span className="expand-hint">{open ? 'Hide analysis' : 'View analysis'} <span aria-hidden="true">{open ? '⌃' : '⌄'}</span></span>
          <span className="price">
            {oddsText(p.odds)} {p.book && <em>{p.book.replace('sgo/', '')}</em>}
          </span>
        </span>
      </button>
      <div className="card-metrics"><span>Projection<b>{p.projStat.toFixed(1)}</b></span><span>Minutes<b>{Math.round(p.projMin)}</b></span><span>Price edge<b className={p.edge>0?'positive':''}>{p.edge>0?'+':''}{(100*p.edge).toFixed(1)}<small> pp</small></b></span><span>Last {p.last5?.length || 5} hits<b>{p.last5?.length ? `${p.l5Hits}/${p.last5.length}` : '—'}</b></span></div>
      {quote.key !== 'recent' && <p className="card-quote-state">{quote.label}</p>}
      {game && <MatchupDetails game={game} data={markets}/>}
      {open && (
        <div className="why">
          <QuoteDetails p={p} compact />
          <PickReason p={p} />
          <div className="facts">
            <p className="lead">
              {p.side === 'UNDER' ? 'Under' : 'Over'} {p.line}: the model gives it {pct(p.prob, 1)}.
              {p.last5?.length > 0 && (
                <>
                  {' '}
                  The {p.side.toLowerCase()} would have hit {p.l5Hits} of his last {p.last5.length} games.
                </>
              )}
            </p>
            <dl>
              <div><dt>Projected</dt><dd>{p.projStat.toFixed(1)} in {Math.round(p.projMin)} min</dd></div>
              {p.last5?.length > 0 && <div><dt>Last {p.last5.length} average</dt><dd>{p.l5Avg.toFixed(1)} in {Math.round(p.minTypical || p.minAvg5)} normal min</dd></div>}
              {p.seasonGames > 0 && <div><dt>This season</dt><dd>{p.seasonAvg.toFixed(1)} over {p.seasonGames} games</dd></div>}
              <div><dt>Edge over the price</dt><dd>{(p.edge * 100).toFixed(1)}%</dd></div>
            </dl>
          </div>

          {p.last5?.length > 0 && (
            <div className="charts">
              <figure>
                <figcaption>{statLabel[p.stat]} in his last {p.last5.length} games</figcaption>
                <FormBars games={p.last5} line={p.line} lineLabel={`line ${p.line}`} side={p.side} />
              </figure>
              <figure>
                <figcaption>
                  Minutes played
                  {p.last5.some((g) => g.short) && <span> (faded night was cut short and is not counted)</span>}
                  {p.last5.some((g) => g.prior) && <span> (faded dashed bars are from last season)</span>}
                </figcaption>
                <FormBars
                  games={p.last5.map((g) => ({ ...g, value: g.minutes }))}
                  line={p.projMin}
                  lineLabel={`projected ${Math.round(p.projMin)}`}
                  side={p.side}
                  tone="minutes"
                />
              </figure>
            </div>
          )}

          {(p.dist?.length > 2 || p.moves?.length > 1) && (
            <div className="charts">
              {p.dist?.length > 2 && <OutcomeCurve dist={p.dist} line={p.line} side={p.side} />}
              {p.moves?.length > 1 && <MoveSpark moves={p.moves} side={p.side} />}
            </div>
          )}

          {p.badges?.length > 0 && (
            <ul className="badgelist">
              {p.badges.map((b) => (
                <li key={b.key} className={b.tone}>
                  <span className={`badge ${b.tone}`}>
                    <Icon name={b.key} />
                  </span>
                  {b.label}
                </li>
              ))}
            </ul>
          )}

          {p.runs > 0 && (
            <p className="tiernote">
              On today's board in {p.runs} {p.runs === 1 ? 'run' : 'runs'}: {p.seenIn.join(', ')}.
              {p.multi && ' The model kept choosing it as lines and lineups changed.'}
            </p>
          )}

          {p.case ? <CaseView p={p} /> : (
            <>
              <h4>Why the model likes it</h4>
              <ul className="reasons">
                {reasons(p).map((r) => (
                  <li key={r}>{r}</li>
                ))}
              </ul>
            </>
          )}
          <p className="tiernote">{p.label}: {info.note}</p>
        </div>
      )}
    </li>
  )
}

// The last day with graded results before the board's day: the record line and every graded play.
// The board only lists the final run's plays; this shows what happened to everything that was posted.
function LastResults({ record, date }) {
  const rows = (record?.recent || []).filter((r) => r.date < date)
  if (rows.length === 0) return null
  const day = rows.reduce((d, r) => (r.date > d ? r.date : d), '')
  const dayRows = rows.filter((r) => r.date === day)
  return (
    <details className="lastres" open>
      <summary>
        Results from {prettyDate(day)}: <DaySummary rows={dayRows} />
      </summary>
      <p className="small">
        Every play graded that day, not only the final board: plays earlier runs posted and later dropped count too.
      </p>
      <ResultList rows={dayRows} />
    </details>
  )
}

function tipText(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', timeZone: 'America/New_York' }) + ' ET'
}

function updatedText(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', timeZone: 'America/New_York' }) + ' ET'
}

export function Seg({ label, value, onChange, options }) {
  return (
    <div className="seg" role="group" aria-label={label}>
      {options.map(([v, text, n]) => (
        <button key={v} type="button" className={value === v ? 'on' : ''} aria-pressed={value === v} onClick={() => onChange(v)}>
          {text}
          {n !== undefined && <i>{n}</i>}
        </button>
      ))}
    </div>
  )
}

export default function Board({ site }) {
  const { board } = site
  const [stat, setStat] = useState('all')
  const [label, setLabel] = useState('all')
  const [side, setSide] = useState('ALL')
  const [q, setQ] = useState('')
  const [sort, setSort] = useState('value')
  const [repeatsOnly, setRepeatsOnly] = useState(false)
  const now = useNow()

  const all = useMemo(() => (board.games || []).flatMap((g) => g.plays || []), [board])
  const current = all.filter((p) => visibleQuote(p))
  const currentStaked = current.filter((p) => p.tier === 'PICK' && p.units > 0).length
  const extras = (board.games || []).flatMap(g => g.more || []).filter(p => visibleQuote(p))
  const available = [...current, ...extras]
  const count = (f) => available.filter(f).length

  const games = useMemo(
    () =>
      (board.games || [])
        .map((g) => ({
          ...g,
          plays: (g.plays || []).filter(
            (p) =>
              visibleQuote(p) &&
              (stat === 'all' || p.stat === stat) &&
              (label === 'all' || p.label === label) &&
              (side === 'ALL' || p.side === side) &&
              (!repeatsOnly || p.multi) &&
              (!q.trim() || `${p.player} ${p.team || ''} ${g.matchup}`.toLowerCase().includes(q.trim().toLowerCase())),
          ),
          // Every visible row follows the selected filters.
          more: (g.more || []).filter(
            (p) =>
              visibleQuote(p) &&
              (label === 'all' || p.label === label) &&
              (!repeatsOnly || p.multi) &&
              (stat === 'all' || p.stat === stat) &&
              (side === 'ALL' || p.side === side) &&
              (!q.trim() || `${p.player} ${p.team || ''} ${g.matchup}`.toLowerCase().includes(q.trim().toLowerCase())),
          ),
        }))
        .filter((g) => (stat === 'all' && label === 'all' && side === 'ALL' && !q && !repeatsOnly) || g.plays.length || g.more.length),
    [board, stat, label, side, q, repeatsOnly, now],
  )
  const shown = games.reduce((n, g) => n + g.plays.length, 0)

  const cards = games.flatMap(g => [...g.plays.map(p=>({p,g,extra:false})), ...g.more.map(p=>({p,g,extra:true}))])
  const groups = groupTimedCards(cards,row=>row.g,(a,b)=>sort==='probability' ? b.p.prob-a.p.prob : Number(a.extra)-Number(b.extra)||comparePlayValue(a.p,b.p))
  const unlistedGames = remainingSlate(board.games,site.gameMarkets?.games).sort(compareGameTip)

  return (
    <div className="boardpage">
      <section className="board-hero">
        <div className="hero-copy"><span className="card-eyebrow"><span className="live-dot"/> NBA INTELLIGENCE / V1</span><h1>Find your<br/><em>edge.</em></h1><p>Player props. A clearer perspective.</p><span className="hero-date">{prettyDate(board.date)} <span>· Updated {updatedText(site.generatedAt) || 'pending'}</span></span></div>
        <div className="hero-art" aria-hidden="true"><img src="./brand/logo.jpg" alt=""/><span className="hero-orbit orbit-one"/><span className="hero-orbit orbit-two"/><span className="art-caption">WRAITHSZN / THE DAILY BOARD</span></div>
        <dl className="hero-stats"><div><dt>On the board</dt><dd>{current.length.toString().padStart(2,'0')}</dd></div><div><dt>Max plays</dt><dd>{currentStaked.toString().padStart(2,'0')}</dd></div><div><dt>Extra leans</dt><dd>{extras.length.toString().padStart(2,'0')}</dd></div><div><dt>{site.status?.scheduleComplete?'Scheduled games':'Games with data'}</dt><dd>{board.games.length.toString().padStart(2,'0')}</dd></div></dl>
      </section>

      <RunStatus site={site} />
      <div className="section-heading prop-section-heading"><div><span className="card-eyebrow">The daily slate</span><h2>Player props<span className="section-dot">/</span></h2></div>{(board.overs||[]).some(o=>!["live","done"].includes(tipState(o.tip,now).state)) && <a className="overs-jump" href="#overs-watch" onClick={e=>{e.preventDefault();document.getElementById("overs-watch")?.scrollIntoView({behavior:"smooth",block:"start"})}}>Overs watch ↗</a>}</div>
      {/* Published picks remain visible; quote freshness is shown on each card. */}

      <section className="filters board-filters" aria-label="Filters">
        <Seg
          label="Stat"
          value={stat}
          onChange={setStat}
          options={[['all', 'All', available.length], ...STATS.map((s) => [s, statLabel[s], count((p) => p.stat === s)])]}
        />
        <Seg
          label="Side"
          value={side}
          onChange={setSide}
          options={[['ALL', 'Both'], ['OVER', 'Over', count((p) => p.side === 'OVER')], ['UNDER', 'Under', count((p) => p.side === 'UNDER')]]}
        />
        <Seg
          label="Play type"
          value={label}
          onChange={setLabel}
          options={[['all', 'Any type'], ...LABELS.map((t) => [t, t, count((p) => p.label === t)]).filter(([t, , n]) => t !== 'Strong play' || n > 0)]}
        />
        {board.runsToday?.length > 1 && (
          <Seg
            label="Runs"
            value={repeatsOnly ? 'rep' : 'all'}
            onChange={(v) => setRepeatsOnly(v === 'rep')}
            options={[['all', 'All plays'], ['rep', 'In several runs', count((p) => p.multi)]]}
          />
        )}
        <label className="search">
          <span className="sr">Search player or team</span>
          <input type="search" placeholder="Search player or team" value={q} onChange={(e) => setQ(e.target.value)} />
        </label>
      </section>

      <div className="filter-summary" aria-live="polite">
        <label className="sort-control">Within each game <select value={sort} onChange={e=>setSort(e.target.value)}><option value="value">Model value</option><option value="probability">Probability</option></select></label>
        <span>{shown} main {shown === 1 ? 'play' : 'plays'} · {games.reduce((n,g)=>n+g.more.length,0)} available leans</span>
        {(stat !== 'all' || label !== 'all' || side !== 'ALL' || q || repeatsOnly) && <button type="button" onClick={()=>{setStat('all');setLabel('all');setSide('ALL');setQ('');setRepeatsOnly(false)}}>Clear filters</button>}
      </div>
      {cards.length === 0 && (
        <p className="notice">
          {board.games.length === 0
            ? 'No qualifying plays in the latest run. A new odds refresh is required to publish new quotes.'
            : 'No plays match these filters.'}
        </p>
      )}

      <p className="schedule-order-note">Games in tip-time order · Eastern time · Card sorting stays within each matchup</p>
      <TimedGroups groups={groups} className="prop-grid" renderCard={({p,g,extra})=><PlayRow key={`${g.id}-${extra?'lean':'pick'}-${playId(p)}`} p={p} game={g} markets={site.gameMarkets} started={['live','done'].includes(tipState(g.tip,now).state)}/>}/>
      <OversWatch overs={board.overs} now={now}/>
      {unlistedGames.length>0 && <details className="slate-rest"><summary>Rest of the slate · {unlistedGames.length} games without player props</summary>{unlistedGames.map(g=><div key={g.id}><p>{g.matchup} · No qualifying player props</p><MatchupDetails game={g} data={site.gameMarkets}/></div>)}</details>}
      <QuarterBoard data={site.quarter}/>
      <LastResults record={site.record} date={board.date}/>
    </div>
  )
}
