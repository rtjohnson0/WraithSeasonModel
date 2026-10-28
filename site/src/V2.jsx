import { useState } from 'react'
import Board, { Seg } from './Board.jsx'
import Combos from './Combos.jsx'
import Record from './Record.jsx'
import { statLabel } from './format.js'

const pctChange = (x) => (Math.abs(x - 1) < 0.0005 ? 'no change' : `${x > 1 ? '+' : ''}${((x - 1) * 100).toFixed(1)}%`)
const joinStats = (obj) =>
  Object.keys(obj || {})
    .map((s) => `${(statLabel[s] || s).toLowerCase()} ${pctChange(obj[s])}`)
    .join(', ')

const stateText = { proven: 'proven', failed: 'not beating the base rate', unproven: 'not enough lines yet' }

// V2's own record by stat and side: what lets a play be Strong, and what blocks one.
function Gates({ gates, minN }) {
  if (!gates?.length) return null
  return (
    <details className="v2gates">
      <summary>V2&apos;s record by stat and side (a play needs {minN}+ graded lines to be judged)</summary>
      <table>
        <thead>
          <tr>
            <th>Stat and side</th>
            <th>Graded lines</th>
            <th>Leans right</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {gates.map((g) => (
            <tr key={`${g.stat}-${g.side}`}>
              <td>
                {(statLabel[g.stat] || g.stat).toLowerCase()} {g.side.toLowerCase()}s
              </td>
              <td>{g.n}</td>
              <td>{g.n > 0 ? `${(g.hit * 100).toFixed(0)}%` : '-'}</td>
              <td className={`state-${g.state}`}>{stateText[g.state] || g.state}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </details>
  )
}

function Changes({ info }) {
  if (!info) return null
  const learned = info.players > 0
  return (
    <div className="v2changes">
      <p>
        <b>What V2 does differently.</b> (1) It levels each stat to the market: the books&apos; lines are right on average, so if the model&apos;s projections
        sit under them across the whole slate, that is a level error, and V2 moves the level while keeping who it likes more or less than the book.
        (2) It adds a live correction from how the model did on earlier days, for starters&apos; minutes and for each stat against the lines, shrunk hard while
        the sample is small. (3) It gives the model {Math.round(info.weight * 100)}% of the say and the book {Math.round((1 - info.weight) * 100)}%; the main
        model&apos;s calibration gave points about 3% and rebounds about 24% at its last fit. (4) {info.model.startsWith('model_v2') ? 'It projects with a second model that also uses who is out and expected missed shots.' : 'It projects with the main model (a second model with the teammates-out and missed-shots features has not been trained yet).'}{' '}
        Combos get the same treatment. Nothing is staked, and no pick on the main board changes.
      </p>
      <p>
        Today&apos;s level correction: {joinStats(info.anchor)}.{' '}
        {learned
          ? `Live correction from ${info.players} player-games over ${info.days} earlier ${info.days === 1 ? 'day' : 'days'}: starters' minutes ${info.min_shift >= 0 ? '+' : ''}${info.min_shift.toFixed(2)}, against the lines ${joinStats(info.ratio)}.`
          : 'There are no earlier days to learn the live correction from yet.'}
      </p>
      <p>
        <b>How V2 tightens the board.</b> A play that clears the {Math.round(info.bar * 100)}% bar is a <b>Strong play</b> only when the real model leans
        the same way (53% or more on that side) and V2&apos;s own record on that stat and side, graded over every line in the market log, beats always
        saying the stat&apos;s base rate on {info.gate_min_n}+ lines. Otherwise it is a <b>Lean</b>: posted and tracked, not backed. It is a <b>Risky
        play</b> when the model and the book are 15+ points apart (more often missing news than an edge) or the player&apos;s status is uncertain. It is
        left out when the real model leans the other way at 53% or more, when fewer than 4 books list it or their lines are scattered, or when V2&apos;s record
        on that stat and side has {info.gate_min_n}+ graded lines and does not beat the base rate. Overs and Unders face identical rules.
      </p>
      <p>
        {info.scored} lines scored: {info.over} lean Over, {info.under} lean Under. {info.posted} posted: {info.strong} Strong, {info.check} Risky,{' '}
        {info.lean} Lean. Left out: {info.dropped?.thin || 0} thin or scattered markets, {info.dropped?.disagree || 0} the real model disagrees with ({info.dropped?.disagree_over || 0} of them Overs),{' '}
        {info.dropped?.failed || 0} on a stat and side that failed its gate. V2 does not force Overs; the mix is what its probabilities say.
      </p>
      <Gates gates={info.gates} minN={info.gate_min_n} />
    </div>
  )
}

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
  const view = { ...site, board: v2.board, record: v2.record, combos: v2.combos, lines: null }
  return (
    <div className="v2page">
      <section className="day">
        <h1>V2 test</h1>
        <p className="honest">
          A challenger model running beside the real one. Its plays are tracked and graded on their own, never staked, and kept out of the main record,
          so you can see whether the changes below earn their way onto the main board.
        </p>
        <Changes info={v2.info} />
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
