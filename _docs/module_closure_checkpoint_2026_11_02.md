# Module-closure SDK checkpoint

Primitive v2026.1.466 adds gotoolchain.Capability.ObserveModuleClosure.
It streams native module selection and lends verified selected cache entries
and available frozen go.sum-pinned go.mod metadata synchronously. Selected
trees, module files and ziphash commitments must agree with the caller’s
frozen expected h1 checksums. Native Go verification does not override them.
Missing historical metadata is omitted; replacement modules are refused.

The caller owns mutation exclusion, byte freezing, complete transport
accounting and isolated offline graph replay. This release does not deliver
Anvil’s source-TAR adoption, shared-cache integration or a speed improvement.

Scoped proof records 75 unique passing nodes including six fuzz seeds,
66 terminal leaves, and two deliberately faulty source controls that fail.
Actual 3-second fuzz campaigns report 4805 and 6569 executions separately;
they do not establish unique or cumulative coverage. Changed custody paths
pass 23 affected nodes, including FIFO and symbolic-link refusal.

Scoped vet and staticcheck report zero findings. Witness reports zero
findings in the new files; 273 findings in unchanged files remain open.
Earlier compilation, EOF, cleanup and unavailable attempts remain retained.

The source delivery and original receipt bindings are recorded in the owning
Anvil module-cache adoption proof under request d635. TimeMatch provider,
Kernel integration and product acceptance are separate.
