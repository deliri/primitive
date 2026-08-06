//go:build !aix && !android && !darwin && !dragonfly && !freebsd && !illumos && !ios && !linux && !netbsd && !openbsd && !solaris

package filestore

import "io/fs"

// observedOwnership reports no owner where the filesystem records no numeric
// identifiers. The unset value is the honest answer: Inspection.Ownership
// refuses it, so a caller on such a host learns the fact is unavailable
// instead of receiving uid 0 and recording root as the owner of everything.
func observedOwnership(_ fs.FileInfo) Ownership {
	return Ownership{}
}
