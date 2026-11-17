import { clock } from './quote.js'
export default function RunStatus({ site }) {
  const at = site.status?.scoredAt
  if (!site.status?.scheduleComplete) return <><p className="small quote-disclaimer">Full schedule confirmation is unavailable. The count shows games with downloaded data.</p><p className="small quote-disclaimer">{at ? `Picks last checked ${clock(at)}. ` : ''}Confirm each line and price at your sportsbook.</p></>
  return <p className="small quote-disclaimer">{at ? `Picks last checked ${clock(at)}. ` : ''}Plays stay on the board for this slate. Confirm the line and price at your sportsbook before betting.</p>
}
