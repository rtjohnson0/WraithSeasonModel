import { useEffect, useMemo, useRef, useState } from 'react'
import { FormBars, LineStrip, MoveSpark, OutcomeCurve } from './Charts.jsx'
import Icon from './Icons.jsx'
import { DaySummary, ResultList } from './Record.jsx'
import { tipState, useNow } from './time.js'
import { LABELS, gradeTone, labelInfo, oddsText, pct, prettyDate, statLabel } from './format.js'

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
  if (p.defense) out.push(`${p.defense.text}, out of ${p.defense.of} teams.`)
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

// The best plays still open: not started, strongest first. Tap one to jump to it.
function TopPlays({ games, now, onPick }) {
  const open = []
  games.forEach((g) => {
    const ts = tipState(g.tip, now)
    if (ts.state === 'live' || ts.state === 'done') return
    ;(g.plays || []).forEach((p) => {
      if (p.label !== 'Lean') open.push({ p, g, ts })
    })
  })
  open.sort((a, b) => b.p.prob - a.p.prob)
  const top = open.slice(0, 5)
  if (top.length === 0) return null
  return (
    <section className="topplays" aria-label="Top plays still open">
      <h2>Best plays still open</h2>
      <ol>
        {top.map(({ p, g, ts }) => (
          <li key={playId(p)}>
            <button type="button" onClick={() => onPick(p)}>
              <span className={`grade ${gradeTone(p.grade)}`}>{p.grade}</span>
              <span className="tp-main">
                <b>{p.player}</b>
                <span>
                  <i className={p.side === 'OVER' ? 'over' : 'under'}>{p.side === 'OVER' ? 'Over' : 'Under'}</i> {p.line} {statLabel[p.stat] || p.stat}
                </span>
              </span>
              <span className="tp-side">
                <b>{pct(p.prob, 1)}</b>
                <small>{ts.label || g.matchup}</small>
              </span>
            </button>
          </li>
        ))}
      </ol>
    </section>
  )
}

// Overs that are not picks: the closest to qualifying, and Overs at books priced a point or more
// below the market. A board with no Overs still shows where they are and why they did not make it.
function OversWatch({ overs, now }) {
  const [all, setAll] = useState(false)
  const open = (overs || []).filter((o) => {
    const s = tipState(o.tip, now).state
    return s !== 'live' && s !== 'done'
  })
  if (open.length === 0) return null
  const shown = all ? open : open.slice(0, 6)
  return (
    <section className="overs" aria-label="Overs watch">
      <h2>Overs watch</h2>
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
                <b>{o.player}</b>
                <span className="ov-bet">
                  Over {o.line} {statLabel[o.stat] || o.stat}
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
                <span>{ts.label || o.matchup}</span>
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

// Who is out and who is uncertain on each side, from the injury report.
function LineupNews({ news }) {
  return (
    <ul className="lineupnews">
      {news.map((n) => (
        <li key={n.team}>
          <b>{n.team}</b>
          {n.out?.length > 0 && <span> out: {n.out.join(', ')}.</span>}
          {n.maybe?.length > 0 && <span className="maybe"> Uncertain: {n.maybe.join(', ')}.</span>}
        </li>
      ))}
    </ul>
  )
}

// The game's betting market in plain terms: the spread says how close it should be (and so how
// many minutes starters keep), the total says how many chances there will be to score.
function GameContext({ c }) {
  const diff = c.total - c.slateAvg
  return (
    <div className="gamectx">
      <p className="chips">
        <span>
          <small>Spread</small>
          <b>{c.favorite ? `${c.favorite} -${c.spread}` : "Pick'em"}</b>
        </span>
        <span>
          <small>Total</small>
          <b>
            {c.total}
            <i className={diff >= 4 ? 'up' : diff <= -4 ? 'down' : ''}>
              {' '}
              {diff >= 0 ? '+' : ''}
              {diff.toFixed(1)} vs slate
            </i>
          </b>
        </span>
        <span>
          <small>Expected score</small>
          <b>
            {c.visitor} {Math.round(c.visitorPts * 10) / 10}, {c.home} {Math.round(c.homePts * 10) / 10}
          </b>
        </span>
      </p>
      <ul>
        {c.reads.map((r) => (
          <li key={r}>{r}</li>
        ))}
      </ul>
    </div>
  )
}

// The next-best lines in a game, behind a button. They did not clear the board's bar, so they are
// weaker than the plays above them.
function MorePlays({ plays, started }) {
  const [open, setOpen] = useState(false)
  return (
    <div className="more">
      <button type="button" className="morebtn" aria-expanded={open} onClick={() => setOpen(!open)}>
        {open ? 'Hide extra plays' : `More plays (${plays.length})`}
      </button>
      {open && (
        <>
          <p className="small">The next best lines in this game. They did not clear the board's bar, so treat them as leans.</p>
          <ul>
            {plays.map((p) => (
              <PlayRow key={`more-${p.player}-${p.stat}-${p.side}`} p={p} started={started} />
            ))}
          </ul>
        </>
      )}
    </div>
  )
}

const playId = (p) => `play-${p.player}-${p.stat}-${p.side}`.toLowerCase().replace(/[^a-z0-9]+/g, '-')

function PlayRow({ p, focus, started }) {
  const [open, setOpen] = useState(false)
  const ref = useRef(null)
  // A tap on the top-plays strip opens this play and scrolls to it.
  useEffect(() => {
    if (focus && focus.id === playId(p)) {
      setOpen(true)
      ref.current?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }, [focus]) // eslint-disable-line react-hooks/exhaustive-deps
  const info = labelInfo[p.label] || labelInfo.Lean
  const conf = Math.max(0, Math.min(1, (p.prob - 0.5) / 0.2))
  return (
    <li ref={ref} id={playId(p)} className={`play ${p.side === 'OVER' ? 'over' : 'under'} ${info.key}${p.multi ? ' multi' : ''}${started ? ' started' : ''}`}>
      <button type="button" className="playbtn" aria-expanded={open} onClick={() => setOpen(!open)}>
        <span className={`grade ${gradeTone(p.grade)}`} aria-label={`Grade ${p.grade}`}>
          {p.grade}
        </span>
        <span className="who">
          <span className="name">
            {p.player}
            {p.team && <span className="team">{p.team}</span>}
          </span>
          <span className="pick">
            <b className={p.side === 'OVER' ? 'over' : 'under'}>{p.side === 'OVER' ? 'Over' : 'Under'}</b> {p.line}{' '}
            {statLabel[p.stat] || p.stat}
          </span>
          {p.defense && (
            <span className={`defense ${p.defense.helps ? 'helps' : 'hurts'}`} title={p.defense.early ? 'Early in the season: this ranking still leans on last season' : undefined}>
              {p.defense.text}
              {p.defense.early ? ', early' : ''}
            </span>
          )}
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
        <span className="strip-wrap">
          <LineStrip line={p.line} proj={p.projStat} games={p.last5} side={p.side} />
          <small>
            proj <b>{p.projStat.toFixed(1)}</b>
            {p.last5?.length ? <> · {p.l5Hits} of {p.last5.length} hit</> : null}
          </small>
        </span>
        <span className="meta">
          <span className="prob">{pct(p.prob, 1)}</span>
          <span className="conf" aria-hidden="true">
            <i style={{ width: `${conf * 100}%` }} />
          </span>
          <span className={`tier ${info.key}`} title={info.note}>
            {p.label}
          </span>
          <span className="price">
            {oddsText(p.odds)} {p.book && <em>{p.book.replace('sgo/', '')}</em>}
          </span>
        </span>
      </button>
      {open && (
        <div className="why">
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
    <details className="lastres">
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

const ODDS = (o) => (o > 0 ? `+${o}` : `${o}`)

// Plays an earlier run posted that a later run dropped but that the model still rates at 58% or more
// on the same side. They are not on the board any more, usually because stronger plays took their
// place in the game, so they are worth a look rather than a pick.
function EarlierPicks({ runs }) {
  const seen = new Map()
  ;(runs || []).forEach((r) =>
    (r.dropped || []).forEach((d) => {
      if (d.now && d.now.side === d.side && d.now.prob >= 0.58) {
        seen.set(`${d.player}|${d.stat}|${d.side}`, { ...d, was: r.prev })
      }
    }),
  )
  const list = [...seen.values()].sort((a, b) => b.now.prob - a.now.prob)
  if (list.length === 0) return null
  return (
    <details className="earlier">
      <summary>
        Earlier picks still at 58% or better: {list.length}
      </summary>
      <p className="small">Posted on an earlier run, off the board now, and the model still rates the same side at 58% or more.</p>
      <ul>
        {list.map((d) => (
          <li key={`${d.player}-${d.stat}-${d.side}`}>
            <b>{d.player}</b> {d.side.toLowerCase()} {d.now.line} {statLabel[d.stat] || d.stat}
            {d.matchup ? ` (${d.matchup})` : ''}: {pc(d.prob)} on the {d.was} run, {pc(d.now.prob)} now
            {d.now.line !== d.line ? ` (line moved from ${d.line})` : ''}
            {d.now.odds ? ` at ${ODDS(d.now.odds)}${d.now.book ? ' ' + d.now.book : ''}` : ''}. <span className="reason">Dropped because {d.reason}.</span>
          </li>
        ))}
      </ul>
    </details>
  )
}

const tierName = { PICK: 'Max play', CHECK: 'Risky play', PAPER: 'Great play', LEAN: 'Lean' }

// What each later run dropped from the run before it, and why.
function RunChanges({ runs }) {
  const later = (runs || []).filter((r) => r.prev)
  if (later.length === 0) return null
  const dropped = later.reduce((n, r) => n + (r.dropped?.length || 0), 0)
  return (
    <details className="changes">
      <summary>
        What changed between runs today: {dropped} {dropped === 1 ? 'play' : 'plays'} dropped
      </summary>
      {later.map((r) => (
        <section key={`${r.label}-${r.clock}`} className="runchange">
          <h3>
            {r.label} run ({r.clock}) compared with the {r.prev} run
          </h3>
          <p className="small">
            {r.plays} plays now, {r.new} new, {r.dropped?.length || 0} dropped
            {r.changed?.length ? `, ${r.changed.length} moved tier` : ''}.
          </p>
          {r.dropped?.length > 0 && (
            <ul>
              {r.dropped.map((d) => (
                <li key={`${d.player}-${d.stat}-${d.side}`}>
                  <b>{d.player}</b> {d.side.toLowerCase()} {d.line} {statLabel[d.stat] || d.stat}
                  {d.matchup ? ` (${d.matchup})` : ''}: <span className="reason">{d.reason}.</span>
                </li>
              ))}
            </ul>
          )}
          {r.changed?.length > 0 && (
            <ul>
              {r.changed.map((c) => (
                <li key={`${c.player}-${c.stat}-${c.side}-t`}>
                  <b>{c.player}</b> {c.side.toLowerCase()} {statLabel[c.stat] || c.stat}: {tierName[c.from] || c.from} became{' '}
                  {tierName[c.to] || c.to}.
                </li>
              ))}
            </ul>
          )}
        </section>
      ))}
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
  const [repeatsOnly, setRepeatsOnly] = useState(false)
  const [focus, setFocus] = useState(null)
  const now = useNow()

  const all = useMemo(() => board.games.flatMap((g) => g.plays || []), [board])
  const count = (f) => all.filter(f).length

  const games = useMemo(
    () =>
      board.games
        .map((g) => ({
          ...g,
          plays: (g.plays || []).filter(
            (p) =>
              (stat === 'all' || p.stat === stat) &&
              (label === 'all' || p.label === label) &&
              (side === 'ALL' || p.side === side) &&
              (!repeatsOnly || p.multi) &&
              (!q || p.player.toLowerCase().includes(q.toLowerCase()) || (p.team || '').toLowerCase() === q.toLowerCase()),
          ),
          // The extras ignore the play-type and repeat filters (they are all lean plays) but follow stat, side and search.
          more: (g.more || []).filter(
            (p) =>
              (stat === 'all' || p.stat === stat) &&
              (side === 'ALL' || p.side === side) &&
              (!q || p.player.toLowerCase().includes(q.toLowerCase()) || (p.team || '').toLowerCase() === q.toLowerCase()),
          ),
        }))
        .filter((g) => g.plays.length || (label === 'all' && !repeatsOnly && g.more.length)),
    [board, stat, label, side, q, repeatsOnly],
  )
  const shown = games.reduce((n, g) => n + g.plays.length, 0)

  return (
    <>
      <div className="bannerwrap">
        <img className="banner" src="./brand/banner.jpg" alt="WraithPicksSZN: data, discipline, profits" width="1600" height="639" />
      </div>

      <section className="day">
        <div className="dayhead">
          <h1>{prettyDate(board.date)}</h1>
          <dl className="tally">
            <div className={board.staked ? 'hot' : ''}>
              <dt>Max plays</dt>
              <dd>{board.staked}</dd>
            </div>
            <div>
              <dt>Plays</dt>
              <dd>{board.plays}</dd>
            </div>
            <div>
              <dt>Games</dt>
              <dd>{board.games.length}</dd>
            </div>
            {site.generatedAt && (
              <div>
                <dt>Updated</dt>
                <dd className="time">{updatedText(site.generatedAt)}</dd>
              </div>
            )}
          </dl>
        </div>
      </section>

      <TopPlays games={board.games} now={now} onPick={(p) => { setStat('all'); setLabel('all'); setSide('ALL'); setQ(''); setRepeatsOnly(false); setFocus({ id: playId(p), n: Date.now() }) }} />
      <OversWatch overs={board.overs} now={now} />
      <LastResults record={site.record} date={board.date} />
      <RunChanges runs={board.runsToday} />
      <EarlierPicks runs={board.runsToday} />

      <section className="filters" aria-label="Filters">
        <Seg
          label="Stat"
          value={stat}
          onChange={setStat}
          options={[['all', 'All', all.length], ...STATS.map((s) => [s, statLabel[s], count((p) => p.stat === s)])]}
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
          options={[['all', 'Any type'], ...LABELS.map((t) => [t, t, count((p) => p.label === t)])]}
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

      {shown === 0 && (
        <p className="notice">
          {board.games.length === 0
            ? 'No plays are posted for this day yet. The board updates after the daily run.'
            : 'No plays match these filters. Pick "All" to see more.'}
        </p>
      )}

      <div className="games">
        {games.map((g) => {
          const [away, home] = g.matchup.split(' @ ')
          const ts = tipState(g.tip, now)
          const started = ts.state === 'live' || ts.state === 'done'
          return (
            <article className={`game${started ? ' started' : ''}`} key={g.id}>
              <header>
                <h2>
                  {home ? (
                    <>
                      {away}
                      <i>at</i>
                      {home}
                    </>
                  ) : (
                    g.matchup
                  )}
                </h2>
                <span>
                  {g.tip && <b className="tip">{tipText(g.tip)}</b>}
                  {ts.label && <em className={`countdown ${ts.state}`}>{ts.label}</em>}
                  {g.plays.length} {g.plays.length === 1 ? 'play' : 'plays'}
                </span>
              </header>
              {g.ctx && <GameContext c={g.ctx} />}
              {g.news?.length > 0 && <LineupNews news={g.news} />}
              {started && <p className="startednote">This game has started or is over, so these lines are closed. The plays are kept for the record.</p>}
              <ul>
                {g.plays.map((p) => (
                  <PlayRow key={`${p.player}-${p.stat}-${p.side}`} p={p} focus={focus} started={started} />
                ))}
              </ul>
              {g.more.length > 0 && <MorePlays plays={g.more} started={started} />}
            </article>
          )
        })}
      </div>
    </>
  )
}
