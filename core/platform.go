package core

import (
	"errors"
	"runtime"
	"strings"
)

const (
	operatingSystemDarwinText  = "darwin"
	operatingSystemLinuxText   = "linux"
	operatingSystemWindowsText = "windows"
	architectureAMD64Text      = "amd64"
	architectureARM64Text      = "arm64"
	platformTokenSeparator     = "-"
	// PlatformTokenMaximumBytes bounds every currently admitted platform token.
	// Exhaustive tests force this constant to grow with any longer enum member.
	PlatformTokenMaximumBytes = len(operatingSystemWindowsText + platformTokenSeparator + architectureAMD64Text)
)

// OperatingSystem is a closed set of operating systems supported by Primitive.
type OperatingSystem uint8

const (
	// OperatingSystemUnknown is the invalid zero operating system.
	OperatingSystemUnknown OperatingSystem = iota
	// OperatingSystemDarwin identifies Darwin.
	OperatingSystemDarwin
	// OperatingSystemLinux identifies Linux.
	OperatingSystemLinux
	// OperatingSystemWindows identifies Windows.
	OperatingSystemWindows
	operatingSystemLimit
)

// CPUArchitecture is a closed set of CPU architectures supported by Primitive.
type CPUArchitecture uint8

const (
	// CPUArchitectureUnknown is the invalid zero architecture.
	CPUArchitectureUnknown CPUArchitecture = iota
	// CPUArchitectureAMD64 identifies amd64.
	CPUArchitectureAMD64
	// CPUArchitectureARM64 identifies arm64.
	CPUArchitectureARM64
	cpuArchitectureLimit
)

// Platform pairs one admitted operating system and CPU architecture.
type Platform struct {
	// OperatingSystem is the platform operating system.
	OperatingSystem OperatingSystem
	// Architecture is the platform CPU architecture.
	Architecture CPUArchitecture
}

// NewPlatform validates and constructs a platform.
func NewPlatform(operatingSystem OperatingSystem, architecture CPUArchitecture) (Platform, error) {
	platform := Platform{OperatingSystem: operatingSystem, Architecture: architecture}
	if err := platform.Validate(); err != nil {
		return Platform{}, err
	}
	return platform, nil
}

// CurrentSupportedPlatform converts runtime.GOOS and runtime.GOARCH into the
// closed Primitive platform domain. It returns ErrPrimitiveContract on hosts
// not admitted by OperatingSystem or CPUArchitecture.
func CurrentSupportedPlatform() (Platform, error) {
	operatingSystem, err := ParseOperatingSystem(runtime.GOOS)
	if err != nil {
		return Platform{}, err
	}
	architecture, err := ParseCPUArchitecture(runtime.GOARCH)
	if err != nil {
		return Platform{}, err
	}
	return NewPlatform(operatingSystem, architecture)
}

// ParsePlatform accepts canonical "operating-system-architecture" text.
func ParsePlatform(value string) (Platform, error) {
	if len(value) == 0 || len(value) > PlatformTokenMaximumBytes {
		return Platform{}, platformError("platform token has invalid length")
	}
	operatingSystemText, architectureText, found := strings.Cut(value, platformTokenSeparator)
	if !found || operatingSystemText == "" || architectureText == "" || strings.Contains(architectureText, platformTokenSeparator) {
		return Platform{}, platformError("platform token is not canonical")
	}
	operatingSystem, err := ParseOperatingSystem(operatingSystemText)
	if err != nil {
		return Platform{}, err
	}
	architecture, err := ParseCPUArchitecture(architectureText)
	if err != nil {
		return Platform{}, err
	}
	return NewPlatform(operatingSystem, architecture)
}

// ParseOperatingSystem accepts one canonical lowercase operating-system token.
func ParseOperatingSystem(value string) (OperatingSystem, error) {
	switch value {
	case operatingSystemDarwinText:
		return OperatingSystemDarwin, nil
	case operatingSystemLinuxText:
		return OperatingSystemLinux, nil
	case operatingSystemWindowsText:
		return OperatingSystemWindows, nil
	default:
		return OperatingSystemUnknown, platformError("operating system is not admitted")
	}
}

// ParseCPUArchitecture accepts one canonical lowercase architecture token.
func ParseCPUArchitecture(value string) (CPUArchitecture, error) {
	switch value {
	case architectureAMD64Text:
		return CPUArchitectureAMD64, nil
	case architectureARM64Text:
		return CPUArchitectureARM64, nil
	default:
		return CPUArchitectureUnknown, platformError("CPU architecture is not admitted")
	}
}

// String returns canonical platform text, or empty text when invalid.
func (p Platform) String() string {
	if err := p.Validate(); err != nil {
		return ""
	}
	return p.OperatingSystem.String() + platformTokenSeparator + p.Architecture.String()
}

// Validate enforces both closed enum members.
func (p Platform) Validate() error {
	if err := p.OperatingSystem.Validate(); err != nil {
		return err
	}
	if err := p.Architecture.Validate(); err != nil {
		return err
	}
	return nil
}

// MarshalJSON emits canonical platform text as a JSON string.
func (p Platform) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return marshalJSONString(p.String())
}

// UnmarshalJSON accepts only canonical admitted platform text.
func (p *Platform) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(ErrJSONContract, errors.New("nil platform receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	decoded, err := ParsePlatform(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*p = decoded
	return nil
}

// String returns the canonical lowercase operating-system token.
func (o OperatingSystem) String() string {
	switch o {
	case OperatingSystemDarwin:
		return operatingSystemDarwinText
	case OperatingSystemLinux:
		return operatingSystemLinuxText
	case OperatingSystemWindows:
		return operatingSystemWindowsText
	default:
		return ""
	}
}

// Validate rejects operating systems outside the closed domain.
func (o OperatingSystem) Validate() error {
	if o <= OperatingSystemUnknown || o >= operatingSystemLimit {
		return platformError("operating system is invalid")
	}
	return nil
}

// IsValid reports whether o belongs to the closed operating-system domain.
func (o OperatingSystem) IsValid() bool { return o.Validate() == nil }

// MarshalJSON emits the canonical operating-system token.
func (o OperatingSystem) MarshalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return marshalJSONString(o.String())
}

// UnmarshalJSON accepts only a canonical admitted operating-system token.
func (o *OperatingSystem) UnmarshalJSON(data []byte) error {
	if o == nil {
		return errors.Join(ErrJSONContract, platformError("nil operating system receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := ParseOperatingSystem(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*o = parsed
	return nil
}

// String returns the canonical lowercase architecture token.
func (a CPUArchitecture) String() string {
	switch a {
	case CPUArchitectureAMD64:
		return architectureAMD64Text
	case CPUArchitectureARM64:
		return architectureARM64Text
	default:
		return ""
	}
}

// Validate rejects architectures outside the closed domain.
func (a CPUArchitecture) Validate() error {
	if a <= CPUArchitectureUnknown || a >= cpuArchitectureLimit {
		return platformError("CPU architecture is invalid")
	}
	return nil
}

// IsValid reports whether a belongs to the closed architecture domain.
func (a CPUArchitecture) IsValid() bool { return a.Validate() == nil }

// MarshalJSON emits the canonical architecture token.
func (a CPUArchitecture) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return marshalJSONString(a.String())
}

// UnmarshalJSON accepts only a canonical admitted architecture token.
func (a *CPUArchitecture) UnmarshalJSON(data []byte) error {
	if a == nil {
		return errors.Join(ErrJSONContract, platformError("nil CPU architecture receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := ParseCPUArchitecture(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*a = parsed
	return nil
}

func platformError(message string) error {
	return errors.Join(ErrPrimitiveContract, errors.New(message))
}
