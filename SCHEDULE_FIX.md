# Full-slate game count

The old daily board discovered games from successful player-prop downloads and then excluded games whose tip time was missing or had passed. That made the count depend on odds coverage and time of day.

The snapshot now saves the complete successful schedule response independently, before fetching props. Prediction, game markets and quarter processing read that schedule. The main board publishes every scheduled matchup, even without usable odds, a known tip time, or qualifying picks. Existing verified pregame plays remain visible after tip with the closed-game indication; removed upcoming plays are not restored.

A successful empty schedule is authoritative and clears the slate. Invalid dates, duplicate game identities and future captures cannot be treated as a complete schedule. Schedule snapshots older than 24 hours are labeled as incomplete coverage. Older installs without the new snapshot use a clearly labeled games-with-data fallback, not a claimed full-slate count.

Install the updated files, push, and run Daily or pre-tip to fetch and publish the separate schedule. A site-only build cannot recover schedule entries that were never downloaded. No game count is hardcoded and missing quotes do not produce manufactured picks.

Regression checks cover four scheduled games with only two prop responses, no prop file, missing tip times, started games, explicit zero-game days, future schedule captures, complete export with no picks, and retaining already-published started plays without restoring dropped upcoming picks.
