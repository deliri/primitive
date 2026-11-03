# TimeMatch recovery authority, 2026-11-03

SDK468 incorrectly changed recover-only into a guarded create on Google and Apple. Kernel's existing read-only safety tests caught the adoption regression. This cut restores that contract without weakening those tests or losing explicit recovery for an unapplied deterministic create.

`EventCreateRecoverOnly` performs only reads. Missing or unavailable evidence returns zero receipt and Indeterminate. `EventCreateGuardedRecovery` is distinct explicit authority to repeat the same current intent under provider create-only safeguards. It cannot replace a Google or Apple event, and it remains read-only on Microsoft, whose create contract lacks the corresponding durable guard. The caller must not grant this authority after booking or consent retirement.

Native local-TLS provider proof: c6be6325 RED observes four unintended Google/Apple writes; proper source269addfa closes the read-only cases. Existing guarded-collision tests keep their exact ownership and request-count assertions and select the newly explicit authority. Their initial failed assertions and the partial test-edit retry are retained, not discarded. Remaining failed-only a4a82587 passes3 nodes. The extended creation-attempt domain passes its exact2-node enum selection; the3-second semantic decoder fuzz passes (retained execution count unavailable).

A genuine Microsoft one-fact control371946e3 dispatches a second create and failscb4afc7a; restoring proper bytes passes76bfa99a. The control is not a main ancestor. Final timematch vet, staticcheck and Witness each report zero findings, indexed by41465078. Original receipts, executions, SHA seals, exact requests and positive reader/author joins remain under `/private/tmp/primitive-recovery-*`.

This is SDK source/native proof, not live provider verification or a finished Kernel booking service. Kernel adoption and its failed-only consumer tests must close separately before a Kernel release or Offgrid sync.
