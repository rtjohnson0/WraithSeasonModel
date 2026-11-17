import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
from run_tail_shadow import run


class ShadowIsolation(unittest.TestCase):
    def test_missing_pair_does_not_run_or_change_primary(self):
        with tempfile.TemporaryDirectory() as tmp, patch('run_tail_shadow.subprocess.run') as command:
            run(tmp)
            command.assert_not_called()

    def test_separate_inputs_models_and_zero_stake_commands(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for name in ('model_tail.json', 'calibration_tail.json', 'model.json', 'stats_2026.jsonl', 'picklog.jsonl'):
                (root/name).write_text(name)
            (root/'snapshots').mkdir()
            (root/'snapshots'/'props_today.jsonl').write_text('quote')
            with patch('run_tail_shadow.subprocess.run') as command:
                run(tmp)
                self.assertEqual(command.call_count, 4)
                for call in [command.call_args_list[1], command.call_args_list[3]]:
                    args = call.args[0]
                    self.assertEqual(args[args.index('-maxunits')+1], '0')
                    self.assertNotEqual(args[args.index('-data')+1], str(root))
            for name in ('tail_shadow', 'value_shadow'):
                target=root/name
                self.assertEqual((target/'model.json').read_text(), 'model_tail.json')
                self.assertFalse((target/'picklog.jsonl').exists())
                (target/'stats_2026.jsonl').write_text('changed')
            self.assertEqual((root/'model.json').read_text(), 'model.json')
            self.assertEqual((root/'picklog.jsonl').read_text(), 'picklog.jsonl')
            self.assertEqual((root/'stats_2026.jsonl').read_text(), 'stats_2026.jsonl')
