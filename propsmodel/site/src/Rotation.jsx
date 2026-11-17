import { statLabel } from './format.js'

export default function Rotation({ p }) {
  const r = p.rotation
  if (!r) return null
  const side = p.side === 'UNDER' ? 'Under' : 'Over'
  return <section className="availability rotation-impact" aria-label="V2 rotation adjustment">
    <div className="availability-heading"><h4>Rotation impact</h4><span>{r.supported ? 'Experimental allocation' : 'Limited rotation evidence'}</span></div>
    {r.ownStatus && <p className="availability-alert">{p.player} is listed {r.ownStatus}. This estimate assumes he plays; a return-to-play minutes restriction is not modeled.</p>}
    <div className="rotation-metrics">
      <div><small>Model baseline</small><b>{r.baselineMinutes.toFixed(1)} min</b></div>
      <div><small>Used in this forecast</small><b>{r.usedMinutes.toFixed(1)} min</b></div>
      <div><small>Matching team rotations</small><b>{r.matchedGames}</b></div>
    </div>
    {r.confirmedOut?.length > 0 && <p><b>Confirmed out:</b> {r.confirmedOut.map(x => x.player).join(', ')}.</p>}
    <p className="small">{r.reason}. {r.teamMinutes > 0 ? `The team allocation totals ${r.teamMinutes.toFixed(0)} regulation minutes.` : 'The baseline forecast is retained.'} Per-minute production is unchanged; extra usage is not assumed.</p>
    {r.scenarios?.length > 0 && <>
      <h4>If another teammate sits</h4>
      <p className="small">Uncertain teammates remain in the main allocation. Each scenario removes only the named player alongside confirmed outs; no probability of sitting is assumed.</p>
      <div className="availability-table-wrap"><table><thead><tr><th>Scenario</th><th>Minutes</th><th>{statLabel[p.stat] || p.stat}</th><th>{side} estimate</th></tr></thead><tbody>
        {r.scenarios.map(s => <tr key={s.playerId}><td>{s.player} out <small>({s.status})</small>{!s.supported && <small className="rotation-sample">Limited matching history</small>}</td><td>{s.minutes.toFixed(1)}</td><td>{s.projection == null ? '—' : s.projection.toFixed(1)}</td><td>{s.probability == null ? '—' : `${(100*s.probability).toFixed(1)}%`}</td></tr>)}
      </tbody></table></div>
    </>}
  </section>
}
