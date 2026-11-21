package opportunity

import (
    "math"
    "propsmodel/internal/data"
)

// ThreeAttemptFactor is the frozen rotation-v5 experiment for 3PM projections.
// It uses *strictly earlier* observed 3PA/minute and 3PM/3PA from Signals;
// no future score, betting result or current-day attempts enter the estimate.
// A 45-attempt prior damps small samples, and a +/-10% hard cap prevents
// unstable volume data from reversing a pick by itself.
func ThreeAttemptFactor(baseMadePerMinute float64, s Signal) float64 {
    if s.Stat != data.Threes || !s.Available || s.Games < 5 ||
       s.Chances < 15 || s.Minutes < 60 ||
       s.Through == "" || !finite(baseMadePerMinute) || baseMadePerMinute <= 0 ||
       !finite(s.Conversion) || s.Conversion < 0 || s.Conversion > 1 ||
       !finite(s.Rate) || s.Rate < 0 {
        return 1
    }
    // Signal.Rate = recent 3PA/minute times long-run made/attempt conversion.
    // The same player/model cannot be fitted to the evaluated day's makes.
    w := .45 * s.Chances / (s.Chances + 45)
    ratio := s.Rate / baseMadePerMinute
    if !finite(ratio) {
        return 1
    }
    factor := 1 + w*(ratio-1)
    return math.Max(.90, math.Min(1.10, factor))
}
