"""Render an offline report. Never copy this into site/public."""
import argparse
import html
import json
from pathlib import Path


def render(report):
    escape = lambda x: html.escape(str(x))
    rows = []
    for s in report.get('slices', []):
        if s['side'] == 'ALL':
            continue
        values = [s['stat'], s['side'], s['n'], s['days'], f"{100*s['hitRate']:.1f}%",
                  f"{100*s['meanProbability']:.1f}%", f"{100*s['breakEven']:.1f}%",
                  f"{s['brier']:.4f}", f"{s['marketBrier']:.4f}",
                  f"{s['gain']:+.5f}", f"{s['dateClusteredZ']:.2f}",
                  f"{s['mae']:.2f}", f"{s['actualMinusProjected']:+.2f}",
                  'Passed provisional criteria' if s['supported'] else 'Research only']
        rows.append('<tr>' + ''.join(f'<td>{escape(v)}</td>' for v in values) + '</tr>')
    m = report.get('minutesExperiment', {})
    headers = ['Stat', 'Side', 'Outcomes', 'Dates', 'Hit rate', 'Predicted', 'Break-even',
               'Brier', 'Book Brier', 'Log-loss gain vs book', 'Date-clustered z', 'Stat MAE',
               'Actual minus projected', 'Evidence']
    return '''<!doctype html><html lang="en"><meta charset="utf-8"><title>Forward accuracy evaluation</title>
<style>body{font:16px system-ui;margin:35px;background:#101018;color:#eee}table{border-collapse:collapse}td,th{padding:12px;text-align:left;border-bottom:1px solid #555}th{background:#272339}p{max-width:1000px;line-height:1.6}.scroll{overflow:auto}</style>
<h1>Forward accuracy evaluation</h1>''' + f'''<p>{report.get('accepted', 0)} verified pregame questions. Through {escape(report.get('through') or 'no eligible completed dates')}. This report stays offline.</p>
<p>{escape(report.get('note', ''))}</p><div class="scroll"><table><thead><tr>''' + ''.join(f'<th>{h}</th>' for h in headers) + '</tr></thead><tbody>' + ''.join(rows) + '</tbody></table></div>' + f'''
<h2>Minutes challenger</h2><p>A recent-role blend is selected using earlier dates, then evaluated on later dates without retuning. Best training weight: {m.get('weight', 0):.0%}. Later-date MAE: existing {m.get('baseMAE', 0):.2f} minutes; challenger {m.get('trialMAE', 0):.2f}. Test: {m.get('testN', 0)} player-games across {m.get('testDays', 0)} dates. Passed provisional criteria: {escape(m.get('supported', False))}. No production adjustment is applied.</p>
<h2>Calibration challengers</h2><p>Each stat is fitted on earlier raw predictions, then compared with the frozen existing forecasts and sportsbook probabilities on later dates. Results are experiments; production weights are unchanged.</p><pre>{escape(json.dumps(report.get('calibrationExperiment', {}), indent=2))}</pre>
<h2>Excluded evidence</h2><p>Missing provenance remains excluded. Neither cached legacy results nor repeated book quotes establish forward accuracy.</p><pre>{escape(json.dumps(report.get('excluded', {}), indent=2))}</pre></html>'''


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--data', default='data')
    args = parser.parse_args()
    directory = Path(args.data)
    report = json.loads((directory / 'accuracy.json').read_text(encoding='utf-8-sig'))
    path = directory / 'accuracy_report.html'
    path.write_text(render(report), encoding='utf-8')
    print(f'Offline report: {path}')
