package main

import (
	"testing"

	"propsmodel/internal/espn"
)

func TestMergeESPN(t *testing.T) {
	byName := map[string]int{"aaron wiggins": 1, "some player": 2, "known out": 3, "has status": 4}
	out := map[int]string{3: "Out"}
	statuses := map[int]string{4: "Questionable"}
	inj := []espn.Injury{
		{Name: "Aaron Wiggins", Team: "ATL", Status: "Day-To-Day"},
		{Name: "Some Player", Team: "ATL", Status: "Out"},
		{Name: "Known Out", Team: "BOS", Status: "Out"},
		{Name: "Has Status", Team: "BOS", Status: "Day-To-Day"},
		{Name: "Nobody Known", Team: "BOS", Status: "Out"},
	}
	o, m := mergeESPN(inj, byName, out, statuses)
	if len(o) != 1 || o[0] != "Some Player (ATL)" || len(m) != 1 || m[0] != "Aaron Wiggins (ATL)" {
		t.Fatalf("merge wrong: %v %v", o, m)
	}
	if _, ok := out[2]; !ok || statuses[4] != "Questionable" || statuses[1] == "" {
		t.Errorf("maps wrong: %v %v", out, statuses)
	}
}
