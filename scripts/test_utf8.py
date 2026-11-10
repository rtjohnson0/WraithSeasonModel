import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
from audit_quotes import read
from audit_periods import read_rows
import sync_state


class UTF8RegressionTests(unittest.TestCase):
    def test_bom_and_non_ascii_player_names_survive_both_readers(self):
        with tempfile.TemporaryDirectory() as tmp:
            path=Path(tmp)/'picklog.jsonl'
            row={'player':'Luka Don\u010di\u0107','stat':'points','side':'OVER','line':20.5}
            path.write_bytes(b'\xef\xbb\xbf'+(json.dumps(row,ensure_ascii=False)+'\n').encode('utf-8'))
            self.assertEqual(read(path),[row])
            self.assertEqual([r for _,r in read_rows(path)],[row])

    def test_audit_runs_with_utf8_mode_off_and_c_locale(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            row=dict(player='Luka Don\u010di\u0107',date='2026-11-10',player_id=1,
                     stat='points',side='OVER',book='dk',line=20.5)
            path=root/'picklog.jsonl'
            original=(json.dumps(row,ensure_ascii=False)+'\n').encode('utf-8')
            path.write_bytes(original)
            with self.assertRaises(UnicodeDecodeError):
                original.decode('cp1252')
            env=os.environ.copy()
            env.update(PYTHONUTF8='0',PYTHONCOERCECLOCALE='0',LC_ALL='C')
            result=subprocess.run([sys.executable,str(Path(__file__).with_name('audit_quotes.py')),
                                   '--data',str(root),'--apply'],env=env,capture_output=True)
            self.assertEqual(result.returncode,0,result.stderr.decode('utf-8',errors='replace'))
            self.assertEqual(path.read_bytes(),original)
            report=json.loads(next(root.glob('quote_audit_*.json')).read_text(encoding='utf-8'))
            self.assertEqual(report[0]['entry']['player'],row['player'])

    def test_git_output_has_explicit_utf8_decoding(self):
        completed=subprocess.CompletedProcess(['git'],0,stdout='Luka Don\u010di\u0107',stderr='')
        with patch.object(sync_state.subprocess,'run',return_value=completed) as run:
            self.assertEqual(sync_state.git('show','HEAD:data/picklog.jsonl').stdout,completed.stdout)
            self.assertEqual(run.call_args.kwargs['encoding'],'utf-8')


if __name__=='__main__':unittest.main()
