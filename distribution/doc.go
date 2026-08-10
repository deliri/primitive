// Package distribution owns the signed, product-neutral agreements that join
// software release publication, Latest discovery, and exact artifact download.
//
// Both a producer or installed tool and its authority use these same types.
// The offering is always a field inside Release facts; there is no product
// dispatch, provider policy, persistence, transfer, installation, retry, or
// customer-facing decision in this package.
package distribution
