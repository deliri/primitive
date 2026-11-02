// Primitive Temporal owns browser time interpretation. RFC3339 agrees with
// Instant's four-digit, explicit-zone, signed-nanosecond profile. Native Date
// interprets whole UTC seconds only; it never receives a fraction to round.
// Native contract: https://tc39.es/ecma262/#sec-date-time-string-format
export const TemporalErrorIdentity = Object.freeze({contract: __TEMPORAL_CONTRACT_IDENTITY__});

function temporalBrowserRefusal() {
  const error = new Error('Primitive time refused');
  error.primitiveIdentity = TemporalErrorIdentity.contract;
  throw error;
}

/** Parse one exact RFC3339 instant into signed Unix nanoseconds as BigInt. */
export function parseRFC3339Nanoseconds(text) {
  if (typeof text !== 'string' || text.length < 20 || text.length > 35) temporalBrowserRefusal();
  const parsed = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?(Z|[+-]\d{2}:\d{2})$/.exec(text);
  if (!parsed || parsed[0] !== text) temporalBrowserRefusal();
  const [, wall, fraction = '', zone] = parsed;
  const milliseconds = Date.parse(`${wall}Z`);
  // Refuse native normalization, including impossible leap dates and 24:00.
  if (!Number.isFinite(milliseconds) || new Date(milliseconds).toISOString().slice(0, 19) !== wall) temporalBrowserRefusal();
  let offset = 0n;
  if (zone !== 'Z') {
    const hours = Number(zone.slice(1, 3));
    const minutes = Number(zone.slice(4, 6));
    if (hours > 23 || minutes > 59) temporalBrowserRefusal();
    offset = BigInt(hours * 60 + minutes) * 60_000_000_000n;
    if (zone[0] === '-') offset = -offset;
  }
  const result = BigInt(milliseconds) * 1_000_000n + BigInt(fraction.padEnd(9, '0')) - offset;
  if (result < -(1n << 63n) || result > (1n << 63n) - 1n) temporalBrowserRefusal();
  return result;
}
