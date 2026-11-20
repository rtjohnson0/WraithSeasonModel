import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
from sync_state import merge_jsonl, sync, is_input, merge_result_audit, is_state, MODEL_BUNDLE


def run(cwd, *args):
    return subprocess.run(['git', *args], cwd=cwd, check=True, text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout.strip()


def line(pid, **kwargs):
    return json.dumps(dict(date='2026-11-10', game_id=9, player_id=pid,
                           stat='points', side='OVER', book='dk', line=20.5, **kwargs))+'\n'


class MergeTests(unittest.TestCase):
    def test_new_model_records_and_calibration_are_retained(self):
        for name in ('v2_gamepicklog.jsonl','v2_gamesettled.jsonl','v2_rotation_strictlog.jsonl','v2_rotation_strictsettled.jsonl','gamecal_research.json'):
            self.assertTrue(is_state('data/'+name),name)
        self.assertIn('data/gamecal_research.json',MODEL_BUNDLE)

    def test_concurrent_result_audits_keep_both_corrections(self):
        older = dict(at='2026-11-10T17:00:00Z', checked=1, history=[dict(id='a', newResult='LOSS')])
        newer = dict(at='2026-11-10T18:00:00Z', checked=2, history=[dict(id='b', newResult='REVIEW')])
        result = json.loads(merge_result_audit(json.dumps(older), json.dumps(newer)))
        self.assertEqual(result['checked'], 2)
        self.assertEqual({r['id'] for r in result['history']}, {'a','b'})

    def test_input_identity_and_latest_local_correction(self):
        name = 'data/stats_2026.jsonl'
        a = dict(player={'id': 7}, game={'id': 1}, pts=10)
        fixed = dict(a, pts=12)
        b = dict(player={'id': 8}, game={'id': 1}, pts=20)
        text = lambda rows: ''.join(json.dumps(r)+'\n' for r in rows)
        result = [json.loads(x) for x in merge_jsonl(name, text([a]),
                  text([a, fixed, b]), text([a])).splitlines()]
        self.assertEqual({r['player']['id']: r['pts'] for r in result}, {7:12,8:20})
        # A partial advanced-stat row cannot replace the full-game row.
        result = merge_jsonl('data/advanced_2026.jsonl', '',
                 text([dict(a, period=0), dict(a, period=1)]), '')
        self.assertEqual(len(result.splitlines()), 2)
        self.assertFalse(is_input('data/.env'))
        self.assertFalse(is_input('scripts/private.jsonl'))
        with self.assertRaises(ValueError):
            merge_jsonl(name, '', '{"pts":1}\n', '')


    def test_input_correction_prefers_newer_download_across_branches(self):
        original = dict(player={'id':7}, game={'id':9}, reb=1)
        fresh = dict(original, reb=13, _stats_fetched_at='2026-11-10T18:00:00Z')
        stale = dict(original, reb=2, _stats_fetched_at='2026-11-10T17:00:00Z')
        text = lambda r: json.dumps(r)+'\n'
        result = json.loads(merge_jsonl('data/stats_2026.jsonl',text(original),text(fresh),text(stale)))
        self.assertEqual(result['reb'],13)
        result = json.loads(merge_jsonl('data/stats_2026.jsonl','',text(fresh)+text(stale),''))
        self.assertEqual(result['reb'],13)

    def test_newer_final_actual_survives_concurrent_older_result(self):
        base = line(7, result='WIN', actual=1)
        fresh = line(7, result='LOSS', actual=13, source_fetched_at='2026-11-10T18:00:00Z')
        stale = line(7, result='WIN', actual=1, source_fetched_at='2026-11-10T17:00:00Z')
        result = json.loads(merge_jsonl('data/settled.jsonl', base, fresh, stale))
        self.assertEqual((result['result'], result['actual']), ('LOSS',13))
        review = line(7, result='REVIEW', actual=0, source_fetched_at='2026-11-10T19:00:00Z')
        result = json.loads(merge_jsonl('data/settled.jsonl', base, fresh, review))
        self.assertEqual(result['result'],'REVIEW')

    def test_game_challenger_keeps_first_question_and_latest_matching_final(self):
        first=dict(date='2026-11-19',game_id=9,market='spread',at='2026-11-19T18:00:00Z',side='BOS',line=-3.5,book='a')
        later=dict(first,at='2026-11-19T19:00:00Z',side='NYK',line=-4.5,book='b')
        text=lambda r:json.dumps(r)+'\n'
        result=merge_jsonl('data/gamepace_picklog.jsonl','',text(later),text(first))
        self.assertEqual(json.loads(result)['at'],first['at'])
        old=dict(first,result='WIN',sourceAt='2026-11-20T06:00:00Z')
        new=dict(first,result='LOSS',sourceAt='2026-11-20T07:00:00Z')
        result=merge_jsonl('data/gamepace_settled.jsonl','',text(new),text(old))
        self.assertEqual(json.loads(result)['result'],'LOSS')
        wrong=dict(later,result='WIN',sourceAt='2026-11-20T08:00:00Z')
        result=merge_jsonl('data/gamepace_settled.jsonl','',text(wrong),text(new))
        self.assertEqual(json.loads(result)['at'],first['at'])

    def test_union_and_remote_correction(self):
        base = line(1, result='LOSS')
        ours = base + line(2)
        theirs = line(1, result='WIN') + line(3)
        rows = [json.loads(x) for x in merge_jsonl('data/settled.jsonl',base,ours,theirs).splitlines()]
        self.assertEqual(len(rows),3)
        self.assertEqual(rows[0]['result'],'WIN')

    def test_deletion_is_not_resurrected(self):
        base=line(1)
        self.assertEqual(merge_jsonl('data/picklog.jsonl',base,base,''),'')
        self.assertEqual(merge_jsonl('data/picklog.jsonl',base,'',base),'')
        self.assertEqual(merge_jsonl('data/picklog.jsonl',base,base.replace('20.5','21.5'),''),
                         line(1).replace('20.5','21.5').replace(' ',''))

    def test_bad_json_stops_resolution(self):
        with self.assertRaises(json.JSONDecodeError):
            merge_jsonl('data/picklog.jsonl','','<<<<<<< conflict\n','')

    def test_real_remote_updates_and_rejected_push(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); remote=root/'remote.git';bot=root/'bot';user=root/'user'
            run(root,'init','--bare',str(remote))
            run(root,'clone',str(remote),str(bot))
            run(bot,'checkout','-b','main')
            for repo in [bot]:
                run(repo,'config','user.email','test@example.invalid');run(repo,'config','user.name','Test')
            (bot/'data').mkdir();(bot/'README.md').write_text('base\n')
            (bot/'data/picklog.jsonl').write_text(line(1))
            (bot/'data/model_tail.json').write_text('{"version":5,"bundle":"base"}\n')
            (bot/'data/calibration_tail.json').write_text('{"config":"tail-base"}\n')
            (bot/'data/model.json').write_text('{"version":3}\n')
            (bot/'data/calibration.json').write_text('{"config":"old"}\n')
            stat = lambda pid, pts: json.dumps(dict(player={'id':pid}, game={'id':9}, pts=pts))+'\n'
            (bot/'data/stats_2026.jsonl').write_text(stat(1,10))
            (bot/'data/board.json').write_text(json.dumps(dict(at='2026-11-10T17:00:00Z', plays=[])))
            (bot/'data/more.json').write_text(json.dumps(dict(at='2026-11-10T17:00:00Z')))
            run(bot,'add','.');run(bot,'commit','-m','base');run(bot,'push','-u','origin','main')
            run(root,'clone','-b','main',str(remote),str(user))
            run(user,'config','user.email','test@example.invalid');run(user,'config','user.name','Test')
            (user/'data/picklog.jsonl').write_text(line(1)+line(3))
            (user/'README.md').write_text('new source\n')
            (user/'data/more.json').write_text(json.dumps(dict(at='2026-11-10T17:30:00Z')))
            (user/'data/stats_2026.jsonl').write_text(stat(1,12)+stat(3,30))
            (user/'data/model_tail.json').write_text('{"version":5,"bundle":"new"}\n')
            (user/'data/model.json').write_text('{"version":4}\n')
            (user/'data/calibration.json').write_text('{"config":"new"}\n')
            run(user,'add','.');run(user,'commit','-m','upstream');run(user,'push')
            (bot/'data/picklog.jsonl').write_text(line(1)+line(2))
            (bot/'data/board.json').write_text(json.dumps(dict(at='2026-11-10T18:00:00Z', plays=[dict(player='Saved pick')])))
            (bot/'data/more.json').write_text(json.dumps(dict(at='2026-11-10T18:00:00Z', combos=[dict(player='Saved combo')])))
            (bot/'data/calibration.json').write_text('{"config":"concurrent-old-model"}\n')
            (bot/'data/calibration_tail.json').write_text('{"config":"concurrent-tail"}\n')
            (bot/'data/stats_2026.jsonl').write_text(stat(1,11)+stat(2,20))
            (bot/'data/stats_2027.jsonl').write_text(stat(4,40))
            # Reject once, as a protected branch race/transient push failure would.
            hook=remote/'hooks/pre-receive'
            hook.write_text('#!/bin/sh\nif [ ! -f rejected-once ]; then touch rejected-once; exit 1; fi\nexit 0\n')
            hook.chmod(0o755)
            previous=Path.cwd()
            try:
                os.chdir(bot)
                sync('main','generated',attempts=3,delay=0)
            finally:
                os.chdir(previous)
            self.assertEqual((bot/'README.md').read_text(),'new source\n')
            self.assertEqual(json.loads((bot/'data/board.json').read_text())['at'], '2026-11-10T18:00:00Z')
            self.assertEqual(json.loads((bot/'data/more.json').read_text())['combos'][0]['player'], 'Saved combo')
            records=[json.loads(x) for x in (bot/'data/picklog.jsonl').read_text().splitlines()]
            self.assertEqual({r['player_id'] for r in records},{1,2,3})
            self.assertEqual(json.loads((bot/'data/calibration.json').read_text())['config'],'new')
            self.assertEqual(json.loads((bot/'data/model_tail.json').read_text())['bundle'],'new')
            self.assertEqual(json.loads((bot/'data/calibration_tail.json').read_text())['config'],'tail-base')
            stats=[json.loads(x) for x in (bot/'data/stats_2026.jsonl').read_text().splitlines()]
            self.assertEqual({r['player']['id']:r['pts'] for r in stats},{1:12,2:20,3:30})
            self.assertEqual(run(bot,'diff','--name-only'),'')
            self.assertEqual(run(bot,'ls-files','--','data/stats_2027.jsonl'),'')
            self.assertEqual(run(bot,'rev-parse','HEAD'),run(bot,'rev-parse','origin/main'))
            self.assertFalse(run(bot,'diff','--name-only','--diff-filter=U'))


if __name__=='__main__':unittest.main()
