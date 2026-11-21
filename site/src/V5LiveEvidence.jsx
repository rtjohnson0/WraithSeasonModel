import { useEffect, useState } from 'react'

// Rotation-v5 is a NEW independent forward cohort. It must never be compared
// as if it shared the three dates/questions of the V4 historical Lab.
export default function V5LiveEvidence() {
  const [data, setData] = useState(null)
  const [message, setMessage] = useState('Checking the new V5 evaluation cohort…')
  useEffect(() => {
    const controller = new AbortController()
    let mounted = true
    const update = async () => {
      try {
        const url = new URL('data/v5-lab.json', document.baseURI)
        url.searchParams.set('t', Date.now())
        const res = await fetch(url, { cache: 'no-store', signal: controller.signal })
        if (!res.ok) throw new Error('No V5 games graded yet.')
        const result = await res.json()
        if (result.schema !== 1 || result.policy !== 'rotation-v5' ||
            result.researchOnly !== true || !Array.isArray(result.stats) ||
            !['playerGames', 'questions', 'days'].every(k => Number.isSafeInteger(result[k]) && result[k] >= 0))
          throw new Error('V5 has not produced a compatible audited report.')
        if (mounted) { setData(result); setMessage('') }
      } catch (e) {
        if (mounted && e.name !== 'AbortError') setMessage(e.message)
      }
    }
    update()
    const timer = setInterval(update, 60000)
    return () => { mounted = false; clearInterval(timer); controller.abort() }
  }, [])
  return (
    <div className="v21-v5-evidence">
      <div className="v21-section-head"><div><p className="v21-eyebrow">NEW / INDEPENDENT V5 RECORD</p><h2>Guarded V2 predictions</h2></div><span className="v21-chip">FUTURE GAMES ONLY</span></div>
      <p className="v21-caption">V5 changes the real V2 minutes and three-point calculations. Its new results are tracked separately and cannot be treated as a matched comparison with the earlier V4 Lab sample.</p>
      {message && <p className="v21-wait" role="status">{message} This doesn't affect today's board.</p>}
      {data && <>
        <div className="v21-overview">
          <div><small>V5 player-games</small><strong>{data.playerGames}</strong></div>
          <div><small>V5 prop questions</small><strong>{data.questions}</strong></div>
          <div><small>Independent dates</small><strong>{data.days}</strong></div>
          <div><small>V5 minutes MAE</small><strong>{Number.isFinite(data.minutesMAE) ? data.minutesMAE.toFixed(2) : 'Pending'}</strong></div>
        </div>
        <div className="v21-v5-stats">
          {data.stats.map(s => <div key={s.stat}>
            <b>{s.stat === 'threes' ? 'Made threes' : s.stat}</b>
            <small>{s.n} questions · {s.days} dates</small>
            <span>{Number.isFinite(s.brier) ? `Brier ${s.brier.toFixed(4)} · stat MAE ${s.statMAE.toFixed(2)}` : 'Not enough independent results'}</span>
          </div>)}
        </div>
        <p className="v21-hint">Results through {data.through || 'pending'}. Five separate days are required before scores appear; more future evidence is needed to establish any improvement.</p>
      </>}
    </div>
  )
}
