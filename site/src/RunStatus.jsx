import { clock } from './quote.js'
export default function RunStatus({ site }) {
  const at = site.status?.scoredAt
  return <p className="small quote-disclaimer">{at ? `Picks last checked ${clock(at)}. ` : ''}Plays stay on the board for this slate. Confirm the line and price at your sportsbook before betting.</p>
}
