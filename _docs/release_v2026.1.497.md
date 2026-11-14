# Primitive 2026.1.497

Release construction now admits exactly Go1.27.4.496 admitted only1.27.3, so Peachfuzz48 refused the current native compiler before constructing an artifact. The closed compiler identity, executable standing/digest check, native version probe and refusal of other versions remain. Historical signed compiler provenance stays readable.

Anvil source proof on base5ef33f3e, with the exact authored source bytes in the sealed snapshot:

- RED6550e1dd31396f85e191ccb48e9803440540a8dd4b86a512dab6505f62e63149: old pin refuses actualGo1.27.4 and fails the new exact-version and previous-patch expectations.
- Failed-only GREEN43609d7405b4b3d1fb583dfe3a5ed6cacdbcd8fdf8724725a6b910354606a66e: all three requested failed leaves passed; actual native executable/platform/digest verified onMac. No skips. Unchanged passing parser boundaries were reused.
- Compass90894e9fa3592c3994afe73d2bbc37122925886a210f031a1abc9e200a08d7c7: three top-level tests, fifteen nodes, passed onFurnace with zero skips.
- Release Vet69b0c0a94725fefad828f752761fddd4c0c290ff8504ad561f12bbbd6e5f4091 and Staticcheck8f48ba07e2b8299102ea54a8b8e89f0de053edbf6e7ee166849a8f529ab4f534: zero findings, parent17aac8b673b4bd5bf408c143235a5bd37be0ba15703e417f13296fad59808c26 passed2/2.

Six original receipt digests match their terminal commitments and details equal independently retrieved execution views. Twelve evidence readers joined0. Retained originals are in Peachfuzz `.tools/repair-20261114/primitive-go1274`, with Anvil durable custody. This is focused source proof, not a whole-repository gate or a native Windows execution claim. Official module publication and Peachfuzz adoption need their own bindings.
