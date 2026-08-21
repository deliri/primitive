package core

import (
	json "encoding/json/v2"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestPlatformExhaustsClosedDomainAndJSONBoundary(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		operatingSystem := OperatingSystem(raw)
		wantOperatingSystem := operatingSystem > operatingSystemUnknown && operatingSystem < operatingSystemLimit
		provePlatformEnumValue(t, raw, operatingSystem.IsValid(), operatingSystem.Validate(), wantOperatingSystem)

		architecture := CPUArchitecture(raw)
		wantArchitecture := architecture > cpuArchitectureUnknown && architecture < cpuArchitectureLimit
		provePlatformEnumValue(t, raw, architecture.IsValid(), architecture.Validate(), wantArchitecture)
	}

	for operatingSystem := OperatingSystemDarwin; operatingSystem < operatingSystemLimit; operatingSystem++ {
		for architecture := CPUArchitectureAMD64; architecture < cpuArchitectureLimit; architecture++ {
			platform := Platform{OperatingSystem: operatingSystem, Architecture: architecture}
			if err := platform.Validate(); err != nil {
				t.Fatalf("Platform{%v, %v}.Validate() error = %v, want nil", operatingSystem, architecture, err)
			}
			parsed, err := parsePlatform(platform.String())
			if err != nil || parsed != platform {
				t.Fatalf("parsePlatform(%q) = (%v, %v), want (%v, nil)", platform, parsed, err, platform)
			}
			wire, err := json.Marshal(platform)
			if err != nil {
				t.Fatalf("json.Marshal(%v) error = %v, want nil", platform, err)
			}
			var decoded Platform
			if err := json.Unmarshal(wire, &decoded); err != nil || decoded != platform {
				t.Fatalf("json.Unmarshal(%s) = (%v, %v), want (%v, nil)", wire, decoded, err, platform)
			}
			if len(platform.String()) > platformTokenMaximumBytes {
				t.Fatalf("platform token %q exceeds compiler-owned maximum %d", platform, platformTokenMaximumBytes)
			}
		}
	}
}

func provePlatformEnumValue(t *testing.T, raw int, gotValid bool, gotErr error, wantValid bool) {
	t.Helper()

	if gotValid != wantValid || (gotErr == nil) != wantValid {
		t.Fatalf("platform enum value %d validity = (%t, %v), want %t", raw, gotValid, gotErr, wantValid)
	}
	if !wantValid && !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("platform enum value %d error = %v, want %v", raw, gotErr, ErrPrimitiveContract)
	}
}

func TestPlatformRejectedJSONPreservesReceiver(t *testing.T) {
	t.Parallel()

	before := Platform{OperatingSystem: OperatingSystemLinux, Architecture: CPUArchitectureAMD64}
	for _, wire := range [][]byte{
		[]byte(`"linux-386"`),
		[]byte(`"darwin-amd64-extra"`),
		[]byte(`"Linux-amd64"`),
		[]byte("null"),
	} {
		got := before
		gotErr := json.Unmarshal(wire, &got)
		if !errors.Is(gotErr, ErrJSONContract) || !errors.Is(gotErr, ErrPrimitiveContract) {
			t.Fatalf("json.Unmarshal(%s) error = %v, want %v and %v", wire, gotErr, ErrJSONContract, ErrPrimitiveContract)
		}
		if got != before {
			t.Fatalf("json.Unmarshal(%s) receiver = %v, want preserved %v", wire, got, before)
		}
	}

	for _, run := range []func() error{
		func() error { return (*Platform)(nil).UnmarshalJSON(nil) },
		func() error { return (*OperatingSystem)(nil).UnmarshalJSON(nil) },
		func() error { return (*CPUArchitecture)(nil).UnmarshalJSON(nil) },
	} {
		if gotErr := run(); !errors.Is(gotErr, ErrJSONContract) {
			t.Fatalf("nil platform receiver error = %v, want %v", gotErr, ErrJSONContract)
		}
	}
}

func TestPlatformTextDecoderBoundsInputAndPreservesReceiver(t *testing.T) {
	t.Parallel()

	before := Platform{OperatingSystem: OperatingSystemLinux, Architecture: CPUArchitectureAMD64}
	got := before
	gotErr := got.UnmarshalText([]byte(strings.Repeat("x", 1<<20)))
	if !errors.Is(gotErr, ErrPrimitiveContract) || got != before {
		t.Fatalf("Platform.UnmarshalText(oversized) = (%v, %v), want preserved %v and %v", got, gotErr, before, ErrPrimitiveContract)
	}
	if gotErr := (*Platform)(nil).UnmarshalText(nil); !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("nil Platform.UnmarshalText() error = %v, want %v", gotErr, ErrPrimitiveContract)
	}
}
