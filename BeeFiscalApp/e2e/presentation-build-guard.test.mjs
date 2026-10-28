import test from 'node:test';import assert from 'node:assert/strict';
import {mkdtempSync,writeFileSync,rmSync,symlinkSync} from 'node:fs';import {tmpdir} from 'node:os';import {join} from 'node:path';import {createHash} from 'node:crypto';
import {validatePresentationBuild} from './presentation-build-guard.ts';
function fixture(){const root=mkdtempSync(join(tmpdir(),'presentation-guard-'));writeFileSync(join(root,'index.html'),'host');const manifest={schema_version:1,kind:'SYNTHETIC_DEVICE_DATA_PRESENTATION',entrypoint:'./e2e/device-data-presentation-host.tsx',production_entrypoint_changed:false,source_sha256:{'e2e/device-data-presentation-host.tsx':'a'.repeat(64)},shared_source_sha256:{},export_sha256:{'index.html':createHash('sha256').update('host').digest('hex')}};const save=()=>writeFileSync(join(root,'presentation-build-manifest.json'),JSON.stringify(manifest));save();return{root,manifest,save};}
test('exact synthetic export is accepted',()=>{const x=fixture();try{assert.equal(validatePresentationBuild(x.root),x.root);}finally{rmSync(x.root,{recursive:true});}});
for(const mutation of ['ordinary','changed','extra','missing','symlink','source'])test(`presentation guard rejects ${mutation}`,()=>{const x=fixture();try{
 if(mutation==='ordinary'){x.manifest.kind='APPLICATION';x.save();}
 if(mutation==='changed')writeFileSync(join(x.root,'index.html'),'other');
 if(mutation==='extra')writeFileSync(join(x.root,'extra.js'),'extra');
 if(mutation==='missing')rmSync(join(x.root,'index.html'));
 if(mutation==='symlink')symlinkSync('/etc/hosts',join(x.root,'linked'));
 if(mutation==='source'){x.manifest.source_sha256={};x.save();}
 assert.throws(()=>validatePresentationBuild(x.root));
 }finally{rmSync(x.root,{recursive:true,force:true});}});
