package release

import (
	"github.com/deliri/primitive/v2026/compass"
	"github.com/deliri/primitive/v2026/version"
)

// currentPrimitiveTag derives Primitive's exact build tag for protocols that
// bind to Primitive itself. Version remains the public value owner; release
// orchestration does not publish a second release facade.
func currentPrimitiveTag() (version.Tag, error) {
	configuration, err := compass.Current()
	if err != nil {
		return version.Tag{}, err
	}
	release, err := version.FromProject(configuration.Project)
	if err != nil {
		return version.Tag{}, err
	}
	return release.Tag(), nil
}
