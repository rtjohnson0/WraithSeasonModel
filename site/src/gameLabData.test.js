import test from 'node:test'
import assert from 'node:assert/strict'
import { validateGameLab, gameScoresVisible, gameEvidenceSupported } from './gameLabData.js'

const sample = () => ({
  schema:1, policy:'v21-game-markets-read-only-v1', researchOnly:true,
  date:'2026-11-21', generatedAt:'2026-11-21T21:05:00Z',
  markets:['moneyline','spread','total'].map(id=>({
    id, n:9, days:1, calibratedN:0,
    rawLogLoss:.70, bookLogLoss:.68, calibratedLogLoss:null,
    calibrationRawLogLoss:null, calibrationBookLogLoss:null,
  }))
})

test('accepts only market-separated aggregate counts and numeric scores', () => {
  const d=sample()
  assert.equal(validateGameLab(d), d)
  assert.equal(d.markets[0].id, 'moneyline')
  assert.equal(d.markets[2].id, 'total')
  assert.ok(!gameScoresVisible(d.markets[0]))
  d.markets[0].n=50
  d.markets[0].days=5
  assert.ok(gameScoresVisible(d.markets[0]))
  assert.ok(!gameEvidenceSupported(d.markets[0]))
  d.markets[0].n=200
  d.markets[0].days=21
  assert.ok(gameEvidenceSupported(d.markets[0]))
})

test('rejects swapped, missing or corrupt markets', () => {
  const d=sample()
  d.markets.reverse()
  assert.throws(()=>validateGameLab(d))
  const e=sample()
  e.markets[0].n=-1
  assert.throws(()=>validateGameLab(e))
  const f=sample()
  f.markets[2].bookLogLoss=Number.NaN
  assert.throws(()=>validateGameLab(f))
  const g=sample()
  g.markets[1].calibratedN=2
  assert.throws(()=>validateGameLab(g))
})
