// Package manual validates and projects product-owned command guidance for
// people and machines from one caller-owned typed book.
//
// Book, Report, Line and TopicName are complete caller-owned values. There are
// no package-imposed total byte or item quotas. Validation scans the supplied
// text and uses duplicate and relation indexes proportional to item count.
// Project copies slices into an independent report; immutable strings are shared.
//
// WriteText validates the complete request before emitting bytes, then streams
// through Go's fixed working buffer. WriteJSON validates a complete Report and
// delegates encoding to Go, whose encoder may buffer an individual string token.
//
// Report is an output projection. Generic JSON decoding does not apply its
// invariants automatically: callers must invoke Report.Validate after decoding.
// Schema, View and SelectionMode validate their own JSON input before replacing
// the receiver. Decoding a Report is not a streaming document-ingress API.
package manual
