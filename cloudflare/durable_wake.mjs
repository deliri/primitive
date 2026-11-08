import {DurableObject} from 'cloudflare:workers';

const scopePattern=/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;
const retryMilliseconds=60000,requestMilliseconds=25000,responseMaximumBytes=1024;
const keyName='scope';

function secret(value){return typeof value==='string'&&value.length>=32&&value.length<=4096&&/^[\x21-\x7e]+$/.test(value);}
function destination(env){
 const url=new URL(env.TARGET_URL);
 if(url.protocol!=='https:'||!url.hostname||url.username||url.password||url.search||url.hash||url.href!==env.TARGET_URL||!secret(env.TARGET_TOKEN)||typeof env.TARGET_SCOPE_HEADER!=='string'||!/^X-[A-Za-z0-9-]{1,64}$/.test(env.TARGET_SCOPE_HEADER))throw Error('Wake destination configuration refused');
 return url;
}
async function authorized(request,token){
 if(!secret(token))return false;
 const value=request.headers.get('Authorization');
 if(typeof value!=='string'||value.length>4103||!value.startsWith('Bearer '))return false;
 const encoder=new TextEncoder();
 const [actual,want]=await Promise.all([crypto.subtle.digest('SHA-256',encoder.encode(value.slice(7))),crypto.subtle.digest('SHA-256',encoder.encode(token))]);
 return crypto.subtle.timingSafeEqual(actual,want);
}
function nextAlarm(value){
 if(value===undefined)return undefined;
 if(typeof value!=='string'||!/^[1-9][0-9]{0,18}$/.test(value))throw Error('Wake deadline refused');
 const instant=BigInt(value);
 if(instant>9223372036854775807n)throw Error('Wake deadline refused');
 return Number((instant+999999n)/1000000n);
}
async function nextFromResponse(response){
 if(response.status!==200||response.headers.get('Content-Type')?.split(';')[0].trim().toLowerCase()!=='application/json')throw Error('Wake response refused');
 const reader=response.body?.getReader();
 if(!reader)throw Error('Wake response refused');
 const bytes=new Uint8Array(responseMaximumBytes);let length=0;
 try{
  for(;;){const part=await reader.read();if(part.done)break;if(part.value.length>bytes.length-length)throw Error('Wake response refused');bytes.set(part.value,length);length+=part.value.length;}
 }finally{await reader.cancel();reader.releaseLock();}
 const envelope=JSON.parse(new TextDecoder('utf-8',{fatal:true}).decode(bytes.subarray(0,length))),result=envelope?.data;
 if(!result||typeof result!=='object'||Array.isArray(result)||Object.keys(result).some(key=>!['processed','succeeded','failed','next_wake_at'].includes(key))||!['processed','succeeded','failed'].every(key=>Number.isSafeInteger(result[key])&&result[key]>=0)||result.succeeded>result.processed||result.failed>result.processed-result.succeeded)throw Error('Wake response refused');
 return nextAlarm(result.next_wake_at);
}

async function emptyBody(request){
 const reader=request.body?.getReader();
 if(!reader)return true;
 try{for(;;){const part=await reader.read();if(part.done)return true;if(part.value.length!==0)return false;}}
 finally{await reader.cancel();reader.releaseLock();}
}

export default {
 async fetch(request,env){
  const url=new URL(request.url),path=url.pathname;
  if(request.method!=='POST')return new Response(null,{status:405,headers:{Allow:'POST'}});
  if(url.search||!path.startsWith('/wake/')||!scopePattern.test(path.slice(6)))return new Response(null,{status:400});
  if(!await authorized(request,env.SCHEDULE_TOKEN))return new Response(null,{status:403});
  if(!await emptyBody(request))return new Response(null,{status:400});
  try{
   destination(env);
   const result=await env.WAKES.getByName(path.slice(6)).schedule(path.slice(6));
   return new Response(null,{status:result});
  }catch{return new Response(null,{status:503});}
 }
};

export class DurableWake extends DurableObject {
 async schedule(scope){
  if(typeof scope!=='string'||!scopePattern.test(scope))throw Error('Wake scope refused');
  await this.ctx.blockConcurrencyWhile(async()=>{
   const saved=await this.ctx.storage.get(keyName);
   if(saved!==undefined&&saved!==scope)throw Error('Wake scope binding refused');
   if(saved===undefined)await this.ctx.storage.put(keyName,scope);
   const at=Date.now(),pending=await this.ctx.storage.getAlarm();
   if(pending===null||at<pending)await this.ctx.storage.setAlarm(at);
  });
  return 204;
 }
 async alarm(){
  const scope=await this.ctx.storage.get(keyName);
  if(scope===undefined)return;
  let next;
  try{
   if(typeof scope!=='string'||!scopePattern.test(scope))throw Error('Wake scope binding refused');
   const target=destination(this.env);
   const response=await fetch(target,{method:'POST',headers:{Authorization:`Bearer ${this.env.TARGET_TOKEN}`,[this.env.TARGET_SCOPE_HEADER]:scope,Accept:'application/json'},redirect:'manual',cache:'no-store',signal:AbortSignal.timeout(requestMilliseconds)});
   next=await nextFromResponse(response);
  }catch{
   // Explicit durable rearming also survives outages beyond Cloudflare's six
   // automatic exception retries. No parked timer or idle polling is used.
   console.error('Durable wake delivery refused; retained for retry');
   next=Date.now()+retryMilliseconds;
  }
  await this.ctx.blockConcurrencyWhile(async()=>{
   const pending=await this.ctx.storage.getAlarm();
   // A new commit may have arrived during fetch. Never clear or postpone it.
   if(next!==undefined){const at=Math.max(Date.now(),next);if(pending===null||at<pending)await this.ctx.storage.setAlarm(at);}
   else if(pending===null)await this.ctx.storage.delete(keyName);
  });
 }
}
