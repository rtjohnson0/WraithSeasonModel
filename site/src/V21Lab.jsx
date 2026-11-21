import { useEffect, useState } from 'react'
import { validateV21Summary, scoresVisible, lowestMinutesMAE } from './v21LabData.js'
import GameMarketLab from './GameMarketLab.jsx'
import V5LiveEvidence from './V5LiveEvidence.jsx'

const EXPLANATIONS = {
  'base-minutes': 'Keep the original minutes estimate while holding V2 production rates constant.',
  'rotation-v4': 'Historical rotation-v4 baseline. This is the earlier cohort; V5 results are evaluated separately.',
  'supported-only': 'Use the rotation shift only with fresh, supported absence evidence and at least three matching past rotations.',
  'capped-shift': 'Use supported rotation shifts but limit the minutes change to 2.5 minutes in either direction.',
}

const number = (value, digits = 2) => Number.isFinite(value) ? value.toFixed(digits) : '—'
const fmtDate = (date) => date || 'Not available'

function ScoreCard({ candidate, minimum }) {
  const enabled = scoresVisible(candidate)
  const isCurrent = candidate.id === 'rotation-v4'
  const best = enabled && Number.isFinite(minimum) && candidate.minutesMAE <= minimum + 1e-6
  return (
    <article className={`v21-candidate ${isCurrent ? 'v21-live' : ''}`}>
      <div className="v21-candidate-head">
        <h3>{candidate.label}</h3>
        <span className="v21-chip">{isCurrent ? 'PRIOR V4' : 'SHADOW ONLY'}</span>
      </div>
      <p className="v21-caption">{EXPLANATIONS[candidate.id]}</p>
      {enabled ? (
        <>
          <div className="v21-primary"><strong>{number(candidate.minutesMAE)}<small>min</small></strong><span>Minutes MAE ↓</span></div>
          <div className="v21-bar" aria-hidden="true"><span style={{ width: `${Math.min(100, minimum > 0 ? 100 * minimum / candidate.minutesMAE : 0)}%` }} /></div>
          <dl className="v21-details">
            <div><dt>Brier score ↓</dt><dd>{number(candidate.brier, 4)}</dd></div>
            <div><dt>Log loss ↓</dt><dd>{number(candidate.logLoss, 4)}</dd></div>
            <div><dt>Stat MAE ↓</dt><dd>{number(candidate.statMAE)}</dd></div>
            <div><dt>Minutes bias</dt><dd>{candidate.minutesBias >= 0 ? '+' : ''}{number(candidate.minutesBias)}</dd></div>
          </dl>
          {best && <p className="v21-hint">Lowest minutes error in this sample—not a validated winner.</p>}
        </>
      ) : <p className="v21-wait">At least 30 verified player-games and questions are needed before numeric scores appear.</p>}
      <p className="v21-count">{candidate.playerGames ?? 0} player-games · {candidate.questions ?? 0} prop questions</p>
    </article>
  )
}

export default function V21Lab() {
  const [section, setSection] = useState('minutes')
  const [data, setData] = useState(null)
  const [error, setError] = useState('')
  useEffect(() => {
    const controller = new AbortController()
    let live = true
    const load = async () => {
      try {
        const url = new URL('data/v21-lab.json', document.baseURI)
        url.searchParams.set('t', Date.now())
        const response = await fetch(url, { cache: 'no-store', signal: controller.signal })
        if (!response.ok) throw new Error('No verified V2.1 report has been published yet.')
        const result = validateV21Summary(await response.json())
        if (live) { setData(result); setError('') }
      } catch (err) {
        if (live && err.name !== 'AbortError') setError(err.message)
      }
    }
    load()
    const interval = setInterval(load, 60000)
    return () => { live = false; clearInterval(interval); controller.abort() }
  }, [])

  const candidates = data?.candidates || []
  const minimum = lowestMinutesMAE(candidates)
  return (
    <section className="v21-page">
      <header className="v21-hero">
        <div>
          <p className="v21-eyebrow">WRAITH RESEARCH / VERSION 2.1</p>
          <h1>THE MODEL LAB<span>.</span></h1>
          <p className="v21-intro">Same pregame evidence. Independent challengers for player minutes, moneylines, spreads and game totals. We only promote what wins a fair future comparison.</p>
          <div className="v21-pills"><span>Forward replay</span><span>No auto-promotion</span><span>0 experimental units</span></div>
        </div>
        <div className="v21-mark" aria-hidden="true">V2<span>.1</span></div>
      </header>

      <div className="v21-attention" role="note">
        <strong>Research mode · Current V2 stays live.</strong>
        <span>These are retrospective comparisons on frozen pregame forecasts, not today's new picks or a verified winning system. A challenger must succeed on new, independent dates before any production change.</span>
      </div>

      <div className="v21-lab-switch" role="group" aria-label="Research category">
        <button type="button" className={section === 'minutes' ? 'is-active' : ''} aria-pressed={section === 'minutes'} onClick={() => setSection('minutes')}>Player props & minutes</button>
        <button type="button" className={section === 'games' ? 'is-active' : ''} aria-pressed={section === 'games'} onClick={() => setSection('games')}>Moneylines · Spreads · O/U</button>
      </div>

      {section === 'minutes' && <>
      <V5LiveEvidence/>
      {error && <p className="notice" role="status">{error} The live V2 Board remains available in <a href="#/v2">V2 test</a>.</p>}
      {!error && !data && <p className="notice" role="status">Loading the V2.1 research scorecard…</p>}
      {data && <>
        <div className="v21-overview">
          <div><small>Verified player-games</small><strong>{data.playerGames?.toLocaleString() ?? '—'}</strong></div>
          <div><small>Compared prop questions</small><strong>{data.questions?.toLocaleString() ?? '—'}</strong></div>
          <div><small>Independent game dates</small><strong>{data.dates?.toLocaleString() ?? '—'}</strong></div>
          <div><small>Results through</small><strong className="v21-date">{fmtDate(data.through)}</strong></div>
        </div>

        <div className="v21-section-head">
          <div><p className="v21-eyebrow">A / HEAD TO HEAD</p><h2>Minutes challengers</h2></div>
          <span className="v21-muted">Lower scores are better</span>
        </div>
        <div className="v21-cards">{candidates.map(candidate => <ScoreCard key={candidate.id} candidate={candidate} minimum={minimum} />)}</div>

        <div className="v21-section-head"><div><p className="v21-eyebrow">B / THE VERDICT</p><h2>Is V2.1 ready?</h2></div></div>
        <div className="v21-verdict">
          <span className="v21-verdict-symbol" aria-hidden="true">◎</span>
          <div><strong>{data.status === 'insufficient_evaluation_dates' ? 'NOT YET — MORE GAMES NEEDED' : 'RESEARCH ONLY — PROSPECTIVE TEST REQUIRED'}</strong>
            <p>{data.dates < 20 ? `Only ${data.dates} different evaluation dates are available. We need a wider variety of slates before trusting small differences.` : 'The existing comparison is exploratory. A future holdout is required before changing production.'}</p>
            <p>Minutes MAE checks playing time; Brier and log loss check probability accuracy. A model must improve beyond the sportsbook benchmark, not merely achieve a higher-looking hit rate.</p>
          </div>
        </div>
      </>}

      </>}
      {section === 'games' && <GameMarketLab/>}

      <footer className="v21-footer">
        <h2>How it works</h2>
        <p>Game outcomes are used only <strong>after</strong> games finish to score frozen pregame forecasts. Player minutes, moneylines, spreads and totals are independently evaluated against their original market prices.</p>
        <p>Results are aggregate-only; no experimental per-player picks or private betting lines are published by this tab.</p>
        <a href="#/v2" className="v21-back">← Back to today's V2 picks</a>
      </footer>
    </section>
  )
}
