#!/usr/bin/env python3
"""Fail closed if isolated transport dependencies drift from reviewed app locks."""
from pathlib import Path, PurePosixPath
import argparse, hashlib, json
ROOTS={'cbor-x':'1.6.4','@noble/ciphers':'2.3.0','@noble/curves':'2.3.0','@noble/hashes':'2.3.0','vite':'6.1.0','vitest':'3.0.7'}
def sha(path):return hashlib.sha256(Path(path).read_bytes()).hexdigest()
def canonical_sha(value):return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()
def resolve(packages,path,name):
 p=PurePosixPath(path)
 while str(p)!='.':
  candidate=str(p/'node_modules'/name)
  if candidate in packages:return candidate
  p=p.parent
 candidate='node_modules/'+name
 if candidate not in packages:raise ValueError(f'unresolved locked dependency {path} -> {name}')
 return candidate

def check(repo,package):
 repo,package=Path(repo),Path(package)
 manifest=json.loads((package/'package.json').read_text());lock=json.loads((package/'package-lock.json').read_text());origins=json.loads((package/'LOCK_ORIGINS.json').read_text())
 if set(manifest)!={'name','version','private','type','engines','packageManager','devDependencies'} or manifest.get('private') is not True:raise ValueError('unexpected transport manifest fields')
 if manifest.get('devDependencies')!=ROOTS or manifest.get('dependencies') or manifest.get('scripts') or manifest.get('type')!='module':raise ValueError('transport-only manifest changed')
 for app,names in [('BeeMiniPOS',['cbor-x','@noble/ciphers','@noble/curves','@noble/hashes']),('miniposweb',['cbor-x','vite','vitest'])]:
  current=json.loads((repo/f'minipos/{app}/package.json').read_text());declared={**current.get('dependencies',{}),**current.get('devDependencies',{})}
  if any(declared.get(name)!=ROOTS[name] for name in names):raise ValueError('actual app direct pin changed: '+app)
 if manifest.get('engines')!={'node':'22.14.0','npm':'10.9.2'} or manifest.get('packageManager')!='npm@10.9.2':raise ValueError('toolchain pin changed')
 if lock.get('name')!=manifest.get('name') or lock.get('version')!=manifest.get('version'):raise ValueError('outer lock identity mismatch')
 if lock.get('lockfileVersion')!=3:raise ValueError('wrong lock format')
 packages=lock['packages'];root=packages['']
 if set(root)!={'name','version','devDependencies','engines'}:raise ValueError('unexpected lock root fields')
 if root.get('devDependencies')!=ROOTS or root.get('name')!=manifest.get('name') or root.get('version')!=manifest.get('version') or root.get('engines')!=manifest.get('engines'):raise ValueError('lock root mismatch')
 app={}
 for source,digest in origins['app_lock_sha256'].items():
  if source not in ['minipos/BeeMiniPOS/package-lock.json','minipos/miniposweb/package-lock.json'] or sha(repo/source)!=digest:raise ValueError('app lock source hash mismatch: '+source)
  app[source]=json.loads((repo/source).read_text())['packages']
 if set(packages)-{''}!=set(origins['entries']):raise ValueError('lock/origin entry set mismatch')
 for path,origin in origins['entries'].items():
  entry=packages[path];source=origin['source']
  if source not in app or path not in app[source]:raise ValueError('entry lacks actual app origin')
  actual=app[source][path]
  if entry!=actual or canonical_sha(actual)!=origin['entry_sha256'] or entry.get('version')!=origin['version'] or entry.get('integrity')!=origin['integrity']:raise ValueError('app registry entry mismatch: '+path)
  if not entry.get('resolved','').startswith('https://registry.npmjs.org/') or not entry.get('integrity','').startswith('sha512-') or entry.get('link'):raise ValueError('nonregistry/unlocked entry: '+path)
 reachable=set();pending=[resolve(packages,'',name) for name in ROOTS]
 while pending:
  path=pending.pop()
  if path in reachable:continue
  reachable.add(path);entry=packages[path]
  deps={**entry.get('dependencies',{}),**entry.get('optionalDependencies',{})}
  deps.update({name:version for name,version in entry.get('peerDependencies',{}).items() if not entry.get('peerDependenciesMeta',{}).get(name,{}).get('optional',False)})
  pending.extend(resolve(packages,path,name) for name in deps)
 if reachable!=set(packages)-{''}:raise ValueError('unreachable or missing dependency lock entries')
 for name,version in ROOTS.items():
  if packages['node_modules/'+name]['version']!=version:raise ValueError('direct pin mismatch')
 return {'registry_entries':len(reachable),'package_sha256':sha(package/'package.json'),'lock_sha256':sha(package/'package-lock.json'),'origins_sha256':sha(package/'LOCK_ORIGINS.json')}
if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('--repo',type=Path,default=Path(__file__).resolve().parents[2]);parser.add_argument('--package',type=Path,default=Path(__file__).resolve().parent);args=parser.parse_args();print(json.dumps(check(args.repo,args.package),indent=2))
