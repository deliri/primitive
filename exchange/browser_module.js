// Primitive Exchange owns browser HTTP execution. Native fetch performs the
// request; the default executor admits only same-origin absolute paths with JSON
// bodies, never follows a redirect (manual mode, refused as redirect), never caches, and never surfaces a failure body.
// Generated error identities come from Primitive Core.
export const BrowserExchangeErrorIdentity = Object.freeze({
  contract: __CONTRACT_IDENTITY__,
  response: __RESPONSE_IDENTITY__,
  contentType: __CONTENT_TYPE_IDENTITY__,
  cancelled: __CANCELLED_IDENTITY__,
  transport: __TRANSPORT_IDENTITY__,
  redirect: __REDIRECT_IDENTITY__,
});

const METHODS = Object.freeze(['GET', 'POST', 'PUT', 'PATCH', 'DELETE']);

function exchangeError(identity, status, cause) {
  const error = new Error('Browser exchange refused', cause === undefined ? undefined : {cause});
  error.primitiveIdentity = identity;
  if (status !== undefined) error.status = status;
  return error;
}

function admittedPath(path, origin) {
  if (typeof path !== 'string' || !path.startsWith('/') || path.startsWith('//') || path.includes('\\') || path.includes('#')) return false;
  const resolved = new URL(path, origin);
  return resolved.origin === origin;
}

const IDEMPOTENCY_HEADER = '__IDEMPOTENCY_HEADER__';
const BROWSER_OWNED_HEADERS = Object.freeze(['accept', 'accept-charset', 'accept-encoding', 'access-control-request-headers', 'access-control-request-method', 'authorization', 'connection', 'content-length', 'content-type', 'cookie', 'cookie2', 'date', 'dnt', 'expect', 'host', 'origin', 'referer', 'set-cookie', 'te', 'trailer', 'transfer-encoding', 'upgrade', 'user-agent', 'via', IDEMPOTENCY_HEADER.toLowerCase()]);

function configuredOrigin(value) {
  if (typeof value !== 'string' || value !== value.trim()) throw exchangeError(BrowserExchangeErrorIdentity.contract);
  let origin;
  try { origin = new URL(value); } catch { throw exchangeError(BrowserExchangeErrorIdentity.contract); }
  if (!['https:', 'http:'].includes(origin.protocol) || origin.username || origin.password || origin.pathname !== '/' || origin.search || origin.hash) throw exchangeError(BrowserExchangeErrorIdentity.contract);
  const loopback = origin.hostname === 'localhost' || origin.hostname.endsWith('.localhost') || origin.hostname === '127.0.0.1' || origin.hostname === '[::1]';
  if (origin.protocol !== 'https:' && origin.origin !== globalThis.location.origin && !loopback) throw exchangeError(BrowserExchangeErrorIdentity.contract);
  return origin.origin;
}

function configuredCSRFHeader(value) {
  if (value === undefined) return undefined;
  if (typeof value !== 'string' || !/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(value)) throw exchangeError(BrowserExchangeErrorIdentity.contract);
  const lower = value.toLowerCase();
  if (BROWSER_OWNED_HEADERS.includes(lower) || lower.startsWith('proxy-') || lower.startsWith('sec-')) throw exchangeError(BrowserExchangeErrorIdentity.contract);
  return value;
}

function headerValue(value) {
  if (typeof value !== 'string' || value.trim() === '' || /[\u0000-\u001f\u007f]/.test(value)) throw exchangeError(BrowserExchangeErrorIdentity.contract);
  return value;
}

/** Bind one trusted application API origin and its CSRF header. Requests can
 * choose only paths on that origin. Mutation headers are fresh per request;
 * this capability retains no tokens, cookies, request bodies or retry keys.
 * Native CORS, cookie policy, TLS and fetch remain the transport authority.
 */
export function createBrowserExchange({origin = globalThis.location.origin, csrfHeader} = {}) {
  const target = configuredOrigin(origin);
  const header = configuredCSRFHeader(csrfHeader);
  return Object.freeze({sendJSON: (request) => executeJSON(target, header, request)});
}

function jsonMedia(value) {
  return typeof value === 'string' && value.split(';')[0].trim().toLowerCase() === 'application/json';
}

/**
 * Send one JSON request to the application's own origin and return the
 * parsed JSON value of a response with exactly the expected status. The
 * caller's AbortSignal is the request lifetime. Any other status rejects with
 * {primitiveIdentity: response, status} and the body is discarded unread.
 */
export async function sendJSON(request = {}) {
  return executeJSON(globalThis.location.origin, undefined, request);
}

async function executeJSON(origin, csrfHeader, {path, method, body, expectedStatus, signal, csrfToken, idempotencyKey} = {}) {
  if (!admittedPath(path, origin) || !METHODS.includes(method) || !Number.isInteger(expectedStatus) || expectedStatus < 200 || expectedStatus > 299 ||
      (signal !== undefined && !(signal instanceof AbortSignal)) || (method === 'GET' && (body !== undefined || csrfToken !== undefined || idempotencyKey !== undefined)) ||
      (csrfToken !== undefined && csrfHeader === undefined)) {
    throw exchangeError(BrowserExchangeErrorIdentity.contract);
  }
  const init = {method, signal, redirect: 'manual', credentials: 'same-origin', cache: 'no-store', referrerPolicy: 'same-origin', headers: {Accept: 'application/json'}};
  if (origin !== globalThis.location.origin) init.credentials = 'include';
  if (csrfToken !== undefined) init.headers[csrfHeader] = headerValue(csrfToken);
  if (idempotencyKey !== undefined) init.headers[IDEMPOTENCY_HEADER] = headerValue(idempotencyKey);
  if (body !== undefined) {
    try { init.body = JSON.stringify(body); } catch { throw exchangeError(BrowserExchangeErrorIdentity.contract); }
    if (init.body === undefined) throw exchangeError(BrowserExchangeErrorIdentity.contract);
    init.headers['Content-Type'] = 'application/json';
  }
  let response;
  try {
    response = await globalThis.fetch(new URL(path, origin), init);
  } catch (cause) {
    if (signal?.aborted || cause?.name === 'AbortError') throw exchangeError(BrowserExchangeErrorIdentity.cancelled, undefined, cause);
    throw exchangeError(BrowserExchangeErrorIdentity.transport, undefined, cause);
  }
  if (response.type === 'opaqueredirect' || response.redirected) {
    try { await response.body?.cancel(); } catch {}
    throw exchangeError(BrowserExchangeErrorIdentity.redirect, response.status);
  }
  if (response.status !== expectedStatus) {
    try { await response.body?.cancel(); } catch {}
    throw exchangeError(BrowserExchangeErrorIdentity.response, response.status);
  }
  if (!jsonMedia(response.headers.get('Content-Type'))) {
    try { await response.body?.cancel(); } catch {}
    throw exchangeError(BrowserExchangeErrorIdentity.contentType, response.status);
  }
  let text;
  try { text = await response.text(); }
  catch (cause) {
    if (signal?.aborted) throw exchangeError(BrowserExchangeErrorIdentity.cancelled, undefined, cause);
    throw exchangeError(BrowserExchangeErrorIdentity.transport, response.status, cause);
  }
  try { return JSON.parse(text); }
  catch (cause) { throw exchangeError(BrowserExchangeErrorIdentity.response, response.status, cause); }
}
