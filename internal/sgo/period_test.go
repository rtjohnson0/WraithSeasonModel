package sgo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"testing"
)

func TestSnapshotPartialCannotOverwriteFullGame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sgo.jsonl")
	var body []byte
	for _, l := range []PlayerLine{
		{PlayerKey: "P", Stat: data.Points, Book: "dk", Line: 24.5, UnderLine: 24.5, PairVerified: true, OverOdds: -110, UnderOdds: -110},
		{PlayerKey: "P", Stat: data.Points, Book: "dk", Line: 6.5, UnderLine: 6.5, PairVerified: true, OverOdds: -110, UnderOdds: -110, Period: "1q"},
	} {
		b, err := json.Marshal(SnapshotRow{PlayerLine: l})
		if err != nil {
			t.Fatal(err)
		}
		body = append(body, append(b, '\n')...)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := LoadSnapshot(path)
	if err != nil || len(rows) != 1 || rows[0].Line != 24.5 {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
}
