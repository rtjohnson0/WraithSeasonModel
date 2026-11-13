import json
import unittest
from sync_state import records, merge_jsonl

class GameMarketMergeTests(unittest.TestCase):
    def test_markets_do_not_collapse_and_first_decision_is_frozen(self):
        rows=[dict(date='2026-11-12',game_id=7,market=m,at='2026-11-12T23:00:00Z') for m in ('moneyline','spread','total')]
        self.assertEqual(len(records('data/gamepicklog.jsonl',''.join(json.dumps(r)+'\n' for r in rows))),3)
        old=dict(rows[1],line=-5.5)
        newer=dict(rows[1],at='2026-11-12T23:30:00Z',line=-1.5)
        got=merge_jsonl('data/gamepicklog.jsonl','',json.dumps(newer)+'\n',json.dumps(old)+'\n')
        self.assertEqual(json.loads(got)['line'],-5.5)

    def test_latest_final_score_correction_wins(self):
        a=dict(date='2026-11-12',game_id=7,market='total',result='WIN',sourceAt='2026-11-13T03:00:00Z')
        b=dict(a,result='LOSS',sourceAt='2026-11-13T04:00:00Z')
        got=merge_jsonl('data/gamesettled.jsonl','',json.dumps(b)+'\n',json.dumps(a)+'\n')
        self.assertEqual(json.loads(got)['result'],'LOSS')
