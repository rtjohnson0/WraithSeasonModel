import test from 'node:test'
import assert from 'node:assert/strict'
import { v2MarketSource, isV2MarketSource } from './v2MarketSource.js'

test('matchup picks the same-date V2 challenger market and not baseline', () => {
 const baseline = {date:'2026-11-27',games:[{id:1}]}
 const challenger = {date:'2026-11-27',games:[{id:1,quotes:[]}]}
 const site={v2:{board:{date:'2026-11-27'}}, gameMarkets:baseline,gameChallenger:challenger}
 assert.equal(v2MarketSource(site),challenger)
 assert.equal(isV2MarketSource(site),true)
})
test('no future or previous game slate is labeled V2',()=>{
 const base={date:'2026-11-27',games:[{id:1}]}
 const v2={date:'2026-11-26',games:[{id:1}]}
 const site={v2:{board:{date:'2026-11-27'}},gameMarkets:base,gameChallenger:v2}
 assert.equal(v2MarketSource(site),base)
 assert.equal(isV2MarketSource(site),false)
 site.gameChallenger={date:'2026-11-27',games:[]}
 assert.equal(v2MarketSource(site),base)
})
