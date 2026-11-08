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
 const f=await fixture({reply:()=>new Response(null,{status:503})});try{
  const observed=f.observed(),response=await f.request();if(response.status!==204)throw Error('Valid schedule refused '+response.status);const state=await observed;
  if(f.calls.length!==1||state.scope!==scope||!Number.isSafeInteger(state.alarm))throw Error('Failed delivery lost persisted retry '+JSON.stringify(state));
  await f.mf.unsafeEvictDurableObject('wake-proof','ProbeWake',{name:scope});
  const restored=await f.stub(scope).inspect();if(restored.alarm!==state.alarm||restored.scope!==scope)throw Error('Eviction changed persisted retry');
 }finally{await f.close();}
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
  const first=f.observed(),response=await f.request();if(response.status!==204)throw Error('Initial request status '+response.status);await entered.promise;
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
