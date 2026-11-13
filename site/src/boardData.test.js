import test from 'node:test'
import assert from 'node:assert/strict'
import { findGameMarket, remainingSlate } from './boardData.js'

test('embedded game context matches IDs and legacy combo matchup names without pairing missing IDs', () => {
  const games = [{away:'NYK',home:'CHI'}, {id:42,away:'GSW',home:'LAL'}]
  assert.equal(findGameMarket({id:'42'},games),games[1])
  assert.equal(findGameMarket({matchup:'GSW @ LAL'},games),games[1])
  assert.equal(findGameMarket({matchup:'BOS @ MIA'},games),undefined)
  assert.equal(findGameMarket({},games),undefined)
})

test('unattached game markets stay reachable and prop matchups are not duplicated', () => {
  const board = [{id:1,matchup:'GSW @ LAL',plays:[{}]}, {id:2,matchup:'NYK @ CHI',plays:[],more:[]}]
  const market = [{id:1,away:'GSW',home:'LAL'}, {id:2,away:'NYK',home:'CHI'}, {id:3,away:'BOS',home:'MIA'}]
  assert.deepEqual(remainingSlate(board,market).map(g=>g.matchup),['NYK @ CHI','BOS @ MIA'])
})
