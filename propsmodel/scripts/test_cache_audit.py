import json
import tempfile
import unittest
from pathlib import Path
from audit_results_cache import run
from repair_replay_identity import repair

class CacheAuditTests(unittest.TestCase):
    def test_corrects_stale_win_without_inventing_download_provenance(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d)
            row=dict(min='32:29',pts=25,reb=13,oreb=1,dreb=12,ast=4,fg3m=4,
                     player=dict(id=7,first_name='Naz',last_name='Reid'),team=dict(id=16,abbreviation='MIN'),
                     game=dict(id=9,date='2026-11-09',status='Final',home_team_id=16,visitor_team_id=2))
            (root/'stats_2026.jsonl.before-result-refresh-20261110T120000Z').write_text(json.dumps(row)+'\n',encoding='utf-8')
            pick=dict(date='2026-11-09',game_id=9,player_id=7,player='Naz Reid',team='MIN',stat='rebounds',side='UNDER',line=9.5,odds=-110,units=1,result='WIN',actual=1)
            log=root/'settled.jsonl';original=json.dumps(pick)+'\n';log.write_text(original,encoding='utf-8')
            evidence,summary=run(root)
            self.assertEqual(log.read_text(),original)
            self.assertEqual(evidence[0]['newResult'],'LOSS')
            self.assertEqual(summary['missingActiveStats'],['stats_2026.jsonl'])
            run(root,True)
            fixed=json.loads(log.read_text());self.assertEqual((fixed['result'],fixed['actual']),('LOSS',13))
            self.assertEqual(fixed['source_fetched_at'],'')
            self.assertTrue(list(root.glob('settled.jsonl.before-cache-audit-*')))

    def test_replay_repair_requires_exact_saved_game_tip_agreement(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d)
            for name in ('replay','lines','snapshots'):(root/name).mkdir()
            decision=dict(date='2026-11-10',at='2026-11-10T17:00:00Z',tip='2026-11-11T00:00:00Z',stat='points',row=dict(identity=dict(PlayerID=7,GameID=0),x=[1,2],s=[]),p1=.57)
            file=root/'replay/decisions_2026-11-10.jsonl';file.write_text(json.dumps(decision)+'\n')
            (root/'lines/market_2026-11-10.jsonl').write_text(json.dumps(dict(pid=7,game=9,stat='points',player='P',team='MIN'))+'\n')
            snapshot=root/'snapshots/props_2026-11-10.jsonl'
            snapshot.write_text(json.dumps(dict(game_id=9,game=dict(datetime='2026-11-11T01:00:00Z')))+'\n')
            self.assertEqual(repair(root,True)[0]['repaired'],0)
            snapshot.write_text(json.dumps(dict(game_id=9,game=dict(datetime='2026-11-11T00:00:00Z')))+'\n')
            self.assertEqual(repair(root,True)[0]['repaired'],1)
            r=json.loads(file.read_text());self.assertEqual(r['row']['identity']['GameID'],9);self.assertEqual(r['row']['x'],[1,2]);self.assertEqual(r['p1'],.57)
