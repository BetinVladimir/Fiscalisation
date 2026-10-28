#!/usr/bin/env python3
"""Trusted unittest controller. Test stdout is never an acceptance protocol.
Run this copy outside the generated checkout. Reports are written by the controller,
not by a generated harness. This is not a sandbox or independent semantic review.
"""
import argparse
import resource
import ast
import contextlib
import inspect
import json
from pathlib import Path
import sys
import unittest
import importlib.util
_spec=importlib.util.spec_from_file_location('mvp_runtime_proof',Path(__file__).with_name('runtime_proof.py'))
runtime_proof=importlib.util.module_from_spec(_spec);_spec.loader.exec_module(runtime_proof)

class Result(unittest.TextTestResult):
    def startTest(self,test):
        super().startTest(test);self.current_assertions=0
    def addSuccess(self,test):
        if self.current_assertions<1:
            try:raise AssertionError('NO_EXECUTED_ASSERTION: use unittest assertions')
            except AssertionError:self.addFailure(test,sys.exc_info())
        else:super().addSuccess(test)

def has_constant_assertion(test):
    try:node=ast.parse(inspect.getsource(type(test)))
    except (OSError,TypeError,IndentationError,SyntaxError):return False
    for call in ast.walk(node):
        if not isinstance(call,ast.Call) or not isinstance(call.func,ast.Attribute):continue
        if call.func.attr not in ('assertTrue','assertFalse','assertEqual','assertIs','assertIsNot','assertNotEqual'):continue
        args=call.args[:1] if call.func.attr in ('assertTrue','assertFalse') else call.args[:2]
        if args and all(isinstance(a,ast.Constant) for a in args):return True
    return False

def flatten(suite):
    for item in suite:
        if isinstance(item,unittest.TestSuite):yield from flatten(item)
        else:yield item

def main():
    p=argparse.ArgumentParser();p.add_argument('--root',type=Path,required=True);p.add_argument('--start',required=True);p.add_argument('--pattern',required=True);p.add_argument('--report',type=Path,required=True);args=p.parse_args()
    resource.setrlimit(resource.RLIMIT_CORE,(0,0))
    root=args.root.resolve();start=(root/args.start).resolve()
    if root not in start.parents or not start.is_dir():raise ValueError('invalid/missing discovery directory')
    sys.path.insert(0,str(root));sys.path.insert(0,str(start))
    report={'schema':1,'tests':0,'failures':0,'errors':0,'skipped':0,'expected_failures':0,'unexpected_successes':0,'constant_assertions':[],'loader_errors':[],'child_calls':0,'assertions':0}
    cleanup_runtime=runtime_proof.install(root,report)
    loader=unittest.TestLoader();suite=loader.discover(str(start),pattern=args.pattern)
    report['loader_errors']=loader.errors
    tests=list(flatten(suite))
    report['product_calls'].clear();report['native_runs'].clear();report['native_failures'].clear();report['runtime_io'].clear()
    report['constant_assertions']=[t.id() for t in tests if has_constant_assertion(t)]
    def audit(event,params):
        if event=='subprocess.Popen':report['child_calls']+=1
    sys.addaudithook(audit)
    # Count assertions actually exercised, not assert-shaped text in stdout.
    original={}
    active=[None]
    for name in dir(unittest.TestCase):
        method=getattr(unittest.TestCase,name)
        if name.startswith('assert') and callable(method):
            original[name]=method
            def wrapped(self,*a,_method=method,**kw):
                report['assertions']+=1
                if active[0] is not None:active[0].current_assertions+=1
                return _method(self,*a,**kw)
            setattr(unittest.TestCase,name,wrapped)
    def factory(*a,**kw):active[0]=Result(*a,**kw);return active[0]
    try:result=unittest.TextTestRunner(verbosity=2,resultclass=factory).run(suite)
    finally:
        for name,method in original.items():setattr(unittest.TestCase,name,method)
    cleanup_runtime()
    report.update(tests=result.testsRun,failures=len(result.failures),errors=len(result.errors),skipped=len(result.skipped),expected_failures=len(result.expectedFailures),unexpected_successes=len(result.unexpectedSuccesses))
    report['product_modules']=[]
    for module in list(sys.modules.values()):
        file=getattr(module,'__file__',None)
        if not file:continue
        resolved=Path(file).resolve()
        if root in resolved.parents:
            relative=resolved.relative_to(root)
            if 'tests' not in relative.parts and '__tests__' not in relative.parts and '__pycache__' not in relative.parts and not str(relative).startswith('docs/MVP1/fixes/'):report['product_modules'].append(str(relative))
    report['successful']=result.wasSuccessful() and result.testsRun>0 and report['assertions']>0 and not any(report[k] for k in ['loader_errors','constant_assertions','skipped','expected_failures','unexpected_successes'])
    args.report.write_text(json.dumps(report,ensure_ascii=False,indent=2))
    return 0 if report['successful'] else 1

if __name__=='__main__':
    try:sys.exit(main())
    except Exception as exc:print(type(exc).__name__+': '+str(exc),file=sys.stderr);sys.exit(2)
