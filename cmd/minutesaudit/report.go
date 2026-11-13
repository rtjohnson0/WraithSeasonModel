package main

import (
	"fmt"
	"html/template"
	"os"
	"propsmodel/internal/minutesaudit"
	"sort"
)

func writeHTML(path string, r Report) error {
	const doc = `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Minutes and Under-bias audit</title><style>body{font:15px system-ui;background:#100c18;color:#eee;max-width:1200px;margin:30px auto;padding:0 18px;line-height:1.6}h1,h2{color:#c6a5ff}table{border-collapse:collapse;width:100%;font-variant-numeric:tabular-nums}td,th{padding:9px;text-align:right;border-bottom:1px solid #463158}td:first-child,th:first-child{text-align:left}.scroll{overflow:auto;margin:20px 0}.note{padding:16px;background:#241931;border-left:3px solid #b187ee}small{color:#baaeca}a{color:#ceacff}</style><h1>Minutes and Under-bias audit</h1><p>{{.Since}} through dates before {{.Before}} · actuals through {{.Through}}</p><p class="note">{{.Note}}</p><p><b>{{.VerifiedQuestions}}</b> verified replay questions · <b>{{.LegacyQuestions}}</b> separate legacy diagnostic questions.</p><p>Negative minutes bias means minutes were underestimated. Positive stat bias means production was underestimated. Subgroups overlap; do not add their counts.</p><h2>1. Minutes accuracy</h2><div class="scroll"><table><tr><th>Cohort / group</th><th>Player-games / dates</th><th>Projected</th><th>Actual</th><th>Bias</th><th>MAE</th><th>P90 error</th><th>Under by 5+</th><th>Typical / model MAE on matched subset (N)</th></tr>{{range .Minutes}}<tr><td>{{.Cohort}} · {{.Group}}</td><td>{{.N}} / {{.Days}}</td><td>{{f .Predicted}}</td><td>{{f .Actual}}</td><td>{{f .Bias}}</td><td>{{f .MAE}}</td><td>{{f .P90AE}}</td><td>{{.UnderFive}}</td><td>{{f .TypicalMAE}} / {{f .TypicalModelMAE}} ({{.TypicalN}})</td></tr>{{end}}</table></div><p>Typical-minute MAE covers only rows with a saved pregame typical workload. It is a baseline comparison, not an automatically installed replacement. Missing injury context stays unknown; historical absences are not reconstructed from who eventually played.</p><h2>2. Where low stat projections come from</h2><p>Actual − projection = (actual minutes − projected minutes) × projected rate + (actual − actual minutes × projected rate). This exact decomposition is retrospective, not proof that either term caused a loss.</p><div class="scroll"><table><tr><th>Cohort / stat</th><th>N / dates</th><th>Projection</th><th>Actual</th><th>Actual − projection</th><th>Minutes term</th><th>Rate term</th><th>MAE</th><th>Rate-scaling change (N)</th><th>Projection below line</th></tr>{{range .Stats}}<tr><td>{{.Cohort}} · {{.Stat}}</td><td>{{.N}} / {{.Days}}</td><td>{{f .MeanProjection}}</td><td>{{f .MeanActual}}</td><td>{{f .Bias}}</td><td>{{f .MinutesContribution}}</td><td>{{f .RateContribution}}</td><td>{{f .MAE}}</td><td>{{f .MeanScaleChange}} ({{.ScaleN}})</td><td>{{.ProjectionBelowLine}}</td></tr>{{end}}</table></div><p>Compare MAE within each stat. Points and threes have different scales. Rate-scaling change is final projection minus its saved before-scale projection at the same projected minutes.</p><h2>3. Does calibration create more Unders?</h2><div class="scroll"><table><tr><th>Cohort / stat</th><th>Probability rows</th><th>Raw Under</th><th>Raw Under despite mean ≥ line</th><th>Final Under</th><th>Over → Under</th><th>Under → Over</th><th>Intercept flips to Under</th><th>Typical-minutes side flips</th></tr>{{range .Stats}}<tr><td>{{.Cohort}} · {{.Stat}}</td><td>{{.ProbabilityN}}</td><td>{{.RawUnder}}</td><td>{{.RawUnderAboveMean}}</td><td>{{.FinalUnder}}</td><td>{{.CalibrationToUnder}}</td><td>{{.CalibrationToOver}}</td><td>{{.InterceptToUnder}}</td><td>{{.TypicalMinutesFlip}}</td></tr>{{end}}</table></div><p>Zero probability rows means unavailable, not zero bias. Legacy projection means cannot reconstruct historical probabilities. Neutral-intercept and typical-minutes scenarios are sensitivity checks only.</p>{{with .CurrentSlate}}<h2>Latest ungraded slate: {{.Date}}</h2><p class="note">Saved side-audit summary at {{.At}}. These are descriptive counts, not verified outcomes or independently replayed probabilities. Each scored market uses its nearest median quote; posted plays can use different quotes or be labelled leans.</p><div class="scroll"><table><tr><th>Stat</th><th>Markets</th><th>Raw Over / Under</th><th>Final Over / Under</th><th>Overs meeting probability / price bars</th><th>Unders meeting probability / price bars</th><th>Posted Over / Under</th><th>Mean projection − line</th><th>Mean minutes − typical</th></tr>{{range .Rows}}<tr><td>{{.Stat}}</td><td>{{.Markets}}</td><td>{{.RawOver}} / {{.RawUnder}}</td><td>{{.FinalOver}} / {{.FinalUnder}}</td><td>{{.OverProbBar}} / {{.OverEdgeBar}}</td><td>{{.UnderProbBar}} / {{.UnderEdgeBar}}</td><td>{{.PostedOver}} / {{.PostedUnder}}</td><td>{{f .AvgProjectionGap}}</td><td>{{f .AvgMinutesShift}}</td></tr>{{end}}</table></div>{{end}}<h2>Ungraded replay: probability-stage diagnosis</h2><p class="note">These frozen forecast rows are not joined to actuals. Rows lacking game IDs or provenance remain forensic diagnostics, not verified outcomes or evidence that a change would improve accuracy. A positive-mean skewed distribution can still have more than half its mass below a line.</p><div class="scroll"><table><tr><th>Stat</th><th>N</th><th>Missing game ID / provenance</th><th>Raw Under</th><th>Final Under</th><th>Zero-intercept Under</th><th>Typical-minutes Under (N)</th><th>Raw Under despite mean ≥ line</th><th>Over / Under at fixed thresholds</th></tr>{{range .Stages}}<tr><td>{{.Stat}}</td><td>{{.N}}</td><td>{{.MissingGameID}} / {{.IncompleteProvenance}}</td><td>{{.RawUnder}}</td><td>{{.FinalUnder}}</td><td>{{.ZeroInterceptUnder}}</td><td>{{.TypicalUnder}} ({{.TypicalN}})</td><td>{{.RawUnderAboveMean}}</td><td>{{.OverThreshold}} / {{.UnderThreshold}}</td></tr>{{end}}</table></div><h2>Largest minutes underestimates</h2><div class="scroll"><table><tr><th>Player / date / cohort</th><th>Projected</th><th>Actual</th><th>Difference</th></tr>{{range worst .Observations}}<tr><td>{{.Player}} · {{.Date}} · {{.Cohort}}</td><td>{{f .Minutes}}</td><td>{{f .ActualMinutes}}</td><td>{{delta .}}</td></tr>{{end}}</table></div><h2>Excluded inputs</h2><ul>{{range $key,$value:=.Excluded}}<li>{{$key}}: {{$value}}</li>{{end}}</ul><h2>Next decision</h2><p>Use verified, newly captured decisions to compare a fixed challenger on later dates. Do not tune on the same final test again, promote a correction from legacy diagnostics, impose a 50/50 Over/Under quota, or copy this research into the public performance record.</p></html>`
	funcs := template.FuncMap{"f": func(v float64) string { return fmtFloat(v) }, "delta": func(o minutesaudit.Observation) string { return fmtFloat(o.ActualMinutes - o.Minutes) }, "worst": func(obs []minutesaudit.Observation) []minutesaudit.Observation {
		best := map[string]minutesaudit.Observation{}
		for _, o := range obs {
			k := o.Cohort + "/" + o.Date + "/" + fmtID(o.GameID, o.PlayerID)
			if p, ok := best[k]; !ok || o.At < p.At {
				best[k] = o
			}
		}
		rows := []minutesaudit.Observation{}
		for _, o := range best {
			rows = append(rows, o)
		}
		sort.Slice(rows, func(i, j int) bool {
			a, b := rows[i], rows[j]
			d1, d2 := a.ActualMinutes-a.Minutes, b.ActualMinutes-b.Minutes
			if d1 != d2 {
				return d1 > d2
			}
			return a.Player < b.Player
		})
		if len(rows) > 25 {
			rows = rows[:25]
		}
		return rows
	}}
	t, e := template.New("audit").Funcs(funcs).Parse(doc)
	if e != nil {
		return e
	}
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return t.Execute(f, r)
}

func fmtFloat(v float64) string { return fmt.Sprintf("%.2f", v) }
func fmtID(g, p int) string     { return fmt.Sprintf("%d/%d", g, p) }
