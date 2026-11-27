// V2 matchup cards must use the V2 game challenger when it has a
// same-slate game feed. Never label the original V1 game forecast as V2.
export function v2MarketSource(site) {
  const challenger = site?.gameChallenger
  if (challenger?.date && challenger?.date === site?.v2?.board?.date && challenger?.games?.length) return challenger
  return site?.gameMarkets ?? null
}

export function isV2MarketSource(site) {
  const chosen = v2MarketSource(site)
  return !!chosen && chosen === site?.gameChallenger
}
