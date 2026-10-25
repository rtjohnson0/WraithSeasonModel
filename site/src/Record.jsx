import { UnitsChart } from './Charts.jsx'
import { pct, prettyDate, shortDate, statLabel } from './format.js'

function Table({ rows, unitsNote }) {
  if (!rows?.length) return null
  return (
    <div className="scroll">
      <table>
        <thead>
          <tr>
            <th scope="col">Group</th>
            <th scope="col">Record</th>
            <th scope="col">Hit rate</th>
            <th scope="col">Model said</th>
            <th scope="col">Units</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name}>
              <th scope="row">{r.name}</th>
              <td>
                {r.wins}-{r.losses}
                {r.pushes ? `-${r.pushes}` : ''}
              </td>
              <td>{r.wins + r.losses ? pct(r.hitRate, 1) : 'n/a'}</td>
              <td>{r.wins + r.losses ? pct(r.predicted, 1) : 'n/a'}</td>
              <td className={r.units > 0 ? 'up' : r.units < 0 ? 'down' : ''}>
                {r.units > 0 ? '+' : ''}
                {r.units.toFixed(1)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {unitsNote && <p className="small">{unitsNote}</p>}
    </div>
  )
}

export default function Record({ site }) {
  const { record } = site
  if (!record.tiers?.length) {
    return <p className="notice">No plays have been graded yet. Results appear here the morning after games finish.</p>
  }
  const staked = record.tiers.find((t) => t.name === 'Max plays (staked)')
  const byDate = {}
  record.recent.forEach((r) => (byDate[r.date] = byDate[r.date] || []).push(r))

  return (
    <div className="record">
      <section className="day">
        <h1>The record</h1>
        <p className="counts">
          {record.since === record.until ? shortDate(record.since) : `${shortDate(record.since)} to ${shortDate(record.until)}`},{' '}
          {record.days} {record.days === 1 ? 'day' : 'days'} of results
        </p>
        <p className="honest">
          Every graded play is here, wins and losses, including the ones that were never staked. The same player, stat and
          side on one day counts once. This is a small sample: a few weeks of results cannot separate skill from luck.
        </p>
        {staked && (
          <p className="headline">
            <b>
              Max plays {staked.wins}-{staked.losses}
              {staked.pushes ? `-${staked.pushes}` : ''}
            </b>{' '}
            {staked.wins + staked.losses ? `${pct(staked.hitRate, 1)} hit, model expected ${pct(staked.predicted, 1)}` : ''}
          </p>
        )}
      </section>

      <h2>Units over time</h2>
      <UnitsChart daily={record.daily} />
      <p className="small">Max plays use the real stake. The other line puts 1 unit on every tracked play, staked or not, so it shows whether the unstaked plays are worth anything.</p>

      <h2>By group</h2>
      <Table
        rows={record.tiers}
        unitsNote="Max plays show real units at the stake used. Every other row is a flat 1 unit per play, for comparison only."
      />

      <h2>By stat and side</h2>
      <Table rows={[...(record.stats || []), ...(record.sides || [])].map((r) => ({ ...r, name: statLabel[r.name] || r.name }))} />

      {record.buckets?.length > 0 && (
        <>
          <h2>Do the percentages hold up?</h2>
          <p className="small">When the model says 60%, it should win about 60% of the time. Here is how close it is.</p>
          <div className="buckets">
            {record.buckets.map((b) => (
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
        </>
      )}

      <h2>Recent results</h2>
      {Object.keys(byDate)
        .sort()
        .reverse()
        .map((d) => (
          <section key={d} className="resultday">
            <h3>{prettyDate(d)}</h3>
            <ul>
              {byDate[d].map((r, i) => (
                <li key={`${r.player}-${r.stat}-${r.side}-${i}`} className={r.result.toLowerCase()}>
                  <span className="res">{r.result === 'WIN' ? 'Hit' : r.result === 'LOSS' ? 'Miss' : r.result === 'PUSH' ? 'Push' : 'Void'}</span>
                  <span className="what">
                    {r.player} {r.team && <em>{r.team}</em>}
                    <br />
                    <small>
                      {r.side} {r.line} {statLabel[r.stat] || r.stat}
                      {r.result !== 'VOID' && <>, finished with {r.actual}</>}
                    </small>
                  </span>
                  <span className="tag">{r.label}</span>
                </li>
              ))}
            </ul>
          </section>
        ))}
    </div>
  )
}
