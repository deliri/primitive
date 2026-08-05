// Package controlwire owns the scalar values a client and a control plane must
// agree on byte for byte before either can say anything else.
//
// A control conversation opens with three facts: which published wire revision
// the build speaks, which unpredictable identity names this request, and which
// one-time token an operator copied in to enrol the installation. The server
// that issues those values and the customer binary that answers them live in
// different modules and ship on different schedules, so a private copy on
// either side is hidden coupling: nothing breaks the build when one of them
// drifts, and the drift surfaces as a rejected request in the field. This
// package is the single definition both ends import.
//
// Controlwire owns the value and its rules. It does not own policy. Which
// offerings exist, which plans admit custody, how long retention runs, what a
// registration means commercially, and when a request is authorized all stay
// with the control plane above. Nothing here reaches the network, the clock, or
// the disk; a caller supplies the substrate and decides what an accepted value
// permits.
//
// Every value is closed. Revision accepts one exact published token and refuses
// an unrecognized one rather than assuming forward compatibility, so an old
// binary can never negotiate itself onto a contract it does not implement.
// RequestNonce refuses the all-zero value, because an identity that is not
// unpredictable would let a replay read as a fresh request. RegistrationToken
// keeps its secret in Core's SecretMaterial and redacts every formatting verb,
// so no log line, wrapped error, or panic can print an unspent enrolment
// secret; its Verifier is the one-way value a control plane persists so it can
// recognise a presented token without ever holding one.
//
// errors.Is separates the causes. Every rejection carries
// core.ErrControlWireContract, and the exact scalar that refused the value
// carries core.ErrControlWireRevision, core.ErrControlWireNonce, or
// core.ErrControlWireToken.
package controlwire
