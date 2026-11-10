import { useState } from 'react'
export default function ResultAudit({ reports = [] }) {
 const [q,setQ] = useState('')
 const [all,setAll] = useState(false)
 const changes = reports.flatMap((r) => (r.history || r.changes || []).map((c) => ({...c,log:r.log}))).reverse()
 const checks = reports.flatMap((r) => (r.checks || []).map((c) => ({...c,log:r.log}))).reverse()
 const source = all ? checks : changes
 const shown = source.filter((c) => `${c.player} ${c.date} ${c.stat} ${c.gameId} ${c.newResult} ${c.reason}`.toLowerCase().includes(q.toLowerCase()))
 return <section className="game-markets result-audit"><h2>Result audit</h2><p className="small">Every confirmed result must match the final player ID, name, game ID, date, team and requested full-game stat. Quarter results use their own record. REVIEW means withheld, not a win or a loss; this also covers games still waiting on final stats.</p>
 {!reports.length && <p>Saved results have not been audited with the new checks yet. Run the result audit after refreshing box scores.</p>}
 {reports.map((r) => <p className="small" key={r.log}>{r.log}: {r.checked} checked · {r.confirmed} confirmed · {r.review} for review · {r.staleSources || 0} older cached sources ignored · {r.changes?.length || 0} changes this run.</p>)}
 {(changes.length > 0 || checks.length > 0) && <><label><input type="checkbox" checked={all} onChange={(e) => setAll(e.target.checked)} /> Show every checked result</label><input aria-label="Search result audit" placeholder="Search player, date, stat or game ID" value={q} onChange={(e) => setQ(e.target.value)} /><p className="small">{shown.length} matching {all ? 'checked results' : 'audit changes'}. Showing up to 200; search narrows the full list. Earlier snapshots are retained as backups.</p><div className="booktable"><table><thead><tr><th>Player / game</th><th>Pick</th><th>Before</th><th>After</th><th>Check</th></tr></thead><tbody>{shown.slice(0,200).map((c,i) => <tr key={i}><td>{c.player}<small>{c.date} · game {c.gameId} · player {c.playerId}</small></td><td>{c.side} {c.line} {c.stat}</td><td>{c.oldResult || 'ungraded'} {c.oldResult ? `(${c.oldActual})` : ''}</td><td>{c.newResult}{c.newActual != null ? ` (${c.newActual})` : ''}</td><td>{c.reason}</td></tr>)}</tbody></table></div></>}
 </section>
}
