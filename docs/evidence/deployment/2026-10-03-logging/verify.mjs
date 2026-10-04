import {spawn,spawnSync} from 'node:child_process';
import {randomBytes,createHash} from 'node:crypto';
import {mkdirSync,writeFileSync} from 'node:fs';
import net from 'node:net';

const cwd=process.cwd(), evidence=cwd+'/.omo/evidence/logging';
mkdirSync(evidence,{recursive:true});
const project='iolink-logging-'+randomBytes(4).toString('hex');
const env={...process.env,GOFLAGS:'-buildvcs=false',IOLINK_PG_PASSWORD:randomBytes(32).toString('hex'),IOLINK_SECRET_KEY:randomBytes(32).toString('hex'),IOLINK_HTTP_PORT:'18091',IOLINK_MQTT_PORT:'11891',IOLINK_DEV_PG_PORT:'55451',IOLINK_LOG_LEVEL:'debug',IOLINK_LOG_MAX_MB:'1',IOLINK_LOG_BACKUPS:'2'};
const password=randomBytes(20).toString('hex'),deviceSecret=randomBytes(24).toString('hex');
const secrets=[env.IOLINK_PG_PASSWORD,env.IOLINK_SECRET_KEY,password,deviceSecret];
const redact=raw=>secrets.reduce((out,key)=>out.replaceAll(key,'<redacted>'),raw).trimEnd()+'\n';
const receipts=[], base=['compose','-p',project,'-f','deploy/docker-compose.yaml','-f','deploy/docker-compose.dev.yaml'];
async function command(name,bin,args,input,expected=0) {
 let out='';const start=Date.now();
 const code=await new Promise((resolve,reject)=>{const p=spawn(bin,args,{cwd,env});p.stdout.on('data',b=>out+=b);p.stderr.on('data',b=>out+=b);p.on('error',reject);p.on('close',resolve);p.stdin.end(input)});
 writeFileSync(evidence+'/'+name+'.log',redact(out));
 const row={name,command:[bin,...args],code,expected,seconds:(Date.now()-start)/1000};receipts.push(row);console.log(JSON.stringify(row));
 if(code!==expected)throw Error(name+' failed');return out;
}
const compose=(name,args,input,expected)=>command(name,'docker',[...base,...args],input,expected);
const readLogs=name=>compose(name,['exec','-T','iolinkd','cat','/var/log/iolink/iolinkd.jsonl']);
function mqttString(text){const b=Buffer.from(text);const length=Buffer.alloc(2);length.writeUInt16BE(b.length);return Buffer.concat([length,b])}
function packet(header,body){let size=body.length,bytes=[];do{let b=size%128;size=Math.floor(size/128);if(size)b|=128;bytes.push(b)}while(size);return Buffer.concat([Buffer.from([header,...bytes]),body])}
async function mqttPublish() {
 await new Promise((resolve,reject)=>{
  const socket=net.createConnection(11891,'127.0.0.1');let buf=Buffer.alloc(0),connected=false;
  const fail=e=>{socket.destroy();reject(e)};socket.setTimeout(10000,()=>fail(Error('MQTT timeout')));socket.on('error',fail);
  socket.on('connect',()=>socket.write(packet(0x10,Buffer.concat([mqttString('MQTT'),Buffer.from([4,0xc2,0,30]),mqttString('diagnostics-fixture'),mqttString('diagnostics-fixture'),mqttString(deviceSecret)]))));
  socket.on('data',b=>{buf=Buffer.concat([buf,b]);if(!connected && buf.length>=4){if(buf[0]!==0x20||buf[3]!==0)return fail(Error('MQTT auth'));buf=buf.subarray(4);connected=true;socket.write(packet(0x32,Buffer.concat([mqttString('iolink/up/diagnostics-fixture/properties'),Buffer.from([0,1]),Buffer.from('{"temperature":26,"message_id":"diagnostics-event"}')])))}if(connected && buf.length>=4 && buf[0]===0x40){socket.end(packet(0xe0,Buffer.alloc(0)));resolve()}});
 });
}
try {
 await compose('compose-config',['config','--quiet']);
 await compose('docker-build',['build','iolinkd']);
 await compose('database-up',['up','-d','--pull','never','--wait','db']);
 await command('docker-version','docker',['version','--format','{{.Server.Version}}']);
 await command('compose-version','docker',['compose','version','--short']);
 await command('database-image','docker',['image','inspect','timescale/timescaledb:latest-pg16','--format','{{json .RepoDigests}}']);
 await compose('migrate',['run','--rm','--pull','never','--no-deps','iolinkd','migrate','up']);
 await compose('database-version',['exec','-T','db','psql','-U','iolink','-d','iolink','-Atc',"SELECT version(); SELECT extversion FROM pg_extension WHERE extname='timescaledb'"]);
 await compose('admin-init',['run','--rm','--pull','never','--no-deps','-T','iolinkd','admin','init','operator'],password+'\n');
 const sql=`INSERT INTO users(id,open_id) VALUES(1000,'diagnostics-user'); INSERT INTO tenants(name) VALUES('diagnostics-tenant'); INSERT INTO tenant_memberships(tenant_id,user_id,role) SELECT id,1000,'owner' FROM tenants WHERE name='diagnostics-tenant'; INSERT INTO farms(id,owner_id,tenant_id,name) SELECT 1000,1000,id,'diagnostics-farm' FROM tenants WHERE name='diagnostics-tenant'; INSERT INTO ponds(id,farm_id,name) VALUES(1000,1000,'diagnostics-pond'); INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(1000,'diagnostics-fixture','${createHash('sha256').update(deviceSecret).digest('hex')}');`;
 await compose('fixtures',['exec','-T','db','psql','-U','iolink','-d','iolink','-v','ON_ERROR_STOP=1'],sql);
 await compose('application-up',['up','-d','--pull','never','--wait','iolinkd']);
 const request=async(path,options,expected)=>{const r=await fetch('http://127.0.0.1:18091'+path,options);if(r.status!==expected)throw Error(path+' status='+r.status);return r};
 await request('/readyz',undefined,200);
 const auth=await request('/admin/v1/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:'operator',password})},200);
 const id=auth.headers.get('x-request-id');if(!id)throw Error('request ID missing');
 await request('/api/v2/devices/path-secret/history?token=query-secret',{headers:{Authorization:'Bearer jwt-secret','X-Request-ID':'spoofed-secret'}},401);
 await mqttPublish();
 const count=await compose('telemetry-count',['exec','-T','db','psql','-U','iolink','-d','iolink','-Atc','SELECT count(*) FROM sensor_data WHERE device_no=\'diagnostics-fixture\' AND temperature=26']);if(count.trim()!=='1')throw Error('telemetry not committed');
 const before=await readLogs('persistent-json');const rows=before.trim().split('\n').map(line=>JSON.parse(line));
 if(!rows.some(x=>x.request_id===id&&x.status===200)||!rows.some(x=>x.msg==='device event persisted'))throw Error('internal event/correlation missing');
 for(const secret of [...secrets,'path-secret','query-secret','jwt-secret','spoofed-secret','diagnostics-event'])if(before.includes(secret))throw Error('sensitive value leaked');
 await compose('restart',['restart','iolinkd']);await compose('restart-wait',['up','-d','--pull','never','--wait','iolinkd']);
 const after=await readLogs('restart-json');if(!after.includes(id))throw Error('log lost after restart');
 await compose('invalid-level',['run','--rm','--pull','never','--no-deps','-e','IOLINK_LOG_LEVEL=invalid','iolinkd','serve'],undefined,1);
 await compose('invalid-path',['run','--rm','--pull','never','--no-deps','-e','IOLINK_LOG_DIR=/proc/iolink-unwritable','iolinkd','serve'],undefined,1);
 env.IOLINK_TEST_PG_DSN=`postgres://iolink:${env.IOLINK_PG_PASSWORD}@127.0.0.1:55451/iolink?sslmode=disable`;
 await command('go-verify','make',['verify']);
 await command('logging-focused','go',['test','-race','-shuffle=on','-count=1','-v','./internal/observability','./internal/operations']);
 await command('cli-focused','go',['test','-race','-shuffle=on','-count=1','-v','./cmd/iolinkd']);
 await command('web-tests','npm',['--prefix','web','run','test']);
 await command('web-typecheck','npm',['--prefix','web','run','typecheck']);
 await command('web-build','npm',['--prefix','web','run','build']);
 await command('contracts','make',['verify-contracts']);
 await command('manifests','.venv/contracts/bin/python',['scripts/check_architecture_manifests.py','--all']);
 await command('diff-check','git',['diff','--check']);
 writeFileSync(evidence+'/verification.json',JSON.stringify({source:spawnSync('git',['rev-parse','HEAD'],{cwd,encoding:'utf8'}).stdout.trim(),project,date:'2026-10-03',result:'PASS',http:'ready200/login200/auth401',mqtt:'QoS1 → PG committed → debug internal event',persistence:'request ID retained after restart',secrets:'all generated password/key/MQTT values and raw request/query excluded',receipts},null,2));
 console.log('LOGGING VERIFICATION PASSED');
} finally {
 await compose('cleanup',['down','-v','--remove-orphans']);
 await command('cleanup-files','docker',['run','--rm','-v',cwd+'/deploy/logs:/logs','alpine:3.20','sh','-c','rm -rf /logs/*']);
}
