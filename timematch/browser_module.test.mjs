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


// Temporal owns every browser instant; these run in the same real engine as
// TimeMatch and exercise the actual Go-emitted module, not a copied parser.
const baseNanos = 1793631600000000000n;
const hourNanos = 3600000000000n;
const minuteNanos = 60000000000n;
for (const row of [
  {name: 'epoch is a set zero instant', text: '1970-01-01T00:00:00Z', want: 0n},
  {name: 'one nanosecond survives', text: '1970-01-01T00:00:00.000000001Z', want: 1n},
  {name: 'one nanosecond before epoch survives', text: '1969-12-31T23:59:59.999999999Z', want: -1n},
  {name: 'one below millisecond stays distinct', text: '1970-01-01T00:00:00.000999999Z', want: 999999n},
  {name: 'exact millisecond remains exact', text: '1970-01-01T00:00:00.001Z', want: 1000000n},
  {name: 'one above millisecond stays distinct', text: '1970-01-01T00:00:00.001000001Z', want: 1000001n},
  {name: 'minimum signed nanoseconds', text: '1677-09-21T00:12:43.145224192Z', want: -(1n << 63n)},
  {name: 'maximum signed nanoseconds', text: '2262-04-11T23:47:16.854775807Z', want: (1n << 63n) - 1n},
  {name: 'Toronto winter offset', text: '2026-11-02T10:00:00-05:00', want: baseNanos},
  {name: 'German winter offset', text: '2026-11-02T16:00:00+01:00', want: baseNanos},
  {name: 'Kathmandu quarter hour offset', text: '2026-11-02T20:45:00+05:45', want: baseNanos},
  {name: 'Chatham quarter hour offset', text: '2026-11-03T04:45:00+13:45', want: baseNanos},
  {name: 'international date boundary east', text: '2026-11-03T05:00:00+14:00', want: baseNanos},
  {name: 'international date boundary west', text: '2026-11-02T03:00:00-12:00', want: baseNanos},
  {name: 'RFC positive offset ceiling', text: '2026-11-03T14:59:00+23:59', want: baseNanos},
  {name: 'RFC negative offset ceiling', text: '2026-11-01T15:01:00-23:59', want: baseNanos},
  {name: 'first Toronto repeated clock', text: '2026-11-01T01:30:00-04:00', want: baseNanos - 33n*hourNanos - 30n*minuteNanos},
  {name: 'second Toronto repeated clock', text: '2026-11-01T01:30:00-05:00', want: baseNanos - 32n*hourNanos - 30n*minuteNanos},
  {name: 'explicit negative zero offset', text: '2026-11-02T15:00:00-00:00', want: baseNanos},
  {name: 'nine fraction digits retained', text: '2026-11-02T15:00:00.123456789Z', want: baseNanos+123456789n},
  {name: 'one below signed minimum', text: '1677-09-21T00:12:43.145224191Z'},
  {name: 'one above signed maximum', text: '2262-04-11T23:47:16.854775808Z'},
  {name: 'impossible February date never normalizes', text: '2026-02-30T00:00:00Z'},
  {name: 'nonleap February never normalizes', text: '2025-02-29T00:00:00Z'},
  {name: 'hour twenty four never normalizes', text: '2026-11-02T24:00:00Z'},
  {name: 'minute sixty refused', text: '2026-11-02T15:60:00Z'},
  {name: 'leap second refused consistently with Instant', text: '2026-11-02T15:00:60Z'},
  {name: 'offset twenty four refused', text: '2026-11-02T15:00:00+24:00'},
  {name: 'offset minute sixty refused', text: '2026-11-02T15:00:00+00:60'},
  {name: 'ten fraction digits cannot round', text: '2026-11-02T15:00:00.1234567890Z'},
  {name: 'timezone missing refused', text: '2026-11-02T15:00:00'},
  {name: 'timezone lowercase refused', text: '2026-11-02T15:00:00z'},
  {name: 'leading whitespace refused', text: ' 2026-11-02T15:00:00Z'},
  {name: 'trailing newline refused', text: '2026-11-02T15:00:00Z' + String.fromCodePoint(10)},
  {name: 'empty text refused', text: ''},
  {name: 'null is no timestamp', text: null},
  {name: 'number is no timestamp', text: 0},
  {name: 'unknown source object refused', text: {time:'2026-11-02T15:00:00Z'}},
]) {
  test(`${engine}: Temporal ${row.name}`, async () => {
    const got = await view.evaluate(`(async () => {
      const {parseRFC3339Nanoseconds, TemporalErrorIdentity} = await import('/timematch.js');
      try { return {value: parseRFC3339Nanoseconds(${JSON.stringify(row.text)}).toString()}; }
      catch(error) { return {refused: error.primitiveIdentity === TemporalErrorIdentity.contract}; }
    })()`);
    if (row.want !== undefined) expect(got).toEqual({value:row.want.toString()});
    else expect(got).toEqual({refused:true});
  });
}

const nanoWindow = {start: '2026-11-02T15:00:00.000000001Z',end: '2026-11-02T15:00:00.000000009Z'};
for (const row of [
  {name:'one nanosecond busy interval remains occupied', busy:[{start:'2026-11-02T15:00:00.000000003Z',end:'2026-11-02T15:00:00.000000004Z'}],valid:true},
  {name:'busy starts before window by one nanosecond',busy:[{start:'2026-11-02T15:00:00Z',end:'2026-11-02T15:00:00.000000004Z'}]},
  {name:'busy ends after window by one nanosecond',busy:[{start:'2026-11-02T15:00:00.000000003Z',end:'2026-11-02T15:00:00.000000010Z'}]},
  {name:'nanosecond zero-length busy is refused',busy:[{start:'2026-11-02T15:00:00.000000003Z',end:'2026-11-02T15:00:00.000000003Z'}]},
  {name:'same-millisecond foreign window is refused',busy:[],responseStart:'2026-11-02T15:00:00.000000002Z'},
]){
 test(`${engine}: TimeMatch ${row.name}`,async()=>{
  const document={...nanoWindow,start:row.responseStart??nanoWindow.start,state:'known',busy:row.busy};
  const got=await run({method:'availability',scenario:'nanoseconds',args:nanoWindow,spyDocument:document});
  if(row.valid)expect(got.ok).toEqual(document);
  else expect(got.identity).toBe(got.identities.response);
 });
}
