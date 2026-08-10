// Package submissionauth binds one device-signed Submission request to the
// authority-issued installation certificate that nominates its sole trusted
// device key.
//
// The package decides no product policy and issues no upload capability. It is
// the authentication seam between Controlplane's installation credential and
// Submission's blind evidence agreement.
package submissionauth
