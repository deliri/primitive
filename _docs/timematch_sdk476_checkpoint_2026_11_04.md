# Primitive 476 source checkpoint

This publishes `c83d4e5e9c5cc92da8943908235b3a4f6f0bc05f` plus this
documentation. Kernel adoption, TimeMatch launch and live provider acceptance
are separate work.

Firestore Query and QueryIntegerJSON now retain the actual native snapshot
reference on Document, alongside the validated record. Get fills it too.
Query observations must pass the existing reference admission and remain inside
the executed query parent. Callers still own collection selection and must bind
the observed path to their configured exact record identity. Validation-only
Document constructions have no observed reference. There is no extra RPC.

The query-reference commits are `85f495ed` and `adcd6f25`. Final focused proof
passed 66 nodes without skips; the existing external projection fuzz passed
with a three-second configured budget. Three first-layout semantic controls
failed the intended refusal/reference leaves. Final vet and staticcheck are
clean. The preexisting errcheck finding and three preexisting complexity
identities remain explicit; no new finding was added. Controls were not replayed
after the final helper extraction. The author's disclosed direct local vet and
compile-only test invocation are excluded from evidence.

Automatic Google video updates now reuse the existing owned metadata lookup
and conditionally read only that exact event at the observed ETag. An admitted
Ready, Pending or Failed conference is omitted from PATCH and preserved. A Ready
receipt must keep the exact prior join URL. An absent conference still permits
explicit automatic conversion; malformed or foreign observations refuse before
PATCH. No public API, parser, event identity or scheduling machinery was added.
Google documents omitted-field preservation in its
[PATCH contract](https://developers.google.com/workspace/calendar/api/v3/reference/events/patch).

Meet RED at `19d6983f` failed 14 leaves and their parent. GREEN at `c83d4e5e`
passed all 18 selected nodes, including the changed incumbent update test.
The existing external conference fuzz passed at three seconds: 450 executions
and 67 new interesting inputs. Final vet, staticcheck and errcheck are clean;
the 32 remaining complexity identities are inherited and unwaived. Readback
and recovery behavior was extracted unchanged and its existing proof reused.

Retained implementation indexes:

- `/private/tmp/primitive-docref-proof/PROOF-INDEX.json`, SHA256
  `541f23feab1c88840896979bee20b80c7486523540f53d0a04fb36b0fcffbc50`.
- `/private/tmp/timematch-sdk-meet-update-proof-index.json`, SHA256
  `4141888cd0f6dd50b9758da185909c8148cb9b83d8defc43172f320936c6758d`.

Root independently compared source and original receipt bytes, seals and
execution details. Two older locally absent query analysis parent originals
were recovered through Anvil without native replay. Root reviews:
`/private/tmp/timematch-root-docref-originals-review-20261104.json` (35f47cff),
`/private/tmp/timematch-root-docref-final-analysis-review-20261104.json`, and
`/private/tmp/timematch-root-meet-sdk-originals-review-20261104.json` (aadbf358).
The Meet index actually lists eight original pairs and sixteen positive reader
joins, including RED; the author's seven/fourteen prose count was inaccurate.

All provider execution used controlled local peers. Live Workspace may omit
conference creation metadata on later reads; such an observation refuses as
indeterminate. Live omission-preserving PATCH behavior was not exercised.
Prior SDK475 qualifications remain. No production provider write occurred.
