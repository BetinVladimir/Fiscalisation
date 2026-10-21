from pathlib import Path,PurePosixPath
import json,hashlib,copy,argparse
parser=argparse.ArgumentParser();parser.add_argument('--repo',type=Path,default=Path(__file__).resolve().parents[2]);parser.add_argument('--output',type=Path,required=True);args=parser.parse_args();r=args.repo.resolve();out=args.output.resolve();out.mkdir(parents=True,exist_ok=True)
locks={name:json.loads((r/f'minipos/{name}/package-lock.json').read_text())['packages'] for name in ['miniposweb','BeeMiniPOS']}
root={'cbor-x':('miniposweb','1.6.4'),'@noble/ciphers':('BeeMiniPOS','2.3.0'),'@noble/curves':('BeeMiniPOS','2.3.0'),'@noble/hashes':('BeeMiniPOS','2.3.0'),'vite':('miniposweb','6.1.0'),'vitest':('miniposweb','3.0.7')}
package={'name':'@beeloy/fiscal-transport-test','version':'0.0.0','private':True,'type':'module','engines':{'node':'22.14.0','npm':'10.9.2'},'packageManager':'npm@10.9.2','devDependencies':{n:v for n,(_,v) in sorted(root.items())}}
selected={};origins={};todo=[]
def add(lock,path):
 v=locks[lock][path]
 if path in selected:
  if (selected[path]['version'],selected[path].get('integrity'))!=(v['version'],v.get('integrity')):raise RuntimeError('conflicting app lock entries: '+path)
  return
 if not v.get('resolved','').startswith('https://registry.npmjs.org/') or not v.get('integrity','').startswith('sha512-'):raise RuntimeError('unpinned nonregistry entry: '+path)
 selected[path]=copy.deepcopy(v);origins[path]={'source':f'minipos/{lock}/package-lock.json','version':v['version'],'integrity':v['integrity'],'entry_sha256':hashlib.sha256(json.dumps(v,sort_keys=True,separators=(',',':')).encode()).hexdigest()};todo.append((lock,path))
def resolve(lock,path,dep):
 current=PurePosixPath(path)
 while str(current)!='.':
  candidate=str(current/'node_modules'/dep)
  if candidate in locks[lock]:return candidate
  current=current.parent
 candidate='node_modules/'+dep
 if candidate in locks[lock]:return candidate
 raise RuntimeError(f'unresolved {lock}:{path} -> {dep}')
for name,(lock,version) in root.items():
 path='node_modules/'+name
 assert locks[lock][path]['version']==version
 add(lock,path)
while todo:
 lock,path=todo.pop();v=locks[lock][path]
 deps={**v.get('dependencies',{}),**v.get('optionalDependencies',{})}
 deps.update({n:version for n,version in v.get('peerDependencies',{}).items() if not v.get('peerDependenciesMeta',{}).get(n,{}).get('optional',False)})
 for dep in deps:add(lock,resolve(lock,path,dep))
# Keep registry vendor metadata untouched. Root install role is only dev deps.
lock={'name':package['name'],'version':package['version'],'lockfileVersion':3,'requires':True,'packages':{'':{k:package[k] for k in ['name','version','devDependencies','engines']},**dict(sorted(selected.items()))}}
for name,obj in [('package.json',package),('package-lock.json',lock),('LOCK_ORIGINS.json',{'app_lock_sha256':{f'minipos/{name}/package-lock.json':hashlib.sha256((r/f'minipos/{name}/package-lock.json').read_bytes()).hexdigest() for name in locks},'entries':origins})]:(out/name).write_text(json.dumps(obj,indent=2)+'\n')
print('registry lock entries',len(selected),'lock bytes',(out/'package-lock.json').stat().st_size)
print('nested entries',[p for p in selected if p.count('node_modules')>1])
print('package sha',hashlib.sha256((out/'package.json').read_bytes()).hexdigest());print('lock sha',hashlib.sha256((out/'package-lock.json').read_bytes()).hexdigest())
