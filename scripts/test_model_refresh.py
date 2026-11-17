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
                with patch('refresh_models.subprocess.run',side_effect=run):
                    with self.assertRaises(subprocess.CalledProcessError):refresh(2026)
                self.assertEqual(json.loads(Path('data/model.json').read_text())['version'],6)
                self.assertEqual(json.loads(Path('data/calibration.json').read_text())['model_version'],6)
            finally:os.chdir(old)
    def test_game_policy_and_cutoff_are_required(self):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'cal.json'
            cal=dict(policy='game-v2',trained_before='2026-06-13',margin=dict(sigma=14),total=dict(sigma=18))
            p.write_text(json.dumps(cal));self.assertTrue(valid_calibration(p,'2026-11-17'))
            cal['trained_before']='2026-11-17';p.write_text(json.dumps(cal));self.assertFalse(valid_calibration(p,'2026-11-17'))
