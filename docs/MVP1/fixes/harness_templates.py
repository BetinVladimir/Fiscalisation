"""Fixed acceptance scaffolding; business code and native test logic stay in units."""
MONEY_VECTORS=[
 {'name':'valid_zero','currency':'EUR','amount':'0.00','want':True},
 {'name':'valid_price','currency':'EUR','amount':'10.99','want':True},
 {'name':'valid_max12','currency':'EUR','amount':'123456789012.00','want':True},
 {'name':'reject_currency','currency':'USD','amount':'10.99','want':False},
 {'name':'reject_minus','currency':'EUR','amount':'-1.00','want':False},
 {'name':'reject_plus','currency':'EUR','amount':'+1.00','want':False},
 {'name':'reject_fraction1','currency':'EUR','amount':'1.0','want':False},
 {'name':'reject_comma','currency':'EUR','amount':'1,00','want':False},
 {'name':'reject_fraction3','currency':'EUR','amount':'1.005','want':False},
 {'name':'reject_unicode','currency':'EUR','amount':'１.００','want':False},
 {'name':'reject_max13','currency':'EUR','amount':'1234567890123.00','want':False},
 {'name':'reject_fraction_sign','currency':'EUR','amount':'1.+1','want':False},
 {'name':'reject_space','currency':'EUR','amount':' 1.00','want':False},
]
BASELINE_TEMPLATE=r'''import json,os,subprocess,sys,tempfile,unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
TOOLS=['python','git','go','c++','node','npm','java','docker']
IDS=['fiscal-backend','s3','protocols','smartdevices','pos','minipos','e2e']
class T(unittest.TestCase):
 def test_collector(self):
  for mode in ['good','fail','missing_tool','missing_cwd','timeout']:
   with self.subTest(mode=mode),tempfile.TemporaryDirectory() as directory:
    temp=Path(directory);commands=[]
    for name in IDS:
     marker=temp/(name+'.ran')
     code='import os;from pathlib import Path;assert Path(os.environ["PWD"]).resolve()==Path.cwd();Path('+repr(str(marker))+').write_text('+repr(name)+');print('+repr('ran-'+name)+',flush=True)'
     commands.append({'id':name,'cwd':str(temp),'argv':[sys.executable,'-c',code]})
    if mode=='fail':commands[0]['argv'][-1]+=';raise SystemExit(9)'
    if mode=='missing_tool':commands[0]['argv']=['mvp1_command_that_does_not_exist']
    if mode=='missing_cwd':commands[0]['cwd']=str(temp/'absent')
    if mode=='timeout':commands[0]['argv'][-1]+=';import time;time.sleep(3)'
    fixture=temp/'commands.json';fixture.write_text(json.dumps(commands))
    output=Path(os.environ['MVP1_ARTIFACT_DIR'])/('baseline-'+mode)
    result=subprocess.run(['bash',str(ROOT/'scripts/mvp1-baseline.sh'),'--fixture',str(fixture),'--output-dir',str(output),'--timeout','1'],cwd=ROOT,capture_output=True,text=True,timeout=90)
    report_path=output/'report.json'
    self.assertTrue(report_path.is_file(),'missing report: '+result.stderr[-1000:])
    report=json.loads(report_path.read_text())
    self.assertEqual(report['schema'],1)
    self.assertIn('commit',report);self.assertIsInstance(report['tools'],dict);self.assertTrue(set(TOOLS)<=set(report['tools']))
    expected_commit=subprocess.run(['git','rev-parse','HEAD'],cwd=ROOT,capture_output=True,text=True)
    self.assertEqual(report['commit'],expected_commit.stdout.strip() if expected_commit.returncode==0 else None)
    if report['commit'] is None:self.assertTrue(report['commit_reason'])
    records=report['modules'];self.assertEqual(len(records),len(IDS))
    self.assertEqual({r['id'] for r in records},set(IDS))
    overall='PASS' if mode=='good' else 'FAIL' if mode=='fail' else 'BLOCKED'
    self.assertEqual(report['overall_status'],overall)
    self.assertEqual(result.returncode,0 if mode=='good' else 1 if mode=='fail' else 3)
    for record in records:
     expected=next(c for c in commands if c['id']==record['id'])
     self.assertEqual(record['cwd'],expected['cwd']);self.assertEqual(record['argv'],expected['argv'])
     self.assertIn('rc',record);self.assertIsInstance(record['reason'],str);self.assertGreaterEqual(record['duration_seconds'],0)
     for field in ['stdout','stderr']:
      self.assertIsInstance(record[field],str);self.assertLessEqual(len(record[field]),8192)
     first=record['id']==IDS[0]
     blocked=first and mode in ['missing_tool','missing_cwd','timeout']
     self.assertEqual(record['status'],'BLOCKED' if blocked else 'FAIL' if first and mode=='fail' else 'PASS')
     if blocked:self.assertTrue(record['reason'])
     else:self.assertEqual(record['rc'],9 if first and mode=='fail' else 0)
     marker=temp/(record['id']+'.ran')
     if first and mode in ['missing_tool','missing_cwd']:self.assertFalse(marker.exists())
     else:self.assertTrue(marker.is_file(),'command was not executed: '+record['id'])
'''
