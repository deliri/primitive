# Primitive v2026.1.112 — exact held-file metadata observation

Parent: v2026.1.111, `5bc71ba475f338c9a7107b8547391919fcf427be`.

`filestore.InspectOpenFile` accepts a validated `HandleInspectionRequest` and
returns the existing typed `Inspection` observation. It interrogates the exact
borrowed native descriptor, reads no content, changes no offset, and owns no
closure. A replaced or removed filename does not substitute another subject.
Nil contexts, cancellation and absent handles refuse before execution. Closed
handles retain the native source failure. The caller owns concurrency and the
borrowed handle's lifetime. No pathname cache, simulated file state, alternate
observation model or compatibility path is introduced.

This supplies the missing metadata execution boundary for products already
holding Primitive-issued file handles. Per-file content extent does not affect
the observation's retained working memory.

Development evidence is retained under
`_docs/work/2026-10-06/held-file-inspection/`. The initial red state was the
absent public API. An overlay substituting `os.Stat(handle.Name())` for the
descriptor observation failed the actual inode-replacement regression.
Three-second semantic fuzzing with two workers passed 174 executions against
native file extents. The new request is classified in the typed struct inventory
and the public ingress is bound to its semantic fuzzer.

Complete Filestore/capability race runs, final targeted gates and exact committed
verification are accounted separately. This document does not issue independent
acceptance or claim repository-wide Primitive verification.
