// Package distributionauth binds installed-tool publication, update, and
// upgrade exchanges to the authority-issued installation certificate that
// nominates their sole trusted device key. Publication keeps release-manifest
// trust independent from installation trust and authenticates the exact
// installed build on both request and provider-evidence completion. It owns no
// release selection, publication admission, or installation policy.
package distributionauth
