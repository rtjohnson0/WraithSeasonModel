// Light schedule dividers keep the player cards as the primary content.
export default function TimedGroups({ groups, className, renderCard }) {
  return <div className="timed-groups">
    {groups.map(({ key, game, cards }) => <section className="timed-game" key={key} aria-label={`${game.matchup || 'Matchup pending'} player props`}>
      <header className="schedule-divider"><div><time dateTime={game.tip || undefined}>{Number.isFinite(Date.parse(game.tip)) ? new Date(game.tip).toLocaleTimeString('en-US', { hour:'numeric', minute:'2-digit', timeZone:'America/New_York' }) + ' ET' : 'Tip time pending'}</time><h3>{game.matchup || 'Matchup pending'}</h3></div><span>{cards.length} {cards.length === 1 ? 'prop' : 'props'}</span></header>
      <ul className={className} aria-label={`${game.matchup || 'Matchup pending'} props`}>{cards.map(renderCard)}</ul>
    </section>)}
  </div>
}
