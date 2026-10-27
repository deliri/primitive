# Reviewed ByteCount admission

Witness already admits TextRecordRequest.MaximumBytes through ByteCount.Uint64.
Its validator was discarding the native refusal, which Witness now preserves.
The remaining classifier failure is missing Primitive capability knowledge:
adding a second Validate call would duplicate admission rather than repair it.

| Assumption | Evidence | Status | Consequence / smallest check |
| --- | --- | --- | --- |
| Uint64 admits its ByteCount receiver | Actual compiled method and NewByteCount tested at zero, one and maximum unsigned | VERIFIED | Advertise only that exact declaring method |
| Every similarly named method has the same contract | Foreign packages, receiver changes, selector changes and package functions lack this authority | REFUTED | Single-coordinate mutations must remove the grant |
| A catalog grant proves a caller preserves refusal | Caller error flow is a separate compiler observation | UNVERIFIED; not claimed | Witness must retain its native field/error-propagation query |
| ByteCount is valid at zero | Both constructor and conversion refuse with ErrPrimitiveContract | REFUTED | Preserve native refusal and zero result; positive full-width values survive exactly |
| This small lookup needs another registry or runtime | Existing StandardSymbol and Core package authority suffice | VERIFIED | Extend the existing pure method; add no types, state or execution |

No native Core or Temporal behavior changes. The bounded lookup uses Core's
existing authoritative package identity. The unchanged Temporal runtime proof
remains valid; its namespace controls and expanded fuzz seeds exercise the
shared lookup again. This is one reviewed symbol, not a multi-fact classifier:
padding a fifty-row matrix would add no distinct admission boundary.

## Retained development proof

Base main0650a8a630730642d7893845622f076025fdff37, including the concurrent
publisher's committed work. All execution uses installed Mac Anvil,
darwin-arm64, JSONv2, CPU1/RAM4GiB, race, recorded shuffle and count=1.
Requests and raw evidence remain outside Git in the owned scratch root
/private/tmp/witness-native-unblock-z5l6ve72. Receipt/output hashes, extents
and native event accounting are independently checked.

- Missing capability red: 2026-10-27-79df13420cdf88319444003eca64e517cdff2e5870d972dac2020838821927a9.
  Snapshot6632665ee4f22b7e3c0818ccca48b618209f7e5ab8ad7e8e5d80af93a339f900;
  four failed nodes (three count states and parent). Native admission itself
  succeeds/refuses correctly; the advertised capability is missing.
- Exact three-leaf fixed retry: 2026-10-27-d5d2fa7287ca904841d44720dd1c14cc3ea7d04c0f0afaa939a569751e5ef61f,
  four passed nodes, original seed4390879486400250904.
- Hostile symbols, existing shared controls/inventory and expanded fuzz seeds:
  2026-10-27-6679c98992e4ed5e6907d41a4ac4e043882c0909e1d19f9641da4ff331b05da3,
  twenty-seven passed nodes, no skips.
- Go vet: 2026-10-27-868b488ee88a8ab83fcc9ee60ea6878a57571407a14cc37e49013d533ab3b092.
  Staticcheck: 2026-10-27-087d13146d4a17593ccbccae1ed36110a70e4a0d39d0af71e615f9d2e6461ba1.
  Each executes once, native0, zero findings and zero retained analysis bytes.
- Serial three-second semantic fuzz:
  2026-10-27-3e0bdc40123f09c648386ce6165642426a32fc057536cbed2aada25a3a4a0bf9.
  Verified raw readback: one passed target, no skips, native reports188
  executions and1 new-interesting input. These counters do not prove unique
  mutations or exhaustive string coverage.

Fixed production/test snapshot192a23ccc22117c7694458f26e1ca96f09f51b9db456996fa300930750863c06.
No benchmark speed or independent acceptance is claimed. The downstream
Witness regression remains red until it adopts the published release.
