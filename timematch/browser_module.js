// Primitive TimeMatch browser client. HTTP is performed only by the injected
// Primitive Exchange browser executor (exchange.BrowserModule sendJSON); this
// module calls no platform HTTP API and stores nothing.
// Provider credentials, calendar identifiers and event details never reach it.
export const TimeMatchErrorIdentity = Object.freeze({
  contract: __CONTRACT_IDENTITY__,
  response: __RESPONSE_IDENTITY__,
});

const CONSENT_HOSTS = Object.freeze({google: 'accounts.google.com', microsoft: 'login.microsoftonline.com'});

function timeMatchError(identity, cause) {
  const error = new Error('TimeMatch refused', cause === undefined ? undefined : {cause});
  error.primitiveIdentity = identity;
  return error;
}

function contract(condition) {
  if (!condition) throw timeMatchError(TimeMatchErrorIdentity.contract);
}

function instant(text) {
  try { return parseRFC3339Nanoseconds(text); } catch { return null; }
}

function samePath(path) {
  return typeof path === 'string' && path.startsWith('/') && !path.startsWith('//');
}

// Admit the server's aggregate projection only when it answers this exact
// window, is known or unknown, and lists sorted, merged, positive busy spans
// inside the window. Unknown carries no busy facts and is never free time.
function admitAvailability(document, start, end) {
  const ok = document !== null && typeof document === 'object' && !Array.isArray(document) &&
    Object.keys(document).sort().join(',') === 'busy,end,start,state' &&
    instant(document.start) === start && instant(document.end) === end &&
    (document.state === 'known' || document.state === 'unknown') && Array.isArray(document.busy) &&
    (document.state === 'known' || document.busy.length === 0);
  if (!ok) throw timeMatchError(TimeMatchErrorIdentity.response);
  let previous = null;
  const busy = document.busy.map((span) => {
    const valid = span !== null && typeof span === 'object' && Object.keys(span).sort().join(',') === 'end,start';
    const from = valid ? instant(span.start) : null;
    const to = valid ? instant(span.end) : null;
    if (from === null || to === null || !(from < to) || from < start || to > end || previous !== null && from <= previous) throw timeMatchError(TimeMatchErrorIdentity.response);
    previous = to;
    return Object.freeze({start: span.start, end: span.end});
  });
  return Object.freeze({start: document.start, end: document.end, state: document.state, busy: Object.freeze(busy)});
}

/**
 * Create a client bound to the application's same-origin routes:
 * {availability, connection, apple}. exchange is Primitive's Exchange browser
 * module namespace; its sendJSON owns every request and AbortSignal lifetime.
 */
export function createTimeMatchClient({exchange, routes} = {}) {
  contract(exchange !== null && typeof exchange === 'object' && typeof exchange.sendJSON === 'function');
  contract(routes !== null && typeof routes === 'object' && samePath(routes.availability) && samePath(routes.connection) && samePath(routes.apple));
  const send = exchange.sendJSON;
  const paths = Object.freeze({availability: routes.availability, connection: routes.connection, apple: routes.apple});

  return Object.freeze({
    /** Busy projection for [start, end) across the visitor's connected calendars. */
    async availability({start, end, signal} = {}) {
      const from = instant(start);
      const to = instant(end);
      contract(from !== null && to !== null && from < to);
      const document = await send({path: paths.availability, method: 'POST', body: {start, end}, expectedStatus: 200, signal});
      return admitAvailability(document, from, to);
    },
    /** Provider consent location; the caller navigates the top-level window to it. */
    async beginConnection({provider, signal} = {}) {
      contract(Object.hasOwn(CONSENT_HOSTS, provider));
      const document = await send({path: paths.connection, method: 'POST', body: {provider}, expectedStatus: 200, signal});
      let location;
      try {
        contract(document !== null && typeof document === 'object' && Object.keys(document).join(',') === 'location' && typeof document.location === 'string');
        location = new URL(document.location);
      } catch (cause) { throw timeMatchError(TimeMatchErrorIdentity.response, cause); }
      if (location.protocol !== 'https:' || location.host !== CONSENT_HOSTS[provider] || location.username || location.password) throw timeMatchError(TimeMatchErrorIdentity.response);
      return location.href;
    },
    /** Send an Apple Account and app-specific password once; nothing is retained. */
    async connectApple({account, password, signal} = {}) {
      contract(typeof account === 'string' && account.length > 0 && typeof password === 'string' && password.length > 0);
      const document = await send({path: paths.apple, method: 'POST', body: {account, password}, expectedStatus: 200, signal});
      if (document === null || typeof document !== 'object' || Object.keys(document).join(',') !== 'state' || document.state !== 'connected') throw timeMatchError(TimeMatchErrorIdentity.response);
      return 'connected';
    },
  });
}
