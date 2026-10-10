package hostfacts

import (
	"runtime"
	"runtime/debug"
)

// GoBuildVCS carries the compiler's VCS facts without a copied settings inventory.
// Revision is compiler data; consumers decide how to display or validate it.
type GoBuildVCS struct {
	Revision string
	Modified bool
}

// GoBuildMetadata carries fixed compiler facts, without modules or settings
// inventories. ModulePath is Go's main module path and GoVersion identifies
// the Go runtime compiled into this executable.
type GoBuildMetadata struct {
	ModulePath string
	GoVersion  string
	VCS        GoBuildVCS
}

const (
	goBuildRevisionKey  = "vcs.revision"
	goBuildModifiedKey  = "vcs.modified"
	goBuildModifiedTrue = "true"
)

// ObserveGoBuildMetadata reads Go-owned executable metadata. The bool is Go's own
// build-info availability result; GoVersion is observed even without build info.
// The projection retains fixed facts and scans settings
// once; runtime/debug owns the allocation of its native BuildInfo record.
func ObserveGoBuildMetadata() (GoBuildMetadata, bool) {
	info, available := debug.ReadBuildInfo()
	if !available {
		return GoBuildMetadata{GoVersion: runtime.Version()}, false
	}
	return goBuildMetadataFromInfo(info), true
}

func goBuildMetadataFromInfo(info *debug.BuildInfo) GoBuildMetadata {
	return GoBuildMetadata{
		ModulePath: info.Main.Path,
		GoVersion:  info.GoVersion,
		VCS:        goBuildVCSFromSettings(info.Settings),
	}
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
