import {expect,test} from 'bun:test';
import {mkdtemp,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';

// A separate native browser process explicitly disables the proposed platform
// feature. No replacement modelContext or simulated registry is installed.
test('native browser without WebMCP reports unsupported without invoking caller',async()=>{
  const directory=await mkdtemp(join(tmpdir(),'primitive-webmcp-unsupported-'));
  const body=await readFile(new URL('./browser_module.js',import.meta.url));
  const server=Bun.serve({hostname:'127.0.0.1',port:0,fetch(request){return new URL(request.url).pathname==='/module.js'?new Response(body,{headers:{'Content-Type':'text/javascript'}}):new Response('<!doctype html><title>Unsupported WebMCP proof</title>');}});
  let view;
  try{
    view=new Bun.WebView({dataStore:{directory},backend:{type:'chrome',url:false,argv:['--headless=new','--disable-features=WebMCP']}});
    await view.navigate(server.url.href);
    expect(await view.evaluate('typeof document.modelContext')).toBe('undefined');
    expect(await view.evaluate(`(async()=>{const {registerTool}=await import('/module.js');let calls=0;const supported=await registerTool({name:'unused',description:'Unused',execute:async()=>{calls++;return {};}});return {supported,calls};})()`)).toEqual({supported:false,calls:0});
  }finally{view?.close();Bun.WebView.closeAll();server.stop(true);await rm(directory,{recursive:true,force:true});}
},30000);
