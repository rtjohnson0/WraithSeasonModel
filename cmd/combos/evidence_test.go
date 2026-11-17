package main

import (
	"propsmodel/internal/lineslog"
	"testing"
)

func TestComboReadinessDistinguishesUncompletedFromOldModelEvidence(t *testing.T) {
	r := lineslog.Market{Date: "2026-11-17", ModelVersion: 6, Config: "same", TrainedBefore: "2026-06-13", AuditVersion: 2, RawOver: .6, MarketOver: .5}
	if got := comboEvidenceExclusion(r, "2026-11-17", 6, "same"); got != "forecast date not completed" {
		t.Fatal(got)
	}
	r.Date = "2026-11-16"
	r.ModelVersion = 4
	if got := comboEvidenceExclusion(r, "2026-11-17", 6, "same"); got != "different model version" {
		t.Fatal(got)
	}
	r.ModelVersion = 6
	if got := comboEvidenceExclusion(r, "2026-11-17", 6, "same"); got != "" {
		t.Fatal(got)
	}
	r.TrainedBefore = r.Date
	if got := comboEvidenceExclusion(r, "2026-11-17", 6, "same"); got == "" {
		t.Fatal("future training accepted")
	}
}
