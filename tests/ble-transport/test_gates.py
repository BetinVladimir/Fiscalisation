"""Negative gates exercise the exact orchestrator/checker helpers, no SDK stubs."""
from pathlib import Path
import copy,hashlib,json,shutil,sys,tempfile,unittest
HERE=Path(__file__).resolve().parent;sys.path.insert(0,str(HERE))
from check_lock import check,sha
from run import verify_sources,validate_fixtures,validate_reports,EXPECTED_CASES
REPO=HERE.parents[1]
class Gates(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory(prefix='ble-transport-gate-');self.addCleanup(self.temp.cleanup);self.root=Path(self.temp.name)
 def test_actual_lock_origins_and_closed_dependency_graph(self):self.assertEqual(check(REPO,HERE)['registry_entries'],109)
 def altered_package(self):
  target=self.root/'package';shutil.copytree(HERE,target,ignore=shutil.ignore_patterns('__pycache__'));return target
 def test_registry_integrity_tamper_rejected(self):
  target=self.altered_package();p=target/'package-lock.json';obj=json.loads(p.read_text());obj['packages']['node_modules/cbor-x']['integrity']='sha512-tampered';p.write_text(json.dumps(obj))
  with self.assertRaisesRegex(ValueError,'entry mismatch'):check(REPO,target)
 def test_missing_required_crypto_dependency_rejected(self):
  target=self.altered_package();p=target/'package.json';obj=json.loads(p.read_text());del obj['devDependencies']['@noble/ciphers'];p.write_text(json.dumps(obj))
  with self.assertRaisesRegex(ValueError,'manifest changed'):check(REPO,target)
 def test_unlocked_file_dependency_rejected(self):
  target=self.altered_package();p=target/'package-lock.json';obj=json.loads(p.read_text());obj['packages']['node_modules/cbor-x']['resolved']='file:../../../BeeloyBackend';p.write_text(json.dumps(obj))
  with self.assertRaises(ValueError):check(REPO,target)
 def test_optional_ui_manifest_dependency_rejected(self):
  target=self.altered_package();p=target/'package.json';obj=json.loads(p.read_text());obj['optionalDependencies']={'@beeloy/design-tokens':'file:external'};p.write_text(json.dumps(obj))
  with self.assertRaisesRegex(ValueError,'manifest fields'):check(REPO,target)
 def test_missing_platform_lock_entry_rejected(self):
  target=self.altered_package();p=target/'package-lock.json';obj=json.loads(p.read_text());del obj['packages']['node_modules/@esbuild/linux-x64'];p.write_text(json.dumps(obj))
  with self.assertRaisesRegex(ValueError,'entry set mismatch'):check(REPO,target)
 def test_changed_actual_app_lock_rejected(self):
  repo=self.root/'repo'
  for app in ['BeeMiniPOS','miniposweb']:
   d=repo/'minipos'/app;d.mkdir(parents=True)
   for name in ['package.json','package-lock.json']:shutil.copyfile(REPO/'minipos'/app/name,d/name)
  (repo/'minipos/BeeMiniPOS/package-lock.json').write_text('{}')
  with self.assertRaisesRegex(ValueError,'source hash mismatch'):check(repo,HERE)
 def sources(self):
  source=self.root/'source';copied=self.root/'copy';source.mkdir();copied.mkdir();(source/'actual.ts').write_text('actual source');shutil.copyfile(source/'actual.ts',copied/'actual.ts');return source,copied,{'actual.ts':sha(source/'actual.ts')}
 def test_copy_and_original_must_both_match(self):
  source,copied,hashes=self.sources();verify_sources(source,copied,hashes);(copied/'actual.ts').write_text('changed')
  with self.assertRaisesRegex(ValueError,'hash changed'):verify_sources(source,copied,hashes)
 def test_missing_runtime_source_rejected(self):
  source,copied,hashes=self.sources();(copied/'actual.ts').unlink()
  with self.assertRaises(ValueError):verify_sources(source,copied,hashes)
 def test_original_changed_after_copy_rejected(self):
  source,copied,hashes=self.sources();(source/'actual.ts').write_text('new source')
  with self.assertRaises(ValueError):verify_sources(source,copied,hashes)
 def test_symlink_source_rejected(self):
  source,copied,hashes=self.sources();(copied/'actual.ts').unlink();(copied/'actual.ts').symlink_to(source/'actual.ts')
  with self.assertRaises(ValueError):verify_sources(source,copied,hashes)
 def fixture(self):
  identifier='10000000-0000-4000-8000-000000000001';payload=b'\xf6';frame=b'BFF1'+bytes.fromhex(identifier.replace('-',''))+len(payload).to_bytes(2,'big')+b'\0\0\1'+hashlib.sha256(payload).digest()+payload
  rows=[{'id':identifier,'state':s,'amount':a,'hex':payload.hex(),'frames':[frame.hex()]} for s,a in EXPECTED_CASES]
  p=self.root/'fixture.jsonl';p.write_text(''.join(json.dumps(row)+'\n' for row in rows));return p,rows
 def test_missing_and_duplicate_fixture_case_rejected(self):
  p,rows=self.fixture();self.assertEqual(validate_fixtures(p)['rows'],12);p.write_text(''.join(json.dumps(row)+'\n' for row in rows[:-1]))
  with self.assertRaisesRegex(ValueError,'cases'):validate_fixtures(p)
  rows[-1]=rows[0];p.write_text(''.join(json.dumps(row)+'\n' for row in rows))
  with self.assertRaisesRegex(ValueError,'cases'):validate_fixtures(p)
 def test_corrupt_real_frame_digest_rejected(self):
  p,rows=self.fixture();frame=bytearray.fromhex(rows[0]['frames'][0]);frame[25]^=1;rows[0]['frames']=[frame.hex()];p.write_text(''.join(json.dumps(row)+'\n' for row in rows))
  with self.assertRaisesRegex(ValueError,'digest'):validate_fixtures(p)
 def reports(self):
  tap=self.root/'mini.tap';tap.write_text('# pass 1\n# skipped 0\n# fail 0\n');report=self.root/'vitest.json';obj={'success':True,'numFailedTests':0,'numPendingTests':0,'numPassedTests':66,'testResults':[{'assertionResults':[{'fullName':f'{kind}: actual WebBluetooth transport actual S3 delivery int64 bytes survive real result projection','status':'passed'} for kind in ['mini','pwa']]}]};report.write_text(json.dumps(obj));return tap,report,obj
 def test_skipped_fixture_waiter_cannot_be_green(self):
  tap,report,obj=self.reports();validate_reports(tap,report);tap.write_text('# pass 0\n# skipped 1\n# fail 0\n')
  with self.assertRaisesRegex(ValueError,'waiter'):validate_reports(tap,report)
 def test_missing_named_browser_projection_cannot_be_green(self):
  tap,report,obj=self.reports();obj['testResults'][0]['assertionResults'][1]['status']='pending';report.write_text(json.dumps(obj))
  with self.assertRaisesRegex(ValueError,'projection'):validate_reports(tap,report)
 def test_reduced_browser_scope_rejected(self):
  tap,report,obj=self.reports();obj['numPassedTests']=2;report.write_text(json.dumps(obj))
  with self.assertRaisesRegex(ValueError,'scope'):validate_reports(tap,report)
if __name__=='__main__':unittest.main()
