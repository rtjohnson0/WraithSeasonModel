package accuracy

import (
	"encoding/json"
	"fmt"
	"os"
)

type Report struct {
	Calibration  CalibrationTrial `json:"calibrationExperiment"`
	At           string           `json:"at"`
	Through      string           `json:"through"`
	Config       string           `json:"config"`
	ModelVersion int              `json:"modelVersion"`
	Accepted     int              `json:"accepted"`
	Excluded     map[string]int   `json:"excluded"`
	Slices       []Slice          `json:"slices"`
	Minutes      MinutesTrial     `json:"minutesExperiment"`
	Note         string           `json:"note"`
}

func Load(path string) (Report, error) {
	var r Report
	b, e := os.ReadFile(path)
	if e != nil {
		return r, e
	}
	e = json.Unmarshal(b, &r)
	return r, e
}
func (r Report) Allows(stat, side, date, config string, version int) (bool, string) {
	if r.Config != config || r.ModelVersion != version || r.Through == "" || r.Through >= date {
		return false, "Forward evidence is missing or belongs to a different model configuration"
	}
	for _, s := range r.Slices {
		if s.Stat == stat && s.Side == side {
			if s.Supported {
				return true, ""
			}
			return false, fmt.Sprintf("Forward validation: %d outcomes across %d dates; this stat and side have not passed the evidence criteria", s.N, s.Days)
		}
	}
	return false, "No verified forward evidence for this stat and side"
}
