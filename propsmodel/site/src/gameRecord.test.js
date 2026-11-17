import test from 'node:test'
import assert from 'node:assert/strict'
import {gameRecordRows,summarizeGameRows} from './gameRecord.js'
test('game record separates paper directions and excludes pending from hit rate',()=>{
 const rows=[{market:'spread',tier:'PICK',result:'WIN',units:.91},{market:'spread',tier:'PAPER',result:'LOSS',units:-1},{market:'spread',tier:'PICK',result:'PENDING',units:0},{market:'spread',tier:'PICK',result:'PUSH',units:0}]
 const s=summarizeGameRows(gameRecordRows(rows,true)).find(r=>r.market==='spread')
 assert.equal(s.wins,1);assert.equal(s.losses,0);assert.equal(s.hitRate,1);assert.equal(s.pending,1);assert.equal(s.pushes,1)
 assert.equal(summarizeGameRows(rows).find(r=>r.market==='spread').hitRate,.5)
})
