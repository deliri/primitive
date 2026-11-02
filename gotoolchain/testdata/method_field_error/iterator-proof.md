# Native iterator refusal propagation

The public request and observation stay unchanged. Witness supplies application policy; Primitive borrows Go's actual functions, source expressions, field slots and error identities. No product-side ownership model is added.

Verified assumptions and limits:

- Primitive437 misses a real field refusal returned through a consumed iterator. Original4b83a140 reports four genuine positive failures plus parent, with six wrong-field/erased/unconsumed controls passing.
- A generated Go range callback returns bool, even when its source return exits the enclosing error-valued method. Actual compiler dump28f3f55a identifies that representation. Observed source returns are tied to native return positions and source values.
- Captured callback variables may live in native cells. Existing ClosureBindings and ArgumentOperand remain the only capture/operand authority; the implementation does not reconstruct a callback environment or store history.
- Each yielded error coordinate must keep its own identity. First-of-two-errors0c98e3b6 is a real red case; original-seed e6769a89 restores it. Wrong-coordinate, callback-replacement, erased-cause and conditional-loss controls pass.
- An observed field read is not an error-flow proof. Both APIs retain their independent floors; unavailable or unsupported bodies grant no authority.
- Deferred/concurrent cleanup can replace the enclosing result after source evaluation. Experimental1af0a3e7 proves both a direct result-cell false negative and a deferred-result false positive. The new source-return fallback must conservatively refuse cleanup authority; exact restoration is pending.

Native development proof through Mac Anvil, Go1.27.3/JSONv2/race/shuffle, base61378a7329bc7ac88456d726776e3250f41f52dd:

- Initial direct/method-expression/joined cases restored in c82c1c3e; helper still failed. Helper01 still missed Go's captured callback cell. Helper02 correctly refused my invalid generic operand query on a static function; that caller misuse was corrected by querying owned frame values only.
- Existing Primitive capture and operand loans restore the helper:334955c4,2PASS. Matrix78095a74:51PASS/0skip (five native call forms by ten cause/receiver/consumption cases).
- Coordinate/control0c98e3b6:13PASS plus first-coordinate failure and parent; exact first-coordinate e6769a89:2PASS. These are retained separately, not a fresh all-case claim.
- Affected27a0ab76:270PASS/0skip, including old native error/identity/format/field contracts and the new semantic seeds/allocation test. Query allocated6328bytes at both32 and16385 preceding statements; compiler construction is excluded and active ancestry is not claimed constant across arbitrary depth.
- Initial vet/Staticcheck are independently retained but precede the result-cell/cleanup extension. They are not substituted for the final affected proof and analyzers.
- The first compiler diagnostic66b4a70e panicked because my test used nil package identity to look up an unexported method. Corrected28f3f55a provides the actual frames. This was a diagnostic fixture mistake, not a tool cause.

Final affected tests, tools and fuzz are recorded below. Publication and actual Witness adoption remain separate. No installed catalog update, whole Witness gate or application verdict is inferred. Raw requests/receipts/output/failed attempts stay outside Git and move with the entire idle scratch root to the HDD warehouse.

The actual Witness whole-layer consumer009ef73d exposed an additional legitimate direct-return miss before its range: Go moves other returns in a function with a returning range callback into native result cells. The experimental native1af0a3e7 then proves both defects: direct_error_before_range is falsely absent, and deferred_named_result_erasure is falsely present. The source-correlated fallback now rejects deferred/concurrent cleanup in the active ancestry. Exact original-seedbd97af42 restores both leaves+parent,3PASS/0skip; receipt/rawbytes/hash/counts verify. Its observed wait is175.907s before launch,90.457s preparation including75.097s native admission, then22.229s build/run; nested intervals are not summed.

Final affected2695394e7222ac2f0d46a8cbd9cd5dd83ba88ef276fcbfa20e48a856408ab134 passes338nodes/0skip on the corrected source: old error/identity/format/field contracts,50native call/flow matrix cases,16coordinate/control cases, new/legacy semantic seeds and allocation checks. This fresh affected run is required because generic result-cell handling changed; no fullpackage campaign is claimed. Final tools and serial fuzz follow below.

Final corrected-source allocation observations in the338-node run are6504query bytes at both32 and16385 preceding statements. The earlier6328 observations belong to the narrower pre-result-home extension. Neither is a universal Go compiler memory bound or a measured throughput improvement. The query retains active native call ancestry only and uses no symbol registry, retained call graph, callback environment copy or arbitrary flow quota.

Final corrected-source Go vet3ef9fe1b17c754440a245fe027c3faca67cd34facce576c936fc92f3c9dc669f andStaticcheck5951ca9affb811730d455e5ce107445a92afc8b47659af21d44460ce0ed928bd each execute once and report native0/zero findings. Independent receipt/rawartifact and analysis-stream hash/extent checks verify zero analysis bytes. The final two3s semantic fuzz runs follow these tools serially; verified results follow below.

Both final3s semantic fuzz runs pass after final tools, serially: new iterator targetecc461e59dede51eea3aab4f4f42134e2f0a3b327428fcfd03d72c03827e0df7 reports31executions/6new-interesting inputs; existing field-error targetdd83aa01fe034ffbc4e1343cc09c63c3dde6a64b02f12da2a5843dc7fc754dce reports69executions/15new-interesting inputs. These are observed native counters, not unique-case or independent acceptance claims. Native338-node proof, both final analyzers and both fuzz targets bind the same captured source digest; hashes/extents/counts verify. All final Go files remain byte-equal to the tested source. No benchmark or throughput improvement is claimed.
