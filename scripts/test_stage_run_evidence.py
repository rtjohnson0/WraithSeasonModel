import json
import tempfile
import unittest
from pathlib import Path
from stage_run_evidence import stage

class EvidenceTests(unittest.TestCase):
    def test_only_current_stats_and_coverage_are_retained(self):
        with tempfile.TemporaryDirectory() as temp:
            data=Path(temp)/'data';out=Path(temp)/'out';data.mkdir()
            (data/'stats_2026.jsonl').write_text(json.dumps({'game':{'date':'2026-11-09','status':'Final'}})+'\n')
            (data/'stats_2025.jsonl').write_text('old')
            (data/'.env').write_text('secret')
            info=stage(data,out,2026)
            self.assertEqual(info['finalThrough'],'2026-11-09')
            self.assertEqual({p.name for p in out.iterdir()},{'stats_2026.jsonl','coverage.json'})
            self.assertTrue((data/'stats_2026.jsonl').exists())
