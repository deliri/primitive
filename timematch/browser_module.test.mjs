import {afterAll, beforeAll, expect, test} from 'bun:test';
import {readFile} from 'node:fs/promises';

// Real browser engine (WebKit or Chrome via Bun.WebView) executing the exact
// modules emitted by exchange.BrowserModule() and timematch.BrowserModule().
// The server is a local same-origin application double for Kernel routes.
const engine = process.env.PRIMITIVE_BROWSER_ENGINE;
const modules = process.env.PRIMITIVE_BROWSER_MODULES;
let server, view;
const received = [];

const W = {start: '2026-11-02T09:00:00Z', end: '2026-11-02T17:00:00Z'};
const availability = {
  known: {...W, state: 'known', busy: [{start: '2026-11-02T10:00:00Z', end: '2026-11-02T11:00:00Z'}, {start: '2026-11-02T13:00:00Z', end: '2026-11-02T14:00:00Z'}]},
  unknown: {...W, state: 'unknown', busy: []},
  unknownWithBusy: {...W, state: 'unknown', busy: [{start: '2026-11-02T10:00:00Z', end: '2026-11-02T11:00:00Z'}]},
  foreignWindow: {start: W.start, end: '2026-11-02T16:59:59Z', state: 'known', busy: []},
  outside: {...W, state: 'known', busy: [{start: '2026-11-02T08:00:00Z', end: '2026-11-02T10:00:00Z'}]},
  unsorted: {...W, state: 'known', busy: [{start: '2026-11-02T13:00:00Z', end: '2026-11-02T14:00:00Z'}, {start: '2026-11-02T10:00:00Z', end: '2026-11-02T11:00:00Z'}]},
  touching: {...W, state: 'known', busy: [{start: '2026-11-02T10:00:00Z', end: '2026-11-02T11:00:00Z'}, {start: '2026-11-02T11:00:00Z', end: '2026-11-02T12:00:00Z'}]},
  extraMember: {...W, state: 'known', busy: [], calendars: ['primary@example.com']},
  freeState: {...W, state: 'free', busy: []},
};
const locations = {
  google: 'https://accounts.google.com/o/oauth2/v2/auth?client_id=x',
  microsoft: 'https://login.microsoftonline.com/common/oauth2/v2.0/authorize?client_id=x',
  attacker: 'https://accounts.google.com.attacker.example/o/oauth2/v2/auth',
  plain: 'http://accounts.google.com/o/oauth2/v2/auth',
};

beforeAll(async () => {
  if (!['webkit', 'chrome'].includes(engine) || !modules?.startsWith('/')) throw new Error('PRIMITIVE_BROWSER_ENGINE and emitted PRIMITIVE_BROWSER_MODULES are required');
  const exchange = await readFile(`${modules}/exchange.js`);
  const timematch = await readFile(`${modules}/timematch.js`);
  server = Bun.serve({hostname: '127.0.0.1', port: 0, async fetch(request) {
    const url = new URL(request.url);
    const [, area, kind, scenario] = url.pathname.split('/');
    if (url.pathname === '/') return new Response('<!doctype html><title>TimeMatch proof</title>', {headers: {'Content-Type': 'text/html'}});
    if (url.pathname === '/exchange.js') return new Response(exchange, {headers: {'Content-Type': 'text/javascript'}});
    if (url.pathname === '/timematch.js') return new Response(timematch, {headers: {'Content-Type': 'text/javascript'}});
    if (area !== 'api') return new Response('missing', {status: 404});
    const body = await request.json();
    received.push({kind, scenario, body});
    if (kind === 'availability') return Response.json(availability[scenario]);
    if (kind === 'connection') return Response.json({location: locations[scenario]});
    if (kind === 'apple') return Response.json({state: 'connected'});
    return new Response('missing', {status: 404});
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

async function pageCall(input) {
  const spyCalls = [];
  const exchange = input.spyDocument ? {sendJSON: async (request) => { spyCalls.push(request.path); return input.spyDocument; }} : await import('/exchange.js');
  const {createTimeMatchClient, TimeMatchErrorIdentity} = await import('/timematch.js');
  const realFetch = globalThis.fetch;
  let fetches = 0;
  globalThis.fetch = (...args) => { fetches++; return realFetch(...args); };
  try {
    const routes = {availability: `/api/availability/${input.scenario}`, connection: `/api/connection/${input.scenario}`, apple: `/api/apple/${input.scenario}`, ...input.routes};
    const client = createTimeMatchClient({exchange, routes});
    const value = await client[input.method](input.args);
    return {ok: value, fetches, spyCalls, storage: localStorage.length + sessionStorage.length, cookie: document.cookie, identities: TimeMatchErrorIdentity};
  } catch (error) {
    return {identity: error.primitiveIdentity, fetches, identities: TimeMatchErrorIdentity};
  } finally {
    globalThis.fetch = realFetch;
  }
}

async function run(input) {
  return view.evaluate(`(${pageCall.toString()})(${JSON.stringify(input)})`);
}

test(`${engine}: known availability is admitted exactly and frozen`, async () => {
  const before = received.length;
  const got = await run({method: 'availability', scenario: 'known', args: W});
  expect(got.ok).toEqual(availability.known);
  expect(got.fetches).toBe(1);
  expect(received.slice(before)).toEqual([{kind: 'availability', scenario: 'known', body: W}]);
});

test(`${engine}: unknown availability carries no busy facts`, async () => {
  const got = await run({method: 'availability', scenario: 'unknown', args: W});
  expect(got.ok).toEqual(availability.unknown);
});

for (const scenario of ['unknownWithBusy', 'foreignWindow', 'outside', 'unsorted', 'touching', 'extraMember', 'freeState']) {
  test(`${engine}: availability ${scenario} is a response refusal`, async () => {
    const got = await run({method: 'availability', scenario, args: W});
    expect(got.ok).toBeUndefined();
    expect(got.identity).toBe(got.identities.response);
  });
}

test(`${engine}: empty requested window is refused before any request`, async () => {
  const before = received.length;
  const got = await run({method: 'availability', scenario: 'known', args: {start: W.start, end: W.start}});
  expect(got.identity).toBe(got.identities.contract);
  expect(got.fetches).toBe(0);
  expect(received.length).toBe(before);
});

for (const [scenario, provider] of [['google', 'google'], ['microsoft', 'microsoft']]) {
  test(`${engine}: ${provider} consent location on its documented host`, async () => {
    const got = await run({method: 'beginConnection', scenario, args: {provider}});
    expect(got.ok).toBe(locations[scenario]);
  });
}

for (const scenario of ['attacker', 'plain']) {
  test(`${engine}: consent location ${scenario} is a response refusal`, async () => {
    const got = await run({method: 'beginConnection', scenario, args: {provider: 'google'}});
    expect(got.identity).toBe(got.identities.response);
  });
}

test(`${engine}: apple has no oauth consent location`, async () => {
  const got = await run({method: 'beginConnection', scenario: 'google', args: {provider: 'apple'}});
  expect(got.identity).toBe(got.identities.contract);
  expect(got.fetches).toBe(0);
});

test(`${engine}: apple password is sent once and left nowhere in the browser`, async () => {
  const before = received.length;
  const got = await run({method: 'connectApple', scenario: 'ok', args: {account: 'visitor@icloud.com', password: 'abcd-efgh-ijkl-mnop'}});
  expect(got.ok).toBe('connected');
  expect(got.storage).toBe(0);
  expect(got.cookie).not.toContain('abcd-efgh');
  expect(received.slice(before)).toEqual([{kind: 'apple', scenario: 'ok', body: {account: 'visitor@icloud.com', password: 'abcd-efgh-ijkl-mnop'}}]);
});

test(`${engine}: foreign route is refused when the client is created`, async () => {
  const got = await run({method: 'availability', scenario: 'known', args: W, routes: {availability: '//attacker.example/x'}});
  expect(got.identity).toBe(got.identities.contract);
  expect(got.fetches).toBe(0);
});

test(`${engine}: timematch performs HTTP only through the injected exchange`, async () => {
  const got = await run({method: 'availability', scenario: 'known', args: W, spyDocument: availability.known});
  expect(got.ok).toEqual(availability.known);
  expect(got.fetches).toBe(0);
  expect(got.spyCalls).toEqual(['/api/availability/known']);
});
