package release

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

var (
	embeddedBuildOffering string
	embeddedBuildVersion  string
	embeddedBuildCommit   string
	embeddedBuildPlatform string
)

type embeddedBuildIdentityText struct {
	offering string
	version  string
	commit   string
	platform string
}

// EmbeddedBuildIdentity reads the installation identity injected into the
// current binary at link time. It never accepts caller-supplied identity facts.
func EmbeddedBuildIdentity() (core.BuildIdentity, error) {
	return parseEmbeddedBuildIdentity(embeddedBuildIdentityText{
		offering: embeddedBuildOffering,
		version:  embeddedBuildVersion,
		commit:   embeddedBuildCommit,
		platform: embeddedBuildPlatform,
	})
}

func parseEmbeddedBuildIdentity(text embeddedBuildIdentityText) (core.BuildIdentity, error) {
	var offering core.Offering
	if err := offering.UnmarshalText([]byte(text.offering)); err != nil {
		return core.BuildIdentity{}, embeddedBuildIdentityError("embedded offering is invalid", err)
	}
	var version core.ReleaseVersion
	if err := version.UnmarshalText([]byte(text.version)); err != nil {
		return core.BuildIdentity{}, embeddedBuildIdentityError("embedded version is invalid", err)
	}
	commit, err := core.ParseBuildCommit(text.commit)
	if err != nil {
		return core.BuildIdentity{}, embeddedBuildIdentityError("embedded commit is invalid", err)
	}
	var platform core.Platform
	if err := platform.UnmarshalText([]byte(text.platform)); err != nil {
		return core.BuildIdentity{}, embeddedBuildIdentityError("embedded platform is invalid", err)
	}
	identity, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: offering,
		Version:  version,
		Commit:   commit,
		Platform: platform,
	})
	if err != nil {
		return core.BuildIdentity{}, embeddedBuildIdentityError("embedded build identity is invalid", err)
	}
	return identity, nil
}

func embeddedBuildIdentityError(message string, cause error) error {
	return errors.Join(core.ErrReleaseContract, errors.New(message), cause)
}
