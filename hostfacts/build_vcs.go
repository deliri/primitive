package hostfacts

import "runtime/debug"

// GoBuildVCS carries the compiler's VCS facts without a copied settings inventory.
// Revision is compiler data; consumers decide how to display or validate it.
type GoBuildVCS struct {
	Revision string
	Modified bool
}

const (
	goBuildRevisionKey  = "vcs.revision"
	goBuildModifiedKey  = "vcs.modified"
	goBuildModifiedTrue = "true"
)

// ObserveGoBuildVCS reads Go-owned executable metadata. The bool is Go's own
// availability result. The projection retains two facts and scans settings
// once; runtime/debug owns the allocation of its native BuildInfo record.
func ObserveGoBuildVCS() (GoBuildVCS, bool) {
	info, available := debug.ReadBuildInfo()
	if !available {
		return GoBuildVCS{}, false
	}
	return goBuildVCSFromSettings(info.Settings), true
}

func goBuildVCSFromSettings(settings []debug.BuildSetting) GoBuildVCS {
	var result GoBuildVCS
	for _, setting := range settings {
		switch setting.Key {
		case goBuildRevisionKey:
			result.Revision = setting.Value
		case goBuildModifiedKey:
			result.Modified = setting.Value == goBuildModifiedTrue
		}
	}
	return result
}
