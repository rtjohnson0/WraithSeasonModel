package sgo

import (
	"bufio"
	"encoding/json"
	"os"
)

// SnapshotRow is one captured line as stored in data/snapshots/sgo_DATE.jsonl.
type SnapshotRow struct {
	CapturedAt string `json:"captured_at"`
	PlayerLine
}

// LoadSnapshot reads an sgo snapshot file. Where it was captured several times, the latest
// capture of each (player, stat, book) wins.
func LoadSnapshot(path string) ([]PlayerLine, error) { return loadSnapshot(path, false) }

// LoadSnapshotCombos is LoadSnapshot for the combined stats only (PRA, PR, PA, RA).
func LoadSnapshotCombos(path string) ([]PlayerLine, error) { return loadSnapshot(path, true) }

func loadSnapshot(path string, combos bool) ([]PlayerLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	type key struct {
		player, stat, book string
	}
	latest := map[key]PlayerLine{}
	var order []key
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var r SnapshotRow
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		if r.Stat.IsCombo() != combos {
			continue
		}
		k := key{r.PlayerKey, string(r.Stat), r.Book}
		if _, ok := latest[k]; !ok {
			order = append(order, k)
		}
		latest[k] = r.PlayerLine
	}
	out := make([]PlayerLine, 0, len(order))
	for _, k := range order {
		out = append(out, latest[k])
	}
	return out, sc.Err()
}
