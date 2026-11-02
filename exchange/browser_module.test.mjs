import {afterAll, beforeAll, expect, test} from 'bun:test';
import {readFile} from 'node:fs/promises';

// Real browser engine (WebKit or Chrome via Bun.WebView) executing the exact
// module emitted by exchange.BrowserModule(). The server below is a local
// same-origin application double; no fetch surrogate is installed.
const engine = process.env.PRIMITIVE_BROWSER_ENGINE;
const modules = process.env.PRIMITIVE_BROWSER_MODULES;
let server, view;
const hits = [];
let aborted = 0;

beforeAll(async () => {
  if (!['webkit', 'chrome'].includes(engine) || !modules?.startsWith('/')) throw new Error('PRIMITIVE_BROWSER_ENGINE and emitted PRIMITIVE_BROWSER_MODULES are required');
  const executor = await readFile(`${modules}/exchange.js`);
  server = Bun.serve({hostname: '127.0.0.1', port: 0, async fetch(request) {
    const url = new URL(request.url);
    hits.push({path: url.pathname, method: request.method});
    switch (url.pathname) {
      case '/': return new Response('<!doctype html><title>Exchange proof</title>', {headers: {'Content-Type': 'text/html'}});
      case '/exchange.js': return new Response(executor, {headers: {'Content-Type': 'text/javascript'}});
      case '/api/echo': {
        const text = request.method === 'GET' ? '' : await request.text();
        return Response.json({method: request.method, type: request.headers.get('content-type'), accept: request.headers.get('accept'), cookie: request.headers.get('cookie'), body: text});
      }
      case '/api/redirect': return new Response(null, {status: 302, headers: {Location: '/api/target'}});
      case '/api/error': return Response.json({secret: 'PRIVATE-PROVIDER-BODY'}, {status: 500});
      case '/api/html': return new Response('<p>x</p>', {headers: {'Content-Type': 'text/html'}});
      case '/api/malformed': return new Response('{"a":', {headers: {'Content-Type': 'application/json'}});
      case '/api/slow': return new Promise((resolve) => request.signal.addEventListener('abort', () => { aborted++; resolve(new Response('late')); }));
      default: return new Response('missing', {status: 404});
    }
  }});
  const backend = engine === 'chrome' ? {type: 'chrome', argv: ['--headless=new']} : undefined;
  view = new Bun.WebView(backend ? {backend} : {});
  await view.navigate(server.url.href);
}, 30_000);

afterAll(() => {
  view?.close();
  Bun.WebView.closeAll?.();
  server?.stop(true);
});

async function run(options) {
  return view.evaluate(`(${pageCall.toString()})(${JSON.stringify(options)})`);
}

async function pageCall(options) {
  const exchange = await import('/exchange.js');
  document.cookie = 'session=same-origin';
  const controller = new AbortController();
  if (options.abortAfter !== undefined) setTimeout(() => controller.abort(), options.abortAfter);
  const request = {...options.request, signal: options.abortAfter === undefined ? undefined : controller.signal};
  try {
    return {ok: await exchange.sendJSON(request), identities: exchange.BrowserExchangeErrorIdentity};
  } catch (error) {
    return {identity: error.primitiveIdentity, status: error.status, text: `${error.message} ${error.cause ?? ''}`, identities: exchange.BrowserExchangeErrorIdentity};
  }
}

test(`${engine}: same-origin JSON round trip carries body, cookie and JSON accept`, async () => {
  const got = await run({request: {path: '/api/echo', method: 'POST', body: {a: 1}, expectedStatus: 200}});
  expect(got.ok).toEqual({method: 'POST', type: 'application/json', accept: 'application/json', cookie: 'session=same-origin', body: '{"a":1}'});
});

const refusals = [
  ['redirect is refused and never followed', {path: '/api/redirect', method: 'GET', expectedStatus: 200}, 'redirect'],
  ['server error rejects without the provider body', {path: '/api/error', method: 'GET', expectedStatus: 200}, 'response'],
  ['html success is a content type refusal', {path: '/api/html', method: 'GET', expectedStatus: 200}, 'contentType'],
  ['malformed json is a response refusal', {path: '/api/malformed', method: 'GET', expectedStatus: 200}, 'response'],
  ['unexpected success status is a response refusal', {path: '/api/echo', method: 'GET', expectedStatus: 201}, 'response'],
  ['protocol-relative path is a contract refusal', {path: '//attacker.example/x', method: 'GET', expectedStatus: 200}, 'contract'],
  ['absolute foreign url is a contract refusal', {path: 'https://attacker.example/x', method: 'GET', expectedStatus: 200}, 'contract'],
  ['unsupported method is a contract refusal', {path: '/api/echo', method: 'TRACE', expectedStatus: 200}, 'contract'],
  ['GET with a body is a contract refusal', {path: '/api/echo', method: 'GET', body: {a: 1}, expectedStatus: 200}, 'contract'],
  ['non-success expected status is a contract refusal', {path: '/api/echo', method: 'GET', expectedStatus: 302}, 'contract'],
];
for (const [name, request, identity] of refusals) {
  test(`${engine}: ${name}`, async () => {
    const before = hits.length;
    const got = await run({request});
    expect(got.ok).toBeUndefined();
    expect(got.identity).toBe(got.identities[identity]);
    expect(got.text).not.toContain('PRIVATE-PROVIDER-BODY');
    if (identity === 'contract') expect(hits.length).toBe(before);
    if (identity === 'redirect') expect(hits.slice(before).map((hit) => hit.path)).toEqual(['/api/redirect']);
  });
}

test(`${engine}: caller abort cancels the native request`, async () => {
  const before = aborted;
  const got = await run({request: {path: '/api/slow', method: 'GET', expectedStatus: 200}, abortAfter: 50});
  expect(got.identity).toBe(got.identities.cancelled);
  for (let i = 0; i < 100 && aborted === before; i++) await Bun.sleep(20); // bounded observation of the server-side abort fact
  expect(aborted).toBe(before + 1);
});
