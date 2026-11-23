package main

import "propsmodel/internal/replay"

func captureSelection(records []replay.Record, all, posted []scored, rules v2Rules) {
	for i := range records {
		r := &records[i]
		var candidate *scored
		for j := range all {
			s := &all[j]
			if s.PlayerID == r.Row.G.PlayerID && s.GameID == r.Row.G.GameID && s.Stat == r.Stat && s.Line == r.Line && s.Vendor == r.Book && s.QuoteAt == r.QuoteAt {
				candidate = s
				break
			}
		}
		if candidate == nil {
			continue
		}
		s := *candidate
		choice := &replay.Selection{Revision: "exact-quote-selection-v1", Status: "excluded", Side: s.Side}
		for _, p := range posted {
			if p.PlayerID != s.PlayerID || p.GameID != s.GameID || p.Stat != s.Stat {
				continue
			}
			choice.Status = "alternate-quote"
			choice.Reason = "another quote or side was posted"
			if p.Line == s.Line && p.Vendor == s.Vendor && p.Side == s.Side && p.QuoteAt == s.QuoteAt {
				choice.Status = "posted"
				choice.Reason = "selected at this quote"
				break
			}
		}
		if choice.Status == "excluded" {
			if !rules.qualifies(s) {
				choice.Reason = "below probability or edge rule"
			} else if _, drop := v2Tier(s, rules); drop != "" {
				choice.Reason = drop
			} else {
				choice.Reason = "selection or correlation cap"
			}
		}
		r.Selection = choice
	}
}
