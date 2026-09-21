# Primitive v2026.1.96 release work

The new `upgradereport` agreement carries signed candidate observations, exact evidence bindings, and server acknowledgments. Product code chooses the stage, outcome, reporting policy, and whether an upgrade completes. Primitive validates the shared structs and proves authentication, content identity, and acknowledgment binding.

`filestore.CopyContent` streams an exact declared extent through caller-owned scratch space, checks the digest and EOF, and returns the observed content identity. It does not choose a product's artifact policy or impose a total stream quota.

The cleanup also fixes report timing: two genuine authorizations with disjoint windows now return a zero result with the validation error. The retained failing run demonstrates that the previous result exposed invalid populated timing. Signed-epoch scheduling is checked against an independent arbitrary-precision arithmetic oracle.

Layout changes use keyed literals to preserve field/value associations. Twenty-two JSON declarations retain their v2026.1.95 field order because Go encoding order contributes to canonical bytes and signatures. Seven deliberately reordered hostile JSON fixtures and two positional ASN.1 oracle declarations retain their exact wire order. The layout gate checks each complete declaration, retains native analyzer diagnostics, and rejects new unlisted findings. Its native analyzer tests prove that adding unlisted layout debt or changing field order/tags fails for the intended reason.

An initial broad alignment pass changed those nine fixture declarations. Native and race runs each passed 61 packages and failed the same five fixture packages; Furnace independently reported the same five failures. These failed attempts remain evidence. Corrected fixture runs are recorded separately.

Real GCS proof uses dedicated test buckets and a keyless service account. Only newly created lifecycle fixtures are removed by those tests. The collected product data in GCS and Firestore is preserved. The first live run passed signed download and soft-delete refusal, but found that the lifecycle absence helper supplied an object name to the directory-prefix parser. The correction lists the owned test prefix, compares the exact object name, and proves it sees the existing object before checking absence.

Final verification results and artifact digests are recorded in `release_v2026.1.96_evidence.json`. These are development execution facts; independent acceptance belongs to the user.

The corrected nine-package behavioral run passed (`primitive-196-layout-recovery-01`). The second real GCS run passed all three selected tests (`primitive-196-gcs-native-02`), including positive object visibility, exact-generation deletion, short-source and wrong-digest refusal, and post-deletion absence. Executed binaries and per-attempt stdout/stderr are retained alongside source digests. The first failed live attempt is retained separately.

The final native suites passed all 66 packages on Mac and Furnace; Furnace also passed the race/shuffle run. The owner then restricted build verification to upgraded packages. The remaining broad Mac race run and broad platform compilation were interrupted and retained as cancellations. Follow-up verification covered only the changed upgrade packages: six benchmark targets and 22 fuzz targets passed, and all six selected packages compiled for Windows amd64 and Linux arm64. The witness diagnostic fix passed its follow-up check. The only source difference from the completed broad suites is the benchmark failure-message wording; its final benchmark ran on the corrected source. No further repository-wide verification is scheduled.
