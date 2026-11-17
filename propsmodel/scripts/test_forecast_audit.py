import json
import tempfile
import unittest
from pathlib import Path
from audit_forecasts import run

class ForecastAuditTests(unittest.TestCase):
    def test_exact_finals_post_tip_exclusion_and_correct_under_loss(self):
        with tempfile.TemporaryDirectory() as tmp:
            d=Path(tmp)
            actual=dict(min='32',pts=20,reb=13,oreb=1,dreb=12,ast=4,fg3m=2,
                player=dict(id=7,first_name='A',last_name='B'),team=dict(id=9,abbreviation='AAA'),
                game=dict(id=1,date='2026-11-01',status='Final',home_team_id=9,visitor_team_id=8,datetime='2026-11-01T23:00:00Z'))
            projection=dict(date='2026-11-01',at='2026-11-01T22:00:00Z',game_id=1,player_id=7,player='A B',team='AAA',proj_min=30,typ_min=34,stats={'rebounds':dict(proj=6,line=9.5)})
            pick=dict(date='2026-11-01',logged_at='2026-11-01T22:00:00Z',game_id=1,player_id=7,player='A B',team='AAA',stat='rebounds',side='UNDER',line=9.5,odds=-110,prob=.6)
            for name,rows in [('stats_2026.jsonl',[actual]),('projlog.jsonl',[projection,dict(projection,at='2026-11-02T00:00:00Z')]),('picklog.jsonl',[pick,dict(pick,logged_at='2026-11-02T00:00:00Z')])]:
                (d/name).write_text(''.join(json.dumps(r)+'\n' for r in rows))
            report=run(d,d/'report')
            self.assertEqual(report['minutes_summary']['n'],1)
            self.assertEqual(report['minutes_summary']['actual_minus_projected'],2)
            self.assertEqual(report['projection_by_stat']['rebounds']['actual_minus_projected'],7)
            self.assertEqual(report['picks_by_provenance']['legacy_quotes']['hit_rate'],0)
            self.assertEqual(report['picks_by_provenance']['legacy_quotes']['flat_roi'],-1)
            self.assertEqual(report['picks_by_provenance']['documented_quotes']['n'],0)
            self.assertEqual(report['exclusions']['pick_not_pregame_or_tip_missing'],1)
            self.assertIn('wrong market period',(d/'report/report.html').read_text())
