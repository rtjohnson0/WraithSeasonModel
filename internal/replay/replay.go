// Package replay preserves the exact model, feature vector, policy and quote used
// at prediction time. Historical box scores must never reconstruct injury knowledge.
package replay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"propsmodel/internal/data"
	"propsmodel/internal/eval"
	"propsmodel/internal/feat"
	"propsmodel/internal/predictor"
)

type Row struct {
	G          data.PlayerGame         `json:"identity"`
	X          []*float64              `json:"x"`
	S          [data.NStats][]*float64 `json:"s"`
	AvgMin15   float64                 `json:"avg_min15"`
	MinTypical *float64                `json:"min_typical"`
	Games      int                     `json:"games"`
}

func number(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
func values(xs []float64) []*float64 {
	out := make([]*float64, len(xs))
	for i, x := range xs {
		out[i] = number(x)
	}
	return out
}
func floats(xs []*float64) []float64 {
	out := make([]float64, len(xs))
	for i, x := range xs {
		out[i] = math.NaN()
		if x != nil {
			out[i] = *x
		}
	}
	return out
}
func Capture(r *feat.Row) Row {
	out := Row{G: r.G, X: values(r.X), AvgMin15: r.AvgMin15, MinTypical: number(r.MinTypical), Games: r.Games}
	for k := range out.S {
		out.S[k] = values(r.S[k])
	}
	return out
}
func (r Row) Features() *feat.Row {
	out := &feat.Row{G: r.G, X: floats(r.X), AvgMin15: r.AvgMin15, MinTypical: math.NaN(), Games: r.Games}
	if r.MinTypical != nil {
		out.MinTypical = *r.MinTypical
	}
	for k := range out.S {
		out.S[k] = floats(r.S[k])
	}
	return out
}

type Record struct {
	Period            string                `json:"period,omitempty"`
	BookCount         int                   `json:"book_count,omitempty"`
	CalibrationBefore string                `json:"calibration_before,omitempty"`
	Date              string                `json:"date"`
	At                string                `json:"at"`
	Tip               string                `json:"tip"`
	ModelID           string                `json:"model_id"`
	Row               Row                   `json:"row"`
	Policy            predictor.Adjustments `json:"policy"`
	Cal               eval.Cal              `json:"cal"`
	Stat              data.Stat             `json:"stat"`
	Line              float64               `json:"line"`
	Book              string                `json:"book"`
	QuoteAt           string                `json:"quote_at"`
	UpdatedAt         string                `json:"updated_at"`
	OverOdds          int                   `json:"over_odds"`
	UnderOdds         int                   `json:"under_odds"`
	Market            float64               `json:"market"`
	P1                float64               `json:"p1"`
	P2                float64               `json:"p2,omitempty"`
}

func Checkpoint(dir string, m *predictor.Model) (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	id := hex.EncodeToString(sum[:])
	path := filepath.Join(dir, "replay", "model_"+id+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if old, err := os.ReadFile(path); err == nil {
		if string(old) != string(b) {
			return "", fmt.Errorf("checkpoint mismatch")
		}
		return id, nil
	}
	return id, os.WriteFile(path, b, 0644)
}
func Append(dir, date string, records []Record) error {
	path := filepath.Join(dir, "replay", "decisions_"+date+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, r := range records {
		b, err := json.Marshal(r)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}
func Recompute(m *predictor.Model, r Record) (float64, error) {
	if len(r.Row.X) != len(feat.CommonSpecs) {
		return 0, fmt.Errorf("feature schema changed; replay with the source version that produced this checkpoint")
	}
	for k := range r.Row.S {
		if len(r.Row.S[k]) != len(feat.StatSpecs[k]) {
			return 0, fmt.Errorf("stat feature schema mismatch")
		}
	}
	pred := m.Forecast(r.Row.Features(), r.Policy)
	_, p, err := predictor.Price(pred, r.Stat, r.Line, r.Market, func(_ data.Stat, raw, market float64) float64 { return r.Cal.Apply(raw, market) })
	return p, err
}
