import test from 'node:test'
import assert from 'node:assert/strict'
import { findGameMarket, remainingSlate, groupTimedCards } from './boardData.js'

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

test('tip-time grouping keeps matchups together and sorts props only within each game', () => {
  const a={id:1,matchup:'GSW @ LAL',tip:'2026-11-14T03:00:00Z'}
  const b={id:2,matchup:'NYK @ CHI',tip:'2026-11-14T00:00:00Z'}
  const c={id:3,matchup:'BOS @ MIA',tip:'2026-11-14T00:00:00Z'}
  const rows=[{game:a,value:9},{game:b,value:8},{game:a,value:10},{game:c,value:4}]
  const groups=groupTimedCards(rows,row=>row.game,(a,b)=>b.value-a.value)
  assert.deepEqual(groups.map(g=>g.game.id),[3,2,1])
  assert.deepEqual(groups[2].cards.map(c=>c.value),[10,9])
  assert.equal(rows[0].value,9)
})

test('unknown or invalid tip times sort last and dated tips keep their real order', () => {
  const rows=[{matchup:'A @ B',tip:'bad'},{matchup:'C @ D',tip:'2026-11-14T00:00:00Z'},{matchup:'E @ F',tip:'2026-11-13T23:00:00Z'}]
  assert.deepEqual(groupTimedCards(rows,row=>row).map(g=>g.game.matchup),['E @ F','C @ D','A @ B'])
})
