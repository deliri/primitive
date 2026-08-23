package secretstore

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// GoogleProjectIDMaximumBytes is Google's exact project-ID byte ceiling.
	GoogleProjectIDMaximumBytes = 30
	googleProjectIDMinimumBytes = 6
	// GoogleSecretIDMaximumBytes is Google's exact secret-ID byte ceiling.
	GoogleSecretIDMaximumBytes       = 255
	googleSecretIDMinimumBytes       = 1
	googleProjectResourcePrefix      = "projects/"
	googleSecretResourceSegment      = "/secrets/"
	googleVersionResourceSegment     = "/versions/"
	googleLatestVersionText          = "latest"
	googleProjectNumberMaximumDigits = 20
	googleVersionMaximumDigits       = 20
	googleResolvedNameMaximumBytes   = len(googleProjectResourcePrefix) + googleProjectNumberMaximumDigits +
		len(googleSecretResourceSegment) + GoogleSecretIDMaximumBytes +
		len(googleVersionResourceSegment) + googleVersionMaximumDigits
)

// GoogleProjectID is one admitted Google Cloud project identifier.
type GoogleProjectID struct{ value string }

// ParseGoogleProjectID admits one canonical Google Cloud project identifier.
func ParseGoogleProjectID(value string) (GoogleProjectID, error) {
	project := GoogleProjectID{value: value}
	if err := project.Validate(); err != nil {
		return GoogleProjectID{}, err
	}
	return project, nil
}

// Validate rejects values outside Google's documented project-ID grammar.
func (p GoogleProjectID) Validate() error {
	if len(p.value) < googleProjectIDMinimumBytes || len(p.value) > GoogleProjectIDMaximumBytes ||
		!utf8.ValidString(p.value) {
		return contractError("Google project ID extent is invalid")
	}
	if !isLowerASCII(p.value[0]) || !isLowerASCIIOrDigit(p.value[len(p.value)-1]) {
		return contractError("Google project ID boundary byte is invalid")
	}
	for index := 1; index < len(p.value)-1; index++ {
		if !isLowerASCIIOrDigit(p.value[index]) && p.value[index] != '-' {
			return contractError("Google project ID contains an invalid byte")
		}
	}
	return nil
}

// String returns the admitted provider identifier or empty text when unset.
func (p GoogleProjectID) String() string { return p.value }

// GoogleSecretID is one admitted Google Secret Manager secret identifier.
type GoogleSecretID struct{ value string }

// ParseGoogleSecretID admits one canonical Google Secret Manager secret ID.
func ParseGoogleSecretID(value string) (GoogleSecretID, error) {
	secret := GoogleSecretID{value: value}
	if err := secret.Validate(); err != nil {
		return GoogleSecretID{}, err
	}
	return secret, nil
}

// Validate rejects values outside Google's documented secret-ID grammar.
func (s GoogleSecretID) Validate() error {
	if len(s.value) < googleSecretIDMinimumBytes || len(s.value) > GoogleSecretIDMaximumBytes ||
		!utf8.ValidString(s.value) {
		return contractError("Google secret ID extent is invalid")
	}
	for index := range len(s.value) {
		value := s.value[index]
		if !isASCIIAlphaNumeric(value) && value != '-' && value != '_' {
			return contractError("Google secret ID contains an invalid byte")
		}
	}
	return nil
}

// String returns the admitted provider identifier or empty text when unset.
func (s GoogleSecretID) String() string { return s.value }

// GoogleVersionSelector is the closed set of admitted input selectors.
type GoogleVersionSelector uint8

const (
	// GoogleVersionSelectorInvalid is the invalid zero selector.
	GoogleVersionSelectorInvalid GoogleVersionSelector = iota
	// GoogleVersionSelectorLatest selects the provider's current enabled version.
	GoogleVersionSelectorLatest
	googleVersionSelectorLimit
)

// Validate rejects selectors outside the closed version domain.
func (v GoogleVersionSelector) Validate() error {
	if !v.IsValid() {
		return contractError("secret version is outside the admitted domain")
	}
	return nil
}

// IsValid reports membership in the closed version domain.
func (v GoogleVersionSelector) IsValid() bool { return v == GoogleVersionSelectorLatest }

// String returns the canonical provider selector or the shared unknown label.
func (v GoogleVersionSelector) String() string {
	if !v.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return googleLatestVersionText
}

// OffWireEnum marks GoogleVersionSelector as a compiler-owned off-wire enum.
func (GoogleVersionSelector) OffWireEnum() {}

func (v GoogleVersionSelector) resourceText() string {
	return v.String()
}

// GoogleProjectNumber identifies one positive provider-resolved project.
type GoogleProjectNumber uint64

// NewGoogleProjectNumber admits one positive provider-resolved project.
func NewGoogleProjectNumber(value uint64) (GoogleProjectNumber, error) {
	project := GoogleProjectNumber(value)
	if err := project.Validate(); err != nil {
		return 0, err
	}
	return project, nil
}

// Validate rejects the unset provider project number.
func (p GoogleProjectNumber) Validate() error {
	if p == 0 {
		return contractError("resolved Google project number is unset")
	}
	return nil
}

// Uint64 returns the provider-resolved numeric project identity.
func (p GoogleProjectNumber) Uint64() uint64 { return uint64(p) }

// GoogleVersionNumber identifies one positive provider-resolved version.
type GoogleVersionNumber uint64

// NewGoogleVersionNumber admits one positive provider-resolved version.
func NewGoogleVersionNumber(value uint64) (GoogleVersionNumber, error) {
	version := GoogleVersionNumber(value)
	if err := version.Validate(); err != nil {
		return 0, err
	}
	return version, nil
}

// Validate rejects the unset provider version number.
func (v GoogleVersionNumber) Validate() error {
	if v == 0 {
		return contractError("resolved Google secret version is unset")
	}
	return nil
}

// Uint64 returns the provider-resolved numeric version.
func (v GoogleVersionNumber) Uint64() uint64 { return uint64(v) }

// String returns the decimal provider version or the shared unknown label.
func (v GoogleVersionNumber) String() string {
	if v.Validate() != nil {
		return core.UnknownEnumDiagnostic
	}
	return strconv.FormatUint(uint64(v), 10)
}

// AccessRequest names one provider secret through the sole admitted selector.
type AccessRequest struct {
	Project GoogleProjectID
	Secret  GoogleSecretID
	Version GoogleVersionSelector
}

// Validate rejects incomplete or unsupported provider references.
func (r AccessRequest) Validate() error {
	if err := errors.Join(r.Project.Validate(), r.Secret.Validate(), r.Version.Validate()); err != nil {
		return errors.Join(core.ErrSecretStoreContract, err)
	}
	return nil
}

func (r AccessRequest) resourceName() (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	var result strings.Builder
	result.Grow(len(googleProjectResourcePrefix) + len(r.Project.value) +
		len(googleSecretResourceSegment) + len(r.Secret.value) +
		len(googleVersionResourceSegment) + len(googleLatestVersionText))
	result.WriteString(googleProjectResourcePrefix)
	result.WriteString(r.Project.value)
	result.WriteString(googleSecretResourceSegment)
	result.WriteString(r.Secret.value)
	result.WriteString(googleVersionResourceSegment)
	result.WriteString(r.Version.resourceText())
	return result.String(), nil
}

// ResolvedReference binds returned material to one exact provider version.
type ResolvedReference struct {
	Secret        GoogleSecretID
	ProjectNumber GoogleProjectNumber
	Version       GoogleVersionNumber
}

// Validate rejects incomplete or nonnumeric resolved references.
func (r ResolvedReference) Validate() error {
	if err := errors.Join(r.ProjectNumber.Validate(), r.Secret.Validate(), r.Version.Validate()); err != nil {
		return errors.Join(core.ErrSecretStoreContract, err)
	}
	return nil
}

func (r ResolvedReference) matches(request AccessRequest) bool {
	return r.Secret == request.Secret
}

// AccessResult binds bounded secret custody to the exact resolved reference.
type AccessResult struct {
	Value     Value
	Request   AccessRequest
	Reference ResolvedReference
}

// Validate rejects incomplete result bindings or unavailable secret custody.
func (r AccessResult) Validate() error {
	if err := errors.Join(r.Request.Validate(), r.Reference.Validate(), r.Value.Validate()); err != nil {
		return errors.Join(core.ErrSecretStoreContract, err)
	}
	if !r.Reference.matches(r.Request) {
		return contractError("resolved Google secret does not match the request")
	}
	return nil
}

func contractError(detail string) error {
	return errors.Join(core.ErrSecretStoreContract, errors.New(detail))
}

func isLowerASCII(value byte) bool { return value >= 'a' && value <= 'z' }

func isLowerASCIIOrDigit(value byte) bool {
	return isLowerASCII(value) || value >= '0' && value <= '9'
}

func isASCIIAlphaNumeric(value byte) bool {
	return isLowerASCIIOrDigit(value) || value >= 'A' && value <= 'Z'
}
