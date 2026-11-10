package main

import (
	"propsmodel/internal/gameboard"
	"propsmodel/internal/team"
)

func makeGameBoard(id int, home, away, tip string, margin, total, totalSigma, marginSigma float64, rows []team.Odds) gameboard.Game {
	g := gameboard.Game{ID: id, Home: home, Away: away, Tip: tip, HomeScore: (total + margin) / 2, AwayScore: (total - margin) / 2, Margin: margin, Total: total, TotalLow: total - 1.28155*totalSigma, TotalHigh: total + 1.28155*totalSigma, Quotes: []gameboard.Quote{}}
	counts := map[string]int{}
	for _, o := range rows {
		if o.HasTotal {
			counts["total"]++
		}
		if o.MLHome != 0 && o.MLAway != 0 {
			counts["moneyline"]++
		}
	}
	for _, o := range rows {
		base := gameboard.Quote{Book: o.Vendor, QuoteAt: o.CapturedAt, SourceUpdatedAt: o.Stamp, Tip: tip}
		if o.HasTotal && counts["total"] >= 3 {
			q := base
			q.Market = "total"
			q.Line = o.Total
			q.BookCount = counts["total"]
			po, pu, pp := gameboard.TotalProb(total, totalSigma, o.Total)
			q.Sides = gameboard.Pair([2]string{"Over", "Under"}, [2]int{o.Over, o.Under}, [2]float64{po, pu}, pp)
			g.Quotes = append(g.Quotes, q)
		}
		if o.MLHome != 0 && o.MLAway != 0 && counts["moneyline"] >= 3 {
			q := base
			q.Market = "moneyline"
			q.BookCount = counts["moneyline"]
			ph := team.NormCDF(margin / marginSigma)
			q.Sides = gameboard.Pair([2]string{home, away}, [2]int{o.MLHome, o.MLAway}, [2]float64{ph, 1 - ph}, 0)
			g.Quotes = append(g.Quotes, q)
		}
	}
	return g
}
