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
async function fixture({reply=complete,directory,bindings={},emptyGuardAfterTransport=false,overflowAtActor=false}={}){
 const ownDirectory=directory===undefined,root=directory||await mkdtemp(join(tmpdir(),'primitive-durable-wake-'));
 const calls=[],observations=[],waiters=[],schedules=[];
 const ingress=emptyGuardAfterTransport?`import receiver from './durable.mjs';export default {async fetch(request,env){const headers=new Headers(request.headers);headers.set('X-Wake-Guard-Nanoseconds','');const supplied=new Request(request,{headers});if(!supplied.headers.has('X-Wake-Guard-Nanoseconds')||supplied.headers.get('X-Wake-Guard-Nanoseconds')!=='')throw Error('Empty-field fixture did not reach the receiver');return receiver.fetch(supplied,env);}};`:`export {default} from './durable.mjs';`;
 const entry=ingress+`import {DurableWake} from './durable.mjs';export class ProbeWake extends DurableWake {async schedule(scope,guard){await this.env.SCHEDULE_OBSERVER.fetch('http://schedule-observer/',{method:'POST',body:JSON.stringify({scope,guard})});return super.schedule(scope,${overflowAtActor?'9223372036855':'guard'});}async inspect(){return {scope:await this.ctx.storage.get('scope'),guard:await this.ctx.storage.get('guard_until'),alarm:await this.ctx.storage.getAlarm()};}async now(){return Date.now();}async putGuardForTest(value){await this.ctx.storage.put('guard_until',value);}async alarm(){await super.alarm();await this.env.OBSERVER.fetch('http://observer/',{method:'POST',body:JSON.stringify(await this.inspect())});}}`;
 const values={SCHEDULE_TOKEN:token,TARGET_TOKEN:targetToken,TARGET_URL:'https://backend.example.test/worker/relay',TARGET_SCOPE_HEADER:'X-Blink-Worker-Tenant',...bindings};
 const mf=new Miniflare({cf:false,telemetry:{enabled:false},resourcePersistencePath:root,resourceTmpPath:join(root,'tmp'),workers:[{config:{
  name:'wake-proof',compatibilityDate:'2026-10-07',
  manifest:{mainModule:'entry.mjs',modules:{'entry.mjs':{type:'esm',contents:entry},'durable.mjs':{type:'esm',contents:source}}},
  exports:{ProbeWake:{type:'durable-object',storage:'sqlite'}},
  env:{...Object.fromEntries(Object.entries(values).map(([key,value])=>[key,{type:'text',value}])),
   WAKES:{type:'durable-object',worker:'wake-proof',exportName:'ProbeWake'},
   OBSERVER:{type:'fetcher',handler:async request=>{const state=await request.json();observations.push(state);waiters.shift()?.resolve(state);return new Response(null,{status:204});}},
   SCHEDULE_OBSERVER:{type:'fetcher',handler:async request=>{schedules.push(await request.json());return new Response(null,{status:204});}}}
 },dev:{outboundService:{type:'fetcher',handler:async request=>{calls.push({url:request.url,method:request.method,authorization:request.headers.get('Authorization'),scope:request.headers.get('X-Blink-Worker-Tenant'),body:await request.text()});return reply(request,calls.length);}}}}]});
 await mf.ready;
 const namespace=await mf.getDurableObjectNamespace('WAKES');
 return {mf,root,calls,observations,schedules,stub:key=>namespace.getByName(key),
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
 {name:'oversized body stops at the streaming bound',reply:()=>new Response(JSON.stringify({data:{processed:0,succeeded:0,failed:0}}).padEnd(1025,' '),{headers:{'Content-Type':'application/json'}})},
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

test('durable wake public ingress fuzz: foreign credentials never create authority',async()=>{
 const fc=(await import(process.env.PRIMITIVE_FAST_CHECK_MODULE||'fast-check')).default;
 const f=await fixture();let refused=0;try{
  await fc.assert(fc.asyncProperty(fc.string({maxLength:128}),async candidate=>{
   // HTTP field bytes are a separate native boundary. Keep only representations
   // which the platform admits, and force a credential different from the grant.
   const presented=Buffer.from(candidate).toString('base64url');
   if(presented===token)throw Error('Fuzz mutation did not change authority');
   const response=await f.request(undefined,{headers:{Authorization:'Bearer '+presented}});
   if(response.status!==403||f.calls.length!==0)throw Error('Foreign authority crossed wake ingress');
   refused++;
  }),{seed:20261108,numRuns:256,endOnFailure:true});
  const ids=await f.mf.listDurableObjectIds('WAKES');
  if(ids.length!==0)throw Error('Rejected credentials manufactured durable state');
  console.log(JSON.stringify({durable_wake_ingress_fuzz:{seed:20261108,refused,backend_calls:f.calls.length,objects:ids.length}}));
 }finally{await f.close();}
});

test('durable wake response fuzz: mutated counters conserve work or retain retry',async()=>{
 const fc=(await import(process.env.PRIMITIVE_FAST_CHECK_MODULE||'fast-check')).default;
 let value,accepted=0,refused=0;
 const f=await fixture({reply:()=>Response.json({data:value})});try{
  await fc.assert(fc.asyncProperty(fc.constantFrom('processed','succeeded','failed'),fc.oneof(fc.integer({min:-2,max:3}),fc.jsonValue({maxDepth:2})),async(field,representation)=>{
   value={processed:1,succeeded:0,failed:0,[field]:representation};
   const before=Date.now(),observed=f.observed(),response=await f.request(),state=await observed;
   if(response.status!==204)throw Error('Valid wake authentication refused');
   const counters=[value.processed,value.succeeded,value.failed],valid=counters.every(Number.isSafeInteger)&&counters.every(n=>n>=0)&&BigInt(value.succeeded)+BigInt(value.failed)<=BigInt(value.processed);
   if(state.scope===undefined){
    accepted++;
    if(state.alarm!==null||!valid)throw Error('Accepted counter representation invented completion '+JSON.stringify(value));
   }else{
    refused++;
    if(valid||state.scope!==scope||!Number.isSafeInteger(state.alarm)||state.alarm<before+60000)throw Error('Counter representation lost its classification or durable retry');
   }
  }),{seed:20261108,numRuns:128,examples:[['processed',1],['succeeded',1],['failed',1],['processed',-1]],endOnFailure:true});
  if(accepted===0||refused===0||f.calls.length!==accepted+refused)throw Error('Fuzz path was vacuous or lost a delivery');
  console.log(JSON.stringify({durable_wake_response_fuzz:{seed:20261108,accepted,refused,backend_calls:f.calls.length}}));
 }finally{await f.close();}
},30000);


// Actual native Worker/SQLite behavior. The clock/state probes exist only in
// the fixture subclass, never in the emitted Worker or its public ingress.
const protocolSource=await readFile(new URL('./durable_wake.go',import.meta.url),'utf8');
const guardHeader=protocolSource.match(/^const DurableWakeGuardHeader = "([^"]+)"$/m)?.[1],hourNs='3600000000000';
if(!guardHeader)throw Error('Exported Go guard header is not bound to the native fixture');
function guardedRequest(f,value=hourNs){return f.request(undefined,{headers:{Authorization:'Bearer '+token,[guardHeader]:value}});}

test('durable wake commit guard: early empty read survives a complete runtime restart',async()=>{
 const root=await mkdtemp(join(tmpdir(),'primitive-durable-guard-'));let f;
 try{
  f=await fixture({directory:root});
  const before=await f.stub(scope).now(),response=await guardedRequest(f),state=await f.stub(scope).inspect(),after=await f.stub(scope).now();
  if(response.status!==201||f.calls.length!==0||state.scope!==scope||!Number.isSafeInteger(state.guard)||state.guard<before+3600000||state.guard>after+3600000||state.alarm!==state.guard)throw Error('Pre-commit reservation did not durably arm its guard '+JSON.stringify({status:response.status,state,calls:f.calls.length}));
  const observed=f.observed();if((await f.request()).status!==204)throw Error('Ordinary immediate wake refused');
  const early=await observed;
  if(f.calls.length!==1||early.scope!==scope||early.guard!==state.guard||early.alarm!==state.guard)throw Error('Empty read before commit retired the reservation '+JSON.stringify(early));
  await f.close();f=await fixture({directory:root});const restored=await f.stub(scope).inspect();
  if(restored.scope!==scope||restored.guard!==early.guard||restored.alarm!==early.guard||f.calls.length!==0)throw Error('Restart lost the unknown writer guard '+JSON.stringify(restored));
 }finally{if(f)await f.close();await rm(root,{recursive:true,force:true});}
},30000);

test('durable wake commit guard: a later shorter writer cannot shorten an existing guard',async()=>{
 const f=await fixture();try{
  if((await guardedRequest(f,'7200000000000')).status!==201)throw Error('First guard reservation refused');
  const first=await f.stub(scope).inspect();
  if((await guardedRequest(f)).status!==201)throw Error('Second guard reservation refused');
  const second=await f.stub(scope).inspect();
  if(second.guard!==first.guard||second.alarm!==first.alarm||f.calls.length!==0)throw Error('Shorter writer shortened outstanding custody '+JSON.stringify({first,second,calls:f.calls.length}));
  const observed=f.observed();await f.request();const empty=await observed;
  if(empty.guard!==first.guard||empty.alarm!==first.guard||empty.scope!==scope)throw Error('Empty read lost the longer writer guard '+JSON.stringify(empty));
 }finally{await f.close();}
});

test('durable wake commit guard: empty read begun after expiry retires the transient guard',async()=>{
 const f=await fixture();try{
  if((await guardedRequest(f)).status!==201)throw Error('Guard reservation refused');
  // A known expired persisted deadline forces the real native expiry branch.
  await f.stub(scope).putGuardForTest(1);
  const observed=f.observed();await f.request();const state=await observed;
  if(f.calls.length!==1||state.scope!==undefined||state.guard!==undefined||state.alarm!==null)throw Error('Completed post-guard read retained manufactured work '+JSON.stringify(state));
 }finally{await f.close();}
});

test('durable wake commit guard: an empty read spanning expiry must read again before retiring',async()=>{
 const entered=deferred(),release=deferred();const f=await fixture({reply:async(_,count)=>{if(count===1){entered.resolve();await release.promise;}return complete();}});
 try{
  if((await guardedRequest(f)).status!==201)throw Error('Guard reservation refused');
  const first=f.observed();await f.request();await entered.promise;
  // Native RPC clock observations synchronize on actual expiry, not a sleep.
  // The snapshot has already started. Move its persisted guard just beyond
  // that fact, then observe its expiry while the backend reply is held.
  const until=(await f.stub(scope).now())+1;await f.stub(scope).putGuardForTest(until);
  const backstop=AbortSignal.timeout(10000);
  while((await f.stub(scope).now())<until){if(backstop.aborted)throw Error('Native clock observation exceeded its deadlock backstop');}
  const second=f.observed();release.resolve();const a=await first;
  if(a.scope!==scope||a.guard!==until||a.alarm===null)throw Error('Pre-expiry snapshot falsely certified post-expiry absence '+JSON.stringify(a));
  const b=await second;
  if(f.calls.length!==2||b.scope!==undefined||b.guard!==undefined||b.alarm!==null)throw Error('Fresh post-expiry snapshot did not retire '+JSON.stringify({b,calls:f.calls.length}));
 }finally{release.resolve();await f.close();}
},30000);


for(const scenario of [
 {name:'one nanosecond receives one native millisecond',text:'1',ns:1n},
 {name:'last nanosecond before one millisecond rounds upward',text:'999999',ns:999999n},
 {name:'exact millisecond remains exact',text:'1000000',ns:1000000n},
 {name:'first nanosecond after a millisecond rounds upward',text:'1000001',ns:1000001n},
 {name:'caller five-minute window remains exact',text:'300000000000',ns:300000000000n},
 // A relative duration must also fit the absolute native alarm domain. The
 // earlier assertion that signed-duration range alone was sufficient was wrong.
 {name:'largest signed duration refuses absolute alarm overflow',text:'9223372036854775807'},
 {name:'present empty duration refuses when preserved by the Worker',text:'',emptyGuardAfterTransport:true},
 {name:'zero cannot invent a reservation',text:'0'},
 {name:'negative duration cannot arm a past guard',text:'-1'},
 {name:'explicit positive sign is not canonical',text:'+1'},
 {name:'leading zero is not a second wire spelling',text:'01'},
 {name:'fractional duration cannot narrow silently',text:'1.0'},
 {name:'exponent spelling cannot bypass the integer domain',text:'1e3'},
 {name:'hexadecimal spelling is not decimal duration',text:'0x10'},
 {name:'nonfinite duration cannot authorize custody',text:'Infinity'},
 {name:'one beyond signed duration maximum refuses',text:'9223372036854775808'},
 {name:'nineteen digits alone do not prove signed range',text:'9999999999999999999'},
 {name:'twentieth digit refuses before BigInt conversion',text:'10000000000000000000'},
 {name:'duplicate duration header cannot select the first value',text:'1,2'},
 {name:'internal header whitespace cannot change numeric meaning',text:'1\t2'},
])test('durable wake commit guard header: '+scenario.name,async()=>{
 const release=deferred(),f=await fixture({emptyGuardAfterTransport:scenario.emptyGuardAfterTransport,reply:async()=>{await release.promise;return complete();}});
 try{
  const response=await guardedRequest(f,scenario.text);
  if(scenario.ns===undefined){
   const ids=await f.mf.listDurableObjectIds('WAKES');
   if(response.status!==400||f.schedules.length!==0||f.calls.length!==0||ids.length!==0)throw Error('Invalid duration escaped admission '+JSON.stringify({status:response.status,schedules:f.schedules.length,calls:f.calls.length,objects:ids.length}));
  }else{
   const observed=f.schedules[0],milliseconds=BigInt(observed?.guard??0);
   if(response.status!==201||f.schedules.length!==1||observed.scope!==scope||milliseconds*1000000n<scenario.ns||(milliseconds-1n)*1000000n>=scenario.ns)throw Error('Public duration changed at native receiver '+JSON.stringify({status:response.status,observed,want:scenario.text}));
  }
 }finally{release.resolve();await f.close();}
});

test('durable wake commit guard header: transport-normalized empty field cannot acknowledge a guarded writer',async()=>{
 const release=deferred(),f=await fixture({reply:async()=>{await release.promise;return complete();}});
 try{
  const response=await guardedRequest(f,''),state=await f.stub(scope).inspect();
  // This native HTTP transport drops empty headers (independently observed at
  // ingress as has=false/get=null). An ordinary 204 must never authorize the
  // caller's guarded write, which requires exactly 201.
  if(response.status!==204||f.schedules.length!==1||f.schedules[0].guard!==0||state.guard!==undefined)throw Error('Normalized absence invented guarded authority '+JSON.stringify({status:response.status,schedules:f.schedules,state}));
 }finally{release.resolve();await f.close();}
});

test('durable wake commit guard header: overflow cannot alter an existing reservation',async()=>{
 const f=await fixture();try{
  if((await guardedRequest(f)).status!==201)throw Error('Initial reservation refused');
  const before=await f.stub(scope).inspect(),count=f.schedules.length;
  const response=await guardedRequest(f,'9223372036854775807'),after=await f.stub(scope).inspect();
  if(response.status!==400||f.schedules.length!==count||f.calls.length!==0||after.scope!==before.scope||after.guard!==before.guard||after.alarm!==before.alarm)throw Error('Overflow mutated existing durable custody '+JSON.stringify({status:response.status,before,after}));
 }finally{await f.close();}
});

test('durable wake commit guard header: native actor revalidates overflow before binding storage',async()=>{
 const f=await fixture({overflowAtActor:true});try{
  // Corrupt only the actor argument after public admission. The real entry
  // catches the actor refusal; observe storage on a fresh native actor stub.
  const response=await guardedRequest(f),state=await f.stub(scope).inspect();
  if(response.status!==503||state.scope!==undefined||state.guard!==undefined||state.alarm!==null||f.calls.length!==0)throw Error('Native actor overflow crossed storage admission '+JSON.stringify({status:response.status,state,calls:f.calls.length}));
 }finally{await f.close();}
});

test('durable wake commit guard header fuzz: typed durations preserve ceiling and real mutations refuse',async()=>{
 const fc=(await import(process.env.PRIMITIVE_FAST_CHECK_MODULE||'fast-check')).default;
 const f=await fixture();let accepted=0,refused=0;
 try{
  await fc.assert(fc.asyncProperty(fc.bigInt({min:3600000000000n,max:2592000000000000n}),fc.integer({min:0,max:4}),async(ns,mutation)=>{
   const canonical=ns.toString(),presented=mutation===0?canonical:mutation===1?'0'+canonical:mutation===2?canonical+'.':mutation===3?'9223372036854775808':'9223372036854775807';
   if(mutation!==0&&presented===canonical)throw Error('Requested duration mutation was vacuous');
   const before=f.schedules.length,response=await guardedRequest(f,presented);
   if(mutation===0){
    const observed=f.schedules[before],ms=BigInt(observed?.guard??0);
    if(response.status!==201||f.schedules.length!==before+1||observed.scope!==scope||ms*1000000n<ns||(ms-1n)*1000000n>=ns)throw Error('Accepted duration lost exact native ceiling');
    accepted++;
   }else{
    if(response.status!==400||f.schedules.length!==before||f.calls.length!==0)throw Error('Refused representation escaped a native scheduler effect');
    refused++;
   }
  }),{seed:20261108,numRuns:128,examples:[[3600000000000n,0],[2592000000000000n,0],[3600000000000n,1],[3600000000000n,2],[3600000000000n,3],[3600000000000n,4]],endOnFailure:true});
  if(accepted===0||refused===0||f.schedules.length!==accepted||f.calls.length!==0)throw Error('Duration pressure was vacuous or manufactured delivery');
  console.log(JSON.stringify({durable_guard_fuzz:{seed:20261108,accepted,refused,scheduler_calls:f.schedules.length,backend_calls:f.calls.length}}));
 }finally{await f.close();}
},30000);
