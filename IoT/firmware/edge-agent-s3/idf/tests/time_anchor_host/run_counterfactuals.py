#!/usr/bin/env python3
"""Run bounded RED mutations only inside disposable copies, never source edits."""
from pathlib import Path
import shutil, subprocess, sys, tempfile
ROOT=Path(__file__).resolve().parents[2]
MUTATIONS=[
 ('omit-rtt-age','scale(now-a.sent,million-policy_.drift_ppm,true,age)','scale(now-a.received,million-policy_.drift_ppm,true,age)','Expired'),
 ('round-upper-down','if(upward&&numerator%denominator)++tail;','(void)upward;','arbitrary-precision oracle mismatch'),
 ('replay-refresh','if(!boot_ready_||quarantined_||!pending_.ticket_||permit.owner_!=this||permit.ticket_!=pending_.ticket_)return TimeAnchorError::StaleTicket;','if(!boot_ready_||quarantined_||!pending_.ticket_||permit.owner_!=this||permit.ticket_!=pending_.ticket_){if(has_anchor_){int64_t replay_now{};if(clock_.read_us(replay_now))anchor_.sent=replay_now;}return TimeAnchorError::StaleTicket;}','Expired')]
for name,before,after,expected in MUTATIONS:
 with tempfile.TemporaryDirectory(prefix='time-anchor-red-') as temp:
  target=Path(temp)/'idf'
  for relative in ['main/time_anchor_core.h','main/time_anchor_core.cpp','tests/time_anchor_host/core_native.cpp','tests/time_anchor_host/run_oracle.py']:
   dest=target/relative;dest.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(ROOT/relative,dest)
  source=target/'main/time_anchor_core.cpp';text=source.read_text();assert text.count(before)==1;source.write_text(text.replace(before,after))
  result=subprocess.run([sys.executable,str(target/'tests/time_anchor_host/run_oracle.py')],text=True,capture_output=True)
  combined=result.stdout+result.stderr
  assert result.returncode!=0 and expected in combined,f'{name}: regression was not detected: {combined}'
  print(f'PASS counterfactual RED detected {name} exit={result.returncode}\n{combined}',flush=True)
