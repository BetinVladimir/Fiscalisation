import {readFileSync,readdirSync,lstatSync} from 'node:fs';
import {isAbsolute,join,relative,resolve} from 'node:path';
import {createHash} from 'node:crypto';

/** Artifact identity/integrity guard, not production workflow acceptance. */
export function validatePresentationBuild(directory:string|undefined):string {
 if(!directory||!isAbsolute(directory))throw Error('PRESENTATION_ABSOLUTE_EXPORT_REQUIRED');
 const root=resolve(directory),name='presentation-build-manifest.json';
 const manifest=JSON.parse(readFileSync(join(root,name),'utf8'));
 if(manifest.schema_version!==1||manifest.kind!=='SYNTHETIC_DEVICE_DATA_PRESENTATION'||manifest.entrypoint!=='./e2e/device-data-presentation-host.tsx'||manifest.production_entrypoint_changed!==false)
  throw Error('PRESENTATION_HOST_IDENTITY_INVALID');
 if(!manifest.source_sha256||typeof manifest.source_sha256['e2e/device-data-presentation-host.tsx']!=='string'||!manifest.shared_source_sha256)
  throw Error('PRESENTATION_SOURCE_INVENTORY_REQUIRED');
 const expected=manifest.export_sha256;
 if(!expected||typeof expected!=='object'||Array.isArray(expected)||!expected['index.html'])throw Error('PRESENTATION_EXPORT_INVENTORY_REQUIRED');
 const actual:Record<string,string>={};
 const visit=(folder:string)=>{for(const item of readdirSync(folder)){
  const file=join(folder,item),stat=lstatSync(file);
  if(stat.isSymbolicLink())throw Error('PRESENTATION_SYMLINK_REJECTED');
  if(stat.isDirectory())visit(file);
  else if(stat.isFile()){const key=relative(root,file).split('\\').join('/');if(key!==name)actual[key]=createHash('sha256').update(readFileSync(file)).digest('hex');}
  else throw Error('PRESENTATION_SPECIAL_FILE_REJECTED');
 }};visit(root);
 if(JSON.stringify(Object.keys(actual).sort())!==JSON.stringify(Object.keys(expected).sort())||Object.entries(actual).some(([key,value])=>expected[key]!==value))throw Error('PRESENTATION_EXPORT_HASH_MISMATCH');
 return root;
}
