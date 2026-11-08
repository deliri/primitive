import {test as nativeTest} from 'node:test';
function test(name,body,timeout=20000){return nativeTest(name,{timeout},body);}
import {readFile,mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
const {Miniflare}=await import(process.env.PRIMITIVE_MINIFLARE_MODULE||'miniflare');
const source=await readFile(process.env.PRIMITIVE_DURABLE_WAKE_MODULE||new URL('./durable_wake.mjs',import.meta.url),'utf8');
const token='s'.repeat(64),targetToken='t'.repeat(64),scope='owned-scope';
const complete=()=>Response.json({data:{processed:0,succeeded:0,failed:0}});
function deferred(){let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b;});return {promise,resolve,reject};}
async function fixture({reply=complete,directory,bindings={}}={}){
 const ownDirectory=directory===undefined,root=directory||await mkdtemp(join(tmpdir(),'primitive-durable-wake-'));
 const calls=[],observations=[],waiters=[];
 const entry=`export {default} from './durable.mjs';import {DurableWake} from './durable.mjs';export class ProbeWake extends DurableWake {async inspect(){return {scope:await this.ctx.storage.get('scope'),alarm:await this.ctx.storage.getAlarm()};}async alarm(){await super.alarm();await this.env.OBSERVER.fetch('http://observer/',{method:'POST',body:JSON.stringify(await this.inspect())});}}`;
 const values={SCHEDULE_TOKEN:token,TARGET_TOKEN:targetToken,TARGET_URL:'https://backend.example.test/worker/relay',TARGET_SCOPE_HEADER:'X-Blink-Worker-Tenant',...bindings};
 const mf=new Miniflare({resourcePersistencePath:root,workers:[{config:{
  name:'wake-proof',compatibilityDate:'2026-10-07',
  manifest:{mainModule:'entry.mjs',modules:{'entry.mjs':{type:'esm',contents:entry},'durable.mjs':{type:'esm',contents:source}}},
  exports:{ProbeWake:{type:'durable-object',storage:'sqlite'}},
  env:{...Object.fromEntries(Object.entries(values).map(([key,value])=>[key,{type:'text',value}])),
   WAKES:{type:'durable-object',worker:'wake-proof',exportName:'ProbeWake'},
   OBSERVER:{type:'fetcher',handler:async request=>{const state=await request.json();observations.push(state);waiters.shift()?.resolve(state);return new Response(null,{status:204});}}}
 },dev:{outboundService:{type:'fetcher',handler:async request=>{calls.push({url:request.url,method:request.method,authorization:request.headers.get('Authorization'),scope:request.headers.get('X-Blink-Worker-Tenant'),body:await request.text()});return reply(request,calls.length);}}}}]});
 await mf.ready;
 const namespace=await mf.getDurableObjectNamespace('WAKES');
 return {mf,root,calls,observations,stub:key=>namespace.getByName(key),
  request:(path='/wake/'+scope,options={})=>mf.dispatchFetch('https://wake.example.test'+path,{method:'POST',headers:{Authorization:'Bearer '+token},...options}),
  observed:()=>{const wait=deferred();waiters.push(wait);const timeout=setTimeout(()=>wait.reject(Error('Native alarm completion not observed')),10000);const result=wait.promise.finally(()=>clearTimeout(timeout));result.catch(()=>{});return result;},
  async close(){await mf.dispose();if(ownDirectory)await rm(root,{recursive:true,force:true});}};
}

test('durable wake: native alarm posts once to the fixed authenticated destination and retires',async()=>{
 const f=await fixture();try{
  const observed=f.observed(),response=await f.request(),state=await observed;
  if(response.status!==204||f.calls.length!==1||state.scope!==undefined||state.alarm!==null)throw Error('Accepted wake did not deliver then retire '+JSON.stringify({status:response.status,calls:f.calls.length,state}));
  const call=f.calls[0];if(call.url!=='https://backend.example.test/worker/relay'||call.method!=='POST'||call.authorization!=='Bearer '+targetToken||call.scope!==scope||call.body!=='')throw Error('Native destination authority changed');
 }finally{await f.close();}
},20000);

test('durable wake: backend failure retains its native alarm beyond exception retries',async()=>{
 const root=await mkdtemp(join(tmpdir(),'primitive-durable-retry-'));let f;try{
  f=await fixture({directory:root,reply:()=>new Response(null,{status:503})});
  const observed=f.observed(),response=await f.request();if(response.status!==204)throw Error('Valid schedule refused '+response.status);const state=await observed;
  if(f.calls.length!==1||state.scope!==scope||!Number.isSafeInteger(state.alarm))throw Error('Failed delivery lost persisted retry '+JSON.stringify(state));
  await f.close();f=await fixture({directory:root});
  const restored=await f.stub(scope).inspect();if(restored.alarm!==state.alarm||restored.scope!==scope||f.calls.length!==0)throw Error('Runtime restart changed persisted retry');
 }finally{if(f)await f.close();await rm(root,{recursive:true,force:true});}
},20000);

test('durable wake: future canonical deadline survives a whole runtime restart',async()=>{
 const root=await mkdtemp(join(tmpdir(),'primitive-durable-restart-'));let f;
 const due='1795000000000000001',want=1795000000001;
 try{
  f=await fixture({directory:root,reply:()=>Response.json({data:{processed:1,succeeded:1,failed:0,next_wake_at:due}})});
  const observed=f.observed();await f.request();const state=await observed;
  if(state.scope!==scope||state.alarm!==want)throw Error('Canonical nanoseconds were not rounded up to the alarm '+JSON.stringify(state));
  await f.close();f=await fixture({directory:root});
  const restored=await f.stub(scope).inspect();if(restored.scope!==scope||restored.alarm!==want||f.calls.length!==0)throw Error('Restart lost deadline or manufactured a delivery '+JSON.stringify(restored));
 }finally{if(f)await f.close();await rm(root,{recursive:true,force:true});}
},30000);

test('durable wake: a commit arriving during delivery cannot be erased by the old reply',async()=>{
 const entered=deferred(),release=deferred();const f=await fixture({reply:async(_,count)=>{if(count===1){entered.resolve();await release.promise;}return complete();}});
 try{
  const first=f.observed(),response=await f.request();if(response.status!==204)throw Error('Initial request status '+response.status);await Promise.race([entered.promise,first.then(()=>{throw Error('Alarm completed without entering the backend');})]);
  if((await f.request()).status!==204)throw Error('Concurrent commit was refused');
  const second=f.observed();release.resolve();const a=await first,b=await second;
  if(f.calls.length!==2||a.scope!==scope||a.alarm===null||b.scope!==undefined||b.alarm!==null)throw Error('Concurrent commit was lost '+JSON.stringify({calls:f.calls.length,a,b}));
 }finally{release.resolve();await f.close();}
},20000);

for(const scenario of [
 {name:'missing bearer has no object or backend effect',options:{headers:{}},want:403},
 {name:'foreign bearer has no object or backend effect',options:{headers:{Authorization:'Bearer '+'f'.repeat(64)}},want:403},
 {name:'GET cannot schedule',options:{method:'GET'},want:405},
 {name:'query cannot carry a second routing instruction',path:'/wake/'+scope+'?target=https://foreign.test',want:400},
 {name:'encoded path cannot alias a scope',path:'/wake/%6fwned-scope',want:400},
 {name:'sixty-four-byte scope exceeds its bound',path:'/wake/'+'a'.repeat(64),want:400},
 {name:'body cannot supply another target',options:{body:'{"target":"https://foreign.test"}'},want:400},
 {name:'unconfigured schedule authority refuses',bindings:{SCHEDULE_TOKEN:''},want:403},
 {name:'HTTP target refuses before any alarm',bindings:{TARGET_URL:'http://backend.example.test/worker/relay'},want:503},
 {name:'target credentials cannot become URL authority',bindings:{TARGET_URL:'https://secret@backend.example.test/worker/relay'},want:503},
])test('durable wake admission: '+scenario.name,async()=>{
 const f=await fixture({bindings:scenario.bindings});try{
  const response=await f.request(scenario.path,scenario.options);
  const ids=await f.mf.listDurableObjectIds('WAKES');
  if(response.status!==scenario.want||f.calls.length!==0||ids.length!==0)throw Error('Rejected wake escaped an effect '+JSON.stringify({status:response.status,want:scenario.want,calls:f.calls.length,objects:ids.length}));
 }finally{await f.close();}
},20000);

const validResults=[
 {name:'empty drain retires without a successor',result:{processed:0,succeeded:0,failed:0}},
 {name:'completed delivery retires',result:{processed:1,succeeded:1,failed:0}},
 {name:'terminal refusal retires',result:{processed:1,succeeded:0,failed:1}},
 {name:'deferred claim does not invent success',result:{processed:1,succeeded:0,failed:0}},
 {name:'mixed outcomes conserve counters',result:{processed:3,succeeded:1,failed:1}},
 {name:'largest exact integer counters remain exact',result:{processed:Number.MAX_SAFE_INTEGER,succeeded:Number.MAX_SAFE_INTEGER,failed:0}},
 {name:'exact millisecond deadline',result:{processed:0,succeeded:0,failed:0,next_wake_at:'2210000000000000000'},want:2210000000000},
 {name:'one nanosecond beyond millisecond rounds up',result:{processed:0,succeeded:0,failed:0,next_wake_at:'2210000000000000001'},want:2210000000001},
 {name:'last nanosecond in millisecond rounds up',result:{processed:0,succeeded:0,failed:0,next_wake_at:'2210000000000999999'},want:2210000000001},
 {name:'next exact millisecond',result:{processed:0,succeeded:0,failed:0,next_wake_at:'2210000000001000000'},want:2210000000001},
 {name:'largest whole millisecond representable by native signed nanoseconds',result:{processed:1,succeeded:1,failed:0,next_wake_at:'9223372036854000000'},want:9223372036854},
];
for(const scenario of validResults)test('durable wake result: '+scenario.name,async()=>{
 const f=await fixture({reply:()=>Response.json({data:scenario.result})});try{
  const observed=f.observed(),response=await f.request(),state=await observed;
  if(response.status!==204||f.calls.length!==1||state.alarm!==(scenario.want??null)||state.scope!==(scenario.want===undefined?undefined:scope))throw Error('Authoritative result changed '+JSON.stringify({scenario:scenario.name,state,calls:f.calls.length,status:response.status}));
 }finally{await f.close();}
});

const refusedResults=[
 {name:'redirect cannot move the credential',reply:()=>new Response(null,{status:302,headers:{Location:'https://foreign.test/'}})},
 {name:'successful status without an acknowledgement',reply:()=>new Response(null,{status:204})},
 {name:'JSON bytes require JSON media',reply:()=>new Response('{"data":{"processed":0,"succeeded":0,"failed":0}}')},
 {name:'empty body cannot acknowledge',reply:()=>new Response('',{headers:{'Content-Type':'application/json'}})},
 {name:'truncated JSON cannot acknowledge',reply:()=>new Response('{"data":', {headers:{'Content-Type':'application/json'}})},
 {name:'malformed UTF8 cannot acknowledge',reply:()=>new Response(new Uint8Array([0xff]),{headers:{'Content-Type':'application/json'}})},
 {name:'missing data cannot acknowledge',data:undefined},
 {name:'array data cannot acknowledge',data:[]},
 {name:'missing counter cannot acknowledge',data:{processed:0,succeeded:0}},
 {name:'wrong-type counter cannot acknowledge',data:{processed:'0',succeeded:0,failed:0}},
 {name:'negative counter cannot acknowledge',data:{processed:-1,succeeded:0,failed:0}},
 {name:'fractional counter cannot acknowledge',data:{processed:0.5,succeeded:0,failed:0}},
 {name:'unsafe integer cannot acknowledge',data:{processed:Number.MAX_SAFE_INTEGER+1,succeeded:0,failed:0}},
 {name:'success cannot exceed work',data:{processed:1,succeeded:2,failed:0}},
 {name:'terminal outcomes cannot exceed work',data:{processed:1,succeeded:1,failed:1}},
 {name:'unknown data member cannot redirect work',data:{processed:0,succeeded:0,failed:0,target:'https://foreign.test/'}},
 {name:'numeric deadline cannot lose nanoseconds',data:{processed:0,succeeded:0,failed:0,next_wake_at:2210000000000000000}},
 {name:'null deadline cannot mean no deadline',data:{processed:0,succeeded:0,failed:0,next_wake_at:null}},
 {name:'zero deadline cannot spin an alarm',data:{processed:0,succeeded:0,failed:0,next_wake_at:'0'}},
 {name:'negative deadline cannot spin an alarm',data:{processed:0,succeeded:0,failed:0,next_wake_at:'-1'}},
 {name:'leading zero deadline is not canonical',data:{processed:0,succeeded:0,failed:0,next_wake_at:'02210000000000000000'}},
 {name:'last signed nanosecond cannot round beyond native range',data:{processed:0,succeeded:0,failed:0,next_wake_at:'9223372036854775807'}},
 {name:'one nanosecond above native millisecond range refuses',data:{processed:0,succeeded:0,failed:0,next_wake_at:'9223372036854000001'}},
 {name:'instant above signed maximum refuses',data:{processed:0,succeeded:0,failed:0,next_wake_at:'9223372036854775808'}},
 {name:'oversized body stops at the streaming bound',reply:()=>new Response(' '.repeat(1025),{headers:{'Content-Type':'application/json'}})},
 {name:'network interruption retains retry',reply:()=>{throw Error('Controlled transport unavailable');}},
];
for(const scenario of refusedResults)test('durable wake response refusal: '+scenario.name,async()=>{
 const f=await fixture({reply:scenario.reply??(()=>Response.json({data:scenario.data}))});try{
  const before=Date.now(),observed=f.observed(),response=await f.request(),state=await observed,after=Date.now();
  if(response.status!==204||f.calls.length!==1||state.scope!==scope||!Number.isSafeInteger(state.alarm)||state.alarm<before+60000||state.alarm>after+60000)throw Error('Refused acknowledgement lost retry '+JSON.stringify({state,calls:f.calls.length,status:response.status}));
 }finally{await f.close();}
});

for(const width of [1023,1024])test('durable wake bounded response: '+width+' bytes accepted',async()=>{
 const body=JSON.stringify({data:{processed:0,succeeded:0,failed:0}}),f=await fixture({reply:()=>new Response(body.padEnd(width,' '),{headers:{'Content-Type':'application/json'}})});try{
  const observed=f.observed();await f.request();const state=await observed;
  if(f.calls.length!==1||state.scope!==undefined||state.alarm!==null)throw Error('Bounded valid response was refused '+JSON.stringify(state));
 }finally{await f.close();}
});
