import {afterAll, beforeAll, expect, test} from 'bun:test';
import {mkdtemp, readFile, readlink, rm} from 'node:fs/promises';
import {hostname} from 'node:os';
import {join} from 'node:path';

// Serial: Bun owns one process-local Chrome backend. Each row owns a new native
// database; no fake IndexedDB or request-success surrogate is used.
let server, view, profile, chrome;
beforeAll(async () => {
  if (process.platform !== 'linux' || !process.env.PRIMITIVE_BROWSER_MOVE_MODULE?.startsWith('/data/')) throw new Error('native Furnace emitted-module fixture is required');
  const module = await readFile(process.env.PRIMITIVE_BROWSER_MOVE_MODULE);
  profile = await mkdtemp('/data/runtime/primitive-browser-move-profile-');
  server = Bun.serve({hostname:'127.0.0.1', port:0, fetch(request) {
    return new URL(request.url).pathname === '/executor.js' ? new Response(module, {headers:{'Content-Type':'text/javascript'}}) : new Response('<!doctype html><title>Native storage proof</title>');
  }});
  view = new Bun.WebView({dataStore:{directory:profile}, backend:{type:'chrome', url:false, argv:['--headless=new']}});
  await view.navigate(server.url.href);
  const lock = await readlink(join(profile, 'SingletonLock'));
  const prefix = `${hostname()}-`;
  if (!lock.startsWith(prefix)) throw new Error('Chrome lock names a foreign host');
  const pid = Number(lock.slice(prefix.length));
  const stat = await readFile(`/proc/${pid}/stat`, 'utf8');
  const fields = stat.slice(stat.lastIndexOf(')')+2).trim().split(/\s+/);
  const command = await readFile(`/proc/${pid}/cmdline`, 'utf8');
  if (Number(fields[1]) !== process.pid || !command.split(/[\0\s]+/).includes(`--user-data-dir=${profile}`)) throw new Error('Chrome process does not belong to this fixture');
  chrome = {pid, ticks:fields[19]};
}, 30_000);

afterAll(async () => {
  if (view) { view.close(); Bun.WebView.closeAll(); }
  if (chrome) {
    const deadline = Date.now()+15_000;
    while (true) {
      try {
        const stat = await readFile(`/proc/${chrome.pid}/stat`, 'utf8');
        if (stat.slice(stat.lastIndexOf(')')+2).trim().split(/\s+/)[19] !== chrome.ticks) break;
      } catch (error) { if (error.code === 'ENOENT') break; throw error; }
      if (Date.now() > deadline) throw new Error('exact owned Chrome process did not exit');
      await Bun.sleep(20); // bounded native PID observation, not synchronization by guessed delay
    }
  }
  server?.stop(true);
  if (profile) await rm(profile, {recursive:true, force:true});
}, 20_000);

async function nativeRow(mode) {
  const {commitGuardedMove, BrowserStoreErrorIdentity:I} = await import('/executor.js');
  const name = `primitive-move-${mode}`;
  const open = () => new Promise((resolve,reject) => {
    const request = indexedDB.open(name, 1);
    request.onupgradeneeded = () => request.result.createObjectStore('records');
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
  const source = {revision:7, effort:{repetitions:5, load:135}};
  const prior = {revision:9, effort:{repetitions:3, load:225}};
  const seed = await open();
  await new Promise((resolve,reject) => {
    const tx = seed.transaction('records','readwrite');
    if (mode !== 'absent-source') tx.objectStore('records').put(source,'source');
    if (mode === 'destination-exists') tx.objectStore('records').put(prior,'destination');
    tx.oncomplete=resolve; tx.onabort=()=>reject(tx.error);
  });
  seed.close();
  const db = await open();
  let reads=0, puts=0, deletes=0, closes=0, calls=0, terminal=false, rollback=false, closeAfterTerminal=true;
  const originalTransaction=db.transaction.bind(db), originalClose=db.close.bind(db);
  const closeFailure=new Error('injected close capability failure');
  db.close=()=>{closes++; closeAfterTerminal &&= terminal; originalClose(); if(['cleanup-failure','refusal-cleanup-failure'].includes(mode))throw closeFailure;};
  db.transaction=(...args)=>{
    const tx=originalTransaction(...args);
    tx.addEventListener('complete',()=>{terminal=true;});
    tx.addEventListener('abort',()=>{terminal=true; rollback=true;});
    const originalStore=tx.objectStore.bind(tx);
    tx.objectStore=(storeName)=>{
      const store=originalStore(storeName);
      const get=store.get.bind(store), put=store.put.bind(store), del=store.delete.bind(store);
      store.get=(...a)=>{reads++; return get(...a);};
      store.put=(...a)=>{puts++; return put(...a);};
      store.delete=(...a)=>{deletes++; return del(...a);};
      return store;
    };
    return tx;
  };
  const refusal=new TypeError('owned application refusal'); refusal.code=37;
  const decide=({source:got,destination})=>{
    calls++;
    if (mode==='policy-refusal'||mode==='refusal-cleanup-failure') throw refusal;
    if (mode==='non-error-refusal') throw 42;
    if (mode==='undefined-result') return undefined;
    if (mode==='async-result') return Promise.resolve(source);
    if (mode==='throwing-then') return Object.defineProperty({},'then',{get(){throw refusal;}});
    if (mode==='clone-failure') return ()=>{};
    if (got===undefined || got.revision!==source.revision || destination!==undefined) throw refusal;
    if (mode==='zero-result') return 0;
    if (mode==='null-result') return null;
    return {revision:got.revision, effort:got.effort, adopted:true};
  };
  if (mode==='commit-aborts') {
    const instrumented=db.transaction;
    db.transaction=(...args)=>{
      const tx=instrumented(...args), objectStore=tx.objectStore.bind(tx);
      tx.objectStore=(storeName)=>{
        const store=objectStore(storeName), put=store.put.bind(store);
        store.put=(...a)=>{const request=put(...a); request.addEventListener('success',()=>tx.abort()); return request;};
        return store;
      };
      return tx;
    };
  }
  if (mode==='closed-database') originalClose();
  let got, error;
  try { got=await commitGuardedMove({database:db,storeName:mode==='missing-store'?'absent':'records',sourceKey:mode==='empty-key'?'':'source',destinationKey:mode==='same-key'?'source':'destination',decide}); }
  catch(cause){error=cause;}
  const atSettlement={terminal,closes,closeAfterTerminal};
  const readback=await open();
  const values=await new Promise((resolve,reject)=>{
    const tx=readback.transaction('records','readonly'), store=tx.objectStore('records');
    const s=store.get('source'), d=store.get('destination');
    tx.oncomplete=()=>resolve({source:s.result,destination:d.result}); tx.onabort=()=>reject(tx.error);
  });
  readback.close();
  const success=['ordinary-commit','zero-result','null-result'].includes(mode);
  const want=mode==='zero-result'?0:mode==='null-result'?null:{revision:7,effort:source.effort,adopted:true};
  const requireTerminal=!['empty-key','same-key','closed-database','missing-store'].includes(mode);
  const identity=error?.primitiveIdentity ?? null;
  const expectedIdentity=['empty-key','same-key','undefined-result','async-result'].includes(mode)?I.contract:
    ['missing-store','closed-database'].includes(mode)?I.source:
    ['clone-failure','commit-aborts'].includes(mode)?I.destination:mode==='cleanup-failure'?I.cleanup:mode==='non-error-refusal'?I.callback:null;
  return {mode,success,errorPresent:error!==undefined,identity,expectedIdentity,nativeCause:error?.cause?.name??null,
    exactRefusal:error===refusal,nonErrorCause:error?.cause===42,cleanupCause:error?.cause===closeFailure,
    joinedRefusal:error instanceof AggregateError&&error.errors.length===2&&error.errors[0]===refusal&&error.errors[1].primitiveIdentity===I.cleanup&&error.errors[1].cause===closeFailure,
    valueMatches:JSON.stringify(got)===JSON.stringify(want),
    sourcePreserved:JSON.stringify(values.source)===JSON.stringify(mode==='absent-source'?undefined:source),
    sourceDeleted:values.source===undefined,
    destinationPreserved:JSON.stringify(values.destination)===JSON.stringify(mode==='destination-exists'?prior:undefined),
    destinationMatches:JSON.stringify(values.destination)===JSON.stringify(want),
    reads,puts,deletes,calls,rollback,requireTerminal,...atSettlement};
}

for (const mode of ['ordinary-commit','zero-result','null-result','absent-source','destination-exists','policy-refusal','non-error-refusal','undefined-result','async-result','throwing-then','clone-failure','commit-aborts','empty-key','same-key','missing-store','closed-database','cleanup-failure','refusal-cleanup-failure']) {
  test(`native guarded move ${mode}`,async()=>{
    const got=await view.evaluate(`(${nativeRow.toString()})(${JSON.stringify(mode)})`);
    expect(got.closes).toBe(1);
    if (got.requireTerminal) { expect(got.terminal).toBe(true); expect(got.closeAfterTerminal).toBe(true); }
    expect(got.reads).toBeLessThanOrEqual(2);
    if(got.success){
      expect(got.errorPresent).toBe(false); expect(got.valueMatches).toBe(true);
      expect(got.sourceDeleted).toBe(true); expect(got.destinationMatches).toBe(true);
      expect([got.reads,got.puts,got.deletes,got.calls]).toEqual([2,1,1,1]);
    }else if(mode==='cleanup-failure'){
      expect(got.errorPresent).toBe(true);expect(got.identity).toBe(got.expectedIdentity);expect(got.cleanupCause).toBe(true);
      expect(got.sourceDeleted).toBe(true);expect(got.destinationMatches).toBe(true);
    }else{
      expect(got.errorPresent).toBe(true); expect(got.sourcePreserved).toBe(true); expect(got.destinationPreserved).toBe(true);
      if(got.expectedIdentity!==null) expect(got.identity).toBe(got.expectedIdentity);
      else if(mode==='refusal-cleanup-failure')expect(got.joinedRefusal).toBe(true);
      else expect(got.exactRefusal).toBe(true);
      if(mode==='clone-failure') expect(got.nativeCause).toBe('DataCloneError');
      if(mode==='closed-database') expect(got.nativeCause).toBe('InvalidStateError');
      if(mode==='missing-store') expect(got.nativeCause).toBe('NotFoundError');
      if(mode==='non-error-refusal') expect(got.nonErrorCause).toBe(true);
    }
  }, 20_000);
}

async function competingMoves(concurrent) {
  const {commitGuardedMove} = await import('/executor.js');
  const name = `primitive-competing-${concurrent}`;
  const open = () => new Promise((resolve,reject)=>{
    const r=indexedDB.open(name,1); r.onupgradeneeded=()=>r.result.createObjectStore('records');
    r.onsuccess=()=>resolve(r.result); r.onerror=()=>reject(r.error);
  });
  const source={revision:11}, seed=await open();
  await new Promise((resolve,reject)=>{const t=seed.transaction('records','readwrite');t.objectStore('records').put(source,'source');t.oncomplete=resolve;t.onabort=()=>reject(t.error);});seed.close();
  const refusal=new TypeError('source CAS refused');
  const firstDB=await open(),secondDB=await open();
  const move=(db,key)=>commitGuardedMove({database:db,storeName:'records',sourceKey:'source',destinationKey:key,decide:({source:got,destination})=>{
    if(got?.revision!==source.revision||destination!==undefined)throw refusal;
    return {revision:got.revision,adopted:key};
  }});
  let outcomes;
  if(concurrent) outcomes=await Promise.allSettled([move(firstDB,'first'),move(secondDB,'second')]);
  else {const first=await move(firstDB,'first');outcomes=await Promise.allSettled([Promise.resolve(first),move(secondDB,'second')]);}
  const db=await open(),values=await new Promise((resolve,reject)=>{const t=db.transaction('records','readonly'),s=t.objectStore('records'),a=s.get('source'),b=s.get('first'),c=s.get('second');t.oncomplete=()=>resolve({source:a.result,first:b.result,second:c.result});t.onabort=()=>reject(t.error);});db.close();
  return {commits:outcomes.filter(x=>x.status==='fulfilled').length,exactRefusals:outcomes.filter(x=>x.status==='rejected'&&x.reason===refusal).length,sourceGone:values.source===undefined,first:values.first,secondAbsent:values.second===undefined};
}
for (const concurrent of [false,true]) test(`native source CAS ${concurrent?'concurrent':'sequential'} competing moves commit only once`,async()=>{
  const got=await view.evaluate(`(${competingMoves.toString()})(${concurrent})`);
  expect(got).toEqual({commits:1,exactRefusals:1,sourceGone:true,first:{revision:11,adopted:'first'},secondAbsent:true});
},20_000);

test('native invalid options produce contract refusal without fabricated transaction completion',async()=>{
  const got=await view.evaluate(`(async()=>{const {commitGuardedMove,BrowserStoreErrorIdentity:I}=await import('/executor.js');const outcomes=[];for(const options of [null,{},undefined]){try{await commitGuardedMove(options);outcomes.push(false);}catch(error){outcomes.push(error.primitiveIdentity===I.contract);}}return outcomes;})()`);
  expect(got).toEqual([true,true,true]);
},20_000);
