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
