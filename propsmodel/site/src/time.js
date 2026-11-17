import { useEffect, useState } from 'react'

// "now" that refreshes every half minute, so countdowns tick without reloading the page.
export function useNow(ms = 30000) {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), ms)
    const onVis = () => document.visibilityState === 'visible' && setNow(Date.now())
    document.addEventListener('visibilitychange', onVis)
    return () => {
      clearInterval(id)
      document.removeEventListener('visibilitychange', onVis)
    }
  }, [ms])
  return now
}

const GAME_MS = 3.3 * 60 * 60 * 1000 // a typical NBA game, tip to final buzzer

// Where a game is in its day: not started, probably under way, or probably over. The tip time is the
// only clock the data has, so "live" and "final" are estimates.
export function tipState(iso, now) {
  const t = iso ? new Date(iso).getTime() : NaN
  if (Number.isNaN(t)) return { state: 'unknown', label: '' }
  const d = t - now
  if (d > 0) {
    const mins = Math.round(d / 60000)
    if (mins >= 90) return { state: 'open', label: `Tips in ${Math.floor(mins / 60)}h ${mins % 60 ? `${mins % 60}m` : ''}`.trim() }
    if (mins >= 2) return { state: 'soon', label: `Tips in ${mins} min` }
    return { state: 'soon', label: 'Tipping now' }
  }
  if (-d < GAME_MS) return { state: 'live', label: 'Under way' }
  return { state: 'done', label: 'Probably final' }
}
