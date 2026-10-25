package data

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
)

// AdvKey identifies one player's advanced stat line in one game.
type AdvKey struct{ Player, Game int }

// AdvRow is the handful of advanced stats the model uses. A missing value is NaN.
type AdvRow struct {
	Usage   float64 // usage percentage
	Pace    float64
	AstPct  float64 // assist percentage
	RebPct  float64 // rebound percentage
	Touches float64
	Passes  float64
	FTR     float64 // free throw attempt rate
	Pct3PA  float64 // share of field goal attempts that are threes

	// Opportunity counts from player tracking: how many chances a player had, not what he did
	// with them. A player with 30 rebound chances and 8 rebounds is converting below his norm.
	RebCh    float64 // rebound chances, offensive plus defensive
	RebChOff float64 // offensive rebound chances
	SecAst   float64 // secondary (hockey) assists
	FtAst    float64 // free throw assists
}

type rawAdv struct {
	Period  *int     `json:"period"`
	Usage   *float64 `json:"usage_percentage"`
	Pace    *float64 `json:"pace"`
	AstPct  *float64 `json:"assist_percentage"`
	RebPct  *float64 `json:"rebound_percentage"`
	Touches *float64 `json:"touches"`
	Passes  *float64 `json:"passes"`
	FTR     *float64 `json:"free_throw_attempt_rate"`
	Pct3PA  *float64 `json:"pct_fga_3pt"`

	RebChTotal *float64 `json:"rebound_chances_total"`
	RebChOff   *float64 `json:"rebound_chances_off"`
	SecAst     *float64 `json:"secondary_assists"`
	FtAst      *float64 `json:"free_throw_assists"`

	Player struct {
		ID int `json:"id"`
	} `json:"player"`
	Game struct {
		ID int `json:"id"`
	} `json:"game"`
}

func nanOr(p *float64) float64 {
	if p == nil {
		return math.NaN()
	}
	return *p
}

// LoadAdvancedDir reads every advanced_*.jsonl file in dir. Having no advanced files is not an
// error: the model simply runs without the advanced features.
func LoadAdvancedDir(dir string) (map[AdvKey]AdvRow, error) {
	out := map[AdvKey]AdvRow{}
	files, err := filepath.Glob(filepath.Join(dir, "advanced_*.jsonl"))
	if err != nil {
		return nil, err
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		for sc.Scan() {
			var r rawAdv
			if json.Unmarshal(sc.Bytes(), &r) != nil {
				continue
			}
			if r.Player.ID == 0 || r.Game.ID == 0 || (r.Period != nil && *r.Period != 0) {
				continue
			}
			out[AdvKey{r.Player.ID, r.Game.ID}] = AdvRow{
				Usage:   nanOr(r.Usage),
				Pace:    nanOr(r.Pace),
				AstPct:  nanOr(r.AstPct),
				RebPct:  nanOr(r.RebPct),
				Touches: nanOr(r.Touches),
				Passes:  nanOr(r.Passes),
				FTR:     nanOr(r.FTR),
				Pct3PA:  nanOr(r.Pct3PA),

				RebCh:    nanOr(r.RebChTotal),
				RebChOff: nanOr(r.RebChOff),
				SecAst:   nanOr(r.SecAst),
				FtAst:    nanOr(r.FtAst),
			}
		}
		err = sc.Err()
		f.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
