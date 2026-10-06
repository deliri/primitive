# Primitive v2026.1.110 — typed image resizing and observed dimensions

Parent: v2026.1.109, `951d6d3a6b91915a0880897b67b730f487736861`.

## Contracts

`ImageResizeRequest` carries one native Hosted Images transform through closed
fit, format and metadata enums. Its source binds an HTTPS origin, public account
hash and image ID. `ImageDetails.PublicResize` requires matching public,
non-draft provider metadata. The caller owns rendition names, breakpoints,
account configuration and completion policy. Primitive adds no pixel ceiling.

`ImagesClient.InspectResize` performs a credential-free metadata GET through
Exchange. An unbuffered pipe feeds JSON v2 and joins its decoder on every exit.
The SDK requests `format=json,anim=false`; no raster body is downloaded or
decoded. It returns actual output dimensions and original dimensions, format
and byte length. Original bytes are not rendition bytes; requested maximum
dimensions are not observations. Automatic encoding is not an assertion that
AVIF was returned. The observed response remains authoritative.

Core owns shared `PixelDimension` and `ImageDimensions`. Zero observed axes
refuse; a resize intent may omit one axis. The full unsigned native pixel
representation is admitted, leaving provider refusals to the provider.

## Development proof and accounting

Evidence root: `/private/tmp/primitive-cache-evidence-20261005`. Runs retain
commands, parent revision, dirty-source manifest, toolchain, stdout/stderr
digests, exit status and test event counts. None is independent acceptance.

- `image-resize-contract-01` and `-02` failed compilation because tests named
  nonexistent error/status helpers. `-03` passed 107 selected events after
  those test construction errors were corrected.
- `image-resize-packages-01` and `image-resize-race-01` each passed 564
  Cloudflare test events, with no failures/skips. The latter used race
  detection, shuffling and `-count=1` after moving dimensions into Core.
- `image-resize-mutation-red-01` was blocked by sandbox cache permissions.
  `-02` deliberately removed zero-dimension refusal through a Go overlay;
  all four invalid axes failed (19 passing and 5 failing test events,
  including the parent). Production did not retain the mutation.
- `image-resize-live-01` could not compile in the sandbox. `-02` executed four
  read-only metadata requests against an existing public image. Requested
  widths 160, 640, 1280 and 3840 returned 160x199, 640x799, 1122x1402 and
  1122x1402. Original facts remained PNG, 1122x1402, 754168 bytes. These are
  four SDK observations, not four Go tests, and prove no browser rendering.
- `image-resize-fuzz-info-01` was blocked by cache permissions. `-02` and
  `image-resize-fuzz-inspect-01` passed semantic fuzzing, each with a
  three-second execution/minimization budget and two workers.
- `image-resize-owners-01` passed 2131 and failed 2 events: missing Core
  documentation and a missing pixel constructor. `-02` passed all 2138
  events after adding the documented constructor and ownership inventories.

Exact committed verification is retained separately. Kernel propagation,
responsive HTML, device evidence, account cache lifetime, repository-wide
lint gates and independent acceptance remain separate work.
