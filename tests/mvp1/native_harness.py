"""Phase-independent native suite assertions shared by generated MVP1 harnesses.

Always demand green. The controller decides whether a real assertion failure
is valid red evidence; tests must never infer their phase from product source.
"""
import json
import os
from pathlib import Path
import subprocess
import time
from runtime_proof import test_count, junit_count


def check_native_tests(case, argv, *, root, cwd='.', required_tests=(),
                       markers=(), minimum=1, timeout=120):
    """Execute a real suite, reject empty/skipped results, demand named tests.

    required_tests contains exact Go JSON Test names (including subtests).
    markers names executed TAP/GTest cases for non-Go suites.
    Missing executables/compile failures retain their real errors, never SKIP.
    """
    if not isinstance(argv, (list, tuple)) or not argv or minimum < 1:
        raise ValueError('nonempty argv and positive minimum required')
    directory=Path(root)/cwd
    started=time.time_ns()
    env=dict(os.environ,PWD=str(directory.resolve()));env.pop('OLDPWD',None)
    result=subprocess.run(list(argv), cwd=directory, capture_output=True,
                          text=True, timeout=timeout,env=env)
    events=[]
    for line in result.stdout.splitlines():
        try:event=json.loads(line)
        except ValueError:continue
        if isinstance(event,dict) and event.get('Package'):events.append(event)
    ran={e.get('Test') for e in events if e.get('Action')=='run'}
    if set(required_tests)-ran or any(e.get('Action')=='skip' for e in events):
        raise RuntimeError('native cases missing/skipped before outcome: '+repr(sorted(set(required_tests)-ran)))
    case.assertEqual(result.returncode, 0,
                     'native rc='+str(result.returncode)+'\n'+result.stdout[-4000:]+'\n'+result.stderr[-4000:])
    count=test_count(result.stdout)+test_count(result.stderr)
    if 'gradlew' in Path(str(argv[0])).name:
        count=junit_count(directory, started)
    case.assertGreaterEqual(count, minimum, 'empty/skipped/failed native suite')
    passed=set()
    for line in result.stdout.splitlines():
        try:event=json.loads(line)
        except ValueError:continue
        if isinstance(event,dict) and event.get('Action')=='pass' and event.get('Test'):
            passed.add(event['Test'])
    case.assertTrue(set(required_tests)<=passed,
                    'missing Go pass events: '+repr(sorted(set(required_tests)-passed)))
    for marker in markers:
        case.assertIn(marker,result.stdout,'missing native scenario marker')
    return result
