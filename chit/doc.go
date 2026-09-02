// Package chit owns the bounded custody ticket above accepted object
// receipts: UUIDv7 identities, versioned collections, streaming manifest
// closure, authority-signed immutable chits, and bounded signed catalog pages.
//
// Chit never interprets product evidence. Consumers decide which local objects
// are eligible and which display names are safe. Receipt
// proves that each exact object entered custody; Chit organizes those facts
// into versions a consumer can list and retrieve.
package chit
