#!/usr/bin/env node
// Generate native projects in an isolated directory; never overwrite local ios/android.
const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const root = path.resolve(__dirname, '..');
const manifest = require('./app.json');
const eas = require('../eas.json');
const variants = process.argv.includes('--demo') ? ['demo'] : process.argv.includes('--production') ? ['production'] : ['production','demo'];
function run(cmd,args,opts={}) {
  const r=cp.spawnSync(cmd,args,{cwd:root,stdio:'inherit',...opts});
  if(r.error) throw r.error;
  if(r.status!==0) throw new Error(`${cmd} failed (${r.status})`);
}
for(const variant of variants) {
 const env={...Object.fromEntries(Object.entries(process.env).filter(([k])=>!k.startsWith('EXPO_PUBLIC_'))),...eas.build[variant].env,...manifest.publicEnv?.[variant],CI:'1',EXPO_NO_DOTENV:'1'};
 const dir=path.join(root,'.store-build',`native-${variant}`);
 fs.mkdirSync(dir,{recursive:true});
 run('rsync',['-a','--delete','--exclude=node_modules','--exclude=android','--exclude=ios','--exclude=.store-build','--exclude=.git','--exclude=.expo','--exclude=dist*','--exclude=.env*','--exclude=*.jks','--exclude=credentials*.json',root+'/',dir+'/']);
 const modules=path.join(dir,'node_modules');
 if(!fs.existsSync(modules)) fs.symlinkSync(path.join(root,'node_modules'),modules,'dir');
 run(process.execPath,[require.resolve('expo/bin/cli'),'prebuild','--no-install','--platform','all'],{cwd:dir,env});
 const id=`org.beeloy.${manifest.app}.${variant==='demo'?'demo':'prod'}`;
 const gradle=fs.readFileSync(path.join(dir,'android/app/build.gradle'),'utf8');
 if(!gradle.includes(id)) throw new Error(`Generated Android package is not ${id}`);
 const iosNames=fs.readdirSync(path.join(dir,'ios')).filter(n=>n.endsWith('.xcodeproj'));
 if(!iosNames.some(n=>fs.readFileSync(path.join(dir,'ios',n,'project.pbxproj'),'utf8').includes(id))) throw new Error(`Generated iOS bundle is not ${id}`);
 console.log(`PASS native generation ${id}`);
}
