import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
from refresh_models import refresh
from calibrate_games import valid_calibration

class RefreshTests(unittest.TestCase):
    def test_failed_calibration_restores_both_files(self):
        with tempfile.TemporaryDirectory() as tmp:
            old=os.getcwd();os.chdir(tmp)
            try:
                Path('data').mkdir()
                Path('data/model.json').write_text('{"version":6}')
                Path('data/calibration.json').write_text('{"model_version":6}')
                def run(args,**kwargs):
                    if './cmd/train' in args:Path('data/model.json').write_text('{"version":7}')
                    if './cmd/backtest' in args:raise subprocess.CalledProcessError(1,args)
                with patch('refresh_models.cached_games',return_value=(1,1)), patch('refresh_models.subprocess.run',side_effect=run):
                    with self.assertRaises(subprocess.CalledProcessError):refresh(2026)
                self.assertEqual(json.loads(Path('data/model.json').read_text())['version'],6)
                self.assertEqual(json.loads(Path('data/calibration.json').read_text())['model_version'],6)
            finally:os.chdir(old)
    def test_partial_history_keeps_pair_and_does_not_train(self):
        with tempfile.TemporaryDirectory() as tmp:
            old=os.getcwd();os.chdir(tmp)
            try:
                Path('data').mkdir()
                Path('data/model.json').write_text('{"version":6}')
                Path('data/calibration.json').write_text('{"model_version":6}')
                with patch('refresh_models.cached_games',return_value=(130,1315)), patch('refresh_models.subprocess.run') as run:
                    self.assertFalse(refresh(2026))
                self.assertEqual(run.call_count,1)
                self.assertIn('-maxminutes',run.call_args.args[0])
                self.assertEqual(json.loads(Path('data/model_refresh.json').read_text())['cachedGames'],130)
                self.assertEqual(json.loads(Path('data/model.json').read_text())['version'],6)
            finally:os.chdir(old)

    def test_partial_history_without_pair_stops(self):
        with tempfile.TemporaryDirectory() as tmp:
            old=os.getcwd();os.chdir(tmp)
            try:
                Path('data').mkdir()
                with patch('refresh_models.cached_games',return_value=(130,1315)), patch('refresh_models.subprocess.run'):
                    with self.assertRaises(RuntimeError):refresh(2026)
            finally:os.chdir(old)

    def test_game_policy_and_cutoff_are_required(self):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'cal.json'
            cal=dict(policy='game-v2',trained_before='2026-06-13',margin=dict(sigma=14),total=dict(sigma=18))
            p.write_text(json.dumps(cal));self.assertTrue(valid_calibration(p,'2026-11-17'))
            cal['trained_before']='2026-11-17';p.write_text(json.dumps(cal));self.assertFalse(valid_calibration(p,'2026-11-17'))

class BackfillCoverageTests(unittest.TestCase):
    def test_count_ignores_duplicate_foreign_and_incomplete_rows(self):
        from refresh_models import cached_games
        with tempfile.TemporaryDirectory() as tmp:
            old=os.getcwd();os.chdir(tmp)
            try:
                Path('data').mkdir()
                Path('data/stats_2025.jsonl').write_text('\n'.join(json.dumps({'game':{'id':i,'season':2025}}) for i in [1,1,2]))
                Path('data/opening_props_2025.jsonl').write_text('\n'.join([json.dumps({'_game_id':1,'response':{'data':[]}}),json.dumps({'_game_id':1,'response':{'data':[]}}),json.dumps({'_game_id':99,'response':{'data':[]}}),'{broken']))
                self.assertEqual(cached_games(2025),(1,2))
            finally:os.chdir(old)
