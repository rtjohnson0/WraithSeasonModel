import test from 'node:test'
import assert from 'node:assert/strict'
import {modelReturn,comparePlayValue} from './format.js'
test('higher favorite confidence does not outrank a better forecast price',()=>{
 const favorite={prob:.67,odds:-188,edge:.017},normal={prob:.552,odds:-105,edge:.04}
 assert(Math.abs(modelReturn(favorite)-.0263829787)<1e-8)
 assert(Math.abs(modelReturn(normal)-.0777142857)<1e-8)
 assert.equal([favorite,normal].sort(comparePlayValue)[0],normal)
})
test('unknown prices cannot appear as the best forecast return',()=>{
 assert.equal(modelReturn({prob:.8,odds:0}),-Infinity)
 assert.equal(modelReturn({prob:NaN,odds:-110}),-Infinity)
})
