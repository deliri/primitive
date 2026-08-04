// Package deploy binds one authenticated Release manifest to exact,
// create-only Google Cloud Storage upload capabilities and returns confirmed
// provider receipts. It does not issue capabilities, choose object names,
// retry ambiguous writes, advance Latest, or mutate tenant and website state.
package deploy
