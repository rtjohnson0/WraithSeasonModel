// A recent API snapshot is not a guarantee that a sportsbook still offers the bet.
export function quoteState(p, now = Date.now()) {
  const tip = Date.parse(p.tip || '')
  if (Number.isFinite(tip) && now >= tip) return { key: 'closed', label: 'Started · quote closed', usable: false }
  const captured = Date.parse(p.quoteAt || '')
  const updated = Date.parse(p.sourceUpdatedAt || '')
  if (!Number.isFinite(captured) || !Number.isFinite(updated) || !Number.isFinite(tip)) {
    return { key: 'unknown', label: 'Quote needs confirmation', usable: false }
  }
  const captureAge = (now - captured) / 1000
  const priceAge = (now - updated) / 1000
  if (captureAge < -5 || priceAge < -5) return { key: 'unknown', label: 'Quote time unavailable', usable: false }
  if (captureAge > (p.captureMaxAgeSeconds || 1800) || priceAge > (p.priceMaxAgeSeconds || 21600)) {
    return { key: 'stale', label: 'Stale snapshot · refresh required', usable: false }
  }
  if ((p.bookCount || 0) < 3) return { key: 'unknown', label: 'Thin market · confirm line', usable: false }
  return { key: 'recent', label: 'Recent feed quote', usable: true }
}

export function bookName(book = '') {
  const id = book.replace(/^sgo\//, '')
  return { draftkings: 'DraftKings', fanduel: 'FanDuel', betmgm: 'BetMGM', caesars: 'Caesars' }[id] || id
}

export function clock(iso) {
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? 'Unknown' : date.toLocaleString('en-US', {
    month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit', timeZone: 'America/New_York',
  }) + ' ET'
}
export function ageText(iso, now = Date.now()) {
  const date = Date.parse(iso || '')
  if (!Number.isFinite(date)) return 'unknown age'
  const minutes = Math.max(0, Math.floor((now - date) / 60000))
  return minutes < 1 ? 'less than 1 min ago' : minutes < 60 ? `${minutes} min ago` : `${Math.floor(minutes / 60)}h ${minutes % 60}m ago`
}
