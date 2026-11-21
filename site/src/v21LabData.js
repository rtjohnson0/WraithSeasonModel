// Pure UI guardrails for the public aggregate-only V2.1 evidence file.
export const V21_POLICY = 'v21-minute-guard-and-cap-20261121'
export const CANDIDATE_IDS = ['base-minutes', 'rotation-v4', 'supported-only', 'capped-shift']

const finite = x => typeof x === 'number' && Number.isFinite(x)
const count = x => Number.isSafeInteger(x) && x >= 0

export function validateV21Summary(raw) {
  if (!raw || raw.schema !== 1 || raw.policy !== V21_POLICY || !Array.isArray(raw.candidates)
      || raw.candidates.length !== CANDIDATE_IDS.length)
    throw new Error('The V2.1 research report is missing or has an unsupported format.')
  if (!['dates', 'playerGames', 'questions'].every(k => count(raw[k])))
    throw new Error('The V2.1 report contains invalid evaluation counts.')
  for (let i = 0; i < CANDIDATE_IDS.length; i++) {
    const x = raw.candidates[i]
    if (!x || x.id !== CANDIDATE_IDS[i] || !count(x.questions) || !count(x.playerGames)
      || !['minutesMAE', 'minutesBias', 'brier', 'logLoss', 'statMAE'].every(k => finite(x[k])))
      throw new Error('The V2.1 report contains invalid challenger statistics.')
  }
  return raw
}

export const scoresVisible = score =>
  score?.playerGames >= 30 && score?.questions >= 30 && Number.isFinite(score?.minutesMAE) && score.minutesMAE > 0

export const lowestMinutesMAE = candidates => Math.min(...candidates.filter(scoresVisible).map(x => x.minutesMAE))
