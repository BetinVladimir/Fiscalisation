import copy,importlib.util,json,os,subprocess,sys,tempfile,unittest,contextlib,io
from pathlib import Path
from unittest.mock import patch
HERE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('runner',HERE/'run_fixes.py');r=importlib.util.module_from_spec(spec);spec.loader.exec_module(r)

def plan():
    return {'schema':2,'context_tokens':65536,'packet_estimate_limit':12000,'policy':'money invariants','rules':'read 3 files','tasks':[{'id':'A-01','priority':'P0','mode':'code','depends':[],'spec':'assert product counter equals one','units':[{'id':'A-01.1','title':'implement counter test','files':['tests/test_counter.py'],'test':'counter=1','accept':'python3 -m unittest discover -s tests -p test_counter.py','sources':[],'phase':'green'}]}]}

class RunnerTests(unittest.TestCase):
    def test_large_prepared_fixtures_use_hashes_without_losing_requirements(self):
        p=plan();u=p['tasks'][0]['units'][0];u['harness_template']='x'*30000;u['scaffolds']={'tests/native.py':'y'*30000};u['title']='Exact business oracle must remain'
        message=r.prompt(p,p['tasks'][0],u,Path('/tmp/isolated/Fiscalisation'))
        self.assertIn(r.digest('x'*30000),message);self.assertIn(r.digest('y'*30000),message)
        self.assertIn('Exact business oracle must remain',message);self.assertIn(u['accept'],message)
        self.assertLessEqual(len(message.encode())//2+1,12000)
    def test_inventory_detects_symlink_retarget_and_ancestor_replacement(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);subprocess.run(['git','init','-q',d],check=True)
            (root/'a').mkdir();(root/'b').mkdir()
            (root/'a/x').write_text('one');(root/'b/x').write_text('two')
            link=root/'link';link.symlink_to('a');before=r.inventory(root)
            link.unlink();link.symlink_to('b');self.assertNotEqual(before,r.inventory(root))
            directory=root/'nested';directory.mkdir();(directory/'x').write_text('one')
            subprocess.run(['git','add','nested/x'],cwd=root,check=True)
            before=r.inventory(root);(directory/'x').unlink();directory.rmdir();directory.symlink_to(root/'b',target_is_directory=True)
            self.assertNotEqual(before,r.inventory(root))
    def test_inventory_rejects_change_during_hash(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);subprocess.run(['git','init','-q',d],check=True);file=root/'x';file.write_text('one')
            original=Path.read_bytes
            def changing(path):
                content=original(path)
                if path==file:path.write_text('two')
                return content
            with patch.object(Path,'read_bytes',changing):
                with self.assertRaisesRegex(ValueError,'changed during inventory'):r.inventory(root)
    def test_compiled_scope_matches_reference_and_rechecks_symlinks(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d)/'Fiscalisation';root.mkdir();(root/'allowed').mkdir();(root/'outside').mkdir()
            (root/'allowed/x').write_text('one');(root/'outside/x').write_text('two')
            alias=root/'alias';alias.symlink_to(root/'allowed',target_is_directory=True)
            leaf=root/'allowed/link';leaf.symlink_to(root/'outside/x')
            scopes=['allowed'];match=r.scope_matcher(root,scopes)
            for path in ['allowed/x','allowed/link','outside/x','alias/x','allowed/missing']:
                self.assertEqual(r.in_scope(root,root,path,scopes),match(root,path),path)
            alias.unlink();alias.symlink_to(root/'outside',target_is_directory=True)
            self.assertFalse(match(root,'alias/x'))
            leaf.unlink();leaf.symlink_to(root/'allowed/x');self.assertTrue(match(root,'allowed/link'))
            leaf.unlink();leaf.symlink_to(root/'outside/x');self.assertFalse(match(root,'allowed/link'))
            self.assertFalse(r.scope_matcher(root,[])(root,'allowed/x'))
    def test_persistence_schema_packet_uses_postgres_and_preserves_ddl_ownership(self):
        p=json.loads((HERE/'plan.json').read_text())
        t=next(t for t in p['tasks'] if t['id']=='PERSIST-01');u=t['units'][0]
        self.assertTrue(t['spec_override'])
        self.assertIn('MUST NOT apply local SQL',t['spec'])
        self.assertIn('TestMVP1SchemaFresh',u['harness_template'])
        self.assertIn('required_tests=',u['harness_template'])
        self.assertIn('immutable native test',u['title'])
        self.assertIn('MVP1_SCHEMA_ADMIN_URL',u['harness_template'])
        self.assertIn('no test edits',u['title'])
        self.assertIn('Do not write or change tests',u['title'])
        self.assertNotIn('fiscal-backend/internal/persistence/migrations.go',u['files'])
    def test_admin_idempotency_packet_scopes_required_jest_setup_and_lock(self):
        p=json.loads((HERE/'plan.json').read_text())
        task=next(t for t in p['tasks'] if t['id']=='AA-01')
        for unit in task['units']:
            self.assertIn('AdminApp/package-lock.json',unit['files'])
            self.assertIn('AdminApp/jest.config.js',unit['files'])
            self.assertIn('not a constant',unit['title'])
            self.assertIn('global.crypto undefined',unit['title'])
            self.assertIn('AA-01.2',unit['title'])
        self.assertNotIn('AdminApp/App.tsx',task['units'][0]['files'])
        self.assertTrue(task['spec_override'])
        first=task['units'][0]
        self.assertIn('AdminApp/src/idempotency.test.ts',first['scaffolds'])
        native=first['scaffolds']['AdminApp/src/idempotency.test.ts']
        self.assertIn('jest.fn',native)
        self.assertIn('jest.clearAllMocks()',native)
        self.assertIn('newKey(); expect(sdk.randomUUID).toHaveBeenCalledTimes(1)',native)
        self.assertIn('self.assertEqual((ROOT/',first['harness_template'])
        self.assertNotIn('babel',first['scaffolds']['AdminApp/jest.config.js'])
    def setUp(self):
        redirect=contextlib.redirect_stdout(io.StringIO());redirect.__enter__();self.addCleanup(redirect.__exit__,None,None,None)
    def test_tap_plan_without_executions_is_not_evidence(self):
        import runtime_proof as proof
        for output in ('1..3\n','ok 1 - one\n1..3\n','ok 1 - a\nok 1 - duplicate\n1..2\n'):
            self.assertEqual(0,proof.test_count(output))
        self.assertEqual(2,proof.test_count('ok 1 - a\nok 2 - b\n1..2\n'))
    def test_ran_tests_without_failure_is_not_red_evidence(self):
        import runtime_proof as proof
        self.assertEqual(0,proof.failed_test_count('Ran 2 tests in 0.01s\nOK'))
        self.assertEqual(1,proof.failed_test_count('Ran 2 tests in 0.01s\nFAILED (failures=1)'))
    def test_fixture_function_is_not_product_runtime(self):
        for directory in ('testdata','fixtures','src/test'):
            with tempfile.TemporaryDirectory() as d:
                root=Path(d);(root/'tests').mkdir();helper=root/directory/'helper.py';helper.parent.mkdir(parents=True,exist_ok=True)
                helper.write_text('def value(): return 1367\n')
                (root/'tests/test_fixture.py').write_text('import unittest,runpy\nclass T(unittest.TestCase):\n def test_value(self): self.assertEqual(runpy.run_path('+repr(str(helper))+')["value"](),1367)\n')
                ok,out=r.execute_check('python3 -m unittest discover -s tests -p test_fixture.py',root,10,require_runtime=True)
                self.assertFalse(ok,out)
    def test_script_exit_one_without_assertion_is_not_red_evidence(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir()
            (root/'product.py').write_text('raise ValueError("configuration missing")\n')
            (root/'tests/test_red.py').write_text('import sys,subprocess,unittest\nclass T(unittest.TestCase):\n def test_red(self):\n  p=subprocess.run([sys.executable,"product.py"],capture_output=True,text=True)\n  self.assertEqual(p.returncode,0,p.stderr)\n')
            self.assertFalse(r.execute_check('python3 -m unittest discover -s tests -p test_red.py',root,10,phase='red',require_runtime=True)[0])
    def test_pilot_never_issues_full_acceptance(self):
        p,here,repo,workspace,args=self.fixture()
        with patch.object(r,'HERE',here):
            rc=r.main(args+['--only','A-01','--pilot-unit','A-01.1'])
        self.assertEqual(4,rc)
        state=workspace/'.mvp-fixes-state'
        self.assertEqual('PILOT_TESTED_REVIEW_REQUIRED',json.loads((state/'pilot-report.json').read_text())['status'])
        self.assertFalse((state/'review-request.json').exists())
    def test_green_native_suite_cannot_prove_red(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir()
            (root/'product.py').write_text('print("ok 1 - value\\n1..1")\n')
            (root/'tests/test_red.py').write_text('import sys,subprocess,unittest\nclass T(unittest.TestCase):\n def test_red(self):\n  p=subprocess.run([sys.executable,"product.py"],capture_output=True,text=True)\n  self.assertEqual(p.returncode,99)\n')
            self.assertFalse(r.execute_check('python3 -m unittest discover -s tests -p test_red.py',root,10,phase='red',require_runtime=True)[0])
    def test_unrelated_live_product_does_not_own_loopback_evidence(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir()
            (root/'product.py').write_text('import time; time.sleep(30)\n')
            (root/'tests/test_socket.py').write_text('import sys,socket,subprocess,unittest\nclass T(unittest.TestCase):\n def test_socket(self):\n  p=subprocess.Popen([sys.executable,"product.py"],stdout=subprocess.PIPE,stderr=subprocess.PIPE)\n  try:\n   with socket.socket() as listener:\n    listener.bind(("127.0.0.1",0));listener.listen(1)\n    with socket.create_connection(listener.getsockname()) as client:\n     conn,_=listener.accept()\n     with conn: conn.sendall(b"fixture");self.assertEqual(client.recv(7),b"fixture")\n  finally: p.terminate();p.communicate()\n')
            ok,out=r.execute_check('python3 -m unittest discover -s tests -p test_socket.py',root,10,require_runtime=True)
            self.assertFalse(ok,out);self.assertIn('"runtime_io": [{',out)
    def test_inventory_cache_checks_metadata_and_content(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);subprocess.run(['git','init','-q',d],check=True)
            file=root/'product.py';file.write_text('aaaa');before=r.inventory(root)
            original=Path.read_bytes
            with patch.object(Path,'read_bytes',side_effect=AssertionError('unchanged file rehashed')):self.assertEqual(before,r.inventory(root))
            stat=file.stat();file.write_text('bbbb');os.utime(file,ns=(stat.st_atime_ns,stat.st_mtime_ns))
            self.assertNotEqual(before,r.inventory(root))
            file.chmod(0o755);mode=r.inventory(root);self.assertNotEqual(before,mode)
            file.unlink();self.assertNotIn('product.py',r.inventory(root))
    def test_pilot_selection_is_explicit_and_keeps_dependency_disclosure(self):
        p=plan();p['tasks'][0]['depends']=['W0-1']
        selected=r.pilot_task(p,'A-01','A-01.1')
        self.assertEqual(['W0-1'],selected['depends']);self.assertEqual(1,len(selected['units']))
        for task,uid in [(None,'A-01.1'),('A-01','B-01.1'),('missing','A-01.1')]:
            with self.assertRaises(ValueError):r.pilot_task(p,task,uid)
    def test_product_script_bad_output_can_reproduce_red(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir();(root/'product.py').write_text('print(0)\n')
            (root/'tests/test_red.py').write_text('import sys,subprocess,unittest\nclass T(unittest.TestCase):\n def test_red(self):\n  p=subprocess.run([sys.executable,"product.py"],capture_output=True,text=True,check=True)\n  self.assertEqual(p.stdout.strip(),"1367")\n')
            ok,out=r.execute_check('python3 -m unittest discover -s tests -p test_red.py',root,10,phase='red',require_runtime=True)
            self.assertTrue(ok,out)
    def test_adjacent_native_file_is_outside_exact_scope(self):
        root=Path('/tmp/isolated/Fiscalisation')
        files=['IoT/firmware/edge-agent-s3/idf/tests/serial_exchange_native.cpp']
        self.assertTrue(r.in_scope(root,root,files[0],files))
        self.assertFalse(r.in_scope(root,root,'IoT/firmware/edge-agent-s3/idf/tests/serial_exchange_datecs_wrap.cpp',files))
    def test_opencode_flags_follow_run_and_preserve_prompt(self):
        args=r.argparse.Namespace(opencode_cmd='opencode',model='ollama/qwen',opencode_run_arg=['--standalone','--auto'])
        text='Тест\nspaces and $literal'
        self.assertEqual(['opencode','run','--standalone','--auto','--model','ollama/qwen',text],r.model_argv(args,text))
    def test_model_transcript_preserves_history_outside_tail(self):
        with tempfile.TemporaryDirectory() as d:
            file=Path(d)/'model.log'
            rc,tail=r.run_process([sys.executable,'-c','print("BEGIN"+"x"*50000+"END")'],d,10,output_file=file)
            self.assertEqual(0,rc);self.assertNotIn('BEGIN',tail)
            self.assertTrue(file.read_text().startswith('BEGIN'));self.assertTrue(file.read_text().endswith('END\n'))
            self.assertEqual(0o600,file.stat().st_mode & 0o777)
    def test_native_helper_uses_real_go_action_events(self):
        from native_harness import check_native_tests
        output='{"Action":"run","Package":"p","Test":"TestMoney"}\n{"Action":"pass","Package":"p","Test":"TestMoney"}\n'
        result=subprocess.CompletedProcess(['go'],0,output,'')
        with patch('native_harness.subprocess.run',return_value=result) as run:
            check_native_tests(self,['go','test','-json'],root='/tmp',required_tests=['TestMoney'])
            self.assertEqual(['go','test','-json'],run.call_args.args[0])
            with self.assertRaises(RuntimeError):check_native_tests(self,['go','test'],root='/tmp',required_tests=['Missing'])
    def test_native_helper_rejects_empty_skip_and_nonzero(self):
        from native_harness import check_native_tests
        for rc,output in [(0,'{"Action":"pass","Package":"p"}'),(0,'{"Action":"skip","Package":"p","Test":"TestX"}'),(1,'1..1\nok 1 - fake\n')]:
            with patch('native_harness.subprocess.run',return_value=subprocess.CompletedProcess(['go'],rc,output,'')):
                with self.assertRaises(RuntimeError if '"Action":"skip"' in output else AssertionError):check_native_tests(self,['go','test'],root='/tmp')
    def test_native_helper_does_not_hide_missing_tool(self):
        from native_harness import check_native_tests
        with patch('native_harness.subprocess.run',side_effect=FileNotFoundError('go missing')):
            with self.assertRaises(FileNotFoundError):check_native_tests(self,['go','test'],root='/tmp')
    def test_early_red_fix_is_discarded_and_baseline_must_fail(self):
        p,here,repo,workspace,args=self.fixture()
        p['tasks'][0]['units'][0]['files'].append('product_counter.py')
        for directory in (here,repo/'docs/MVP1/fixes'):(directory/'plan.json').write_text(json.dumps(p))
        (repo/'product_counter.py').write_text('def count(): return 0\n')
        model=Path(r.shlex.split(args[args.index('--opencode-cmd')+1])[1])
        # Explicit simple model: proposes a fix in red, repeats only production in green.
        test='import unittest\nfrom product_counter import count\nclass T(unittest.TestCase):\n def test_counter(self): self.assertEqual(count(),1)\n'
        model.write_text('import sys\nfrom pathlib import Path\nif "[MVP_RED_STAGE]" not in sys.argv[-1]: assert "return 0" in Path("product_counter.py").read_text()\nPath("product_counter.py").write_text("def count(): return 1\\n")\np=Path("tests/test_counter.py");p.parent.mkdir(exist_ok=True);p.write_text('+repr(test)+')\n')
        with patch.object(r,'HERE',here):self.assertEqual(4,r.main(args))
        state=workspace/'.mvp-fixes-state'
        receipt=json.loads((state/'A-01.1.json').read_text())
        self.assertTrue(receipt['red_verified'])
        self.assertIn('BASELINE_RED_REPLAY',(state/receipt['red_log']).read_text())
    def test_child_pwd_matches_isolated_cwd_even_with_stale_env(self):
        with tempfile.TemporaryDirectory() as d:
            code='import os,json;print(json.dumps([os.getcwd(),os.environ.get("PWD"),os.environ.get("OLDPWD")]))'
            rc,out=r.run_process([sys.executable,'-c',code],d,10,env={'PWD':'/wrong/source','OLDPWD':'/original'})
            cwd,pwd,old=json.loads(out)
            self.assertEqual(0,rc);self.assertEqual(str(Path(d).resolve()),cwd);self.assertEqual(cwd,pwd);self.assertIsNone(old)
    def test_prepare_fixed_harness_preserves_existing_work(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);unit=plan()['tasks'][0]['units'][0];unit['harness_template']='VALUE=1\n'
            r.prepare_unit(root,unit);file=root/'tests/test_counter.py'
            self.assertEqual(unit['harness_template'],file.read_text())
            file.write_text('user work\n')
            with self.assertRaisesRegex(ValueError,'existing fixed harness differs'):r.prepare_unit(root,unit)
            self.assertEqual('user work\n',file.read_text())
    def test_prepare_scaffold_is_only_for_absent_declared_file(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);unit=plan()['tasks'][0]['units'][0]
            unit['files'].append('scripts/new.sh');unit['scaffolds']={'scripts/new.sh':'#!/bin/sh\nexit 0\n'}
            r.prepare_unit(root,unit);file=root/'scripts/new.sh';self.assertTrue(file.is_file());self.assertEqual(0o755,file.stat().st_mode & 0o777)
            file.write_text('existing implementation\n');r.prepare_unit(root,unit);self.assertEqual('existing implementation\n',file.read_text())
            unit['scaffolds']={'other.sh':'exit 0'}
            with self.assertRaisesRegex(ValueError,'outside scope'):r.prepare_unit(root,unit)
    def test_prepare_detects_typed_golden_corruption(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);unit=plan()['tasks'][0]['units'][0];unit.update(test_vectors_file='docs/MVP1/fixes/golden.json',test_vectors=[{'want':False}])
            file=root/unit['test_vectors_file'];file.parent.mkdir(parents=True);file.write_text('[{"want":false}]')
            r.prepare_unit(root,unit);file.write_text('[{"want":0}]')
            with self.assertRaisesRegex(ValueError,'immutable vectors'):r.prepare_unit(root,unit)
    def test_fixed_harness_change_cannot_pass_red(self):
        source='import unittest\nfrom product_counter import count\nclass T(unittest.TestCase):\n def test_counter(self): self.assertEqual(count(),1)\n'
        p,here,repo,workspace,args=self.fixture(source);p['tasks'][0]['units'][0]['harness_template']=source
        for directory in (here,repo/'docs/MVP1/fixes'):(directory/'plan.json').write_text(json.dumps(p))
        model=Path(r.shlex.split(args[args.index('--opencode-cmd')+1])[1]);model.write_text(model.read_text()+'\nPath("tests/test_counter.py").write_text(Path("tests/test_counter.py").read_text()+"# changed\\n")\n')
        with patch.object(r,'HERE',here),self.assertRaisesRegex(ValueError,'fixed harness changed'):r.main(args)
        self.assertFalse((workspace/'.mvp-fixes-state/A-01.1.json').exists())
    def test_native_helper_case_discovery_error_is_not_red_assertion(self):
        from native_harness import check_native_tests
        output='{"Action":"run","Package":"p","Test":"TestMoney"}\n{"Action":"fail","Package":"p","Test":"TestMoney"}\n'
        with patch('native_harness.subprocess.run',return_value=subprocess.CompletedProcess(['go'],1,output,'')):
            with self.assertRaises(RuntimeError):check_native_tests(self,['go','test'],root='/tmp',required_tests=['TestMoney/reject_plus'])
    def test_model_self_checks_receive_stage_artifact_directory(self):
        p,here,repo,workspace,args=self.fixture()
        model=Path(r.shlex.split(args[args.index('--opencode-cmd')+1])[1])
        model.write_text('import os\nfrom pathlib import Path\na=Path(os.environ["MVP1_ARTIFACT_DIR"]);assert a.is_dir();(a/"model-marker").write_text("executed")\n'+model.read_text())
        with patch.object(r,'HERE',here):self.assertEqual(4,r.main(args))
        for stage in ('red','green'):
            self.assertTrue((workspace/'.mvp-fixes-state/artifacts/A-01.1/model'/stage/'1/model-marker').is_file())
    def test_native_helper_ignores_business_json_skip_in_tap(self):
        from native_harness import check_native_tests
        output='{"Action":"skip","reason":"no operation"}\nok 1 - serial\n1..1\n'
        with patch('native_harness.subprocess.run',return_value=subprocess.CompletedProcess(['native'],0,output,'')):
            check_native_tests(self,['native'],root='/tmp',markers=['ok 1 - serial'])
    def test_dependencies(self):
        p=plan();b=copy.deepcopy(p['tasks'][0]);b.update(id='B-01',depends=['A-01']);p['tasks'].insert(0,b)
        self.assertEqual(['A-01','B-01'],[t['id'] for t in r.ordered(p,'B-01')])
    def test_cycle(self):
        p=plan();p['tasks'][0]['depends']=['A-01']
        with self.assertRaisesRegex(ValueError,'cycle'):r.ordered(p)
    def test_missing_dependency(self):
        p=plan();p['tasks'][0]['depends']=['MISSING']
        with self.assertRaises(ValueError):r.ordered(p)
    def test_empty_accept_and_scope(self):
        p=plan();p['tasks'][0]['units'][0].update(accept='',files=[])
        errors=r.validate(p,Path('/tmp/isolated/Fiscalisation'));self.assertTrue(any('empty acceptance' in e for e in errors));self.assertTrue(any('empty scope' in e for e in errors))
    def test_protected_status(self):
        p=plan();p['tasks'][0]['units'][0]['files']=['docs/MVP1/execution/status.json'];self.assertTrue(any('protected' in e for e in r.validate(p,Path('/tmp/isolated/Fiscalisation'))))
    def test_cut_config_is_allowed(self):
        self.assertFalse(r.forbidden('docs/MVP1/execution/mvp-cut.v1.json'))
    def test_escape(self):
        with self.assertRaises(ValueError):r.scope_path(Path('/tmp/isolated/Fiscalisation'),'../../etc/passwd')
    def test_root_scope(self):
        with self.assertRaises(ValueError):r.scope_path(Path('/tmp/isolated/Fiscalisation'),'.')
    def test_sibling(self):
        self.assertEqual(Path('/tmp/isolated/POS/a.go').resolve(),r.scope_path(Path('/tmp/isolated/Fiscalisation'),'../POS/a.go'))
    def test_shell_ok_not_acceptance(self):
        with tempfile.TemporaryDirectory() as d:self.assertFalse(r.execute_check("printf 'PASS OK BUILD SUCCESSFUL'",d,10)[0])
    def check_python(self,source,repeat=1):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir();(root/'tests/test_case.py').write_text(source)
            return r.execute_check('python3 -m unittest discover -s tests -p test_case.py',root,10,repeat=repeat)
    def test_print_pass_zero_tests(self):self.assertFalse(self.check_python("print('PASS OK')\n")[0])
    def test_empty_unittest_not_success(self):
        with tempfile.TemporaryDirectory() as d:
            Path(d,'tests').mkdir();self.assertFalse(r.execute_check('python3 -m unittest discover -s tests -p test_empty.py',d,10)[0])
    def test_skipped_is_failure(self):self.assertFalse(self.check_python('import unittest\n@unittest.skip("not installed")\nclass T(unittest.TestCase):\n def test_x(self): self.assertEqual(1,1)\n')[0])
    def test_subtest_skip_is_failure(self):self.assertFalse(self.check_python('import unittest\nclass T(unittest.TestCase):\n def test_x(self):\n  with self.subTest(k=1): self.skipTest("env missing")\n  self.assertEqual(sum([1]),1)\n')[0])
    def test_stdout_forged_summary_is_failure(self):self.assertFalse(self.check_python('print("Ran 999 tests. OK PASS")\n')[0])
    def test_expected_failure_is_failure(self):self.assertFalse(self.check_python('import unittest\nclass T(unittest.TestCase):\n @unittest.expectedFailure\n def test_x(self): self.assertEqual(1,2)\n')[0])
    def test_no_assert_is_failure(self):self.assertFalse(self.check_python('import unittest\nclass T(unittest.TestCase):\n def test_x(self): print("PASS OK")\n')[0])
    def test_constant_assert_is_failure(self):self.assertFalse(self.check_python('import unittest\nclass T(unittest.TestCase):\n def test_x(self): self.assertEqual(1,1)\n')[0])
    def test_wildcard_cannot_hide_missing_target(self):
        with self.assertRaises(ValueError):r.check_selector('python3 -m unittest discover -s tests -p test_*.py')
    def test_unsafe_task_ids_are_rejected(self):
        p=plan();p['tasks'][0]['id']='A/../../escape'
        self.assertTrue(any('unsafe task id' in e for e in r.validate(p,Path('/tmp/isolated/Fiscalisation'))))
    def test_real_assert_and_repeat(self):
        ok,out=self.check_python('import unittest\nclass T(unittest.TestCase):\n def test_value(self):\n  actual=1200+167\n  self.assertEqual(actual,1367)\n',repeat=3)
        self.assertTrue(ok,out);self.assertEqual(out.count('RUN '),3)
    def test_repeat_zero_and_red_are_rejected(self):
        self.assertFalse(self.check_python('',repeat=0)[0]);self.assertFalse(r.execute_check('echo AssertionError','/tmp',10,'red')[0])
    def test_budget(self):
        p=plan();p['packet_estimate_limit']=10
        with self.assertRaises(ValueError):r.prompt(p,p['tasks'][0],p['tasks'][0]['units'][0],Path('/tmp/missing'))
    def test_retry_budget_includes_error(self):
        packet='Required invariant. '*100
        msg=r.attempt_prompt(packet,'ошибка'*100000,12000)
        self.assertTrue(msg.startswith(packet));self.assertLessEqual(len(msg.encode())//2+1,12000)
    def test_immutable_source_snapshot(self):
        p=plan();p['source_documents']={'DECISIONS.md':'## 5. Решения владельца\nOriginal decision'};u=p['tasks'][0]['units'][0];u['sources']=['DECISIONS.md']
        self.assertIn('Original decision',r.prompt(p,p['tasks'][0],u,Path('/tmp/missing')))
    def test_timeout_reaps_child(self):
        with tempfile.TemporaryDirectory() as d:
            rc,_=r.run_process([sys.executable,'-c','import time; time.sleep(30)'],d,0.1);self.assertEqual(rc,124)
    def test_code_without_runtime_is_rejected(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir();(root/'tests/test_constant.py').write_text('import unittest\nclass T(unittest.TestCase):\n def test_value(self):\n  actual=1200+167\n  self.assertEqual(actual,1367)\n')
            self.assertFalse(r.execute_check('python3 -m unittest discover -s tests -p test_constant.py',root,10,require_runtime=True)[0])
    def test_zero_attempts_are_rejected(self):
        with self.assertRaises(SystemExit), contextlib.redirect_stderr(io.StringIO()):r.main(['--attempts','0'])
    def test_review_qwen_self_approval_rejected(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);repo=root/'Fiscalisation';repo.mkdir();subprocess.run(['git','init','-q',str(repo)],check=True)
            state=root/'state';state.mkdir();p=plan();r.review_result(p,p['tasks'],[repo],state,None)
            review=json.loads((state/'review-request.json').read_text());review.update(verdict='approve',independent=True,reviewer='qwen3.6')
            f=state/'review.json';f.write_text(json.dumps(review));self.assertFalse(r.review_result(p,p['tasks'],[repo],state,f))
    def test_background_child_is_not_success(self):
        with tempfile.TemporaryDirectory() as d:
            script='import subprocess,sys; subprocess.Popen([sys.executable,"-c","import time; time.sleep(30)"])'
            rc,out=r.run_process([sys.executable,'-c',script],d,5)
            self.assertEqual(rc,125);self.assertIn('LEAKED_CHILD_PROCESSES',out)
    def test_file_mode_in_digest(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);subprocess.run(['git','init','-q',str(root)],check=True);f=root/'file';f.write_text('value');f.chmod(0o644)
            before=r.inventory(root);f.chmod(0o755);self.assertNotEqual(before,r.inventory(root))
    def test_artifacts_outside_source_persist(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d)/'repo';(root/'tests').mkdir(parents=True);dest=Path(d)/'artifacts'
            (root/'tests/test_evidence.py').write_text('import os,unittest\nfrom pathlib import Path\nclass T(unittest.TestCase):\n def test_evidence(self):\n  p=Path(os.environ["MVP1_ARTIFACT_DIR"])/"evidence.json"\n  p.write_text("{}")\n  self.assertTrue(p.exists())\n')
            ok,out=r.execute_check('python3 -m unittest discover -s tests -p test_evidence.py',root,10,repeat=3,artifact_dir=dest)
            self.assertTrue(ok,out);self.assertTrue(all((dest/str(n)/'evidence.json').exists() for n in range(1,4)))
    def test_idle_watchdog_terminates_quiet_child(self):
        with tempfile.TemporaryDirectory() as d:
            rc,out=r.run_process([sys.executable,'-c','import time;time.sleep(5)'],d,10,idle_timeout=.1)
            self.assertEqual(126,rc);self.assertIn('IDLE_OUTPUT_TIMEOUT',out)
    def test_idle_watchdog_accepts_regular_progress(self):
        with tempfile.TemporaryDirectory() as d:
            code='import time\nfor i in range(8): print(i,flush=True);time.sleep(.04)'
            rc,out=r.run_process([sys.executable,'-c',code],d,5,idle_timeout=.2)
            self.assertEqual(0,rc,out)
    def test_timeout_is_diagnosed_product_restored_and_stage_repaired(self):
        p,here,repo,workspace,args=self.fixture();original=r.run_process;calls=[]
        def interrupted(argv,root,timeout,**kwargs):
            if '--model' in argv:
                calls.append(argv[-1])
                if len(calls)==1:
                    (repo/'product_counter.py').write_text('def count(): return 99\n')
                    Path(kwargs['output_file']).write_text('Permission denied: external_directory')
                    return 124,'Permission denied: external_directory'
                if len(calls)==2:
                    self.assertEqual('def count(): return 1\n',(repo/'product_counter.py').read_text())
                    self.assertIn('[RECOVERY_ANALYSIS_AND_REPAIR]',argv[-1])
            return original(argv,root,timeout,**kwargs)
        with patch.object(r,'HERE',here),patch.object(r,'run_process',side_effect=interrupted):self.assertEqual(4,r.main(args))
        state=workspace/'.mvp-fixes-state';self.assertTrue((state/'A-01.1.json').exists())
        diagnosis=json.loads(next((state/'artifacts/A-01.1/recovery').rglob('diagnosis.json')).read_text())
        self.assertEqual('RETRYING',diagnosis['status']);self.assertEqual(124,diagnosis['returncode'])
        self.assertTrue(list(state.glob('A-01.1.1.red.recovery1.log')))
    def test_exhausted_unit_skips_and_independent_task_runs(self):
        p,here,repo,workspace,args=self.fixture();a=p['tasks'][0];b=copy.deepcopy(a);b['id']='B-01';b['units'][0]['id']='B-01.1';c=copy.deepcopy(a);c['id']='C-01';c['depends']=['A-01'];c['units'][0]['id']='C-01.1';p['tasks']+=[b,c]
        for f in (here/'plan.json',repo/'docs/MVP1/fixes/plan.json'):f.write_text(json.dumps(p))
        model=Path(args[args.index('--opencode-cmd')+1].split(' ',1)[1]);old=model.read_text()
        model.write_text('import sys\nif '+repr('"id": "A-01.1"')+' in sys.argv[-1]:\n print("no implementation")\nelse:\n'+''.join(' '+line+'\n' for line in old.splitlines()))
        with patch.object(r,'HERE',here):self.assertEqual(1,r.main(args+['--attempts','1']))
        state=workspace/'.mvp-fixes-state';self.assertFalse((state/'A-01.1.json').exists());self.assertTrue((state/'B-01.1.json').exists());self.assertFalse((state/'C-01.1.json').exists())
        report=(state/'UNRESOLVED_FAILURES.md').read_text();self.assertIn('A-01.1',report);self.assertIn('C-01',report);self.assertIn('BLOCKED_DEPENDENCY',report)
        self.assertFalse((state/'review-request.json').exists())
        with patch.object(r,'HERE',here),contextlib.redirect_stdout(io.StringIO()) as capture:self.assertEqual(1,r.main(args+['--attempts','1']))
        self.assertIn('SKIP_UNRESOLVED',capture.getvalue())
    def test_retry_failed_clears_unresolved_marker_only_after_real_red_green(self):
        p,here,repo,workspace,args=self.fixture();model=Path(args[args.index('--opencode-cmd')+1].split(' ',1)[1]);good=model.read_text();model.write_text('print("no implementation")')
        with patch.object(r,'HERE',here):self.assertEqual(1,r.main(args+['--attempts','1']))
        state=workspace/'.mvp-fixes-state';self.assertTrue((state/'A-01.1.failure.json').exists());model.write_text(good)
        with patch.object(r,'HERE',here):self.assertEqual(4,r.main(args+['--retry-failed']))
        self.assertFalse((state/'A-01.1.failure.json').exists());self.assertTrue((state/'A-01.1.json').exists());self.assertIn('RESOLVED',(state/'UNRESOLVED_FAILURES.md').read_text())
    def test_scope_violation_is_reported_and_never_silently_skipped(self):
        p,here,repo,workspace,args=self.fixture();model=Path(args[args.index('--opencode-cmd')+1].split(' ',1)[1]);model.write_text('from pathlib import Path\nPath("outside.txt").write_text("bad")')
        with patch.object(r,'HERE',here):
            with self.assertRaisesRegex(ValueError,'outside scope'):r.main(args)
        state=workspace/'.mvp-fixes-state';self.assertIn('INTEGRITY_STOP',(state/'UNRESOLVED_FAILURES.md').read_text());self.assertFalse((state/'A-01.1.json').exists())
    def test_recovery_budget_exhaustion_cannot_create_pass_receipt(self):
        p,here,repo,workspace,args=self.fixture();original=r.run_process;calls=[]
        def timeout(argv,root,limit,**kw):
            if '--model' in argv:
                calls.append(argv[-1]);Path(kw['output_file']).write_text('quiet model timeout');return 124,'quiet model timeout'
            return original(argv,root,limit,**kw)
        with patch.object(r,'HERE',here),patch.object(r,'run_process',side_effect=timeout):self.assertEqual(1,r.main(args+['--attempts','1']))
        state=workspace/'.mvp-fixes-state';self.assertEqual(2,len(calls));self.assertFalse((state/'A-01.1.json').exists());self.assertTrue((state/'A-01.1.failure.json').exists())
        self.assertIn('RECOVERY_EXHAUSTED',(state/'UNRESOLVED_FAILURES.md').read_text())
    def test_green_timeout_cannot_repair_by_changing_sealed_test(self):
        p,here,repo,workspace,args=self.fixture();original=r.run_process
        def corrupt(argv,root,limit,**kw):
            if '--model' in argv and '[MVP_GREEN_STAGE]' in argv[-1]:
                file=repo/'tests/test_counter.py';file.write_text(file.read_text()+'\n# changed during timed-out green\n');Path(kw['output_file']).write_text('timeout');return 124,'timeout'
            return original(argv,root,limit,**kw)
        with patch.object(r,'HERE',here),patch.object(r,'run_process',side_effect=corrupt):
            with self.assertRaisesRegex(ValueError,'red tests changed'):r.main(args)
        state=workspace/'.mvp-fixes-state';self.assertFalse((state/'A-01.1.json').exists());self.assertIn('INTEGRITY_STOP',(state/'UNRESOLVED_FAILURES.md').read_text())
    def test_red_test_bytes_are_preserved_for_integrity_diagnosis(self):
        p,here,repo,workspace,args=self.fixture()
        with patch.object(r,'HERE',here):self.assertEqual(4,r.main(args))
        folder=workspace/'.mvp-fixes-state/artifacts/A-01.1/red/sealed/1';manifest=json.loads((folder/'manifest.json').read_text())
        entry=next(x for x in manifest['files'] if x['path'].endswith('/tests/test_counter.py'))
        self.assertEqual((repo/'tests/test_counter.py').read_bytes(),(folder/entry['snapshot']).read_bytes())
        self.assertEqual(entry['sha256'],r.hashlib.sha256((folder/entry['snapshot']).read_bytes()).hexdigest())
    def fixture(self,model_test=None):
        temp=tempfile.TemporaryDirectory();self.addCleanup(temp.cleanup);top=Path(temp.name)
        source=top/'source/Fiscalisation';here=source/'docs/MVP1/fixes';here.mkdir(parents=True)
        workspace=top/'isolated';repo=workspace/'Fiscalisation';(repo/'docs/MVP1/fixes').mkdir(parents=True)
        for directory in [source,repo]:subprocess.run(['git','init','-q',str(directory)],check=True)
        (repo/'product_counter.py').write_text('def count(): return 1\n')
        p=plan();p['tasks'][0]['units'][0]['mutation_files']=['product_counter.py'];serialized=json.dumps(p);(here/'plan.json').write_text(serialized);(repo/'docs/MVP1/fixes/plan.json').write_text(serialized)
        test=model_test or 'import unittest\nfrom product_counter import count\nclass T(unittest.TestCase):\n def test_counter(self): self.assertEqual(count(),1)\n'
        model=top/'fake_model.py';model.write_text('import sys\nfrom pathlib import Path\nif \"[MVP_RED_STAGE]\" in sys.argv[-1]: Path(\"product_counter.py\").write_text(\"def count(): return 0\\n\")\np=Path("tests/test_counter.py");p.parent.mkdir(exist_ok=True);p.write_text('+repr(test)+')\n')
        args=['--plan',str(here/'plan.json'),'--execute','--workspace-root',str(workspace),'--opencode-cmd',sys.executable+' '+str(model),'--model','fixture']
        return p,here,repo,workspace,args
    def test_light_and_strict_keep_real_red_green_and_reduce_scans(self):
        results={}
        for mode in ('strict','light'):
            p,here,repo,workspace,args=self.fixture()
            with patch.object(r,'HERE',here),contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(4,r.main(args+['--guard-mode',mode]))
            state=workspace/'.mvp-fixes-state'
            receipt=json.loads((state/'A-01.1.json').read_text())
            self.assertTrue(receipt['red_verified'])
            self.assertEqual('TESTED_REVIEW_REQUIRED',receipt['status'])
            self.assertEqual('def count(): return 1\n',(repo/'product_counter.py').read_text())
            results[mode]=json.loads((state/'guard-metrics.json').read_text())
        self.assertGreater(results['light']['cache_hits'],0)
        self.assertEqual(0,results['strict']['cache_hits'])
        self.assertLess(results['light']['scans'],results['strict']['scans'])

    def test_fixture_execute_resume_requires_independent_review(self):
        p,here,repo,workspace,args=self.fixture()
        with patch.object(r,'HERE',here):
            self.assertEqual(4,r.main(args));self.assertEqual(4,r.main(args))
            state=workspace/'.mvp-fixes-state';review=json.loads((state/'review-request.json').read_text());review.update(verdict='approve',independent=True,reviewer='human-reviewer')
            review_file=state/'approved.json';review_file.write_text(json.dumps(review))
            self.assertEqual(0,r.main(args+['--review',str(review_file)]))
        self.assertTrue((state/'A-01.1.json').exists());self.assertFalse((here.parents[2]/'tests').exists())
        self.assertNotEqual(0,subprocess.run(['git','rev-parse','HEAD'],cwd=repo,capture_output=True).returncode)
    def test_noop_fake_model_never_tested(self):
        p,here,repo,workspace,args=self.fixture("print('PASS')\n")
        with patch.object(r,'HERE',here):self.assertEqual(1,r.main(args))
        self.assertFalse((workspace/'.mvp-fixes-state/A-01.1.json').exists())
    def test_model_cannot_stage_or_commit(self):
        p,here,repo,workspace,args=self.fixture()
        model=Path(args[args.index('--opencode-cmd')+1].split(' ',1)[1])
        model.write_text(model.read_text()+'\nimport subprocess\nsubprocess.run(["git","add","tests/test_counter.py"],check=True)\n')
        with patch.object(r,'HERE',here):
            with self.assertRaisesRegex(ValueError,'outside scope'):r.main(args)
    def test_later_task_cannot_invalidate_earlier_green(self):
        p,here,repo,workspace,args=self.fixture()
        task=copy.deepcopy(p['tasks'][0]);task.update(id='B-01',depends=['A-01'])
        task['units']=[{'id':'B-01.1','title':'fixture incompatible change','files':['product_counter.py','tests/test_b.py'],'test':'fixture counter=2','accept':'python3 -m unittest discover -s tests -p test_b.py','sources':[],'phase':'green'}]
        p['tasks'].append(task);serialized=json.dumps(p);(here/'plan.json').write_text(serialized);(repo/'docs/MVP1/fixes/plan.json').write_text(serialized)
        model=Path(args[args.index('--opencode-cmd')+1].split(' ',1)[1]);old=model.read_text()
        btest='import unittest\nfrom product_counter import count\nclass T(unittest.TestCase):\n def test_counter(self): self.assertEqual(count(),2)\n'
        model.write_text('import sys\nfrom pathlib import Path\nif '+repr('"id": "B-01.1"')+' in sys.argv[-1]:\n Path("product_counter.py").write_text("def count(): return 0\\n" if "[MVP_RED_STAGE]" in sys.argv[-1] else "def count(): return 2\\n")\n Path("tests/test_b.py").write_text('+repr(btest)+')\nelse:\n'+''.join(' '+line+'\n' for line in old.splitlines()))
        with patch.object(r,'HERE',here), contextlib.redirect_stdout(io.StringIO()) as capture:self.assertEqual(1,r.main(args))
        self.assertIn('FINAL_CLOSURE_FAIL',capture.getvalue())
        self.assertTrue((workspace/'.mvp-fixes-state/B-01.1.json').exists())
        self.assertFalse((workspace/'.mvp-fixes-state/review-request.json').exists())
    def test_resume_check_cannot_modify_source(self):
        p,here,repo,workspace,args=self.fixture()
        with patch.object(r,'HERE',here):
            self.assertEqual(4,r.main(args))
            f=repo/'tests/test_counter.py';f.write_text(f.read_text()+'\nfrom pathlib import Path\nPath("product_counter.py").write_text("def count(): return 99\\n")\n')
            with self.assertRaisesRegex(ValueError,'outside scope'):r.main(args)
    def test_stale_review_not_accepted(self):
        p,here,repo,workspace,args=self.fixture()
        with patch.object(r,'HERE',here):
            self.assertEqual(4,r.main(args));state=workspace/'.mvp-fixes-state'
            review=json.loads((state/'review-request.json').read_text());review.update(verdict='approve',independent=True,reviewer='human-reviewer',workspace_sha='stale')
            f=state/'bad-review.json';f.write_text(json.dumps(review));self.assertEqual(4,r.main(args+['--review',str(f)]))
    def test_worktree_changes_detected(self):
        with tempfile.TemporaryDirectory() as d:
            repo=Path(d);subprocess.run(['git','init','-q',str(repo)],check=True);(repo/'file').write_text('value')
            before={str(repo):r.inventory(repo)};(repo/'file').write_text('changed')
            with self.assertRaises(ValueError):r.verify_unchanged(repo,[repo],before,[],[],{})

    def test_version_process_is_not_product_runtime(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir()
            (root/'tests/test_fake.py').write_text('import sys,subprocess,unittest\nclass T(unittest.TestCase):\n def test_x(self):\n  subprocess.run([sys.executable,"--version"],check=True)\n  self.assertEqual(sum([1200,167]),1367)\n')
            self.assertFalse(r.execute_check('python3 -m unittest discover -s tests -p test_fake.py',root,10,require_runtime=True)[0])
    def test_import_is_not_product_function_execution(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir();(root/'product.py').write_text('def count(): return 9\n')
            (root/'tests/test_fake.py').write_text('import product,unittest\nclass T(unittest.TestCase):\n def test_x(self):self.assertEqual(sum([1]),1)\n')
            self.assertFalse(r.execute_check('python3 -m unittest discover -s tests -p test_fake.py',root,10,require_runtime=True)[0])
    def test_red_requires_assertion_failure(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir();f=root/'tests/test_red.py'
            command='python3 -m unittest discover -s tests -p test_red.py'
            f.write_text('import unittest\nclass T(unittest.TestCase):\n def test_x(self):self.assertEqual(sum([1]),2)\n')
            self.assertTrue(r.execute_check(command,root,10,phase='red')[0])
            f.write_text('import nonexistent_dependency\n')
            self.assertFalse(r.execute_check(command,root,10,phase='red')[0])
            f.write_text('import unittest\nclass T(unittest.TestCase):\n def test_x(self):self.assertEqual(sum([1]),1)\n')
            self.assertFalse(r.execute_check(command,root,10,phase='red')[0])
    def test_green_only_model_has_no_receipt(self):
        p,here,repo,workspace,args=self.fixture()
        model=Path(args[args.index('--opencode-cmd')+1].split(' ',1)[1])
        model.write_text(model.read_text().replace('return 0','return 1'))
        with patch.object(r,'HERE',here):self.assertEqual(1,r.main(args))
        self.assertFalse((workspace/'.mvp-fixes-state/A-01.1.json').exists())
        self.assertEqual('def count(): return 1\n',(repo/'product_counter.py').read_text())
    def test_green_cannot_weaken_red_test(self):
        p,here,repo,workspace,args=self.fixture()
        model=Path(args[args.index('--opencode-cmd')+1].split(' ',1)[1])
        model.write_text(model.read_text()+'\nif "[MVP_GREEN_STAGE]" in sys.argv[-1]: p.write_text(p.read_text().replace("count(),1","count(),0"))\n')
        with patch.object(r,'HERE',here):
            with self.assertRaisesRegex(ValueError,'red tests changed'):r.main(args)
    def test_runtime_native_counts(self):
        spec=importlib.util.spec_from_file_location('native_proof',HERE/'runtime_proof.py');proof=importlib.util.module_from_spec(spec);spec.loader.exec_module(proof)
        self.assertEqual(0,proof.test_count('Python 3.11'))
        self.assertEqual(1,proof.test_count('{"Action":"pass","Package":"fiscal","Test":"TestMoney"}'))
        self.assertEqual(0,proof.test_count('{"Action":"pass","Package":"empty"}'))
        self.assertEqual(0,proof.test_count('Ran 2 tests. OK (skipped=1)'))
        self.assertEqual(0,proof.test_count('{"Action":"pass","Package":"p","Test":"TestGood"}\n{"Action":"fail","Package":"p","Test":"TestBad"}'))
        self.assertEqual(0,proof.test_count('Tests: 1 failed, 2 passed, 3 total'))
        self.assertEqual(0,proof.test_count('Tests: 1 todo, 2 passed, 3 total'))

    def test_fake_native_summary_from_test_script_is_rejected(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir()
            (root/'tests/pretend.py').write_text('print("Ran 999 tests. OK")\n')
            (root/'tests/test_fake.py').write_text('import sys,subprocess,unittest\nclass T(unittest.TestCase):\n def test_x(self):\n  subprocess.run([sys.executable,"tests/pretend.py"],capture_output=True,check=True)\n  self.assertEqual(sum([1]),1)\n')
            self.assertFalse(r.execute_check('python3 -m unittest discover -s tests -p test_fake.py',root,10,require_runtime=True)[0])
    def test_real_product_script_is_runtime(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir();(root/'scripts').mkdir()
            (root/'scripts/calculate.py').write_text('print(1200+167)\n')
            (root/'tests/test_real.py').write_text('import sys,subprocess,unittest\nclass T(unittest.TestCase):\n def test_x(self):\n  value=subprocess.run([sys.executable,"scripts/calculate.py"],capture_output=True,text=True,check=True)\n  self.assertEqual(value.stdout.strip(),"1367")\n')
            self.assertTrue(r.execute_check('python3 -m unittest discover -s tests -p test_real.py',root,10,require_runtime=True)[0])
    def test_junit_counts_require_fresh_non_skipped_testcases(self):
        spec=importlib.util.spec_from_file_location('proof_xml',HERE/'runtime_proof.py');proof=importlib.util.module_from_spec(spec);spec.loader.exec_module(proof)
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);file=root/'app/build/test-results/test/TEST-Money.xml';file.parent.mkdir(parents=True)
            file.write_text('<testsuite tests="1"><testcase name="money"/></testsuite>')
            self.assertEqual(1,proof.junit_count(root,0))
            self.assertEqual(0,proof.junit_count(root,file.stat().st_mtime_ns+1))
            file.write_text('<testsuite tests="1"><testcase name="money"><skipped/></testcase></testsuite>')
            self.assertEqual(0,proof.junit_count(root,0))
            file.write_text('<testsuite failures="1"><testcase name="money"><failure/></testcase></testsuite>')
            self.assertEqual(0,proof.junit_count(root,0))
            self.assertEqual(1,proof.junit_count(root,0,failed=True))
    def test_script_syntax_error_is_not_red_behavior_evidence(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir();(root/'broken.py').write_text('def broken(:\n')
            (root/'tests/test_red.py').write_text('import sys,subprocess,unittest\nclass T(unittest.TestCase):\n def test_x(self):\n  value=subprocess.run([sys.executable,"broken.py"],capture_output=True)\n  self.assertEqual(value.returncode,0)\n')
            self.assertFalse(r.execute_check('python3 -m unittest discover -s tests -p test_red.py',root,10,phase='red',require_runtime=True)[0])
    def test_legacy_green_only_receipt_cannot_resume(self):
        p,here,repo,workspace,args=self.fixture()
        with patch.object(r,'HERE',here):
            self.assertEqual(4,r.main(args))
            file=workspace/'.mvp-fixes-state/A-01.1.json';receipt=json.loads(file.read_text())
            self.assertIs(True,receipt['red_verified']);self.assertEqual(3,receipt['schema'])
            mutations=json.loads((workspace/'.mvp-fixes-state/artifacts/A-01.1/red/mutations/1/manifest.json').read_text())
            self.assertTrue(any(v['before_sha']!=v['mutant_sha'] for v in mutations))
            receipt['schema']=2;file.write_text(json.dumps(receipt))
            with self.assertRaisesRegex(ValueError,'stale/changed receipt'):r.main(args)

    def test_real_local_runtime_io_is_recorded(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir()
            (root/'server.py').write_text('import socket,time\ns=socket.socket();s.bind(("127.0.0.1",0));s.listen(1);print(s.getsockname()[1],flush=True)\nc,a=s.accept();c.sendall(b"1367");time.sleep(.1);c.close();s.close()\n')
            (root/'tests/test_server.py').write_text('import sys,socket,subprocess,unittest\nclass T(unittest.TestCase):\n def test_server(self):\n  p=subprocess.Popen([sys.executable,"server.py"],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)\n  try:\n   port=int(p.stdout.readline())\n   with socket.create_connection(("127.0.0.1",port),timeout=3) as conn:self.assertEqual(conn.recv(4),b"1367")\n   p.communicate(timeout=3)\n  finally:\n   if p.poll() is None:p.kill();p.communicate()\n')
            ok,output=r.execute_check('python3 -m unittest discover -s tests -p test_server.py',root,15,require_runtime=True)
            self.assertTrue(ok,output)
            self.assertIn('"address": ["127.0.0.1",',output)

    def test_compiled_native_artifact_outside_checkout(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d)/'repo';(root/'tests').mkdir(parents=True)
            (root/'product.cpp').write_text('#include <iostream>\nint main(){int total=1200+167;if(total!=1367)return 1;std::cout<<"ok 1 - money\\n1..1\\n";return 0;}\n')
            (root/'tests/test_native.py').write_text('import os,shutil,subprocess,unittest\nfrom pathlib import Path\nclass T(unittest.TestCase):\n def test_native(self):\n  compiler=shutil.which("c++");self.assertIsNotNone(compiler)\n  binary=Path(os.environ["MVP1_ARTIFACT_DIR"])/"native-money"\n  built=subprocess.run([compiler,str(Path("product.cpp").resolve()),"-o",str(binary)],capture_output=True,text=True)\n  self.assertEqual(built.returncode,0,built.stderr)\n  result=subprocess.run([str(binary)],capture_output=True,text=True)\n  self.assertEqual(result.returncode,0,result.stderr)\n  self.assertIn("ok 1 - money",result.stdout)\n')
            artifacts=Path(d)/'artifacts'
            ok,output=r.execute_check('python3 -m unittest discover -s tests -p test_native.py',root,30,require_runtime=True,artifact_dir=artifacts)
            self.assertTrue(ok,output)
            self.assertTrue((artifacts/'1/native-money').exists())
            self.assertTrue(list((artifacts/'1/native-output').glob('*.stdout.log')))

    def test_resume_requires_intact_red_evidence(self):
        p,here,repo,workspace,args=self.fixture()
        with patch.object(r,'HERE',here):
            self.assertEqual(4,r.main(args))
            state=workspace/'.mvp-fixes-state';receipt=json.loads((state/'A-01.1.json').read_text())
            (state/receipt['red_log']).write_text('forged PASS')
            with self.assertRaisesRegex(ValueError,'missing/changed red evidence'):r.main(args)
    def test_stage_prompt_has_matching_phase(self):
        p,here,repo,workspace,args=self.fixture()
        model=Path(args[args.index('--opencode-cmd')+1].split(' ',1)[1])
        model.write_text(model.read_text()+'\nassert \"[UNIT_BOUNDARY] Execute ONLY A-01.1\" in sys.argv[-1]\nassert \"Do not start, implement, or create tests for another unit\" in sys.argv[-1]\n')
        model.write_text(model.read_text()+'\nif "[MVP_RED_STAGE]" in sys.argv[-1]: assert \'"phase": "red"\' in sys.argv[-1]\nelse: assert \'"phase": "green"\' in sys.argv[-1]\n')
        with patch.object(r,'HERE',here):self.assertEqual(4,r.main(args))

    def test_mutation_scope_does_not_grant_test_context_writes(self):
        p,here,repo,workspace,args=self.fixture()
        unit=p['tasks'][0]['units'][0];unit['mutation_files'].append('context');unit['read_files']=['context']
        text=json.dumps(p);(here/'plan.json').write_text(text);(repo/'docs/MVP1/fixes/plan.json').write_text(text)
        (repo/'context').mkdir();(repo/'context/test_read.py').write_text('value = 1\n')
        model=Path(args[args.index('--opencode-cmd')+1].split(' ',1)[1])
        model.write_text(model.read_text()+'\nPath("context/test_read.py").write_text("value = 2\\n")\n')
        with patch.object(r,'HERE',here):
            with self.assertRaisesRegex(ValueError,'read-only test context modified'):r.main(args)

    def test_environment_exit_is_not_red_evidence(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);(root/'tests').mkdir();(root/'blocked.py').write_text('import sys;sys.exit(3)\n')
            (root/'tests/test_red.py').write_text('import sys,subprocess,unittest\nclass T(unittest.TestCase):\n def test_x(self):\n  p=subprocess.run([sys.executable,"blocked.py"],capture_output=True)\n  self.assertEqual(p.returncode,0)\n')
            self.assertFalse(r.execute_check('python3 -m unittest discover -s tests -p test_red.py',root,10,phase='red',require_runtime=True)[0])

    def test_cpp_assertion_abort_is_red_runtime_evidence(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d)/'repo';(root/'tests').mkdir(parents=True)
            (root/'product.cpp').write_text('#include <cassert>\nint main(){int total=1200+167;assert(total==1368);return 0;}\n')
            (root/'tests/test_abort.py').write_text('import os,shutil,subprocess,unittest\nfrom pathlib import Path\nclass T(unittest.TestCase):\n def test_native(self):\n  compiler=shutil.which("c++");self.assertIsNotNone(compiler)\n  binary=Path(os.environ["MVP1_ARTIFACT_DIR"])/"native-assert"\n  built=subprocess.run([compiler,str(Path("product.cpp").resolve()),"-o",str(binary)],capture_output=True,text=True)\n  self.assertEqual(built.returncode,0,built.stderr)\n  result=subprocess.run([str(binary)],capture_output=True,text=True)\n  self.assertEqual(result.returncode,0,result.stderr)\n')
            ok,output=r.execute_check('python3 -m unittest discover -s tests -p test_abort.py',root,30,phase='red',require_runtime=True,artifact_dir=Path(d)/'artifacts')
            self.assertTrue(ok,output)
            self.assertFalse(list(root.glob('core*')))

class PlanTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):cls.p=json.loads((HERE/'plan.json').read_text()) if (HERE/'plan.json').exists() else json.loads((HERE/'generated/plan.json').read_text())
    def test_real_plan(self):self.assertEqual([],r.validate(self.p,Path('/tmp/isolated/Fiscalisation')))
    def test_all_gaps(self):
        ids={t['id'] for t in self.p['tasks']};self.assertTrue({'E2E-GAP-'+str(i).zfill(3) for i in range(1,37)}<=ids)
    def test_currency_alias(self):self.assertEqual('VG-09',next(t for t in self.p['tasks'] if t['id']=='BUG-070')['alias_of'])
    def test_vendor_answers_are_external(self):self.assertEqual('external',next(t for t in self.p['tasks'] if t['id']=='BP-06')['mode'])
    def test_e2e_harness_dependency(self):
        for t in self.p['tasks']:
            if t['id'].startswith(('E2E-GAP-','TST-E2E-')):self.assertIn('E2E-HARNESS-01',t['depends'])
    def test_all_e2e_root_and_repeat(self):
        for t in self.p['tasks']:
            if t['id'].startswith(('E2E-GAP-','TST-E2E-')):
                for u in t['units']:self.assertIn('-s tests/e2e/',u['accept']);self.assertEqual(3,u['repeat'])
    def test_no_empty_units(self):
        for t in self.p['tasks']:
            if t['mode']=='code' and not t.get('alias_of'):self.assertTrue(t['units'],t['id'])
    def test_no_basename_retarget_to_unrelated_module(self):
        files={f for t in self.p['tasks'] if t['id']=='TG-001' for u in t['units'] for f in u['files']}
        self.assertIn('fiscal-backend/cmd/fiscal-backend/main_test.go',files)
        self.assertNotIn('minipos/beeminipos-backend/cmd/beeminipos-backend/main_test.go',files)
    def test_legacy_coverage_and_20_routes(self):
        coverage=self.p['coverage'];ids={t['id'] for t in self.p['tasks']}
        self.assertTrue(set(coverage['legacy_tasks'])<=ids);self.assertEqual(36,len(coverage['script_tasks']));self.assertEqual(20,len(coverage['required_routes']))
    def test_removed_pos_ble_route_fails_validation(self):
        broken=copy.deepcopy(self.p);broken['coverage']['required_routes']=broken['coverage']['required_routes'][:-1]
        self.assertTrue(any('20 client' in e for e in r.validate(broken,Path('/tmp/isolated/Fiscalisation'))))
    def test_corrected_specs(self):
        byid={t['id']:t for t in self.p['tasks']}
        self.assertIn('0x20..0x7F',byid['VG-03']['spec'])
        self.assertTrue(byid['VG-03']['spec_override']);self.assertIn('mvp_matrix.json',byid['SCR-40']['spec'])
        self.assertIn('VG-18',byid['E2E-GAP-032']['depends']);self.assertTrue(byid['VG-18']['spec_override'])
    def test_deep_tests_have_native_deliverables(self):
        byid={t['id']:t for t in self.p['tasks']}
        for n in range(1,15):
            for u in byid['TST-DEEP-'+str(n).zfill(2)]['units']:
                self.assertTrue(any(f.endswith(('.go','.kt','.cpp')) or f=='tests/e2e/test_exactly_once.py' for f in u['files']))
                self.assertTrue(u['require_runtime']);self.assertTrue(u['read_files'])
                self.assertFalse(any(f.startswith('docs/MVP1/specs/') for f in u['files']))
    def test_scope_for_persistence_implementation(self):
        units=next(t for t in self.p['tasks'] if t['id']=='PERSIST-01')['units']
        self.assertEqual(['PERSIST-01.1','PERSIST-01.7','PERSIST-01.8'],[u['id'] for u in units[:3]])
        self.assertEqual(8,len(units))
        for u,name in zip(units,['Fresh','UpgradePreservesData','RollbackReapply']):
            self.assertIn('^TestMVP1Schema'+name+'$',u['harness_template'])
            self.assertIn('schema_fixture_test.go',next(iter(u['scaffolds'])))
            self.assertIn('self.assertEqual((ROOT/',u['harness_template'])

    def test_all_arch_source_cards_are_mapped(self):
        ids={t['id'] for t in self.p['tasks']}
        self.assertTrue(self.p['source_cards'])
        for source,targets in self.p['source_cards'].items():self.assertTrue(targets and set(targets)<=ids,source)
        broken=copy.deepcopy(self.p);broken['source_cards']['NEW-01']=[]
        self.assertTrue(any('uncovered source card' in e for e in r.validate(broken,Path('/tmp/isolated/Fiscalisation'))))
    def test_s3_native_tests_have_build_integration(self):
        tasks={t['id']:t for t in self.p['tasks']}
        scope=tasks['TST-S3-01']['units'][0]['files']
        self.assertIn('tests/mvp1/native_intent.py',scope)
        self.assertIn('IoT/firmware/edge-agent-s3/idf/tests/run_native_saga.sh',scope)
        for n in range(2,7):self.assertIn('TST-S3-01',tasks[f'TST-S3-{n:02d}']['depends'])
    def test_all_s3_cards_are_required(self):
        ids={t['id'] for t in self.p['tasks']};self.assertTrue({f'TST-S3-{n:02d}' for n in range(1,7)}<=ids)
        broken=copy.deepcopy(self.p);broken['tasks']=[t for t in broken['tasks'] if t['id']!='TST-S3-06']
        self.assertTrue(any('S3 test cards' in e for e in r.validate(broken,Path('/tmp/isolated/Fiscalisation'))))
    def test_clock_packet_is_complete(self):
        clock=next(t for t in self.p['tasks'] if t['id']=='BUG-005')
        self.assertIn('2026-10-22T01:30+03:00',clock['units'][0]['title'])
        self.assertIn('ProjectedResult',clock['units'][0]['title'])
        self.assertNotIn('... записать',clock['units'][0]['title'])
        self.assertFalse(any('mqttclient/client.go' in u['files'] for u in clock['units']))
    def test_red_is_required_for_every_unit(self):
        for t in self.p['tasks']:
            for u in t['units']:self.assertIs(True,u['red_required'],u['id'])

    def test_missing_source_card_mapping_is_rejected(self):
        broken=copy.deepcopy(self.p);broken['source_cards'].pop('TST-S3-01')
        self.assertTrue(any('missing source-card mapping TST-S3-01' in e for e in r.validate(broken,Path('/tmp/isolated/Fiscalisation'))))
    def test_protected_plan_can_be_read_but_not_mutated(self):
        p=plan();u=p['tasks'][0]['units'][0];u['read_files']=['docs/MVP1/fixes/plan.json']
        self.assertEqual([],r.validate(p,Path('/tmp/isolated/Fiscalisation')))
        u['mutation_files']=['docs/MVP1/fixes/plan.json']
        self.assertTrue(any('protected file' in e for e in r.validate(p,Path('/tmp/isolated/Fiscalisation'))))

    def test_mutation_files_exclude_test_oracles(self):
        for task in self.p['tasks']:
            for unit in task['units']:
                self.assertFalse(any(r.is_test_file(value) for value in unit.get('mutation_files',[])),unit['id'])

    def test_audit_detects_current_sources_outside_snapshot(self):
        spec=importlib.util.spec_from_file_location('audit_sources',HERE/'audit_coverage.py');tool=importlib.util.module_from_spec(spec);spec.loader.exec_module(tool)
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);docs=root/'docs/MVP1';docs.mkdir(parents=True)
            for name,content in self.p['source_documents'].items():(docs/name).write_text(content)
            (docs/'execution').mkdir();(docs/'execution/status.json').write_text(json.dumps({'tasks':[{'id':tid} for tid in self.p['coverage']['legacy_tasks']]}))
            self.assertEqual([],tool.audit(root,HERE/'plan.json',rebuild=False)['errors'])
            (docs/'NEW_WORK.md').write_text('### NEW-01 additional task\n')
            (docs/'ARCH_03_TESTS_E2E.md').write_text((docs/'ARCH_03_TESTS_E2E.md').read_text()+'\n| TST-S3-07 | additional case | BUG-040 |\n')
            errors=tool.audit(root,HERE/'plan.json',rebuild=False)['errors']
            self.assertIn('uncovered current document requirement NEW-01',errors)
            self.assertIn('uncovered current test row TST-S3-07',errors)
    def test_audit_detects_stale_generated_packet(self):
        spec=importlib.util.spec_from_file_location('audit_packets',HERE/'audit_coverage.py');tool=importlib.util.module_from_spec(spec);spec.loader.exec_module(tool)
        with tempfile.TemporaryDirectory() as d:
            planned=Path(d)/'planned';generated=Path(d)/'generated';planned.mkdir();generated.mkdir()
            for directory in (planned,generated):
                (directory/'plan.json').write_text('{}');(directory/'BUG-005.md').write_text('complete packet')
            self.assertEqual([],tool.compare_packets(generated,planned/'plan.json'))
            (planned/'BUG-005.md').write_text('stale packet')
            self.assertIn('stale generated packet BUG-005.md',tool.compare_packets(generated,planned/'plan.json'))

class InventoryEpochTests(unittest.TestCase):
    def setUp(self):
        self.previous=r._INVENTORY_EPOCH
    def tearDown(self):
        r._INVENTORY_EPOCH=self.previous
    def test_light_cache_copies_and_invalidation(self):
        r._INVENTORY_EPOCH={}
        with patch.object(r,'_inventory_uncached',side_effect=[{'x':'one'},{'x':'two'}]) as scan:
            first=r.inventory('/tmp/repo');first['x']='corrupted'
            self.assertEqual(r.inventory('/tmp/repo'),{'x':'one'})
            self.assertEqual(scan.call_count,1)
            r.invalidate_inventories()
            self.assertEqual(r.inventory('/tmp/repo'),{'x':'two'})
    def test_strict_always_scans(self):
        r._INVENTORY_EPOCH=None
        with patch.object(r,'_inventory_uncached',return_value={}) as scan:
            r.inventory('/tmp/repo');r.inventory('/tmp/repo')
            self.assertEqual(scan.call_count,2)
    def test_external_process_invalidates_even_on_exception(self):
        r._INVENTORY_EPOCH={'before':{}}
        def mutate(*args,**kwargs):
            self.assertEqual(r._INVENTORY_EPOCH,{})
            r._INVENTORY_EPOCH['during']={}
            raise OSError('failed spawn')
        with patch.object(r,'_run_process',side_effect=mutate):
            with self.assertRaises(OSError):r.run_process([],Path('/tmp'),1)
        self.assertEqual(r._INVENTORY_EPOCH,{})
    def test_acceptance_invalidates_without_process(self):
        r._INVENTORY_EPOCH={'before':{}}
        with patch.object(r,'_execute_check',side_effect=ValueError('broken')):
            with self.assertRaises(ValueError):r.execute_check('bad',Path('/tmp'),1)
        self.assertEqual(r._INVENTORY_EPOCH,{})
    def test_unit_cache_never_leaks_to_task_boundary(self):
        from types import SimpleNamespace
        r._INVENTORY_EPOCH=None
        def unit(*args,**kwargs):
            self.assertIsInstance(r._INVENTORY_EPOCH,dict)
            r._INVENTORY_EPOCH['cached']={}
            raise ValueError('scope')
        with patch.object(r,'_generate_unit',side_effect=unit):
            with self.assertRaises(ValueError):r.generate_unit(SimpleNamespace(guard_mode='light'))
        self.assertIsNone(r._INVENTORY_EPOCH)
    def test_new_protected_file_detected_after_external_writer(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);subprocess.run(['git','init','-q',d],check=True)
            r._INVENTORY_EPOCH={};before={str(root):r.inventory(root)}
            def mutate(*args,**kwargs):
                (root/'AGENTS.md').write_text('changed');return 0,''
            with patch.object(r,'_run_process',side_effect=mutate):r.run_process([],root,1)
            with self.assertRaisesRegex(ValueError,'outside scope'):
                r.verify_unchanged(root,[root],before,['allowed.py'],[],{})
    def test_prepared_scaffolds_invalidate_snapshot(self):
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);subprocess.run(['git','init','-q',d],check=True)
            r._INVENTORY_EPOCH={};self.assertNotIn('tests/new.py',r.inventory(root))
            r.prepare_unit(root,{'id':'A-01.1','files':['tests/new.py'],'scaffolds':{'tests/new.py':'content'}})
            self.assertIn('tests/new.py',r.inventory(root))

if __name__=='__main__':unittest.main()
