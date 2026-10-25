import { useMemo, useState } from 'react'
import { FormBars } from './Charts.jsx'
import Icon from './Icons.jsx'
import { LABELS, gradeTone, labelInfo, oddsText, pct, prettyDate, statLabel } from './format.js'

const STATS = ['points', 'rebounds', 'assists', 'threes']

function Chip({ on, onClick, children }) {
  return (
    <button type="button" className={`chip${on ? ' on' : ''}`} aria-pressed={on} onClick={onClick}>
      {children}
    </button>
  )
}

// Plain-language reasons: the model's own notes plus what the recent games show.
function reasons(p) {
  const out = []
  if (p.last5?.length) {
    const beat = p.side === 'UNDER' ? 'stayed under' : 'cleared'
    out.push(`He ${beat} ${p.line} in ${p.l5Hits} of his last ${p.last5.length} games and averaged ${p.l5Avg.toFixed(1)}.`)
    const diff = p.projMin - p.minAvg5
    if (Math.abs(diff) >= 2.5) {
      out.push(`Projected minutes (${Math.round(p.projMin)}) are ${Math.abs(diff).toFixed(0)} ${diff < 0 ? 'below' : 'above'} his last-${p.last5.length} average (${Math.round(p.minAvg5)}).`)
    }
  }
  out.push(`Projection ${p.projStat.toFixed(1)} against a line of ${p.line}.`)
  ;(p.why || '')
    .split(';')
    .map((x) => x.trim())
    .filter(Boolean)
    .forEach((x) => out.push(x.charAt(0).toUpperCase() + x.slice(1) + '.'))
  return out
}

function PlayRow({ p }) {
  const [open, setOpen] = useState(false)
  const info = labelInfo[p.label] || labelInfo.Lean
  return (
    <li className="play">
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
            <b className={p.side === 'OVER' ? 'over' : 'under'}>{p.side}</b> {p.line} {statLabel[p.stat] || p.stat}
          </span>
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
        <span className="spark">
          {p.last5?.length ? <FormBars games={p.last5} line={p.line} side={p.side} compact /> : null}
          {p.last5?.length ? <small>{p.l5Hits} of {p.last5.length} last games</small> : null}
        </span>
        <span className="meta">
          <span className={`tier ${info.key}`} title={info.note}>
            {p.label}
          </span>
          <span className="prob">{pct(p.prob, 1)}</span>
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
              {p.last5?.length > 0 && <div><dt>Last {p.last5.length} average</dt><dd>{p.l5Avg.toFixed(1)} in {Math.round(p.minAvg5)} min</dd></div>}
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
                <figcaption>Minutes played</figcaption>
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

          <h4>Why the model likes it</h4>
          <ul className="reasons">
            {reasons(p).map((r) => (
              <li key={r}>{r}</li>
            ))}
          </ul>
          <p className="tiernote">{p.label}: {info.note}</p>
        </div>
      )}
    </li>
  )
}

export default function Board({ site }) {
  const { board } = site
  const [stats, setStats] = useState(new Set(STATS))
  const [labels, setLabels] = useState(new Set(LABELS))
  const [side, setSide] = useState('ALL')
  const [q, setQ] = useState('')

  const toggle = (set, setter, v) => {
    const n = new Set(set)
    n.has(v) ? n.delete(v) : n.add(v)
    setter(n)
  }

  const games = useMemo(
    () =>
      board.games
        .map((g) => ({
          ...g,
          plays: g.plays.filter(
            (p) =>
              stats.has(p.stat) &&
              labels.has(p.label) &&
              (side === 'ALL' || p.side === side) &&
              (!q || p.player.toLowerCase().includes(q.toLowerCase()) || (p.team || '').toLowerCase() === q.toLowerCase()),
          ),
        }))
        .filter((g) => g.plays.length),
    [board, stats, labels, side, q],
  )

  return (
    <>
      <div className="bannerwrap">
        <img className="banner" src="./brand/banner.jpg" alt="WraithPicksSZN: data, discipline, profits" width="1600" height="639" />
      </div>

      <section className="day">
        <h1>{prettyDate(board.date)}</h1>
        <p className="counts">
          {board.games.length} games, {board.plays} plays, <b>{board.staked} Max {board.staked === 1 ? 'play' : 'plays'}</b>
        </p>
        <p className="honest">
          Max plays are the only ones the model stakes. Great, risky and lean plays are tracked so the record shows
          whether they work, but they are not recommendations.
        </p>
      </section>

      <section className="filters" aria-label="Filters">
        <div className="group" role="group" aria-label="Stat">
          {STATS.map((s) => (
            <Chip key={s} on={stats.has(s)} onClick={() => toggle(stats, setStats, s)}>
              {statLabel[s]}
            </Chip>
          ))}
        </div>
        <div className="group" role="group" aria-label="Side">
          {['ALL', 'OVER', 'UNDER'].map((s) => (
            <Chip key={s} on={side === s} onClick={() => setSide(s)}>
              {s === 'ALL' ? 'Both sides' : s === 'OVER' ? 'Over' : 'Under'}
            </Chip>
          ))}
        </div>
        <div className="group" role="group" aria-label="Play type">
          {LABELS.map((t) => (
            <Chip key={t} on={labels.has(t)} onClick={() => toggle(labels, setLabels, t)}>
              {t}
            </Chip>
          ))}
        </div>
        <label className="search">
          <span className="sr">Search player or team</span>
          <input type="search" placeholder="Player or team" value={q} onChange={(e) => setQ(e.target.value)} />
        </label>
      </section>

      {games.length === 0 && (
        <p className="notice">
          {board.games.length === 0
            ? 'No plays are posted for this day yet. The board updates after the daily run.'
            : 'No plays match these filters. Turn a filter back on to see more.'}
        </p>
      )}

      <div className="games">
        {games.map((g, i) => (
          <article className="game" key={g.id} style={{ '--i': Math.min(i, 5) }}>
            <header>
              <h2>{g.matchup.split(" @ ").length === 2 ? <>{g.matchup.split(" @ ")[0]}<i>@</i>{g.matchup.split(" @ ")[1]}</> : g.matchup}</h2>
              <span>{g.plays.length} plays</span>
            </header>
            <ul>
              {g.plays.map((p) => (
                <PlayRow key={`${p.player}-${p.stat}-${p.side}`} p={p} />
              ))}
            </ul>
          </article>
        ))}
      </div>
    </>
  )
}
