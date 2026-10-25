export default function About() {
  return (
    <article className="about">
      <h1>How the picks work</h1>
      <p>
        For every NBA player prop with a posted line, a model projects the player's minutes and then his points,
        rebounds or assists in those minutes. It compares that with the sportsbook line and the price, and gives the
        chance that the over or the under hits.
      </p>

      <h2>What the model looks at</h2>
      <p>
        Recent minutes and how steady they are, the player's own history, the opponent's defense against his position,
        pace, rest and back-to-backs, which teammates are out, and the game's spread and total. For rebounds and assists it
        also uses tracking data: rebound chances, passes and touches.
      </p>

      <h2>Play types</h2>
      <ul>
        <li><b>Max play.</b> The only kind the model stakes: at least 55% for the over or 45% or lower for the under, a real edge over the price, and a stat that has held up in testing.</li>
        <li><b>Great play.</b> A strong number (58% or more) that is not staked, because the stat is unproven in testing or the team is still early in its season.</li>
        <li><b>Risky play.</b> Clears the bar, but minutes, an injury status or an odd line need a look first.</li>
        <li><b>Lean.</b> A smaller edge, listed so every game has plays. Not a recommendation.</li>
      </ul>

      <h2>The icons</h2>
      <p>
        Small icons flag what is behind a play: a weak or tough defense against the player's position, a hot or cold
        streak, minutes projected up or down, a back-to-back, teammates out, or an injury status to check. Green means
        it helps that pick, red means it works against it, and purple is a note. They are context, not extra
        confidence: the percentage already counts most of them.
      </p>

      <h2>Letter grades</h2>
      <p>
        The grade comes straight from the model's chance for that side: A+ at 66% and up, A at 63%, A- at 60%, then B+
        at 58%, B at 56%, B- at 55%, and C and D below that. Only Max plays can earn an A. Everything else tops out at
        B+, because a high number on an unproven stat is still unproven. A grade is confidence, not a promise.
      </p>

      <h2>What we do not claim</h2>
      <p>
        The model is still being tested, and no profitable edge has been proven. The record page shows every graded play,
        including the losses and the plays that were never staked, so you can judge it yourself. Sportsbooks see most of
        the same information, and the model's lean is often small.
      </p>

      <p className="small">
        For information and entertainment. 21+ only. Bet only what you can afford to lose. If gambling is a problem, call
        1-800-GAMBLER.
      </p>
    </article>
  )
}
