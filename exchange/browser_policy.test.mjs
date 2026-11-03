import {afterAll, beforeAll, test} from 'bun:test';
import {readFile} from 'node:fs/promises';

const engine = process.env.PRIMITIVE_BROWSER_ENGINE;
const modules = process.env.PRIMITIVE_BROWSER_MODULES;
let page, api, view;
const requests = [];

beforeAll(async () => {
  if (!['webkit', 'chrome'].includes(engine) || !modules?.startsWith('/')) throw new Error('browser engine and emitted module directory are required');
  const source = await readFile(`${modules}/exchange.js`);
  api = Bun.serve({hostname: '127.0.0.1', port: 0, async fetch(request) {
    const headers = {'Access-Control-Allow-Origin': page.url.origin, 'Access-Control-Allow-Credentials': 'true', 'Access-Control-Allow-Headers': 'Content-Type, X-CSRF-Token, Idempotency-Key', 'Access-Control-Allow-Methods': 'POST, GET'};
    requests.push({method: request.method, path: new URL(request.url).pathname});
    if (request.method === 'OPTIONS') return new Response(null, {status: 204, headers});
    return Response.json({method: request.method, cookie: request.headers.get('cookie'), csrf: request.headers.get('x-csrf-token'), key: request.headers.get('idempotency-key'), body: request.method === 'GET' ? null : await request.json()}, {headers});
  }});
  page = Bun.serve({hostname: '127.0.0.1', port: 0, async fetch(request) {
    if (new URL(request.url).pathname === '/exchange.js') return new Response(source, {headers: {'Content-Type': 'text/javascript'}});
    return new Response('<!doctype html><title>Origin-bound API transport</title>', {headers: {'Content-Type': 'text/html'}});
  }});
  view = new Bun.WebView(engine === 'chrome' ? {backend: {type: 'chrome', argv: ['--headless=new']}} : {});
  await view.navigate(page.url.href);
}, 30_000);

afterAll(() => { view?.close(); page?.stop(true); api?.stop(true); });

async function browserCall(input) {
  const exchange = await import('/exchange.js');
  try {
    document.cookie = 'session=private-api-fixture; SameSite=Lax; path=/';
    const client = exchange.createBrowserExchange(input.config);
    return {value: await client.sendJSON(input.request)};
  } catch (error) {
    return {identity: error.primitiveIdentity, contract: exchange.BrowserExchangeErrorIdentity.contract};
  }
}

function call(input) { return view.evaluate(`(${browserCall.toString()})(${JSON.stringify(input)})`); }
function ordinary() {
  return {config: {origin: api.url.origin, csrfHeader: 'X-CSRF-Token'}, request: {path: '/appointments', method: 'POST', expectedStatus: 200, csrfToken: 'signed-form-fixture', idempotencyKey: 'stable-booking-intent-fixture', body: {name: 'A visitor', start: '1793620800000001001'}}};
}

test(`${engine}: configured API origin carries CSRF, stable identity and native credential policy`, async () => {
  const got = await call(ordinary());
  const value = got.value;
  if (!value || value.method !== 'POST' || value.csrf !== 'signed-form-fixture' || value.key !== 'stable-booking-intent-fixture' || value.cookie !== 'session=private-api-fixture' || value.body.start !== '1793620800000001001' || value.body.name !== 'A visitor') throw new Error(`native API result = ${JSON.stringify(got)}, want exact request and native cross-origin cookie`);
});

test(`${engine}: configured headers are per request and never stick to a later read`, async () => {
  const got = await view.evaluate(`(async()=>{const x=await import('/exchange.js');const c=x.createBrowserExchange(${JSON.stringify({origin: api.url.origin, csrfHeader: 'X-CSRF-Token'})});await c.sendJSON({path:'/appointments',method:'POST',expectedStatus:200,csrfToken:'first-token',idempotencyKey:'first-key',body:{}});return c.sendJSON({path:'/appointments',method:'GET',expectedStatus:200});})()`);
  if (got.csrf !== null || got.key !== null || got.method !== 'GET') throw new Error(`later read = ${JSON.stringify(got)}, want no inherited mutation authority`);
});

const refusals = [
  ['origin userinfo cannot carry a credential', (v) => { v.config.origin = 'https://secret@example.test'; }],
  ['origin path cannot retarget a request', (v) => { v.config.origin = `${api.url.origin}/another`; }],
  ['origin query cannot carry a bearer', (v) => { v.config.origin = `${api.url.origin}?token=secret`; }],
  ['origin fragment is not configuration', (v) => { v.config.origin = `${api.url.origin}#secret`; }],
  ['empty query delimiter is not an origin', (v) => { v.config.origin = `${api.url.origin}?`; }],
  ['empty fragment delimiter is not an origin', (v) => { v.config.origin = `${api.url.origin}#`; }],
  ['origin whitespace cannot be normalized into authority', (v) => { v.config.origin = ` ${api.url.origin}`; }],
  ['embedded newline cannot be normalized into authority', (v) => { v.config.origin = 'https:\n//example.test'; }],
  ['backslash cannot be normalized into authority', (v) => { v.config.origin = 'https:\\example.test'; }],
  ['protocol relative origin is refused', (v) => { v.config.origin = '//example.test'; }],
  ['plain remote origin cannot receive private headers', (v) => { v.config.origin = 'http://example.test'; }],
  ['non HTTP origin is refused', (v) => { v.config.origin = 'file:///tmp/api'; }],
  ['opaque origin is refused', (v) => { v.config.origin = 'null'; }],
  ['non text origin is refused', (v) => { v.config.origin = 1; }],
  ['absolute request cannot escape configured origin', (v) => { v.request.path = page.url.href; }],
  ['protocol relative request cannot escape configured origin', (v) => { v.request.path = '//example.test'; }],
  ['backslash request cannot change authority', (v) => { v.request.path = '/\\example.test'; }],
  ['fragment request cannot carry a bearer', (v) => { v.request.path = '/appointments#private'; }],
  ['authorization cannot masquerade as CSRF policy', (v) => { v.config.csrfHeader = 'Authorization'; }],
  ['cookie cannot masquerade as CSRF policy', (v) => { v.config.csrfHeader = 'Cookie'; }],
  ['origin cannot masquerade as CSRF policy', (v) => { v.config.csrfHeader = 'Origin'; }],
  ['content type cannot masquerade as CSRF policy', (v) => { v.config.csrfHeader = 'Content-Type'; }],
  ['idempotency cannot masquerade as CSRF policy', (v) => { v.config.csrfHeader = 'Idempotency-Key'; }],
  ['proxy authorization is refused', (v) => { v.config.csrfHeader = 'Proxy-Authorization'; }],
  ['fetch metadata is browser owned', (v) => { v.config.csrfHeader = 'Sec-Fetch-Site'; }],
  ['CSRF name cannot inject a second header', (v) => { v.config.csrfHeader = 'X-CSRF\r\nX-Other'; }],
  ['empty CSRF header policy is refused', (v) => { v.config.csrfHeader = ''; }],
  ['non text CSRF header policy is refused', (v) => { v.config.csrfHeader = 1; }],
  ['CSRF token cannot inject a second header', (v) => { v.request.csrfToken = 'token\r\nX-Other: secret'; }],
  ['CSRF token with NUL is refused', (v) => { v.request.csrfToken = 'token\u0000'; }],
  ['empty CSRF token is refused', (v) => { v.request.csrfToken = ''; }],
  ['non text CSRF token is refused', (v) => { v.request.csrfToken = 1; }],
  ['padded CSRF token cannot silently change on the wire', (v) => { v.request.csrfToken = ' token'; }],
  ['non ASCII CSRF token has no header representation', (v) => { v.request.csrfToken = 'token\u0100'; }],
  ['CSRF token needs explicitly configured header', (v) => { delete v.config.csrfHeader; }],
  ['idempotency key cannot inject a second header', (v) => { v.request.idempotencyKey = 'key\nX-Other: secret'; }],
  ['empty idempotency key is refused', (v) => { v.request.idempotencyKey = ''; }],
  ['non text idempotency key is refused', (v) => { v.request.idempotencyKey = 1; }],
  ['padded idempotency key cannot silently change on the wire', (v) => { v.request.idempotencyKey = 'key '; }],
  ['GET cannot carry mutation authority', (v) => { v.request.method = 'GET'; delete v.request.body; }],
];

for (const [name, edit] of refusals) {
  test(`${engine}: ${name}`, async () => {
    const input = ordinary(); edit(input);
    const before = requests.length;
    const got = await call(input);
    if (got.identity !== got.contract || got.value !== undefined || requests.length !== before) throw new Error(`refusal = ${JSON.stringify(got)}, native requests ${requests.length - before}, want typed contract and zero HTTP`);
  });
}
