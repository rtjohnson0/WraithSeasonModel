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
