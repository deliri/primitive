// Package upgrade stages one authenticated Release artifact into the fixed
// unselected installation slot, exposes its exact command path for
// product-owned trials, and atomically selects it only after a typed passing
// report. Successful promotion removes the former slot.
//
// Upgrade streams downloads and verification through Objectstore, Filestore,
// Hostfacts, and Go. It owns no command arguments, test semantics, live-data
// sandbox, user-consent UI, ticket submission, retry, scheduler, background
// worker, transport, release authority, or general persistence framework.
package upgrade
