package props

import (
	"propsmodel/internal/data"
	"testing"
)

func quote(book string, line float64) Line {
	return Line{GameID: 9, PlayerID: 7, Stat: data.Rebounds, Vendor: book, Line: line, OverOdds: -110, UnderOdds: -110}
}
func TestConflictingSourcesCannotAdvertiseWrongBookLine(t *testing.T) {
	lines := []Line{quote("draftkings", 8.5), quote("sgo/draftkings", 6.5), quote("fanduel", 6.5), quote("caesars", 6.5), quote("betmgm", 6.5)}
	got, n := ReconcileBooks(lines)
	if n != 1 || len(got) != 3 {
		t.Fatalf("conflicting book was not withheld: %+v count %d", got, n)
	}
	for _, l := range got {
		if BookID(l.Vendor) == "draftkings" {
			t.Fatal("disputed DK quote advertised")
		}
	}
	got, n = FilterConsensus(got, 3, 0.5)
	if len(got) != 3 || n != 0 {
		t.Fatalf("corroborated 6.5 quotes rejected: %+v", got)
	}
}
func TestLargeMarketOutlierIsWithheldInsteadOfBecomingHugeEdge(t *testing.T) {
	got, n := FilterConsensus([]Line{quote("dk", 8.5), quote("fd", 6.5), quote("caesars", 6.5), quote("betmgm", 6.5)}, 3, 0.5)
	if n != 1 || len(got) != 3 {
		t.Fatalf("outlier not withheld: %+v rejected %d", got, n)
	}
	for _, l := range got {
		if l.Line != 6.5 {
			t.Fatalf("wrong current line %+v", l)
		}
	}
}
func TestThinRemainingMarketCannotCountRejectedBooks(t *testing.T) {
	got, _ := FilterConsensus([]Line{quote("dk", 8.5), quote("fd", 6.5), quote("caesars", 6.5)}, 3, 0.5)
	if len(got) != 0 {
		t.Fatalf("counted rejected outlier as corroboration: %+v", got)
	}
}
func TestEachBookKeepsItsOwnLineAndPrice(t *testing.T) {
	a := quote("draftkings", 6.5)
	a.UnderOdds = -125
	b := quote("fanduel", 7)
	b.UnderOdds = 105
	c := quote("caesars", 6.5)
	views := MarketViews([]Line{a, b, c})
	view := views[MarketKey{9, 7, "rebounds"}]
	if view.Median != 6.5 || view.Books != 3 {
		t.Fatalf("bad consensus %+v", view)
	}
	for _, q := range view.Quotes {
		if q.Book == "fanduel" && (q.Line != 7 || q.UnderOdds != 105) {
			t.Fatalf("book price moved to another line: %+v", q)
		}
	}
}
