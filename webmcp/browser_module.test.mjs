import {afterAll,beforeAll,expect,test} from 'bun:test';
import {mkdtemp,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';

// Serial: one owned native Chrome backend. The module is served byte-for-byte
// from the file whose equality with BrowserModule is proved through Anvil.
let view,server,directory;
beforeAll(async()=>{
  const module=await readFile(new URL('./browser_module.js',import.meta.url));
  directory=await mkdtemp(join(tmpdir(),'primitive-webmcp-'));
  server=Bun.serve({hostname:'127.0.0.1',port:0,fetch(request){
    return new URL(request.url).pathname==='/module.js'
      ?new Response(module,{headers:{'Content-Type':'text/javascript'}})
      :new Response('<!doctype html><title>Native WebMCP proof</title>');
  }});
  view=new Bun.WebView({dataStore:{directory},backend:{type:'chrome',url:false,argv:['--headless=new','--enable-features=WebMCP']}});
  await view.navigate(server.url.href);
  console.log(JSON.stringify({nativeUserAgent:await view.evaluate('navigator.userAgent'),flags:['--headless=new','--enable-features=WebMCP']}));
  expect(await view.evaluate('typeof document.modelContext?.registerTool')).toBe('function');
  expect(await view.evaluate('typeof document.modelContext?.getTools')).toBe('function');
  expect(await view.evaluate('typeof document.modelContext?.executeTool')).toBe('function');
},30000);
afterAll(async()=>{
  view?.close();Bun.WebView.closeAll();server?.stop(true);
  if(directory)await rm(directory,{recursive:true,force:true});
},20000);

async function row(mode){
  return view.evaluate(`(${nativeRow.toString()})(${JSON.stringify(mode)})`);
}

async function nativeRow(mode){
  const {registerTool}=await import('/module.js');
  const context=document.modelContext,principal=new AbortController(),call=new AbortController();
  const reason=new DOMException('fixture principal ended','AbortError');
  const refusal=new DOMException('fixture policy refused','NotAllowedError');
  const entered=Promise.withResolvers(),release=Promise.withResolvers();
  const name='primitive_'+mode.replaceAll('-','_');
  let calls=0,received,callbackSignal,callbackRefusal=false;
  const definition={name,description:'Native execution proof',inputSchema:{type:'object',properties:{value:{type:'string'}},required:['value'],additionalProperties:false},annotations:{readOnlyHint:true},
    execute:async(input,{signal})=>{
      calls++;received=input;callbackSignal=signal;
      entered.resolve();
      if(mode==='policy-refusal'){callbackRefusal=true;throw refusal;}
      if(mode==='principal-inflight'||mode==='caller-inflight')await release.promise;
      return {value:input.value};
    }};
  try{
    if(mode==='principal-before')principal.abort(reason);
    if(mode==='invalid-callback')definition.execute=null;
    if(mode==='missing-description')delete definition.description;
    let registered,registrationError;
    try{registered=await registerTool(definition,mode==='without-principal'?{}:{signal:principal.signal});}
    catch(error){registrationError=error;}
    const tools=await context.getTools(),tool=tools.find(tool=>tool.name===name);
    if(registrationError)return {registered:false,calls,present:tool!==undefined,name:registrationError.name,sameCause:registrationError===reason};
    if(mode==='unregistered'){
      principal.abort(reason);
      const remaining=await context.getTools();
      return {registered,calls,present:remaining.some(tool=>tool.name===name)};
    }
    const input={value:mode==='unicode'?'café 🏋️':mode==='empty-value'?'':'222'};
    let result,error,nativeErrorName,nativeErrorIsDOMException;
    // Pin the actual browser's own rejection class. Its current implementation
    // may differ from the draft; Primitive must preserve, never translate it.
    if(mode==='principal-inflight'||mode==='policy-refusal'){
      const nativeName=name+'_direct_rejection';
      try{
        await context.registerTool({name:nativeName,description:'Direct native refusal',execute:async()=>{throw mode==='policy-refusal'?refusal:reason;}},{signal:principal.signal});
        const nativeTool=(await context.getTools()).find(tool=>tool.name===nativeName);
        try{await context.executeTool(nativeTool,{});}catch(cause){nativeErrorName=cause.name;nativeErrorIsDOMException=cause instanceof DOMException;}
      }finally{ /* The same principal signal owns both native registrations. */ }
    }
    const pending=context.executeTool(tool,input,{signal:call.signal}).then(value=>{result=value;},cause=>{error=cause;});
    if(mode==='principal-inflight'||mode==='caller-inflight'){
      await entered.promise;
      if(mode==='principal-inflight')principal.abort(reason);else {
        const aborted=callbackSignal.aborted?Promise.resolve():new Promise(resolve=>callbackSignal.addEventListener('abort',resolve,{once:true}));
        call.abort(reason);await aborted;
      }
      release.resolve();
    }
    await pending;
    return {registered,calls,present:tool!==undefined,inputExact:received?.value===input.value,
      resultType:typeof result,result,errorName:error?.name??null,errorIsDOMException:error instanceof DOMException,nativeErrorName,nativeErrorIsDOMException,callbackRefusal,
      callbackAborted:callbackSignal?.aborted??null,principalCause:callbackSignal?.reason===reason,
      invocationCause:callbackSignal?.reason?.name??null};
  }finally{principal.abort();release.resolve();}
}

for(const mode of ['ordinary','unicode','empty-value','without-principal'])test(`native registration forwards exact ${mode} input and result`,async()=>{
  const got=await row(mode);
  expect(got.registered).toBe(true);expect(got.calls).toBe(1);expect(got.present).toBe(true);expect(got.inputExact).toBe(true);
  expect(got.errorName).toBe(null);expect(got.callbackAborted).toBe(false);
  const body=typeof got.result==='string'?JSON.parse(got.result):got.result;
  expect(body).toEqual({value:mode==='unicode'?'café 🏋️':mode==='empty-value'?'':'222'});
});
test('principal abort before registration preserves its exact native cause and creates no tool',async()=>{
  expect(await row('principal-before')).toEqual({registered:false,calls:0,present:false,name:'AbortError',sameCause:true});
});
test('principal abort removes discovery without invoking the tool',async()=>{
  expect(await row('unregistered')).toEqual({registered:true,calls:0,present:false});
});
test('principal abort reaches an already running callback and prevents result release',async()=>{
  const got=await row('principal-inflight');
  expect(got.registered).toBe(true);expect(got.calls).toBe(1);expect(got.callbackAborted).toBe(true);expect(got.principalCause).toBe(true);
  expect(got.result).toBeUndefined();expect(got.errorIsDOMException).toBe(true);expect(got.nativeErrorIsDOMException).toBe(true);expect(got.errorName).toBe(got.nativeErrorName);
});
test('caller cancellation reaches the callback without a successful result',async()=>{
  const got=await row('caller-inflight');
  expect(got.calls).toBe(1);expect(got.callbackAborted).toBe(true);expect(got.invocationCause).toBe('AbortError');expect(got.result).toBeUndefined();expect(got.errorName).toBe('AbortError');
});
test('native invocation refuses the application policy rejection instead of reporting success',async()=>{
  const got=await row('policy-refusal');
  expect(got.calls).toBe(1);expect(got.callbackRefusal).toBe(true);expect(got.result).toBeUndefined();expect(got.errorIsDOMException).toBe(true);expect(got.nativeErrorIsDOMException).toBe(true);expect(got.errorName).toBe(got.nativeErrorName);
});
for(const mode of ['invalid-callback','missing-description'])test(`browser admission refuses ${mode} before invocation`,async()=>{
  const got=await row(mode);
  expect(got.registered).toBe(false);expect(got.calls).toBe(0);expect(got.present).toBe(false);expect(got.name).toBe('TypeError');
});
