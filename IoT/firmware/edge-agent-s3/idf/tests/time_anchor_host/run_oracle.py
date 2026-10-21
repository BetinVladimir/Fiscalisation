#!/usr/bin/env python3
"""Native isolated core vs independent arbitrary-precision Fraction oracle.
No production verifier, entropy, HIL, UTC health or provider claim.
"""
from fractions import Fraction
from pathlib import Path
import os, random, subprocess, tempfile
MAX=(1<<63)-1
M=1000000
MIN=1700000000000
ROOT=Path(__file__).resolve().parents[2]
def ceil(v):return -(-v.numerator//v.denominator)
def checked(v):return 0<=v<=MAX
def oracle(v):
 t,e,m0,m1,m2,drift,hold,width=v
 def failure(a,b='Unavailable'):return f'{a} {b} 0 0'
 if not(0<hold<=3600000 and 0<width<=3600000 and 0<=drift<=100000):return failure('InvalidPolicy')
 if m0<0 or m1<m0:return failure('ClockFault')
 if t<MIN or e<0 or e>250:return failure('InvalidSample')
 if not checked(t*1000):return failure('Arithmetic')
 r=m1-m0
 rate=Fraction(M,M-drift)
 rhi=ceil(r*rate)
 if not checked(rhi):return failure('Arithmetic')
 if rhi>5000000:return failure('RoundTripExceeded')
 lo=t*1000-e*1000
 hi=t*1000+e*1000+rhi
 if not checked(hi):return failure('Arithmetic')
 if rhi>=hold*1000:return failure('Expired')
 if hi-lo>width*1000:return failure('WidthExceeded')
 if m2<m1:return failure('Ok','ClockFault')
 age=ceil((m2-m0)*rate)
 if not checked(age):return failure('Ok','Arithmetic')
 if age>=hold*1000:return failure('Ok','Expired')
 d=m2-m1
 lower=lo+int(d*Fraction(M,M+drift))
 upper=hi+ceil(d*rate)
 if not checked(lower) or not checked(upper):return failure('Ok','Arithmetic')
 if upper-lower>width*1000:return failure('Ok','WidthExceeded')
 return f'Ok Ok {lower} {upper}'
def vectors():
 values=[(1792555200000,1,0,5000000,30000000,0,30000,6000),
 (1792555200000,0,0,1,2,1,30000,6000),
 (1792555200000,0,0,5000000,5000000,1,30000,6000),
 (1792555200000,0,-1,0,0,0,30000,6000),
 (1792555200000,0,2,1,2,0,30000,6000),
 (1792555200000,0,0,1,0,0,30000,6000),
 (1792555200000,0,0,0,0,-1,30000,6000),
 (1792555200000,0,0,0,0,100001,30000,6000),
 (1792555200000,0,0,0,0,0,0,6000),
 (1792555200000,0,0,0,0,0,30000,0),
 (1792555200000,0,0,0,0,0,3600001,6000),
 (9007199254740993,0,0,0,0,0,30000,6000),
 (MAX//1000,0,0,0,0,0,30000,6000),
 (MAX//1000,250,0,1,2,1000,30000,6000),
 (MAX//1000+1,0,0,0,0,0,30000,6000),
 (MAX,0,0,0,0,0,30000,6000),
 (MIN-1,0,0,0,0,0,30000,6000),
 (MIN,0,0,0,MAX,100000,30000,6000),
 (1792555200000,1,MAX-10,MAX-9,MAX,1000,30000,6000)]
 for drift in [0,1,333,1000,99999,100000]:
  for r in [0,1,2,999,1000,5000000,5000001]:
   for now in [r,r+1,29999999,30000000,30000001]:
    values.append((1792555200000,1,0,r,max(now,r),drift,30000,6000))
 rng=random.Random(20261021)
 for _ in range(5000):
  t=rng.choice([1792555200000,9007199254740993,MAX//1000-rng.randrange(10000),MAX//1000+1])
  m0=rng.randrange(1000000000);r=rng.randrange(6000000);d=rng.randrange(40000000)
  values.append((t,rng.randrange(-1,252),m0,m0+r,m0+r+d,rng.choice([0,1,13,333,1000,99999,100000]),rng.choice([1,5000,30000,60000]),rng.choice([1,100,6000,60000])))
 return values

def main():
 cxx=os.environ.get('CXX','c++')
 with tempfile.TemporaryDirectory(prefix='time-anchor-host-') as temp:
  p=Path(temp);binary=p/'core'
  args=[cxx,'-std=c++17','-Wall','-Wextra','-Werror','-pthread','-DBEEFISCAL_TIME_ANCHOR_HOST_TEST','-I',str(ROOT/'main'),str(ROOT/'main/time_anchor_core.cpp'),str(ROOT/'tests/time_anchor_host/core_native.cpp'),'-o',str(binary)]
  subprocess.run(args,check=True)
  subprocess.run([str(binary)],check=True)
  cases=vectors();wire=''.join(' '.join(map(str,v))+'\n' for v in cases)
  lines=subprocess.run([str(binary),'--math'],input=wire,text=True,capture_output=True,check=True).stdout.splitlines()
  assert len(lines)==len(cases)
  for i,(v,result) in enumerate(zip(cases,lines)):
   expected=oracle(v)
   assert result==expected,f'arbitrary-precision oracle mismatch case={i} input={v} actual={result!r} expected={expected!r}'
  from collections import Counter
  print(f'PASS independent Fraction oracle {len(cases)} checked-int64/outward-rounding/RTT-holdover vectors outcomes={dict(Counter(x.split()[0]+"/"+x.split()[1] for x in lines))}')
  production=p/'production.cpp';production.write_text('#include "time_anchor_core.h"\n#include <type_traits>\nusing namespace beefiscal::idf;static_assert(!std::is_constructible<VerifiedTimeAnchorPermit,const TimeAnchorRequest&,const TimeAnchorClaims&>::value);int main(){return 0;}\n')
  subprocess.run([cxx,'-std=c++17','-Wall','-Wextra','-Werror','-I',str(ROOT/'main'),str(ROOT/'main/time_anchor_core.cpp'),str(production),'-o',str(p/'production')],check=True)
  forbidden=p/'forbidden.cpp';forbidden.write_text('#include "time_anchor_core.h"\nint main(){beefiscal::idf::TimeAnchorHostPermitFactory factory;}\n')
  denied=subprocess.run([cxx,'-std=c++17','-I',str(ROOT/'main'),'-c',str(forbidden),'-o',str(p/'forbidden.o')],capture_output=True,text=True)
  assert denied.returncode!=0 and 'TimeAnchorHostPermitFactory' in denied.stderr,'host factory leaked into production compile'
  print('PASS production compile has no raw permit constructor or host permit factory')
if __name__=='__main__':main()
