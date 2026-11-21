// Game markets cannot pool ML/spread/total outcomes or compare scores
// on different forecast cohorts. Keep them separate throughout the UI.
export const GAME_MARKET_POLICY = 'v21-game-markets-read-only-v1'
export const GAME_IDS = ['moneyline', 'spread', 'total']
const validCount = v => Number.isSafeInteger(v) && v >= 0
const score = v => v === null || (typeof v === 'number' && Number.isFinite(v) && v >= 0 && v < 100)

export function validateGameLab(raw) {
  if (!raw || raw.schema !== 1 || raw.policy !== GAME_MARKET_POLICY
    || raw.researchOnly !== true || !Array.isArray(raw.markets)
    || raw.markets.length !== GAME_IDS.length)
    throw new Error('Game-market lab report is missing or invalid.')
  raw.markets.forEach((m, i) => {
    if (!m || m.id !== GAME_IDS[i] || !validCount(m.n) || !validCount(m.days)
      || !validCount(m.calibratedN) || m.calibratedN > m.n
      || !['rawLogLoss', 'bookLogLoss', 'calibratedLogLoss',
           'calibrationRawLogLoss', 'calibrationBookLogLoss'].every(k => score(m[k])))
      throw new Error('Game-market evaluation is not usable.')
    if (m.n > 0 && (m.rawLogLoss == null || m.bookLogLoss == null))
      throw new Error('Game-market baseline comparison is missing.')
    if (m.calibratedN > 0 && (m.calibratedLogLoss == null || m.calibrationBookLogLoss == null))
      throw new Error('Calibrated scores must share verified games with the book.')
  })
  return raw
}

// Minimum gates are display safeguards, not evidence of a profitable strategy.
export const gameScoresVisible = m => m.n >= 30 && m.days >= 5
export const gameEvidenceSupported = m => m.n >= 200 && m.days >= 20
