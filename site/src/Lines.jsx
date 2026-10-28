import { pct } from './format.js'

const gap = (a, b) => {
  const d = 100 * (a - b)
  return `${d > 0 ? '+' : ''}${d.toFixed(1)}`
}

// Calibration of every line the model scored, and the record of lines priced off the market.
// Rendered inside the Record page; nothing here is a pick.
export default function Lines({ lines }) {
  if (!lines || (!lines.graded && !lines.offGraded)) return null
  const beats = lines.brierModel > 0 && lines.brierModel < lines.brierBase
  const slices = (lines.slices || []).filter((s) => s.n >= 20)
  return (
    <section className="lineslog">
      {lines.graded > 0 && (
        <>
          <h2>Every line, not just the picks</h2>
          <p className="small">
            The picks are about 50 plays a day, too few to judge the model quickly. Each day the model also prices every line on the
            slate at the books' middle number, and those are graded here too: {lines.graded} lines over {lines.days}{' '}
            {lines.days === 1 ? 'day' : 'days'}. When the model says a line is 45% to go Over, about 45% of those should.
          </p>
          <div className="buckets">
            {lines.bins.map((b) => (
              <div key={b.label} className="bucket">
                <span className="lbl">{b.label}</span>
                <span className="pair">
                  <i className="pred" style={{ width: `${b.predicted * 100}%` }} />
                  <i className="act" style={{ width: `${b.actual * 100}%` }} />
                </span>
                <span className="nums">
                  said {pct(b.predicted)}, went Over {pct(b.actual)} over {b.n}
                </span>
              </div>
            ))}
          </div>
          {lines.brierBase > 0 && (
            <p className="small">
              Score (Brier, lower is better): the model {lines.brierModel.toFixed(4)}, a guess that every line is the day's average{' '}
              {lines.brierBase.toFixed(4)}.{' '}
              {beats
                ? "The model's leans carry real information."
                : "The model's leans are not beating that guess yet, so treat its edges with caution."}
            </p>
          )}
          {slices.length > 0 && (
            <>
              <h3>Where it is off</h3>
              <p className="small">The share of lines that went Over in each situation, against what the model expected. A gap of 5 points or more on a few hundred lines is a real miss.</p>
              <div className="scroll">
                <table>
                  <thead>
                    <tr>
                      <th scope="col">Situation</th>
                      <th scope="col">Lines</th>
                      <th scope="col">Went Over</th>
                      <th scope="col">Model said</th>
                      <th scope="col">Gap (pts)</th>
                    </tr>
                  </thead>
                  <tbody>
                    {slices.map((s) => (
                      <tr key={s.name}>
                        <th scope="row">{s.name}</th>
                        <td>{s.n}</td>
                        <td>{pct(s.overRate, 1)}</td>
                        <td>{pct(s.modelOver, 1)}</td>
                        <td className={Math.abs(s.overRate - s.modelOver) >= 0.05 && s.n >= 100 ? 'down' : ''}>{gap(s.overRate, s.modelOver)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </>
      )}
      {lines.offGraded > 0 && (
        <>
          <h2>Lines priced off the market</h2>
          <p className="small">
            When one book's number sits a point or more from the middle of four or more books, the side that gap favours is cheaper there than
            anywhere else: the Under at a high line, the Over at a low one. Each is graded at that book's own price, flat 1 unit, once per
            player, stat and side per day.
          </p>
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
                {lines.off.map((r) => (
                  <tr key={r.name}>
                    <th scope="row">{r.name}</th>
                    <td>
                      {r.wins}-{r.losses}
                      {r.pushes ? `-${r.pushes}` : ''}
                    </td>
                    <td>{r.wins + r.losses ? pct(r.hitRate, 1) : 'n/a'}</td>
                    <td>{pct(r.predicted, 1)}</td>
                    <td className={r.units > 0 ? 'up' : r.units < 0 ? 'down' : ''}>
                      {r.units > 0 ? '+' : ''}
                      {r.units.toFixed(1)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </section>
  )
}
