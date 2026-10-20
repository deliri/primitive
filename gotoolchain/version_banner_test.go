package gotoolchain

import (
	"bytes"
	"errors"
	"go/version"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestNativeVersionBannerOwnsProviderAndFirstIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "release", source: "go version go1.27.2 linux/amd64\n", want: "go1.27.2"},
		{name: "release candidate", source: "go version go1.27rc1\n", want: "go1.27rc1"},
		{name: "beta", source: "go version go1.28beta2\n", want: "go1.28beta2"},
		{name: "custom", source: "go version go1.27.2-custom\n", want: "go1.27.2-custom"},
		{name: "unicode spaces", source: "\tgo\u0085version\u2000go1.27.2\n", want: "go1.27.2"},
		{name: "unterminated", source: "go version go1.27.2", want: "go1.27.2"},
		{name: "first valid owns output", source: "go version invalid\ngo version go1.27.2\ngo version go1.28.0\n", want: "go1.27.2"},
		{name: "foreign diagnostic before banner", source: "foreign version go1.28.0\ngo version go1.27.2\n", want: "go1.27.2"},
		{name: "empty"},
		{name: "whitespace", source: " \t\n\r\n"},
		{name: "foreign provider", source: "foreign version go1.27.2\n"},
		{name: "prefixed provider", source: "prefix go version go1.27.2\n"},
		{name: "wrong verb", source: "go diagnostic go1.27.2\n"},
		{name: "missing version", source: "go version\n"},
		{name: "split header across physical lines", source: "go\nversion go1.27.2\n"},
		{name: "split version across physical lines", source: "go version\ngo1.27.2\n"},
		{name: "bad native version", source: "go version go1.027.2\n"},
		{name: "invalid utf8", source: "go version go1.27.2-\xff\n"},
		{name: "BOM is not native whitespace", source: "\ufeffgo version go1.27.2\n"},
		{name: "many trailing fields", source: "go version go1.27.2 " + strings.Repeat("ignored ", 4097), want: "go1.27.2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ObserveVersionBanner(VersionBannerRequest{Output: []byte(tc.source)})
			if err := got.Validate(); err != nil {
				t.Fatal(err)
			}
			if (got.Presence == VersionBannerPresent) != (tc.want != "") || got.Version.String() != tc.want {
				t.Fatalf("native banner=%+v want%q", got, tc.want)
			}
		})
	}
}

func TestNativeVersionBannerContractsRejectContradictoryPresence(t *testing.T) {
	t.Parallel()
	value, err := ParseToolchainVersion("go1.27.2")
	if err != nil {
		t.Fatal(err)
	}
	for ordinal := range 256 {
		presence := VersionBannerPresence(ordinal)
		err := presence.Validate()
		want := presence == VersionBannerAbsent || presence == VersionBannerPresent
		if (err == nil) != want || err != nil && !errors.Is(err, core.ErrGoToolchainContract) {
			t.Fatalf("presence%d=%v want%t", ordinal, err, want)
		}
	}
	VersionBannerPresenceUnknown.OffWireEnum()
	for _, observation := range []VersionBannerObservation{{}, {Presence: VersionBannerAbsent, Version: value}, {Presence: VersionBannerPresent}, {Presence: 255, Version: value}} {
		if err := observation.Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
			t.Fatalf("contradiction=%+v/%v", observation, err)
		}
	}
}

func TestNativeVersionBannerAllocationDoesNotFollowIgnoredOutput(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, extent := range []int{64, 2 << 20} {
		source := bytes.Repeat([]byte("x"), extent+64)
		copy(source, []byte("go version go1.27.2 linux/amd64\n"))
		before, err := hostfacts.ObserveGoAllocationTotal(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		got := ObserveVersionBanner(VersionBannerRequest{Output: source})
		after, err := hostfacts.ObserveGoAllocationTotal(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		allocated := int64(after.Uint64()) - int64(before.Uint64())
		t.Logf("native ignored bytes%d version allocation%d", extent, allocated)
		if got.Version.String() != "go1.27.2" || allocated > 256<<10 {
			t.Fatalf("native version collected output: %+v/%dbytes", got, allocated)
		}
		runtime.KeepAlive(got)
	}
}

func FuzzNativeVersionBannerMatchesIndependentProviderOracle(f *testing.F) {
	for _, source := range []string{"go version go1.27.2 linux/amd64\n", "go version go1.27rc1\n", "foreign version go1.27.2\n", "go version invalid\ngo version go1.27.2\n", "go\nversion go1.27.2\n", "go version go1.27.2-\xff\n", ""} {
		f.Add([]byte(source))
	}
	pattern := regexp.MustCompile(`(?m)^[\t\v\f\r \x{0085}\p{Z}]*go[\t\v\f\r \x{0085}\p{Z}]+version[\t\v\f\r \x{0085}\p{Z}]+(go[^\t\v\f\r \x{0085}\p{Z}\n]+)(?:[\t\v\f\r \x{0085}\p{Z}]+[^\n]*)?$`)
	f.Fuzz(func(t *testing.T, data []byte) {
		want := ""
		remaining := data
		for {
			match := pattern.FindSubmatchIndex(remaining)
			if match == nil {
				break
			}
			candidate := string(remaining[match[2]:match[3]])
			if utf8.ValidString(candidate) && version.IsValid(candidate) {
				want = candidate
				break
			}
			remaining = remaining[match[1]:]
		}
		got := ObserveVersionBanner(VersionBannerRequest{Output: data})
		if got.Validate() != nil || got.Version.String() != want || (got.Presence == VersionBannerPresent) != (want != "") {
			t.Fatalf("native version observation=%+v want%q", got, want)
		}
		if want != "" {
			second := ObserveVersionBanner(VersionBannerRequest{Output: []byte("go version " + want + "\n")})
			if second != got {
				t.Fatalf("canonical native banner=%+v want%+v", second, got)
			}
		}
	})
}
