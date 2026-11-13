import { useEffect, useState } from 'react'
import Board from './Board.jsx'
import Record from './Record.jsx'
import Combos from './Combos.jsx'
import V2 from './V2.jsx'
import About from './About.jsx'

const PRIVATE_BOARD = import.meta.env.VITE_PRIVATE_BOARD === 'true'
const WHOP_URL = 'https://whop.com/wraithsznbets'

const PAGES = [
  ['board', 'Board'],
  ['combos', 'Combos'],
  ['record', 'Record'],
  ['v2', 'V2 test'],
  ['about', 'How it works'],
]

function pageFromHash() {
  const h = window.location.hash.replace('#/', '').replace('#', '')
  return PAGES.some(([id]) => id === h) ? h : 'board'
}

export default function App() {
  const [page, setPage] = useState(pageFromHash())
  const [site, setSite] = useState(null)
  const [error, setError] = useState('')
  const [refreshTick, setRefreshTick] = useState(0)
  const [loading, setLoading] = useState(false)
  const [accessRequired,setAccessRequired] = useState(false)

  useEffect(() => {
    const onHash = () => { setPage(pageFromHash()); window.scrollTo({ top: 0 }) }
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  useEffect(() => {
    let active = true
    let pending = false
    const controller = new AbortController()
    const load = async () => {
      if (pending) return
      pending = true
      setLoading(true)
      try {
        const url = new URL(PRIVATE_BOARD ? '/api/picks' : 'data/picks.json', document.baseURI)
        url.searchParams.set('t', Date.now())
        const r = await fetch(url, { cache: 'no-store', signal: controller.signal })
        if (PRIVATE_BOARD && (r.status===401 || r.status===403)) {
          if(active) {setSite(null);setAccessRequired(true);setError(r.status===401 ? 'Open this dashboard inside Whop to verify your account.' : 'Your account does not have access to WraithSznBets NBA Props Dashboard.')}
          return
        }
        if (!r.ok) throw new Error(`The board did not load (${r.status}).`)
        const next = await r.json()
        if (active) {
          setSite((prev) => !prev || (next.generatedAt || '') >= (prev.generatedAt || '') ? next : prev)
          setError('')
          setAccessRequired(false)
        }
      } catch (e) {
        if (active && e.name !== 'AbortError') {
          if(PRIVATE_BOARD) {setSite(null);setError('Could not verify access or load the board. Refresh to try again.')}
          else setError('Could not refresh the board. Your last loaded plays are still shown. Use Refresh board to try again.')
        }
      } finally { pending = false; if (active) setLoading(false) }
    }
    load()
    const timer = setInterval(load, 60000)
    const onVisible = () => document.visibilityState === 'visible' && load()
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      active = false
      controller.abort()
      clearInterval(timer)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [refreshTick])

  return (
    <>
      <header className="top">
        <a className="brand" href="#/board" aria-label="WraithPicksSZN home">
          <img src="./brand/logo.jpg" alt="" width="44" height="44" />
          <span>WraithPicksSZN</span>
        </a>
        <nav aria-label="Pages">
          {PAGES.map(([id, label]) => (
            <a key={id} href={`#/${id}`} aria-current={page === id ? 'page' : undefined}>
              {label}
            </a>
          ))}
        </nav>
        <button type="button" className="reload-board" disabled={loading} onClick={() => setRefreshTick((x) => x + 1)}>{loading ? 'Refreshing…' : 'Refresh board'}</button>
      </header>

      {site?.sample && (
        <div className="sample" role="alert">
          Sample data. These plays and results are made up to show the layout. They are not real picks.
        </div>
      )}

      <main>
        {error && <div className="notice" role="alert"><p>{error}</p>{accessRequired && <a className="whop-access" href={WHOP_URL} target="_blank" rel="noopener noreferrer">Open WraithSznBets on Whop ↗</a>}</div>}
        {!site && !error && <div className="board-loading" role="status"><p>Loading today’s board…</p><div /><div /><div /></div>}
        {site && page === 'board' && <Board site={site} />}
        {site && page === 'combos' && <Combos site={site} />}
        {site && page === 'record' && <Record site={site} />}
        {site && page === 'v2' && <V2 site={site} />}
        {site && page === 'about' && <About />}
      </main>

      <footer className="foot">
        <p>@wraithpicksszn on Instagram, X and Discord.</p>
      </footer>
    </>
  )
}
