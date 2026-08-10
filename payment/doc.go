// Package payment owns authority-signed customer payment receipts and bounded
// receipt catalogs. It knows account and offering identities, exact currency,
// settlement time, and service period; it does not know Stripe, OGS, or any
// product-specific billing implementation.
package payment
