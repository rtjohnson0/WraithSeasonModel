import test from 'node:test'
import assert from 'node:assert/strict'
import { comboGroups } from './combo.js'

test('an extreme raw estimate cannot enter the selected combo board', () => {
  const pick = {tier:'COMBO', prob:.57}
  const comparison = {tier:'COMBOWATCH', prob:.98}
  const groups = comboGroups([comparison,pick,{prob:.99,tier:'UNKNOWN'}])
  assert.deepEqual(groups.picks,[pick])
  assert.deepEqual(groups.comparisons,[comparison])
  assert.deepEqual(comboGroups().picks,[])
})

test('experimental V2 combos remain comparisons even with a legacy selected tier', () => {
 const research={tier:'COMBORESEARCH',prob:.8}
 const legacy={tier:'COMBO',rotation:{policy:'rotation-v1'}}
 const explicit={tier:'COMBO',research:true}
 const groups=comboGroups([research,legacy,explicit])
 assert.equal(groups.picks.length,0)
 assert.deepEqual(groups.comparisons,[research,legacy,explicit])
})
