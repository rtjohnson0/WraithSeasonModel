import { useEffect, useState } from 'react'
import Board from './Board.jsx'
import Record from './Record.jsx'
import Combos from './Combos.jsx'
import About from './About.jsx'

const PAGES = [
  ['board', 'Board'],
  ['combos', 'Combos'],
  ['record', 'Record'],
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
  const [fresh, setFresh] = useState(false)

  useEffect(() => {
    const onHash = () => setPage(pageFromHash())
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  useEffect(() => {
    const url = new URL('data/picks.json', document.baseURI)
    // A timestamp keeps a phone from showing yesterday's board out of its cache.
    url.searchParams.set('t', Date.now())
    fetch(url)
      .then((r) => {
        if (!r.ok) throw new Error(`The picks file did not load (${r.status}).`)
        return r.json()
      })
      .then(setSite)
      .catch((e) => setError(e.message))
  }, [])

  // Check for a newer board every few minutes and whenever the tab comes back into view.
  useEffect(() => {
    if (!site) return undefined
    const check = () => {
      const url = new URL('data/picks.json', document.baseURI)
      url.searchParams.set('t', Date.now())
      fetch(url, { cache: 'no-store' })
        .then((r) => (r.ok ? r.json() : null))
        .then((d) => {
          if (d && d.generatedAt && d.generatedAt !== site.generatedAt) setFresh(true)
        })
        .catch(() => {})
    }
    const id = setInterval(check, 5 * 60 * 1000)
    const onVis = () => document.visibilityState === 'visible' && check()
    document.addEventListener('visibilitychange', onVis)
    return () => {
      clearInterval(id)
      document.removeEventListener('visibilitychange', onVis)
    }
  }, [site])

  return (
    <>
      {fresh && (
        <div className="new-banner" role="status">
          New picks are posted.
          <button type="button" onClick={() => window.location.reload()}>
            Reload
          </button>
        </div>
      )}
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
      </header>

      {site?.sample && (
        <div className="sample" role="alert">
          Sample data. These plays and results are made up to show the layout. They are not real picks.
        </div>
      )}

      <main>
        {error && <p className="notice">{error} Try again in a minute.</p>}
        {!site && !error && <p className="notice">Loading the board</p>}
        {site && page === 'board' && <Board site={site} />}
        {site && page === 'combos' && <Combos site={site} />}
        {site && page === 'record' && <Record site={site} />}
        {site && page === 'about' && <About />}
      </main>

      <footer className="foot">
        <p>@wraithpicksszn on Instagram, X and Discord.</p>
      </footer>
    </>
  )
}
