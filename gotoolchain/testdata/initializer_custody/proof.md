# Native package-initializer closure custody

Assumptions and first principles:

- VERIFIED: Go creates an actual synthetic package initializer without a named `types.Func`; lexical closures can have that initializer as their authenticated native parent. The ordinary named-function lookup cannot establish the root identity. The minimal real compiler regression below fails before repair.
- VERIFIED: the exact `SSAUnitView.Package.Func("init")` member is Go's authority. Pointer equality authenticates it; spelling alone and copied function fields do not. Every intervening parent/AnonFuncs membership check remains required.
- VERIFIED: this repair adds no model, registry, cache or bypass. Primitive owns the existing query and Witness consumes it.
- UNVERIFIED here: downstream Kernel/Distro scope completion and installed Anvil analyzer refresh. Publishing this dependency alone does not establish either.

The smallest repair admits the exact native initializer when authenticating lexical ancestry. It preserves rejection of copied and detached closures.

Execution through installed Mac Anvil, Go1.27.3, darwin-arm64, JSONv2, race and retained shuffle, latest435 base06dc853982e29cf60bdb89ac4824856319d8743d:

- RED: 2026-10-27-2eb9a9a5faaa261ccd57f14843f96cdf775de3b9f421995e6edc7592c92afe48; source17c06bff9ab268dcca104f9c4f2dbe9458254ebb273596e851d55c4ef0147901. Genuine initializer closure fails; copied/detached cases pass. Four native nodes:2pass/2fail, including failed parent.
- GREEN affected ownership/call contracts: 2026-10-27-8b3d3071f50b9e58bb9e660b07a0ac27bab388db315491cba7a207ca18220494; sourceaf2c32142a095653c2e821e303011806e62cbc4c0be8f583ccc04de0d0772953. All65native nodes pass, zero skips.
- Existing semantic call-query fuzz, `-run=^$ -fuzz=^FuzzSSACallFunctionConservesNativeObjectAndRefusal$ -fuzztime=3s -fuzzminimizetime=3s`: run2026-10-27-657cbca783bfd56aefdd2e0c53a7d23b8c1bce6bd5d97a8abeae0161ec7a20fe;41observed executions,21new interesting, one passing fuzz node.
- Existing `BenchmarkSSACallFunction`,3s: run2026-10-27-91df2c8062116f5c138cd98c29bc432c1676ff509c6941f212d4b8ae1151f59a;696104iterations,5169ns/op,0B/op,0allocs/op. Actual CPU/memory profiles, receipt and output independently hash/extent verified. This measures this native query, not complete compiler memory or fleet speed.

Raw receipts/output/profiles remain outside Git in the owning Witness scratch/warehouse. No independent owner acceptance or downstream deployment is claimed.

Final affected vet728a71ee andStaticchecke06786de each exit0/zero findings; receipt/stdout/stderr and analyzer streams independently verified. No passing proof was replayed for timing.
