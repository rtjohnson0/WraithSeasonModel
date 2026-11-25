import test from 'node:test'
import assert from 'node:assert/strict'
import {liveFresh,liveBookConfidence,liveRows} from './inGameData.js'
const now=Date.parse('2026-11-25T21:00:00Z')
const valid={schema:1,capturedAt:'2026-11-25T20:57:00Z',plays:[]}
test('stale halftime scoreboard never masquerades as live',()=>{
 assert.equal(liveFresh(valid,now),true)
 assert.equal(liveFresh({...valid,capturedAt:'2026-11-25T20:49:00Z'},now),false)
 assert.equal(liveFresh({...valid,capturedAt:'2026-11-25T21:02:00Z'},now),false)
})
test('book implied is not invented when quote missing or invalid',()=>{
 assert.equal(liveBookConfidence(null),null)
 assert.equal(liveBookConfidence({overMarketProb:.51,underMarketProb:.49}).over,.51)
 assert.equal(liveBookConfidence({overMarketProb:.7,underMarketProb:.7}),null)
})
test('sorts eligible quotes by higher market confidence and filters halftime',()=>{
 const rows=[
 {gameId:2,player:'B',gameStatus:'2nd Qtr',live:null},
 {gameId:1,player:'A',gameStatus:'Halftime',live:{overMarketProb:.52,underMarketProb:.48}},
 {gameId:3,player:'C',gameStatus:'Halftime',live:{overMarketProb:.35,underMarketProb:.65}},
 ]
 assert.deepEqual(liveRows(rows).map(x=>x.player),['C','A','B'])
 assert.deepEqual(liveRows(rows,'percentage','half').map(x=>x.player),['C','A'])
})
