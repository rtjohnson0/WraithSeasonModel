import Availability from './Availability.jsx'
import { explainProp } from './explain.js'
export default function PickReason({ p }) {
 const e = explainProp(p)
 return <section className="pick-reason"><Availability p={p}/><h4>Why this {p.side === 'UNDER' ? 'Under' : 'Over'}?</h4><p>{e.decision}</p>
  <div className="reason-columns"><div><b>What supports it</b>{e.support.length ? <ul>{e.support.map((s,i) => <li key={i}>{s}</li>)}</ul> : <p>No clear supporting context beyond the quoted probability.</p>}</div><div><b>What works against it</b>{e.against.length ? <ul>{e.against.map((s,i) => <li key={i}>{s}</li>)}</ul> : <p>No opposing factor is available in these inputs. That does not make the pick certain.</p>}</div></div>
  {e.sensitivity && <p className="notice">{e.sensitivity}</p>}
  {e.fitted.length > 0 && <details><summary>How the model used matchup, minutes and injuries</summary>{e.fitted.map((s,i) => <p className="small" key={i}>{s}</p>)}<p className="small">These are fitted-model sensitivities and terms, not proven causal effects or a complete explanation of the calibrated probability.</p></details>}
 </section>
}
