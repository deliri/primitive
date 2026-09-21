# Exclusive directory creation and custody reads

The proposed v2026.1.98 change adds two narrow filesystem effects. Products
continue to own directory names, evidence policy, and upgrade decisions.

`filestore.CreateDirectory` consumes the existing `DirectoryRequest`. It creates
only the final directory under an existing parent, refuses every occupied name,
sets the requested permission bits, and synchronizes the directory and parent.
A native failure after creation may leave that directory. The caller receives
the error; Primitive does not erase the partial effect or invent rollback.

`filestore.OpenCustodyRead` consumes the existing `ReadHandleRequest`. It reuses
the custody acquisition already used by `Touch` and `ConfirmDurable`: refuse a
nonregular final entry, open a regular file, and compare observed inode identity.
The caller owns the real Go file handle and coordinates namespace changes.
This does not change `OpenRead`, which may follow confined symbolic links.

The changed layer is native filesystem acquisition/effect execution. Existing
request validators and nominal path types remain the shared schema. No product
state, classifier, transport, evidence manifest, or new production struct is
introduced. Consumer migration and consumer integration proof remain separate
work after publication; these package tests do not prove a completed upgrade.

Tests exercise real temporary directories, read-only handles, inode identity,
exact bytes, permission preservation, exclusive ownership under contention,
missing parents, cancellation, missing capabilities, and occupied/nonregular
names. Standard-library filesystem observations serve as independent oracles.
The public-ingress inventory binds each new door to its semantic fuzz target.
These narrow adapters reuse the existing validation domain; their tables name
distinct effect and refusal classes rather than padding policy-parser quotas.

Development runs are retained under
`/private/tmp/peachfuzz-live-20260920/primitive-filestore-198-*`.
The first focused run passed. Two retained Go-overlay mutations fail behavioral
tests: replacing exclusive creation with `EnsureDirectory`, and replacing custody
acquisition with `OpenRead`. Neither mutation modifies the working tree.

The first full filestore run failed the Temporal ownership source ratchet because
the new contention test used `context.WithTimeout`. The test now uses the
package's existing typed filesystem backstop. The failure is retained alongside
the follow-up execution, never replaced by it.

Initial analyzer attempts retain sandbox cache/network failures. The initially
installed Witness binary was a newer dirty local build and reported four
preexisting parameter-count findings in content copying/sorting. The repository
gate pins Witness `v0.0.0-20260803211814-57582de85018`; that exact pinned linter
passes the changed package. Its installation attempts are not test passes.
Dead-code checking explicitly filters the selected filestore package because
unrelated dependency APIs are not roots of a filestore-only test executable.

Execution records are development facts, not independent acceptance. Publication,
consumer adoption, and the remaining product work must be reported separately.

Final development evidence is indexed in
`filestore-exclusive-custody-executions.json`. The Mac native suite passed 5,508
test events; its two race/shuffle repetitions passed 11,016. Furnace's initial
run exposed an existing oracle that accidentally cleared inherited setgid bits
when predicting a no-effect directory operation. The corrected oracle compares
preserved modes against the observed original entry, and uses a native chmod
oracle only for the branch that applies permissions. Furnace then passed all
5,508 events with race detection at committed revision `481f8a0`.

The Mac race run began at `3c637fd` and overlapped that test-only oracle edit in
the live source tree. It is not represented as an exact final-revision receipt.
The changed oracle separately passed on the Mac with race detection, and the
full Furnace run used the clean final source commit. Production files did not
change between those commits.

Both new semantic fuzz campaigns passed on each host. Each configured two
seconds of fuzz work and at most one second of minimization; actual process
durations, including compilation/startup/corpus work, remain in the records.
No benchmark improvement or native Windows execution is claimed. The final
pinned doctrine, vet, staticcheck, and vulnerability checks passed. Other
selected package gates and their initial failures are retained in the index.
