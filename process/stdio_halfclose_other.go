//go:build !unix

package process

import "os"

// Non-Unix standard files keep ordinary Close semantics. Unix descriptor
// lending and its aliased-socket relay are refused separately on these hosts.
func shutdownOutput(*os.File) error { return nil }
