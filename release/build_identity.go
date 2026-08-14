package release

import (
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// embeddedBuildIdentityPackagePath owns the Go linker package path used by
	// every build-identity variable. Consumer builders use the exported symbols
	// below instead of copying this path or a private variable name.
	embeddedBuildIdentityPackagePath = "github.com/deliri/primitive/v2026/release"
	// embeddedBuildIdentityVariablePrefix names the complete family of
	// linker-injected build-identity variables for the structural ratchet.
	embeddedBuildIdentityVariablePrefix = "embeddedBuild"

	embeddedBuildOfferingVariableName    = "embeddedBuildOffering"
	embeddedBuildVersionVariableName     = "embeddedBuildVersion"
	embeddedBuildCommitVariableName      = "embeddedBuildCommit"
	embeddedBuildPlatformVariableName    = "embeddedBuildPlatform"
	embeddedBuildAssignmentsVariableName = "embeddedBuildAssignments"

	embeddedBuildOfferingFramePrefix    = "primitive-build-offering:"
	embeddedBuildVersionFramePrefix     = "primitive-build-version:"
	embeddedBuildCommitFramePrefix      = "primitive-build-commit:"
	embeddedBuildPlatformFramePrefix    = "primitive-build-platform:"
	embeddedBuildAssignmentsFramePrefix = "primitive-build-assignments:"
)

// Embedded build-identity linker symbols are owned beside the variables they
// name. Consumer release builders use these constants with the Go linker's -X
// flag instead of copying Primitive's package path or private variable names.
const (
	EmbeddedBuildOfferingLinkSymbol    = embeddedBuildIdentityPackagePath + "." + embeddedBuildOfferingVariableName
	EmbeddedBuildVersionLinkSymbol     = embeddedBuildIdentityPackagePath + "." + embeddedBuildVersionVariableName
	EmbeddedBuildCommitLinkSymbol      = embeddedBuildIdentityPackagePath + "." + embeddedBuildCommitVariableName
	EmbeddedBuildPlatformLinkSymbol    = embeddedBuildIdentityPackagePath + "." + embeddedBuildPlatformVariableName
	embeddedBuildAssignmentsLinkSymbol = embeddedBuildIdentityPackagePath + "." + embeddedBuildAssignmentsVariableName
)

var (
	embeddedBuildOffering    string
	embeddedBuildVersion     string
	embeddedBuildCommit      string
	embeddedBuildPlatform    string
	embeddedBuildAssignments string
)

type embeddedBuildIdentityText struct {
	offering    string
	version     string
	commit      string
	platform    string
	assignments string
}

// EmbeddedBuildIdentity reads the installation identity injected into the
// current binary at link time. It never accepts caller-supplied identity facts.
func EmbeddedBuildIdentity() (core.BuildIdentity, error) {
	return parseEmbeddedBuildIdentity(embeddedBuildIdentityText{
		offering:    embeddedBuildOffering,
		version:     embeddedBuildVersion,
		commit:      embeddedBuildCommit,
		platform:    embeddedBuildPlatform,
		assignments: embeddedBuildAssignments,
	})
}

func parseEmbeddedBuildIdentity(text embeddedBuildIdentityText) (core.BuildIdentity, error) {
	unframed, err := unframeEmbeddedBuildIdentity(text)
	if err != nil {
		return core.BuildIdentity{}, err
	}
	var offering core.Offering
	if err := offering.UnmarshalText([]byte(unframed.offering)); err != nil {
		return core.BuildIdentity{}, embeddedBuildIdentityError("embedded offering is invalid", err)
	}
	var version core.ReleaseVersion
	if err := version.UnmarshalText([]byte(unframed.version)); err != nil {
		return core.BuildIdentity{}, embeddedBuildIdentityError("embedded version is invalid", err)
	}
	commit, err := core.ParseBuildCommit(unframed.commit)
	if err != nil {
		return core.BuildIdentity{}, embeddedBuildIdentityError("embedded commit is invalid", err)
	}
	var platform core.Platform
	if err := platform.UnmarshalText([]byte(unframed.platform)); err != nil {
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

func frameEmbeddedBuildIdentity(build core.BuildIdentity, assignments string) embeddedBuildIdentityText {
	return embeddedBuildIdentityText{
		offering:    embeddedBuildOfferingFramePrefix + build.Offering().String(),
		version:     embeddedBuildVersionFramePrefix + build.Version().String(),
		commit:      embeddedBuildCommitFramePrefix + build.Commit().String(),
		platform:    embeddedBuildPlatformFramePrefix + build.Platform().String(),
		assignments: embeddedBuildAssignmentsFramePrefix + assignments,
	}
}

func unframeEmbeddedBuildIdentity(text embeddedBuildIdentityText) (embeddedBuildIdentityText, error) {
	offering, err := unframeEmbeddedBuildValue(text.offering, embeddedBuildOfferingFramePrefix)
	if err != nil {
		return embeddedBuildIdentityText{}, embeddedBuildIdentityError("embedded offering frame is invalid", err)
	}
	version, err := unframeEmbeddedBuildValue(text.version, embeddedBuildVersionFramePrefix)
	if err != nil {
		return embeddedBuildIdentityText{}, embeddedBuildIdentityError("embedded version frame is invalid", err)
	}
	commit, err := unframeEmbeddedBuildValue(text.commit, embeddedBuildCommitFramePrefix)
	if err != nil {
		return embeddedBuildIdentityText{}, embeddedBuildIdentityError("embedded commit frame is invalid", err)
	}
	platform, err := unframeEmbeddedBuildValue(text.platform, embeddedBuildPlatformFramePrefix)
	if err != nil {
		return embeddedBuildIdentityText{}, embeddedBuildIdentityError("embedded platform frame is invalid", err)
	}
	assignments, err := unframeEmbeddedBuildValue(text.assignments, embeddedBuildAssignmentsFramePrefix)
	if err != nil {
		return embeddedBuildIdentityText{}, embeddedBuildIdentityError("embedded assignment frame is invalid", err)
	}
	var digest core.SHA256Digest
	if err := digest.UnmarshalText([]byte(assignments)); err != nil {
		return embeddedBuildIdentityText{}, embeddedBuildIdentityError("embedded assignment commitment is invalid", err)
	}
	return embeddedBuildIdentityText{
		offering: offering, version: version, commit: commit, platform: platform, assignments: assignments,
	}, nil
}

func unframeEmbeddedBuildValue(value, prefix string) (string, error) {
	unframed, found := strings.CutPrefix(value, prefix)
	if !found || unframed == "" {
		return "", errors.New("embedded build value lacks its domain frame")
	}
	return unframed, nil
}

func embeddedBuildIdentityError(message string, cause error) error {
	return errors.Join(core.ErrReleaseContract, errors.New(message), cause)
}
