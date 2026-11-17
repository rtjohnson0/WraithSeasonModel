import test from 'node:test'
import assert from 'node:assert/strict'
import {photoFor,initials} from './photos.js'
import {matchupSignal,defenseEffect} from './matchup.js'
test('photos require an exact normalized NBA roster name, never a different provider ID',()=>{
 assert.equal(photoFor('Karl-Anthony Towns'),'https://cdn.nba.com/headshots/nba/latest/260x190/1626157.png')
 assert.equal(photoFor('Not An NBA Player'),null);assert.equal(photoFor('Karl Towns'),null);assert.equal(initials('Karl-Anthony Towns'),'KT')
})
test('favorable rebounding opponent is explicitly against an under',()=>{
 const s=matchupSignal({stat:'rebounds',side:'UNDER',matchupContext:{opponent:{name:'NYK'},position:[{stat:'rebounds',rank:26,of:30,position:'centers'}]}})
 assert.equal(s.helps,false);assert(s.label.includes('5th-most'))
})
test('defense impact uses measured fitted sensitivity and never the displayed allowance percentage',()=>{
 assert.equal(defenseEffect('defense vs centers: 5th-most (+18%); fitted defense-rate sensitivity +0.6% versus league-neutral').percent,.6)
 assert.equal(defenseEffect('opponent-defense rate features are disabled').active,false)
 assert.equal(defenseEffect('Allows 18% more').active,null)
})
