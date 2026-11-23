import { useEffect, useState } from 'react'
import { validateGameLab, gameEvidenceSupported, gameScoresVisible } from './gameLabData.js'

const NAMES = {
  moneyline: { title: 'Moneyline', description: 'Which team wins? Compare the predicted winner probability against the sportsbook before tip.' },
  spread: { title: 'Spread', description: 'Who covers the handicap? Compare calibrated cover probability with the offered line and price.' },
  total: { title: 'Game O/U', description: 'Is the combined score over or under? Pace, offensive/defensive efficiency, rotations and the line matter.' },
}
const f = n => Number.isFinite(n) ? n.toFixed(4) : '—'

function MarketCard({ row }) {
  const info = NAMES[row.id]
  const enough = gameScoresVisible(row)
  const supported = gameEvidenceSupported(row)
  return (
    <article className="v21-game-card">
      <div className="v21-game-card-head"><h3>{info.title}</h3><span className="v21-chip">PAPER / 0 UNITS</span></div>
      <p className="v21-caption">{info.description}</p>
      <div className="v21-game-count"><strong>{row.n}</strong><span>verified outcomes</span><b>{row.days} game dates</b></div>
      {enough ? <>
        <p className="v21-game-label">Same-game log loss ↓</p>
        <div className="v21-game-scores">
          <div><small>Game model</small><strong>{f(row.rawLogLoss)}</strong></div>
          <div><small>Sportsbook</small><strong>{f(row.bookLogLoss)}</strong></div>
        </div>
        {row.calibratedN >= 30 && row.calibratedLogLoss != null && row.calibrationBookLogLoss != null && 
          <p className="v21-game-cal">Calibrated: {f(row.calibratedLogLoss)} · Book: {f(row.calibrationBookLogLoss)} · matched games {row.calibratedN}</p>}
        <p className="v21-hint">{supported ? 'Research cohort is large enough to review; no edge certified without held-out evaluation.' : 'Early comparison only. Keep recording independent game dates.'}</p>
      </> : <p className="v21-wait">No reliable head-to-head score yet. We display metrics after 30 settled forecasts across 5 separate dates.</p>}
      <p className="v21-count">{row.calibratedN > 0 ? row.calibratedN + ' earlier-fitted calibration comparisons' : 'Separate calibration not yet evaluated on future games'}</p>
    </article>
  )
}

export default function GameMarketLab() {
  const [data, setData] = useState(null)
  const [error, setError] = useState('')
  useEffect(() => {
    const controller = new AbortController()
    let alive = true
    const update = async () => {
      try {
        const url = new URL('data/v21-games.json', document.baseURI)
        url.searchParams.set('t', Date.now())
        const response = await fetch(url, { cache: 'no-store', signal: controller.signal })
        if (!response.ok) throw new Error('Game-market research has not been published yet.')
        const result = validateGameLab(await response.json())
        if (alive) { setData(result); setError('') }
      } catch (e) {
        if (alive && e.name !== 'AbortError') setError(e.message)
      }
    }
    update()
    const interval = setInterval(update, 60000)
    return () => { alive = false; controller.abort(); clearInterval(interval) }
  }, [])
  return (
    <div className="v21-games">
      <div className="v21-section-head">
        <div><p className="v21-eyebrow">B / WHOLE GAME MODELS</p><h2>Moneyline, spread & totals</h2></div>
        <span className="v21-muted">Three independent evaluations</span>
      </div>
      <p className="v21-intro">Game markets are evaluated separately against the sportsbook. A model should beat the market over new games before any pick earns real units; results from spreads cannot validate moneylines or totals.</p>
      {error && <p className="notice" role="status">{error}</p>}
      {!error && !data && <p className="notice" role="status">Loading independent game-market scores…</p>}
      {data && <>
        <div className="v21-game-cards">{data.markets.map(m => <MarketCard key={m.id} row={m}/>)}</div>
        {data.pointStudy&&data.pointStudy.games>0&&<section className="v21-verdict"><div><strong>RECENT ERROR CORRECTION · HISTORICAL TEST</strong><p>{data.pointStudy.games} matching games across {data.pointStudy.dates} dates. Each correction uses earlier out-of-fold forecasts only.</p><p>Margin error: {data.pointStudy.rawMarginMAE.toFixed(3)} → {data.pointStudy.correctedMarginMAE.toFixed(3)} points. Total error: {data.pointStudy.rawTotalMAE.toFixed(3)} → {data.pointStudy.correctedTotalMAE.toFixed(3)} points. Lower is better.</p><p>This is a point-forecast comparison, not a sportsbook edge test. It does not change live picks.</p></div></section>}
        <div className="v21-verdict">
          <span className="v21-verdict-symbol" aria-hidden="true">◎</span>
          <div>
            <strong>NO MARKET EDGE CERTIFIED YET</strong>
            <p>Current challenger: possessions × scoring efficiency, expected rotations and opponent-adjusted matchup context. The next experiments are lineup-adjusted team strength, calibrated margin distributions, and pace/shot-mix-aware total distributions.</p>
            <p>Each must beat the unadjusted sportsbook baseline at the same pregame odds, on an independent forward sample. Game markets remain paper-only until then.</p>
            <p className="v21-muted">Research report: {data.date || 'date unknown'} · {data.generatedAt || 'generation time unavailable'}</p>
          </div>
        </div>
      </>}
    </div>
  )
}
