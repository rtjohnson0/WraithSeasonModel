package comboaudit

import (
	"fmt"
	"strings"
)

func (r Report) Markdown() string {
	var b strings.Builder
	b.WriteString("# Frozen V2 forecast review\n\n" + r.Note + "\n\n")
	b.WriteString("Counts are prop questions; player-game counts expose shared exposure. Stat errors use different units; compare within each stat. Missing tracking cannot establish whether usage or conversion caused a miss. Shooting opportunities are FGA + 0.44 FTA, not official usage percentage.\n\n")
	if len(r.Workloads) > 0 {
		b.WriteString("Minutes comparison uses the earliest retained decision per player-game, with a lexical stat tie break. This covers posted players only, not the whole league.\n\n| Workload slice | Player-games | Primary minutes MAE | V2 minutes MAE |\n|---|---:|---:|---:|\n")
		for _, s := range r.Workloads {
			fmt.Fprintf(&b, "| %s | %d | %.2f | %.2f |\n", s.Name, s.PlayerGames, s.PrimaryMAE, s.V2MAE)
		}
		b.WriteString("\n")
	}
	b.WriteString("| Slice | Questions | Player-games | W–L | Minutes error | Covered opportunities | Volume component | Conversion component |\n|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, s := range r.Slices {
		v, c := "Unavailable", "Unavailable"
		if s.OpportunityN > 0 {
			v, c = fmt.Sprintf("%+.2f", s.OpportunityVolumeComponent), fmt.Sprintf("%+.2f", s.OpportunityConversionComponent)
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d–%d | %+.2f | %d | %s | %s |\n", s.Name, s.N, s.PlayerGames, s.Wins, s.Losses, s.MinutesBias, s.OpportunityN, v, c)
	}
	b.WriteString("\n| Date | Player | Play | Result / actual | Minutes forecast → actual | Opportunities forecast → actual | Volume / conversion / context components |\n|---|---|---|---|---|---|---|\n")
	for _, row := range r.Rows {
		volume, components := "Unavailable", "Unavailable"
		if o := row.Opportunity; o != nil && o.Available {
			volume = fmt.Sprintf("%.1f → %.1f", o.ExpectedVolume, *o.ActualVolume)
			components = fmt.Sprintf("%+.2f / %+.2f / %+.2f", o.VolumeComponent, o.ConversionComponent, o.ContextComponent)
		}
		fmt.Fprintf(&b, "| %s | %s | %s %.1f %s | %s / %.1f | %.1f → %.1f | %s | %s |\n", row.Date, strings.ReplaceAll(row.Player, "|", "/"), row.Side, row.Line, row.Stat, row.Result, row.Actual, row.ProjectedMinutes, row.ActualMinutes, volume, components)
	}
	return b.String()
}
