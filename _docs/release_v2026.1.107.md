# Primitive v2026.1.107

Cloudflare now accepts typed intent to purge one exact URL in one zone.
`CacheServer` owns its cloned credential, bounded JSON v2 response decoding and
single HTTP attempt. Its receipt records the requested URL, zone and optional
provider operation ID. Acceptance does not establish delivery absence; product
policy must verify that separately. Redirects, cancellation, conflicting API
facts and malformed responses cannot produce a receipt.

`R2CacheControl` adds exact whole-second max-age metadata to signed PUT and
multipart-create requests. Grants carry the same metadata to native and browser
clients. Zero omits the optional field; a constructed zero duration explicitly
emits max-age=0. Read, delete and other multipart operations refuse write
metadata. The SDK admits 31 nonnegative integer bits for interoperable HTTP
delta-seconds. Kernel and distros choose actual cache lifetimes.

No upload body buffering, local delivery model, retry loop or product state
machine was added. Exchange owns HTTP and signing. Cloudflare test JSON paths
now use JSON v2; multipart's provider protocol remains XML.

## Behavioral evidence

Working-source receipts originate under
`/private/tmp/primitive-cache-evidence-20261005`, with Furnace retention at
`/data/evidence/primitive-cache-20261005`. Receipts retain the exact command,
base revision, dirty-tree fact, source hashes, output and artifact hashes.
Committed-source proof and copy verification are separate run artifacts.

`cache-release-packages-01` ran against initial checkpoint
`65dcaf48514fef913784a1334b4798c701c1a8bd`: 2,516 pass, two failures, no skips.
The broader core ratchets found six missing entries in the provider-owned
constant inventory and a missing public admission door for the purge operation
ID. The constants now have explicit compiler witnesses for Cloudflare ownership;
`ParseCachePurgeOperationID` owns identifier admission and has its own semantic
fuzz binding. No additional consumer or compatibility path was manufactured.
Attempt 02 retained a compile failure because the inventory's fixed array bound
still named 108 entries; it now names the actual 114 compiler-owned entries.
`cache-release-packages-03` then passed 2,523 test events with no failures or
skips across Cloudflare, core, Compass, version and the authored Hammer claims.

- `cache-capability-red-01`: missing SDK failed compilation.
- `cache-capability-green-01`: sandbox denied local listeners; retained failure.
  `cache-capability-green-02`: 9 passing selected test events, no skips.
- `cache-fuzz-seeds-01`: a test referenced a nonexistent header helper;
  corrected to Exchange's compiler-owned header. Attempt 02 passed 17 events.
- `cache-boundaries-01`: 35 pass, 2 fail (including parent) because the test
  expected the wrong redirect error identity. Exchange returns the refused
  response status without following it. Attempt 02 passed 41 events.
- `r2-cache-red-01`: missing cache contract failed compilation. First green
  attempt still failed compilation on a signed/unsigned duration constant;
  corrected attempt 02 passed 10 selected events.
- `cloudflare-cache-suite-01`: 392 passing events, no failures or skips.
- `cloudflare-cache-race-01`: the same 392 events passed with race detection,
  shuffled order and count=1. Neither run enables the live-provider build tag.
- Thirty-second, two-worker semantic fuzz budgets cover zone identity, purge
  responses and signed R2 cache metadata. Initial zone/response runs overlapped
  due to a runner sequencing mistake and remain retained. The explicit
  `cache-zone-fuzz-serial-02` and `cache-response-fuzz-serial-02` runs execute
  separately. `r2-cache-fuzz-01` ran separately and passed 254,433 executions.

This SDK checkpoint does not close Kernel's cached-PDF deletion defect. Bridge
integration, scoped credentials, public absence observations, actual CDN cache
headers and native browser journeys remain required. Repository-wide lint and
application gates are deferred to the user's final verification phase.

Provider contracts:
[exact-URL purge](https://developers.cloudflare.com/api/resources/cache/methods/purge/),
[R2 metadata support](https://developers.cloudflare.com/r2/api/s3/api/),
[HTTP delta-seconds](https://www.rfc-editor.org/rfc/rfc9111.html#section-1.2.2).
