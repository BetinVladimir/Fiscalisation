#!/usr/bin/env node
// Creates upload keys locally, never prints passwords and never overwrites a key.
const fs = require('node:fs');
const path = require('node:path');
const { randomBytes } = require('node:crypto');
const { spawnSync } = require('node:child_process');
const root = path.resolve(__dirname, '..');
const app = require('./app.json');
const file = path.join(__dirname, 'credentials.local.json');
const data = fs.existsSync(file) ? JSON.parse(fs.readFileSync(file,'utf8')) : {};
for (const variant of ['production','demo']) {
  const target = path.join(__dirname, `${variant}.jks`);
  if (data[variant]) { if (!fs.existsSync(target)) throw new Error(`Missing saved key ${target}; restore backup`); continue; }
  if (fs.existsSync(target)) throw new Error(`Refusing to replace existing ${target}`);
  const password = randomBytes(32).toString('base64url');
  const result = spawnSync('keytool',['-genkeypair','-v','-storetype','JKS','-keystore',target,'-alias','upload','-keyalg','RSA','-keysize','2048','-validity','10000','-dname',`CN=${app.app} ${variant}, OU=Mobile, O=Beeloy`,'-storepass:env','BEELOY_KEY_PASSWORD','-keypass:env','BEELOY_KEY_PASSWORD'],{ env:{...process.env,BEELOY_KEY_PASSWORD:password},encoding:'utf8' });
  if(result.status!==0) throw new Error(`keytool failed: ${result.stderr}`);
  fs.chmodSync(target,0o600);
  data[variant]={android:{keystore:{keystorePath:`store/${variant}.jks`,keystorePassword:password,keyAlias:'upload',keyPassword:password}}};
  fs.writeFileSync(file,JSON.stringify(data,null,2)+'\n',{mode:0o600});
}
console.log(`${app.app}: Android upload keys ready; back up store/*.jks and store/credentials.local.json securely.`);
