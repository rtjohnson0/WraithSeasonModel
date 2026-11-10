import test from 'node:test'
import assert from 'node:assert/strict'
import { quoteState, bookName } from './quote.js'
const now = Date.parse('2026-11-10T18:00:00Z')
const recent = { tip: '2026-11-10T20:00:00Z', quoteAt: '2026-11-10T17:55:00Z', sourceUpdatedAt: '2026-11-10T17:50:00Z', bookCount: 3 }
test('recent, stale, and started quotes are distinct', () => {
  assert.equal(quoteState(recent, now).usable, true)
  assert.equal(quoteState({ ...recent, quoteAt: '2026-11-10T16:00:00Z' }, now).key, 'stale')
  assert.equal(quoteState(recent, Date.parse(recent.tip)).key, 'closed')
})
test('new publication cannot make an old quote fresh', () => {
  assert.equal(quoteState({ ...recent, quoteAt: '2026-11-10T16:00:00Z', generatedAt: '2026-11-10T18:00:00Z' }, now).usable, false)
})
test('missing provenance and thin markets do not appear as executable', () => {
  assert.equal(quoteState({}, now).usable, false)
  assert.equal(quoteState({ ...recent, bookCount: 2 }, now).usable, false)
})
test('book names do not mix provider IDs with physical books', () => {
  assert.equal(bookName('sgo/fanduel'), 'FanDuel')
  assert.equal(bookName('draftkings'), 'DraftKings')
})

import { marketLabel } from './format.js'
test('period labels distinguish full game, quarter, and unknown legacy entries', () => {
 assert.equal(marketLabel({stat:'points',period:'game'}), 'Full game · Points')
 assert.equal(marketLabel({stat:'points',period:'1q'}), '1Q · Points')
 assert.equal(marketLabel({stat:'points'}), 'Period unverified · Points')
 assert.equal(quoteState({...recent,period:'1q'},now).usable,false)
})

test('1Q quotes require the quarter context and cannot enter the full-game board', () => {
 const q = {...recent,period:'1q'}
 assert.equal(quoteState(q,now).usable,false)
 assert.equal(quoteState(q,now,'1q').usable,true)
 assert.equal(quoteState({...q,period:'game'},now,'1q').usable,false)
})

test('published plays remain visible after price expiry and tip, with freshness separate', async () => {
  const { visibleQuote } = await import('./quote.js')
  assert.equal(visibleQuote(recent), true)
  assert.equal(visibleQuote({ ...recent, quoteAt: '2026-11-10T12:00:00Z' }), true)
  assert.equal(visibleQuote({ ...recent, period: '1q' }), false)
  assert.equal(visibleQuote({ ...recent, period: '1q' }, '1q'), true)
  assert.equal(quoteState(recent, Date.parse(recent.tip) + 1000).usable, false)
  assert.equal(visibleQuote(recent), true)
})
