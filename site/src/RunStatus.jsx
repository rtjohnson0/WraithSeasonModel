import { ageText, clock } from './quote.js'
import { useNow } from './time.js'

export default function RunStatus({ site }) {
  const now = useNow()
  const s = site.status
  const age = s?.scoredAt ? (now - Date.parse(s.scoredAt)) / 1000 : Infinity
  const stale = age > (s?.captureMaxAgeSeconds || 1800)
  return (
    <section className={`run-status ${stale ? 'stale' : 'recent'}`} aria-label="Board refresh status">
      <div><b>{stale ? 'Board needs an odds refresh' : 'Latest successful scoring run'}</b>
        <p>{s ? `${clock(s.scoredAt)} · ${ageText(s.scoredAt, now)}` : 'No current-run snapshot yet. Run the updated prediction pipeline.'}</p>
      </div>
      {s && <dl><div><dt>Usable quotes</dt><dd>{s.usableQuotes}</dd></div><div><dt>Feed conflicts</dt><dd>{s.conflictingBooks}</dd></div><div><dt>Off consensus / thin</dt><dd>{s.offConsensus}</dd></div></dl>}
      {s?.feeds?.length > 0 && <p className="feed-status">{s.feeds.map((f) => `${f.name}: ${f.quotes} usable`).join(' · ')}</p>}
      <p className="small">Published {clock(site.generatedAt)}. {stale ? 'Older quotes are hidden from the open board.' : 'A recent feed quote still needs confirmation at the book.'}</p>
    </section>
  )
}
