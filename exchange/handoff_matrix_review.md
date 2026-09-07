# Exchange handoff matrix

The new matrix uses the testing protocol's exhaustive-domain alternative for
finite classification inputs. Its generated executions are not claimed as a
count of distinct earned regression rows or as exhaustive HTTP wire coverage.

`TestHTTPProducerClassifierStatusDomainLayerTriad` enumerates the entire
`uint16` status carrier through Core's admission boundary. For each of the 500
admitted statuses it checks both an exact caller expectation and an independently
valid different expectation. Exact agreement must succeed even when the caller
expects a 5xx status. Mismatches retain the exact actual and expected values.

The aggregate classifier receives five named policy conditions: single attempt,
one remaining retry, no remaining retry, maximum remaining counter, and
cancellation after production. Its counter decision distinguishes zero from
positive; one and the unsigned maximum pressure that partition. The stream
classifier has no replay-mode or remaining-counter parameter, so it runs only
its relevant live and canceled conditions. Request-semantics admission remains
owned by the existing typed contract tests.

`TestHTTPProducerClassifierRefusalLatticeLayerTriad` exhausts all 128 subsets of
seven compiler-visible error identities: cancellation, request, redirect,
content type, body limit, response, and transport. Each fixture includes a real
native closed-pipe cause returned through Go's HTTP client. Every set is checked
in original, reversed, and duplicated order, for live and canceled operations,
and through both aggregate and streaming producers. The order variants are
cross-case invariant checks, not additional primary quota rows.

Each row has exactly one primary class. Status disagreement is a contradiction
between a caller's exact expected response and the observed response for that
request. Empty matching responses conserve neutral byte/header evidence.
Selected status transitions are boundary cases. The joined-error sets are typed
refusals, not mislabeled neutral or contradiction cases.

Every path checks producer facts before classification, then classification and
error identity separately, exact retained status/expected bindings, absent
body/header/declaration evidence where none was produced, and the unchanged
provider/read/close counts. The classifier cannot perform an additional HTTP
effect. Aggregate retry scheduling legitimately returns no terminal error while
the original producer cause remains in its input; terminal decisions must retain
that cause. Stream replay additionally proves that admission cannot stamp a
counter onto an absent refusal.

`TestHandoffErrorDomainBindsClassifierInputs` obtains real function locations
from compiler-bound function values, reads their source through Filestore, and
uses Go's AST and import ownership to check that classifier Core identity
symbols occur in the matrix fixture. Adding an inspected identity requires
updating the fixture domain. This source binding supplements the behavioral
matrix; it is not a substitute for it.

Existing named extent, header-capture, deadline, and body-custody tests and fuzz
oracles continue to pressure their producing boundaries. The new matrix closes
the finite retry/status decision domains; it does not relabel those separate
raw-input tests as matrix rows.

The matrix reproduced one production defect: aggregate classification retried
`ErrExchangeRequest`, including combinations with response/transport errors.
Four failing subsets reproduced that one defect. Aggregate now preserves the
request refusal and stops, matching streaming behavior. Seven deliberately
broken decision/retention variants were killed by the matrix; two additional
mutations test replacement of the caller's expected status with hardcoded OK.
