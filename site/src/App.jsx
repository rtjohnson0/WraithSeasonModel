import { useEffect, useState } from 'react'
import Board from './Board.jsx'
import Record from './Record.jsx'
import About from './About.jsx'

const PAGES = [
  ['board', 'Board'],
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
        {site && page === 'record' && <Record site={site} />}
        {site && page === 'about' && <About />}
      </main>

      <footer className="foot">
        <p>
          Picks are model output, not advice, and nothing here is guaranteed to win. 21+ only. If gambling is a
          problem for you or someone you know, call 1-800-GAMBLER.
        </p>
        <p>@wraithpicksszn on Instagram, X and Discord.</p>
      </footer>
    </>
  )
}
