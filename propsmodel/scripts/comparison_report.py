"""Render isolated historical research; never copy into the public site's record."""
import argparse
import html
import json
from pathlib import Path


def render(study, manifest):
    esc = lambda v: html.escape(str(v))
    def table(rows):
        headers = ['Version', 'Outcomes', 'Dates', 'Log loss ↓', 'Brier ↓', 'Stat MAE ↓', 'Minutes MAE ↓', 'Picks', 'Over / Under', 'Pick hit rate', 'Flat ROI']
        body = []
        for r in rows:
            values = [r['name'], r['n'], r['days'], f"{r['logLoss']:.5f}", f"{r['brier']:.5f}",
                      '—' if r['name'] == 'sportsbook' else f"{r['statMAE']:.2f}",
                      '—' if r['name'] == 'sportsbook' else f"{r['minutesMAE']:.2f}", r['picks'],
                      f"{r['overs']} / {r['unders']}", '—' if not r['picks'] else f"{100*r['hitRate']:.1f}%",
                      '—' if not r['picks'] else f"{100*r['roi']:+.1f}%"]
            body.append('<tr>' + ''.join(f'<td>{esc(v)}</td>' for v in values) + '</tr>')
        return '<div class="scroll"><table><tr>' + ''.join(f'<th>{h}</th>' for h in headers) + '</tr>' + ''.join(body) + '</table></div>'
    result = '''<!doctype html><html lang="en"><meta charset="utf-8"><title>Model comparison</title>
<style>body{font:16px system-ui;background:#100d18;color:#eee;margin:30px;line-height:1.6}main{max-width:1400px;margin:auto}.scroll{overflow:auto}table{border-collapse:collapse}th,td{padding:10px;text-align:left;border-bottom:1px solid #4b3b61;white-space:nowrap}th{background:#271d36}p{max-width:1000px}.status{padding:18px;background:#2b2439}</style><main><h1>Model comparison</h1>'''
    result += '<p>' + esc(manifest['note']) + '</p>'
    if manifest.get('initialRunLimitation'):
        result += '<p>' + esc(manifest['initialRunLimitation']) + '</p>'
    result += f"<p>Calibration fit: through {esc(study['fitThrough'])}. Challenger selection: through {esc(study['validationThrough'])}. Final test: {esc(study['testFrom'])}–{esc(study['testThrough'])}.</p>"
    result += '<h2>Validation: choose once</h2>' + table(study['validation'])
    result += f"<p>Selected challenger: <b>{esc(study['selected'])}</b>. That choice is made before evaluating the final test outcomes.</p>"
    result += '<h2>Final test</h2>' + table(study['test'])
    result += f"<p class='status'>Passed provisional historical screen: <b>{esc(study['passedScreen'])}</b>. Log-loss gain versus current method: {study['gainVsReference']:+.5f} (date-clustered z {study['dateClusteredZVsReference']:.2f}); versus sportsbook: {study['gainVsBook']:+.5f} (z {study['dateClusteredZVsBook']:.2f}). Production files are unchanged. A historical pass is not a verified forward result.</p>"
    if study.get('reasons'):
        result += '<ul>' + ''.join('<li>' + esc(r) + '</li>' for r in study['reasons']) + '</ul>'
    result += '<p>Pick rules are fixed at 55% probability and a 1 percentage point edge at the recorded price. Pushes are excluded from binary scores. Minutes MAE is reported per prop question, so a player with several markets appears several times. Stat MAE is most useful within each stat, not across different stat scales.</p>'
    for row in study.get('byStat', []):
        result += f"<h2>{esc(row['stat'])}</h2>" + table(row['scores'])
    result += '<h2>What happens next</h2><p>Inspect projection errors and stat/side behavior. Keep the final test closed to further tuning; redesigns after seeing it need a new test period. Any proposed production change must be implemented in shared prediction/replay, calibrated for the new policy, and then evaluated on newly captured forward decisions. Do not copy research results into the public performance record.</p></main></html>'
    return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--root', default='research')
    args = parser.parse_args()
    runs = sorted(p.parent for p in Path(args.root).glob('*/study.json'))
    if not runs:
        raise SystemExit('No completed comparison found.')
    directory = runs[-1]
    study = json.loads((directory / 'study.json').read_text(encoding='utf-8-sig'))
    manifest = json.loads((directory / 'manifest.json').read_text(encoding='utf-8-sig'))
    path = directory / 'comparison.html'
    path.write_text(render(study, manifest), encoding='utf-8')
    print(f'Comparison report: {path}')
