// Package gate converts one authentic Lease assessment into permission to
// begin new paid work.
//
// Gate owns only the fixed new-work disposition: current and continuity Lease
// assessments permit; not-yet-valid, expired, refused, and revoked assessments
// deny. Products own the exhaustive mapping from commands to new-work
// boundaries. Existing-record inspection, registration, check-in, recovery,
// persistence, transport, rendering, and customer-facing copy do not pass
// through Gate.
//
// OGS separately owns its control-plane authentication, route protection, and
// authoritative operation authorization. Those server gates distrust clients;
// this client gate trusts only an authentic OGS-signed Lease. Neither gate's
// result substitutes for the other.
//
// Callers evaluate Lease and durably commit its returned high-water before
// calling AuthorizeNewWork. A NewWorkPermit is an in-process capability for the
// immediate boundary being entered; it is not serializable or a reusable
// bearer token.
//
// Two result contracts hold on every path. AuthorizeNewWork returns the zero
// permit with every error, so a caller can never hold a populated capability
// alongside a failure. A denial is a sealed DenialError interface backed only
// by Gate's package-private value implementation, so no caller-built zero or
// typed-nil carrier can claim Gate's denial identity without an assessment.
// Both capabilities revalidate the state they close over before answering, and
// errors.Is separates the two outcomes: a denial carries core.ErrGateDenied,
// and a rejected contract carries core.ErrGateContract with the exact
// ContractBoundary that refused the value.
package gate
