// Package version derives immutable release identities and canonical Git tags
// from validated Compass project declarations.
//
// A project does not construct its current version from copied constants or a
// string. Its Compass owns the human-authored coordinates; this package owns
// every derived release representation.
package version
