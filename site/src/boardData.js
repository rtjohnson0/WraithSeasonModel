// Pair cards with their own game. Missing IDs must never match another missing ID.
export function findGameMarket(game, games = []) {
  return games.find(g => game.id != null && g.id != null && String(g.id) === String(game.id))
    || games.find(g => game.matchup && `${g.away} @ ${g.home}` === game.matchup)
}

// Keep game-only projections reachable without a second competing picks board.
export function remainingSlate(games = [], markets = []) {
  const remaining = games.filter(g => !(g.plays?.length || g.more?.length))
  for (const market of markets) {
    if (!games.some(g => findGameMarket(g, [market]))) {
      remaining.push({ id: market.id, matchup: `${market.away} @ ${market.home}`, tip: market.tip })
    }
  }
  return remaining
}

export function compareGameTip(a, b) {
  const time = game => Number.isFinite(Date.parse(game.tip)) ? Date.parse(game.tip) : Infinity
  const at = time(a), bt = time(b)
  if (at !== bt) return at < bt ? -1 : 1
  return String(a.matchup || '').localeCompare(String(b.matchup || ''))
}

// Keep every matchup together, even when several games share the same tip.
// Value/probability ordering applies only to cards inside that matchup.
export function groupTimedCards(rows, gameOf, compareWithin) {
  const groups = new Map()
  rows.forEach((row, index) => {
    const game = gameOf(row)
    const key = game.id != null ? `id:${game.id}` : game.matchup || `pending:${index}`
    if (!groups.has(key)) groups.set(key, { key, game, cards: [] })
    groups.get(key).cards.push(row)
  })
  return [...groups.values()].sort((a, b) => compareGameTip(a.game, b.game)).map(group => ({
    ...group, cards: compareWithin ? [...group.cards].sort(compareWithin) : group.cards,
  }))
}
