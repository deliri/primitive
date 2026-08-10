// Package submission authenticates one evidence declaration from an installed
// product and binds one authority-issued upload capability to that exact
// declaration, request, lifetime, and retention promise.
//
// Submission does not decide product plans, inspect evidence, create cloud
// objects, or perform transfers. The product owns the evidence projection, the
// authority owns the commercial decision and capability issuance, Objectstore
// owns the transfer, and Receipt owns accepted-object evidence after custody.
package submission
