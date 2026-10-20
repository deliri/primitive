package gotoolchain

import (
	"errors"
	"go/version"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

func TestToolchainVersionUsesNativeGoGrammar(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"go1.0", "go1.27", "go1.27.0", "go1.27.99", "go1.27beta1", "go1.27rc1", "go1.27rc2", "go1.27foo", "go1.0-custom", "go1.27.1-custom", "go2.0", "go2.1rc1",
		"", "go", "go1", "1.27.0", "Go1.27.0", "go1..2", "go1.027.1", "go1.27.01", "go1.27.0.1", "go1.27rc01", "go1.27rc1.1", "go1.27.0 ", " go1.27.0", "devel go1.28",
		"go1.27.0-", "go1.27.0-α", "go1.27.0-\x00", "go1.27.0-\xff", "go1.27RC1", "go1.27beta", "go1.27rc0", "go0.0", "go1.27.0\n", "go1.27.0\x00",
		"go1." + strings.Repeat("9", 64), "go1." + strings.Repeat("9", 65), "go1." + strings.Repeat("9", 4097), "go1.27.0-" + strings.Repeat("custom", 4097),
	} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			got, err := ParseToolchainVersion(value)
			want := utf8.ValidString(value) && version.IsValid(value)
			if (err == nil) != want {
				t.Fatalf("native version parity for%q=%v want admitted%t", value, err, want)
			}
			if !want {
				if !errors.Is(err, core.ErrGoToolchainContract) || got != (ToolchainVersion{}) {
					t.Fatalf("rejected version=%+v/%v", got, err)
				}
				return
			}
			if got.String() != value || got.Validate() != nil {
				t.Fatalf("native version data changed:%q want%q", got.String(), value)
			}
		})
	}
}

func FuzzToolchainVersionNativeAdmissionParity(f *testing.F) {
	for _, value := range []string{"go1.27.2", "go1.27rc1", "go1.0-custom", "go2.0", "go1.27.0-\xff", "", "go1..2", "go1." + strings.Repeat("9", 65)} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		want := utf8.ValidString(value) && version.IsValid(value)
		got, err := ParseToolchainVersion(value)
		if (err == nil) != want {
			t.Fatalf("native admission%t got%+v/%v for%q", want, got, err, value)
		}
		if err != nil {
			if !errors.Is(err, core.ErrGoToolchainContract) || got != (ToolchainVersion{}) {
				t.Fatalf("refused value leaked:%+v/%v", got, err)
			}
			return
		}
		roundtrip, err := ParseToolchainVersion(got.String())
		if got.Validate() != nil || got.String() != value || err != nil || roundtrip != got {
			t.Fatalf("native version closure:%+v/%+v/%v", got, roundtrip, err)
		}
	})
}
