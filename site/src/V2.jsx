import { useState } from 'react'
import Board, { Seg } from './Board.jsx'
import Combos from './Combos.jsx'
import Record from './Record.jsx'

// The challenger model: the same board, combos and record pages, fed with V2's plays and results.
export default function V2({ site }) {
  const [tab, setTab] = useState('board')
  const v2 = site.v2
  if (!v2) {
    return (
      <section className="day">
        <h1>V2 test</h1>
        <p className="notice">V2 has not run yet. It appears after the next scheduled run.</p>
      </section>
    )
  }
  const view = { ...site, hideGameRecord:true, gameRecord:[], gameMarkets:null, comboHealth:null, board: v2.board, record: v2.record, archive:v2.archive, historicalCombos:v2.historicalCombos, combos: v2.combos, lines: null }
  return (
    <div className="v2page">
      <section className="day">
        <h1>V2 test</h1>
        <p className="small">Experimental model · separate picks and record · paper tracking.</p>
        <div className="v2-status"><span><b>{v2.board?.plays||0}</b> plays on this board</span><span><b>{v2.info?.strong||0}</b> strong</span><span><b>{v2.info?.lean||0}</b> leans</span><a href="#/about">How V2 works ↗</a></div>
        <Seg
          label="V2 section"
          value={tab}
          onChange={setTab}
          options={[
            ['board', 'Board'],
            ['combos', 'Combos'],
            ['record', 'Record'],
          ]}
        />
      </section>
      {tab === 'board' && <Board site={view} />}
      {tab === 'combos' && <Combos site={view} />}
      {tab === 'record' && <Record site={view} />}
    </div>
  )
}
