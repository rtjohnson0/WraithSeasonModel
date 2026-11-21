import test from 'node:test'
import assert from 'node:assert/strict'
import { validateV21Summary, scoresVisible, lowestMinutesMAE } from './v21LabData.js'

function demo() {
  return {
    schema:1, policy:'v21-minute-guard-and-cap-20261121', dates:3, playerGames:319, questions:1138,
    candidates:['base-minutes','rotation-v4','supported-only','capped-shift'].map((id, i) => ({
      id, playerGames:319, questions:1138, minutesMAE:4+i*.1, minutesBias:.2,
      brier:.25, logLoss:.69, statMAE:1.6
    }))
  }
}

test('V2.1 model lab accepts correct aggregate-only schema', () => {
  const d = demo()
  assert.equal(validateV21Summary(d), d)
  assert.equal(lowestMinutesMAE(d.candidates), 4)
  assert.ok(scoresVisible(d.candidates[0]))
})

test('no missing or fake samples qualify as evidence', () => {
  const d=demo()
  d.candidates[0].questions=12
  assert.ok(!scoresVisible(d.candidates[0]))
  assert.equal(lowestMinutesMAE(d.candidates), 4.1)
  for(const c of d.candidates) c.questions=1
  assert.equal(lowestMinutesMAE(d.candidates), Infinity)
})

test('invalid schema and private per-player masquerade do not render', () => {
  const d=demo()
  d.candidates[0].id='player-special'
  assert.throws(() => validateV21Summary(d))
  d.candidates[0].id='base-minutes'
  d.candidates[0].brier=Number.NaN
  assert.throws(() => validateV21Summary(d))
  d.candidates[0].brier=.25
  d.questions=-1
  assert.throws(() => validateV21Summary(d))
})
