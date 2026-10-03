import {spawnSync,spawn} from 'node:child_process';
import {randomBytes} from 'node:crypto';
import {mkdirSync,writeFileSync} from 'node:fs';
const cwd='/tmp/iolink-server-compose';
const artifact=cwd+'/.omo/evidence/server-compose';
mkdirSync(artifact,{recursive:true});
const env={...process.env,IOLINK_PG_PASSWORD:randomBytes(32).toString('hex'),IOLINK_SECRET_KEY:randomBytes(32).toString('hex'),IOLINK_HTTP_PORT:'18089',IOLINK_MQTT_PORT:'11889',IOLINK_DEV_PG_PORT:'55449',IOLINKD_IMAGE:'iolinkd:local-dev'};
const base=['compose','-p','iolink-compose-verify','-f','deploy/docker-compose.yaml'];
const dev=[...base,'-f','deploy/docker-compose.dev.yaml'];
const receipts=[];
async function run(name,args,input,expected=0){
 const started=Date.now();let output='';
 const code=await new Promise((resolve)=>{const p=spawn('docker',args,{cwd,env});p.stdout.on('data',d=>output+=d);p.stderr.on('data',d=>output+=d);p.on('close',resolve);if(input)p.stdin.write(input);p.stdin.end();});
 for(const secret of [env.IOLINK_PG_PASSWORD,env.IOLINK_SECRET_KEY])output=output.replaceAll(secret,'<redacted>');
 writeFileSync(artifact+'/'+name+'.log',output);receipts.push({name,args,code,expected,seconds:(Date.now()-started)/1000});console.log(JSON.stringify(receipts.at(-1)));if(code!==expected)throw Error(name+' failed');return output;
}
await run('config-server',[...base,'config','--quiet']);
await run('config-development',[...dev,'config','--quiet']);
const model=JSON.parse(await run('model-development',[...dev,'config','--format','json']));
writeFileSync(artifact+'/model-development.log',JSON.stringify({image:model.services.iolinkd.image,pull_policy:model.services.iolinkd.pull_policy,build:model.services.iolinkd.build,ports:model.services.db.ports},null,2));
await run('source-build',[...dev,'build','iolinkd']);
await run('database-up',[...dev,'up','-d','--wait','db']);
await run('migrations',[...base,'run','--rm','--pull','never','--no-deps','iolinkd','migrate','up']);
const password=randomBytes(20).toString('hex');
await run('admin-init',[...base,'run','--rm','--pull','never','--no-deps','-T','iolinkd','admin','init','operator'],password+'\n');
await run('application-up',[...base,'up','-d','--pull','never','--wait','iolinkd']);
for(const path of ['/readyz','/healthz','/']){const r=await fetch('http://127.0.0.1:18089'+path);if(r.status!==200)throw Error(path+' '+r.status);console.log(path+' HTTP200');}
const login=async()=>{const r=await fetch('http://127.0.0.1:18089/admin/v1/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:'operator',password})});if(r.status!==200)throw Error('login '+r.status);const body=await r.json();if(!body.token)throw Error('missing token');};
await login();
await run('restart',[...base,'restart','iolinkd']);
await run('restart-ready',[...base,'up','-d','--pull','never','--wait','iolinkd']);
await login();
await run('migration-repeat',[...base,'run','--rm','--pull','never','--no-deps','iolinkd','migrate','up']);
await run('admin-repeat-rejected',[...base,'run','--rm','--pull','never','--no-deps','-T','iolinkd','admin','init','other'],password+'\n',1);
await run('containers',[...base,'ps']);
writeFileSync(artifact+'/verification.json',JSON.stringify({source:spawnSync('git',['rev-parse','HEAD'],{cwd,encoding:'utf8'}).stdout.trim(),date:'2026-10-03',docker:'29.8.2',compose:'5.5.1',receipts,http:'healthz/readyz/SPA200; login200; restart login200; no credentials recorded'},null,2));
console.log('COMPOSE VERIFICATION PASSED');
