package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

const (
	offeringBugToken       = "bug"
	offeringWitnessToken   = "witness"
	offeringPeachfuzzToken = "peachfuzz"
	// OfferingTokenMaximumBytes bounds the closed product identity domain.
	OfferingTokenMaximumBytes = len(offeringPeachfuzzToken)
	// ReleaseVersionMaximumBytes bounds three uint32 decimal components and
	// their two separators.
	ReleaseVersionMaximumBytes = 3*10 + 2
	// BuildCommitSHA1Bytes is the decoded width of a SHA-1 Git object name.
	BuildCommitSHA1Bytes = 20
	// BuildCommitSHA256Bytes is the decoded width of a SHA-256 Git object name.
	BuildCommitSHA256Bytes = 32
	// BuildCommitMaximumBytes bounds the canonical hexadecimal projection.
	BuildCommitMaximumBytes = BuildCommitSHA256Bytes * 2
	// EmbeddedBuildOfferingLinkSymbol is the exact linker symbol for the
	// current binary's offering token.
	EmbeddedBuildOfferingLinkSymbol = "github.com/deliri/primitive/v2026/core.embeddedBuildOffering"
	// EmbeddedBuildVersionLinkSymbol is the exact linker symbol for the
	// current binary's release version.
	EmbeddedBuildVersionLinkSymbol = "github.com/deliri/primitive/v2026/core.embeddedBuildVersion"
	// EmbeddedBuildCommitLinkSymbol is the exact linker symbol for the current
	// binary's source commit.
	EmbeddedBuildCommitLinkSymbol = "github.com/deliri/primitive/v2026/core.embeddedBuildCommit"
	// EmbeddedBuildPlatformLinkSymbol is the exact linker symbol for the
	// current binary's target platform.
	EmbeddedBuildPlatformLinkSymbol = "github.com/deliri/primitive/v2026/core.embeddedBuildPlatform"
)

// Offering is the closed set of products sharing the release protocol.
type Offering uint8

const (
	// OfferingUnknown is the invalid zero offering.
	OfferingUnknown Offering = iota
	// OfferingBug identifies Bug.
	OfferingBug
	// OfferingWitness identifies Witness.
	OfferingWitness
	// OfferingPeachfuzz identifies Peachfuzz.
	OfferingPeachfuzz
	offeringLimit
)

// ReleaseOfferingMismatchError carries the exact observed and expected
// offering facts when authenticated Release input names the wrong stream.
type ReleaseOfferingMismatchError struct {
	observed Offering
	expected Offering
}

// NewReleaseOfferingMismatchError constructs one typed offering mismatch.
func NewReleaseOfferingMismatchError(
	observed Offering,
	expected Offering,
) (ReleaseOfferingMismatchError, error) {
	mismatch := ReleaseOfferingMismatchError{observed: observed, expected: expected}
	if err := mismatch.Validate(); err != nil {
		return ReleaseOfferingMismatchError{}, err
	}
	return mismatch, nil
}

// Validate proves both offerings and their contradiction.
func (e ReleaseOfferingMismatchError) Validate() error {
	if err := e.observed.Validate(); err != nil {
		return releaseIdentityError("observed release offering is invalid", err)
	}
	if err := e.expected.Validate(); err != nil {
		return releaseIdentityError("expected release offering is invalid", err)
	}
	if e.observed == e.expected {
		return releaseIdentityError("release offering mismatch names equal offerings")
	}
	return nil
}

// Error returns the operator-facing offering contradiction.
func (e ReleaseOfferingMismatchError) Error() string {
	if e.Validate() != nil {
		return "release offering mismatch is invalid"
	}
	return "release offering " + e.observed.String() +
		" differs from expected " + e.expected.String()
}

// Unwrap preserves the stable Release verification identity.
func (e ReleaseOfferingMismatchError) Unwrap() error {
	return ErrReleaseVerification
}

// Observed returns the offering carried by the authenticated document.
func (e ReleaseOfferingMismatchError) Observed() Offering { return e.observed }

// Expected returns the caller-selected offering.
func (e ReleaseOfferingMismatchError) Expected() Offering { return e.expected }

// Validate rejects offerings outside the closed domain.
func (o Offering) Validate() error {
	if o <= OfferingUnknown || o >= offeringLimit {
		return releaseIdentityError("offering is outside the closed domain")
	}
	return nil
}

// IsValid reports whether o belongs to the closed offering domain.
func (o Offering) IsValid() bool { return o.Validate() == nil }

// String returns canonical offering text, or empty text when invalid.
func (o Offering) String() string {
	switch o {
	case OfferingBug:
		return offeringBugToken
	case OfferingWitness:
		return offeringWitnessToken
	case OfferingPeachfuzz:
		return offeringPeachfuzzToken
	default:
		return ""
	}
}

// ParseOffering accepts one canonical offering token.
func ParseOffering(value string) (Offering, error) {
	switch value {
	case offeringBugToken:
		return OfferingBug, nil
	case offeringWitnessToken:
		return OfferingWitness, nil
	case offeringPeachfuzzToken:
		return OfferingPeachfuzz, nil
	default:
		return OfferingUnknown, releaseIdentityError("offering token is unsupported")
	}
}

// MarshalJSON emits canonical offering text.
func (o Offering) MarshalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(o.String())
}

// UnmarshalJSON accepts only canonical offering text.
func (o *Offering) UnmarshalJSON(data []byte) error {
	if o == nil {
		return errors.Join(ErrJSONContract, releaseIdentityError("offering receiver is nil"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := ParseOffering(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*o = parsed
	return nil
}

// ReleaseVersion is one exact three-component release order.
type ReleaseVersion struct {
	major uint32
	minor uint32
	patch uint32
	set   bool
}

// NewReleaseVersion constructs the complete uint32 release-version domain.
func NewReleaseVersion(major, minor, patch uint32) ReleaseVersion {
	return ReleaseVersion{major: major, minor: minor, patch: patch, set: true}
}

// ParseReleaseVersion accepts one canonical three-component release version.
func ParseReleaseVersion(value string) (ReleaseVersion, error) {
	if len(value) == 0 || len(value) > ReleaseVersionMaximumBytes {
		return ReleaseVersion{}, releaseIdentityError("release version has invalid length")
	}
	majorText, remainder, found := strings.Cut(value, ".")
	if !found {
		return ReleaseVersion{}, releaseIdentityError("release version is incomplete")
	}
	minorText, patchText, found := strings.Cut(remainder, ".")
	if !found || strings.Contains(patchText, ".") {
		return ReleaseVersion{}, releaseIdentityError("release version is not tripartite")
	}
	major, err := parseVersionComponent(majorText)
	if err != nil {
		return ReleaseVersion{}, err
	}
	minor, err := parseVersionComponent(minorText)
	if err != nil {
		return ReleaseVersion{}, err
	}
	patch, err := parseVersionComponent(patchText)
	if err != nil {
		return ReleaseVersion{}, err
	}
	return NewReleaseVersion(major, minor, patch), nil
}

func parseVersionComponent(value string) (uint32, error) {
	if value == "" || len(value) > 1 && value[0] == '0' {
		return 0, releaseIdentityError("release version component is not canonical")
	}
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil || strconv.FormatUint(parsed, 10) != value {
		return 0, releaseIdentityError("release version component is invalid")
	}
	return uint32(parsed), nil
}

// Validate proves the version crossed a constructor or decode boundary.
func (v ReleaseVersion) Validate() error {
	if !v.set {
		return releaseIdentityError("release version is unset")
	}
	parsed, err := ParseReleaseVersion(v.String())
	if err != nil || parsed != v {
		return releaseIdentityError("release version is invalid")
	}
	return nil
}

// String returns the canonical decimal release version.
func (v ReleaseVersion) String() string {
	return strconv.FormatUint(uint64(v.major), 10) + "." +
		strconv.FormatUint(uint64(v.minor), 10) + "." +
		strconv.FormatUint(uint64(v.patch), 10)
}

// Compare orders two validated release versions.
func (v ReleaseVersion) Compare(other ReleaseVersion) (Comparison, error) {
	if err := v.Validate(); err != nil {
		return ComparisonUnknown, err
	}
	if err := other.Validate(); err != nil {
		return ComparisonUnknown, err
	}
	switch {
	case v.major != other.major:
		return compareUint32(v.major, other.major), nil
	case v.minor != other.minor:
		return compareUint32(v.minor, other.minor), nil
	default:
		return compareUint32(v.patch, other.patch), nil
	}
}

func compareUint32(left, right uint32) Comparison {
	switch {
	case left < right:
		return ComparisonLess
	case left > right:
		return ComparisonGreater
	default:
		return ComparisonEqual
	}
}

// MarshalJSON emits the canonical version as a JSON string.
func (v ReleaseVersion) MarshalJSON() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(v.String())
}

// UnmarshalJSON accepts only canonical version text.
func (v *ReleaseVersion) UnmarshalJSON(data []byte) error {
	if v == nil {
		return errors.Join(ErrJSONContract, releaseIdentityError("release version receiver is nil"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := ParseReleaseVersion(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*v = parsed
	return nil
}

// BuildCommit is a canonical SHA-1 or SHA-256 Git object name.
type BuildCommit struct {
	value [BuildCommitSHA256Bytes]byte
	size  uint8
}

const buildCommitWidthDiagnostic = "build commit has unsupported width"

// ParseBuildCommit accepts canonical lower hexadecimal at a supported Git
// object-name width.
func ParseBuildCommit(value string) (BuildCommit, error) {
	size := len(value) / 2
	if len(value)%2 != 0 || size != BuildCommitSHA1Bytes && size != BuildCommitSHA256Bytes {
		return BuildCommit{}, releaseIdentityError(buildCommitWidthDiagnostic)
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || hex.EncodeToString(decoded) != value {
		return BuildCommit{}, releaseIdentityError("build commit is not canonical lowercase hexadecimal")
	}
	var commit BuildCommit
	copy(commit.value[:], decoded)
	if size == BuildCommitSHA1Bytes {
		commit.size = BuildCommitSHA1Bytes
	} else {
		commit.size = BuildCommitSHA256Bytes
	}
	return commit, nil
}

// Validate proves supported width and zero padding.
func (c BuildCommit) Validate() error {
	if c.size != BuildCommitSHA1Bytes && c.size != BuildCommitSHA256Bytes {
		return releaseIdentityError(buildCommitWidthDiagnostic)
	}
	for _, value := range c.value[c.size:] {
		if value != 0 {
			return releaseIdentityError("build commit padding is nonzero")
		}
	}
	return nil
}

// String returns canonical lower hexadecimal, or empty text when invalid.
func (c BuildCommit) String() string {
	if c.Validate() != nil {
		return ""
	}
	return hex.EncodeToString(c.value[:c.size])
}

// MarshalJSON emits canonical lower hexadecimal.
func (c BuildCommit) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(c.String())
}

// UnmarshalJSON accepts a canonical supported Git object name.
func (c *BuildCommit) UnmarshalJSON(data []byte) error {
	if c == nil {
		return errors.Join(ErrJSONContract, releaseIdentityError("build commit receiver is nil"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := ParseBuildCommit(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*c = parsed
	return nil
}

// BuildIdentityRequest carries the immutable facts shared by Release and
// Upgrade.
type BuildIdentityRequest struct {
	// Version identifies the ordered product release.
	Version ReleaseVersion
	// Commit identifies the exact source commit.
	Commit BuildCommit
	// Platform identifies the compiled target.
	Platform Platform
	// Offering identifies the released product.
	Offering Offering
}

// BuildIdentity identifies immutable release bytes without claiming that the
// current process embeds those facts.
type BuildIdentity struct {
	version  ReleaseVersion
	commit   BuildCommit
	platform Platform
	offering Offering
}

type buildIdentityWire struct {
	// Offering is the required offering wire field.
	Offering *Offering `json:"offering"`
	// Version is the required version wire field.
	Version *ReleaseVersion `json:"version"`
	// Commit is the required commit wire field.
	Commit *BuildCommit `json:"commit"`
	// Platform is the required platform wire field.
	Platform *Platform `json:"platform"`
}

// NewBuildIdentity validates and constructs immutable build facts.
func NewBuildIdentity(request BuildIdentityRequest) (BuildIdentity, error) {
	identity := BuildIdentity{
		offering: request.Offering,
		version:  request.Version,
		commit:   request.Commit,
		platform: request.Platform,
	}
	if err := identity.Validate(); err != nil {
		return BuildIdentity{}, err
	}
	return identity, nil
}

// Validate proves every owned build-identity field.
func (i BuildIdentity) Validate() error {
	for _, err := range []error{
		i.offering.Validate(), i.version.Validate(), i.commit.Validate(), i.platform.Validate(),
	} {
		if err != nil {
			return releaseIdentityError("build identity is invalid", err)
		}
	}
	return nil
}

// Offering returns the product identity.
func (i BuildIdentity) Offering() Offering { return i.offering }

// Version returns the release version.
func (i BuildIdentity) Version() ReleaseVersion { return i.version }

// Commit returns the source commit.
func (i BuildIdentity) Commit() BuildCommit { return i.commit }

// Platform returns the compiled target.
func (i BuildIdentity) Platform() Platform { return i.platform }

// MarshalJSON emits the exact typed build-identity projection.
func (i BuildIdentity) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	offering, version, commit, platform := i.offering, i.version, i.commit, i.platform
	return json.Marshal(buildIdentityWire{
		Offering: &offering, Version: &version, Commit: &commit, Platform: &platform,
	})
}

// UnmarshalJSON accepts one bounded strict build-identity projection.
func (i *BuildIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return errors.Join(ErrJSONContract, releaseIdentityError("build identity receiver is nil"))
	}
	maximum, err := NewByteCount(2 << 10)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	wire, err := DecodeStrictJSONStructure[buildIdentityWire](data, StrictJSONLimits{
		DocumentMaximumBytes: maximum,
		NestingDepthMaximum:  2,
		ObjectFieldMaximum:   4,
		ArrayItemMaximum:     1,
	})
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	if wire.Offering == nil || wire.Version == nil || wire.Commit == nil || wire.Platform == nil {
		return errors.Join(ErrJSONContract, releaseIdentityError("build identity field is missing"))
	}
	candidate, err := NewBuildIdentity(BuildIdentityRequest{
		Offering: *wire.Offering, Version: *wire.Version,
		Commit: *wire.Commit, Platform: *wire.Platform,
	})
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*i = candidate
	return nil
}

var (
	embeddedBuildOffering string
	embeddedBuildVersion  string
	embeddedBuildCommit   string
	embeddedBuildPlatform string
)

// EmbeddedBuildIdentity reads the installation identity injected into the
// current binary at link time. It never accepts caller-supplied identity facts.
func EmbeddedBuildIdentity() (BuildIdentity, error) {
	offering, err := ParseOffering(embeddedBuildOffering)
	if err != nil {
		return BuildIdentity{}, releaseIdentityError("embedded offering is invalid", err)
	}
	version, err := ParseReleaseVersion(embeddedBuildVersion)
	if err != nil {
		return BuildIdentity{}, releaseIdentityError("embedded version is invalid", err)
	}
	commit, err := ParseBuildCommit(embeddedBuildCommit)
	if err != nil {
		return BuildIdentity{}, releaseIdentityError("embedded commit is invalid", err)
	}
	platform, err := ParsePlatform(embeddedBuildPlatform)
	if err != nil {
		return BuildIdentity{}, releaseIdentityError("embedded platform is invalid", err)
	}
	return NewBuildIdentity(BuildIdentityRequest{
		Offering: offering, Version: version, Commit: commit, Platform: platform,
	})
}

func releaseIdentityError(message string, causes ...error) error {
	return errors.Join(append([]error{ErrPrimitiveContract, errors.New(message)}, causes...)...)
}

var (
	_ ValidatedJSONMarshaler = OfferingUnknown
	_ ValidatedJSONMarshaler = ReleaseVersion{}
	_ ValidatedJSONMarshaler = BuildCommit{}
	_ ValidatedJSONMarshaler = BuildIdentity{}
	_ error                  = ReleaseOfferingMismatchError{}
)
