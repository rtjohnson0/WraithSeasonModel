export const pct = (x, d = 0) => `${(100 * x).toFixed(d)}%`

export const oddsText = (o) => (o ? (o > 0 ? `+${o}` : `${o}`) : '')

export function prettyDate(iso) {
  if (!iso) return ''
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(Date.UTC(y, m - 1, d)).toLocaleDateString('en-US', {
    weekday: 'long', month: 'long', day: 'numeric', timeZone: 'UTC',
  })
}

export function shortDate(iso) {
  if (!iso) return ''
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(Date.UTC(y, m - 1, d)).toLocaleDateString('en-US', {
    month: 'short', day: 'numeric', timeZone: 'UTC',
  })
}

export const statLabel = { points: 'Points', rebounds: 'Rebounds', assists: 'Assists', threes: '3-pointers' }

// The four play names. They describe the evidence, not only the number: only Max plays are staked.
export const labelInfo = {
  'Max play': { key: 'max', note: 'Staked: the model backs this one with a stake, on a stat that has held up in testing.' },
  'Great play': { key: 'great', note: 'A strong number, but not staked: the stat is unproven in testing or the team is early in its season.' },
  'Risky play': { key: 'risky', note: 'Clears the bar, but minutes, an injury status or an odd line need a look before you bet.' },
  Lean: { key: 'lean', note: 'A smaller edge, listed so every game has plays. Not a recommendation.' },
}
export const LABELS = Object.keys(labelInfo)

// The letter-grade tile takes its colour from the letter, so a quick scan shows the strong plays.
export function gradeTone(g) {
  if (g.startsWith('A')) return 'a'
  if (g.startsWith('B')) return 'b'
  if (g.startsWith('C')) return 'c'
  return 'd'
}
