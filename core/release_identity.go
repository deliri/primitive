package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

const (
	offeringBugToken     = "bug"
	offeringWitnessToken = "witness"
	// offeringTokenMaximumBytes bounds the closed product identity domain.
	offeringTokenMaximumBytes = len(offeringWitnessToken)
	// releaseVersionMaximumBytes bounds three uint32 decimal components and
	// their two separators.
	releaseVersionMaximumBytes = 3*10 + 2
	// buildCommitSHA1Bytes is the decoded width of a SHA-1 Git object name.
	buildCommitSHA1Bytes = 20
	// buildCommitSHA256Bytes is the decoded width of a SHA-256 Git object name.
	buildCommitSHA256Bytes = 32
	// buildIdentityJSONMaximumBytes bounds one complete identity projection.
	buildIdentityJSONMaximumBytes = 2 << 10
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
	offeringLimit
)

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
	default:
		return ""
	}
}

// parseOffering accepts one canonical offering token.
func parseOffering(value string) (Offering, error) {
	switch value {
	case offeringBugToken:
		return OfferingBug, nil
	case offeringWitnessToken:
		return OfferingWitness, nil
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
		return errors.Join(ErrPrimitiveContract, err)
	}
	parsed, err := parseOffering(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*o = parsed
	return nil
}

// UnmarshalText accepts one canonical offering through encoding.TextUnmarshaler.
func (o *Offering) UnmarshalText(text []byte) error {
	if o == nil {
		return releaseIdentityError("offering receiver is nil")
	}
	if len(text) == 0 || len(text) > offeringTokenMaximumBytes {
		return releaseIdentityError("offering token has invalid length")
	}
	parsed, err := parseOffering(string(text))
	if err != nil {
		return err
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

// parseReleaseVersion accepts one canonical three-component release version.
func parseReleaseVersion(value string) (ReleaseVersion, error) {
	if len(value) == 0 || len(value) > releaseVersionMaximumBytes {
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
	parsed, err := parseReleaseVersion(v.String())
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
		return errors.Join(ErrPrimitiveContract, err)
	}
	parsed, err := parseReleaseVersion(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*v = parsed
	return nil
}

// UnmarshalText accepts canonical release-version text through encoding.TextUnmarshaler.
func (v *ReleaseVersion) UnmarshalText(text []byte) error {
	if v == nil {
		return releaseIdentityError("release version receiver is nil")
	}
	if len(text) == 0 || len(text) > releaseVersionMaximumBytes {
		return releaseIdentityError("release version has invalid length")
	}
	parsed, err := parseReleaseVersion(string(text))
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}

// BuildCommit is a canonical SHA-1 or SHA-256 Git object name.
type BuildCommit struct {
	value [buildCommitSHA256Bytes]byte
	size  uint8
}

const buildCommitWidthDiagnostic = "build commit has unsupported width"

// ParseBuildCommit accepts canonical lower hexadecimal at a supported Git
// object-name width.
func ParseBuildCommit(value string) (BuildCommit, error) {
	size := len(value) / 2
	if len(value)%2 != 0 || size != buildCommitSHA1Bytes && size != buildCommitSHA256Bytes {
		return BuildCommit{}, releaseIdentityError(buildCommitWidthDiagnostic)
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || hex.EncodeToString(decoded) != value {
		return BuildCommit{}, releaseIdentityError("build commit is not canonical lowercase hexadecimal")
	}
	var commit BuildCommit
	copy(commit.value[:], decoded)
	if size == buildCommitSHA1Bytes {
		commit.size = buildCommitSHA1Bytes
	} else {
		commit.size = buildCommitSHA256Bytes
	}
	return commit, nil
}

// Validate proves supported width and zero padding.
func (c BuildCommit) Validate() error {
	if c.size != buildCommitSHA1Bytes && c.size != buildCommitSHA256Bytes {
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
		return errors.Join(ErrPrimitiveContract, err)
	}
	parsed, err := ParseBuildCommit(value)
	if err != nil {
		return errors.Join(ErrPrimitiveContract, ErrJSONContract, err)
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
	wire, err := DecodeStrictJSONStructure[buildIdentityWire](data, StrictJSONLimits{
		DocumentMaximumBytes: ByteCount{value: buildIdentityJSONMaximumBytes},
		NestingDepthMaximum:  2,
		ObjectFieldMaximum:   4,
		ArrayItemMaximum:     1,
	})
	if err != nil {
		return errors.Join(ErrPrimitiveContract, ErrJSONContract, err)
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

func releaseIdentityError(message string, causes ...error) error {
	return errors.Join(append([]error{ErrPrimitiveContract, errors.New(message)}, causes...)...)
}
