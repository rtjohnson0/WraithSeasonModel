package predictor

import (
	"propsmodel/internal/data"
	"propsmodel/internal/feat"
	"propsmodel/internal/glm"
	"strings"
	"testing"
)

func TestExplainUsesFittedDefenseInputsWithoutChangingModel(t *testing.T) {
	var row feat.Row
	row.X = make([]float64, len(feat.CommonSpecs))
	row.S[0] = make([]float64, len(feat.StatSpecs[0]))
	_, i := feat.FeatureIndex("opp_allow", data.Points)
	_, j := feat.FeatureIndex("pos5_allow", data.Points)
	row.S[0][i] = .1
	row.S[0][j] = .2
	m := &Model{}
	m.Stat[0] = &glm.Model{Names: []string{"opp_allow", "pos5_allow"}, Mean: []float64{0, 0}, Std: []float64{1, 1}, Coef: []float64{.5, .25}}
	note := m.ExplainContext(&row, data.Points)
	if !strings.Contains(note, "+10.5%") || row.S[0][i] != .1 || row.S[0][j] != .2 {
		t.Fatalf("bad sensitivity %s", note)
	}
	m.Stat[0] = &glm.Model{}
	if note = m.ExplainContext(&row, data.Points); !strings.Contains(note, "disabled") {
		t.Fatal("missing feature not identified")
	}
}
