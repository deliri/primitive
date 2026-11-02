// Primitive Exchange owns browser HTTP execution. Native fetch performs the
// request; this executor admits only same-origin absolute paths with JSON
// bodies, refuses redirects, never caches, and never surfaces a failure body.
// Generated error identities come from Primitive Core.
export const BrowserExchangeErrorIdentity = Object.freeze({
  contract: __CONTRACT_IDENTITY__,
  response: __RESPONSE_IDENTITY__,
  contentType: __CONTENT_TYPE_IDENTITY__,
  cancelled: __CANCELLED_IDENTITY__,
  transport: __TRANSPORT_IDENTITY__,
});

const METHODS = Object.freeze(['GET', 'POST', 'PUT', 'PATCH', 'DELETE']);

function exchangeError(identity, status, cause) {
  const error = new Error('Browser exchange refused', cause === undefined ? undefined : {cause});
  error.primitiveIdentity = identity;
  if (status !== undefined) error.status = status;
  return error;
}

function admittedPath(path) {
  if (typeof path !== 'string' || !path.startsWith('/') || path.startsWith('//') || path.includes('\\') || path.includes('#')) return false;
  const resolved = new URL(path, globalThis.location.origin);
  return resolved.origin === globalThis.location.origin;
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
export async function sendJSON({path, method, body, expectedStatus, signal} = {}) {
  if (!admittedPath(path) || !METHODS.includes(method) || !Number.isInteger(expectedStatus) || expectedStatus < 200 || expectedStatus > 299 ||
      (signal !== undefined && !(signal instanceof AbortSignal)) || (method === 'GET' && body !== undefined)) {
    throw exchangeError(BrowserExchangeErrorIdentity.contract);
  }
  const init = {method, signal, redirect: 'error', credentials: 'same-origin', cache: 'no-store', referrerPolicy: 'same-origin', headers: {Accept: 'application/json'}};
  if (body !== undefined) {
    init.body = JSON.stringify(body);
    init.headers['Content-Type'] = 'application/json';
  }
  let response;
  try {
    response = await globalThis.fetch(new URL(path, globalThis.location.origin), init);
  } catch (cause) {
    if (signal?.aborted || cause?.name === 'AbortError') throw exchangeError(BrowserExchangeErrorIdentity.cancelled, undefined, cause);
    throw exchangeError(BrowserExchangeErrorIdentity.transport, undefined, cause);
  }
  if (response.redirected || response.type === 'opaqueredirect' || response.status !== expectedStatus) {
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
