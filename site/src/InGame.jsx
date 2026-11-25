import { useEffect, useMemo, useState } from 'react'
import { statLabel, oddsText } from './format.js'
import { liveRows, liveFresh, liveBookConfidence } from './inGameData.js'

export default function InGame({ site }) {
  const [now, setNow] = useState(Date.now())
  const [sort, setSort] = useState('percentage')
  const [filter, setFilter] = useState('all')
  useEffect(() => { const id = setInterval(() => setNow(Date.now()), 30000); return () => clearInterval(id) }, [])
  const data = site?.halftime
  const valid = liveFresh(data, now)
  const rows = useMemo(() => liveRows(valid ? data?.plays : [], sort, filter), [data, valid, sort, filter])
  return <section className="day ingame">
    <header className="ingame-hero">
      <p className="card-eyebrow">WRAITH / LIVE GAME TRACKER</p>
      <h1>HALFTIME<span> CHECK.</span></h1>
      <p>Compare saved pregame picks with the player's current box score and any verified in-play full-game sportsbook lines.</p>
      <div className="ingame-hero-notes"><span>Original picks stay frozen</span><span>No automatic halftime bets</span><span>Live quote or no live ranking</span></div>
    </header>
    {!data && <p className="notice">No halftime refresh published yet. Run the separate <b>Wraith halftime refresh</b> GitHub Action once games start.</p>}
    {data && !valid && <p className="notice" role="status">The last halftime snapshot is too old for a live comparison. Refresh the halftime action to see verified current stats. Earlier pregame plays remain available on the Board and V2 pages.</p>}
    {valid && <>
      <div className="ingame-bar">
        <span><b>{rows.length}</b> original props tracked</span>
        <span>Box score checked {new Date(data.capturedAt).toLocaleTimeString([], {hour:'numeric',minute:'2-digit'})}</span>
        <span><b>{data.plays.filter(p=>p.live).length}</b> verified fresh in-play quotes</span>
      </div>
      <div className="ingame-controls">
        <label>Games <select value={filter} onChange={e=>setFilter(e.target.value)}><option value="all">All in-progress games</option><option value="half">Halftime only</option></select></label>
        <label>Order <select value={sort} onChange={e=>setSort(e.target.value)}><option value="percentage">Highest sportsbook-implied %</option><option value="remaining">Fewest remaining to original Over</option><option value="game">Matchup</option></select></label>
      </div>
      <p className="ingame-warning">Percentages below are calculated from the CURRENT BOOK’S PRICE after removing its margin. They are not Wraith's updated in-game win probabilities, betting edges, or recommendations. Live line availability is independently verified and may be missing.</p>
      {rows.length===0 && <p className="notice">No in-progress original picks have current verified box scores in this snapshot.</p>}
      <div className="ingame-cards">{rows.map((p,i)=>{
        const live = p.live
        const market = live ? liveBookConfidence(live) : null
        const isHalf = /half/i.test(p.gameStatus||'')
        return <article className="ingame-card" key={[p.model,p.gameId,p.player,p.stat,p.originalLine,p.originalSide,i].join('-')}>
          <div className="ingame-card-top"><span className="ingame-phase">{isHalf?'HALFTIME':'IN PROGRESS'} · {p.gameStatus}</span><span className="ingame-model">{p.model} pregame</span></div>
          <h2>{p.player}</h2><p className="ingame-sub">{p.matchup || 'Matchup pending'} · {statLabel[p.stat]||p.stat}</p>
          <div className="ingame-statgrid">
            <div><small>Original pick</small><strong>{p.originalSide} {p.originalLine}</strong><em>{oddsText(p.originalOdds)} · pregame</em></div>
            <div><small>Live box score</small><strong>{p.hasLiveStats?p.current:'—'}</strong><em>{p.hasLiveStats?(p.remainingToOver===0?'Over original line cleared':`${p.remainingToOver} more to clear original Over`):'Live score unavailable'}</em></div>
          </div>
          {p.halftimeBox && <div className="ingame-halfbox">
            <p className="ingame-market-head">RECORDED AT HALFTIME · FIRST-HALF BOX</p>
            <div className="ingame-halfbox-values">
              <span><b>{p.halftimeBox.points}</b><small>PTS</small></span>
              <span><b>{p.halftimeBox.rebounds}</b><small>REB</small></span>
              <span><b>{p.halftimeBox.assists}</b><small>AST</small></span>
              <span><b>{p.halftimeBox.threes}</b><small>3PM</small></span>
            </div>
          </div>}
          {live && market && <div className="ingame-market">
            <p className="ingame-market-head">CURRENT IN-PLAY FULL-GAME BOOK · {live.book}</p>
            <strong>O/U {live.line}</strong>
            <div className="ingame-market-pcts"><span>Over <b>{Math.round(market.over*100)}%</b> <small>{oddsText(live.overOdds)}</small></span><span>Under <b>{Math.round(market.under*100)}%</b> <small>{oddsText(live.underOdds)}</small></span></div>
            <small>Book-updated {new Date(live.updatedAt).toLocaleTimeString([], {hour:'numeric',minute:'2-digit'})}. Market-implied only, NOT a V2 probability.</small>
          </div>}
          {(!live || !market) && <div className="ingame-absent">Live sportsbook line not verified. Original line is for comparison only; don't treat it as a currently available bet.</div>}
        </article>
      })}</div>
    </>}
    <p className="ingame-footer">The original pregame result is evaluated only against its saved pregame book line. A different halftime bet requires its own timestamped live price, separate selection record, and forward-validated live model before it can be ranked as a Wraith pick.</p>
  </section>
}
