// Package release verifies a clean repository at its exact commit and exact
// build tools, constructs deterministic fixed-target Garble build and process
// plans, inspects the resulting native executables, binds signed tool and
// metadata provenance into immutable manifests, authenticates Latest
// selections, then makes the installed-versus-latest decision.
//
// Release owns no file creation, transport, download, installation,
// scheduling, persistence, retry, or customer-facing policy. Executable
// inspection owns one bounded Filestore read handle for the duration of the
// inspection and closes it before returning.
package release
