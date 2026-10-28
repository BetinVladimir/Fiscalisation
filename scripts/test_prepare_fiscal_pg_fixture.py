import importlib.util
from pathlib import Path
import tempfile
import unittest
spec=importlib.util.spec_from_file_location('fixture',Path(__file__).with_name('prepare-fiscal-pg-fixture.py'))
fixture=importlib.util.module_from_spec(spec);spec.loader.exec_module(fixture)
class FixtureTests(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.base=Path(self.temp.name);self.source=self.base/'sql';self.source.mkdir()
  (self.source/'004_runtime_login.sh').write_bytes(b'#!/bin/sh\npsql -c \"alter role tenant login\"\n')
  (self.source/'004_runtime_login.sh').chmod(0o755)
  (self.source/'017_old.sql').write_text('CREATE FUNCTION example() RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_temp AS $$ BEGIN END $$;')
  (self.source/'022_new.sql').write_text('SET LOCAL search_path=fiscal,pg_temp; LOCK TABLE fiscal.fiscal_runtime_audit;')
 def test_minipos_namespace_and_sources_unchanged(self):
  (self.source/'022_new.sql').unlink()
  before={p.name:p.read_bytes() for p in self.source.iterdir()};out=self.base/'minipos'
  manifest=fixture.prepare(self.source,out,'minipos')
  self.assertIn('SET search_path=minipos,public',(out/'017_old.sql').read_text())
  self.assertIn('TO beeminipos_tenant',(out/'999_fixture_namespace.sql').read_text())
  self.assertEqual(before,{p.name:p.read_bytes() for p in self.source.iterdir()});self.assertFalse(manifest['production_migration'])
 def test_fiscal_adapter_keeps_new_migration_and_pins_old_function(self):
  out=self.base/'fiscal';fixture.prepare(self.source,out,'fiscal')
  self.assertIn('SET search_path = fiscal, pg_temp',(out/'017_old.sql').read_text())
  self.assertEqual((out/'022_new.sql').read_bytes(),(self.source/'022_new.sql').read_bytes())
  self.assertIn('GRANT USAGE',(out/'999_fixture_namespace.sql').read_text())
 def test_runtime_login_is_copied_byte_exact_with_manifest_and_mode(self):
  out=self.base/'fiscal';manifest=fixture.prepare(self.source,out,'fiscal')
  self.assertEqual((out/'004_runtime_login.sh').read_bytes(),(self.source/'004_runtime_login.sh').read_bytes())
  self.assertEqual((out/'004_runtime_login.sh').stat().st_mode&0o777,0o755)
  self.assertEqual(manifest['runtime_scripts']['004_runtime_login.sh']['source_sha256'],manifest['runtime_scripts']['004_runtime_login.sh']['fixture_sha256'])
 def test_missing_or_unknown_login_script_fails_before_output(self):
  for mode in ('missing','unknown'):
   if mode=='missing':(self.source/'004_runtime_login.sh').unlink()
   else:
    (self.source/'004_runtime_login.sh').write_text('#!/bin/sh\n')
    (self.source/'005_unknown.sh').write_text('#!/bin/sh\n')
   out=self.base/('output-'+mode)
   with self.assertRaises(ValueError):fixture.prepare(self.source,out,'fiscal')
   self.assertFalse(out.exists())
 def test_unreviewed_migration_fails_before_output(self):
  (self.source/'023_future.sql').write_text('SELECT 1;');out=self.base/'output'
  with self.assertRaises(ValueError):fixture.prepare(self.source,out,'fiscal')
  self.assertFalse(out.exists())
 def test_existing_output_preserved(self):
  out=self.base/'output';out.mkdir();(out/'keep').write_text('preserve')
  with self.assertRaises(ValueError):fixture.prepare(self.source,out,'fiscal')
  self.assertEqual((out/'keep').read_text(),'preserve')
if __name__=='__main__':unittest.main()
