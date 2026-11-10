import test from 'node:test'
import assert from 'node:assert/strict'
import { explainProp, explainGame } from './explain.js'
test('a favorable scoring matchup is a counterargument to an under', () => {
 const e = explainProp({side:'UNDER',period:'game',stat:'points',line:20.5,projStat:18.9,projMin:29,minTypical:34,prob:.57,last5:[1,2,3,4,5],l5Hits:3,defense:{helps:false,text:'Allows the 2nd-most points to centers (+37%)'}})
 assert(e.against.some((s) => s.includes('2nd-most') && s.includes('against the Under')))
 assert(e.support.some((s) => s.includes('29.0')))
 assert(e.sensitivity.includes('22.2'))
})
test('a probable winner is not automatically a value pick', () => {
 const g={home:'BOS',away:'NYK',margin:5,total:220}
 const q={market:'moneyline',sides:[{label:'BOS',conditional:.65,ev:-.02,edge:-.01,breakEven:.66},{label:'NYK',conditional:.35,ev:-.05,edge:-.02,breakEven:.37}]}
 const e=explainGame(g,q);assert.equal(e.choice,'Pass at these prices');assert(e.why.includes('Projected winner BOS'));assert(e.price.includes('Neither side'))
})
test('total explanations apply to the selected book threshold', () => {
 const g={home:'BOS',away:'NYK',margin:5,total:220}
 const sides=[{label:'Under',conditional:.6,ev:.1,edge:.05,breakEven:.55},{label:'Over',conditional:.4,ev:-.2,edge:-.15,breakEven:.55}]
 const e=explainGame(g,{market:'total',line:225.5,sides});assert(e.choice.includes('Under 225.5'));assert(e.why.includes('Projected total 220.0 versus 225.5'))
})
