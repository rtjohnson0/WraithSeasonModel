import test from 'node:test'
import assert from 'node:assert/strict'
import { explainProp, explainGame, bestGameQuote, positionReasons } from './explain.js'
test('a favorable scoring matchup is a counterargument to an under', () => {
 const e = explainProp({side:'UNDER',period:'game',stat:'points',line:20.5,projStat:18.9,projMin:29,minTypical:34,prob:.57,last5:[1,2,3,4,5],l5Hits:3,defense:{helps:false,text:'Allows the 2nd-most points to centers (+37%)'}})
 assert(e.against.some((s) => s.includes('2nd-most') && s.includes('against the Under')))
 assert(e.support.some((s) => s.includes('29.0')))
 assert(e.sensitivity.includes('22.2'))
})
test('a probable winner is not automatically a value pick', () => {
 const g={home:'BOS',away:'NYK',margin:5,total:220}
 const q={market:'moneyline',sides:[{label:'BOS',odds:-200,conditional:.65,ev:-.02,edge:-.01,breakEven:.66},{label:'NYK',odds:170,conditional:.35,ev:-.05,edge:-.02,breakEven:.37}]}
 const e=explainGame(g,q);assert.equal(e.choice,'BOS ML');assert(e.why.includes('Projected winner BOS'));assert(e.price.includes('costs more'))
})
test('total explanations apply to the selected book threshold', () => {
 const g={home:'BOS',away:'NYK',margin:5,total:220}
 const sides=[{label:'Under',conditional:.6,ev:.1,edge:.05,breakEven:.55},{label:'Over',conditional:.4,ev:-.2,edge:-.15,breakEven:.55}]
 const e=explainGame(g,{market:'total',line:225.5,sides});assert(e.choice.includes('Under 225.5'));assert(e.why.includes('Projected total 220.0 versus 225.5'))
})
test('Milwaukee payout value never replaces the predicted Knicks winner',()=>{
 const e=explainGame({home:'NYK',away:'MIL',margin:16.2,awayScore:103.7,homeScore:119.9},{market:'moneyline',sides:[{label:'NYK',conditional:.873,odds:-1430},{label:'MIL',conditional:.127,odds:750}]})
 assert.equal(e.choice,'NYK ML');assert(e.alternative.includes('MIL'));assert(e.price.includes('93.5%'))
})
test('Thunder 55.9 percent at minus 120 clears the price even if stale stored fields disagree',()=>{
 const e=explainGame({home:'POR',away:'OKC',margin:-2.1},{market:'moneyline',sides:[{label:'OKC',conditional:.559,odds:-120,edge:-.1,ev:-.1},{label:'POR',conditional:.441,odds:100}]})
 assert.equal(e.choice,'OKC ML');assert(e.primary.edge>0);assert(e.primary.ev>0);assert(e.price.includes('54.5%'));assert(e.shortPrice.includes('Positive model value'))
})
test('default book compares quotes for predicted winner instead of feed order',()=>{
 const quotes=[{book:'ballybet',market:'moneyline',sides:[{label:'OKC',conditional:.559,odds:-143},{label:'POR',conditional:.441,odds:120}]},{book:'draftkings',market:'moneyline',sides:[{label:'OKC',conditional:.559,odds:-120},{label:'POR',conditional:.441,odds:100}]}]
 assert.equal(bestGameQuote({home:'POR',away:'OKC',margin:-2.1},quotes).book,'draftkings')
})
test('raw combo estimates are explicitly distinct from calibrated probabilities',()=>{
 const e=explainProp({side:'OVER',period:'game',stat:'pra',line:30.5,projStat:34,prob:.637,probabilityBasis:'estimate'})
 assert(e.decision.includes('63.7%'));assert(e.decision.includes('uncalibrated simulation'));assert(!e.decision.includes('final probability also reflects'))
})

test('spread labels use each side handicap and analyze cover rather than winning outright',()=>{
 const e=explainGame({home:'BOS',away:'NYK',margin:2,total:220,homeScore:111,awayScore:109},{market:'spread',line:-5.5,sides:[{label:'BOS',conditional:.4,odds:-110},{label:'NYK',conditional:.6,odds:-110}]})
 assert.equal(e.choice,'NYK +5.5');assert(e.why.includes('cover probability'));assert(!e.why.includes('Projected total'))
})

test('combo matchup explanation covers its components rather than borrowing points-only defense',()=>{
 const context={opponent:{name:'NYK'},position:[{stat:'points',rank:1,of:30,average:10,games:8,position:'centers'},{stat:'rebounds',rank:29,of:30,average:14,games:8,position:'centers'},{stat:'assists',rank:2,of:30,average:3,games:8,position:'centers'}]}
 const reasons=positionReasons({stat:'ra',side:'UNDER',matchupContext:context})
 assert.equal(reasons.length,2);assert(reasons.some(x=>!x.helps&&x.text.includes('Rebounds')));assert(!reasons.some(x=>x.text.includes('points allowed')))
})
