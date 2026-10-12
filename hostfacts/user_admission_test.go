package hostfacts

import (
	"errors"
	"fmt"
	"github.com/deliri/primitive/v2026/core"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestEffectiveUserLabelAdmissionExhaustsByteDomain(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		t.Run(fmt.Sprintf("native_label_byte_%02x", value), func(t *testing.T) {
			t.Parallel()
			label := UserName{value: string([]byte{byte(value)})}
			var want error
			if value < 32 || value >= 127 {
				want = core.ErrHostFactsContract
			}
			if err := label.Validate(); !errors.Is(err, want) {
				t.Fatalf("label byte=%02x validation=%v, want %v", value, err, want)
			}
		})
	}
}

func FuzzEffectiveUserLabelSemanticAdmission(f *testing.F) {
	value, err := ObserveEffectiveUser(f.Context())
	if err == nil {
		f.Add(value.String())
	} else if !errors.Is(err, core.ErrHostFactsUnsupported) {
		f.Fatal(err)
	}
	f.Add("")
	f.Add("label\x00control")
	f.Add("équipe native")
	f.Fuzz(func(t *testing.T, text string) {
		label := UserName{value: text}
		admitted := len(text) > 0 && utf8.ValidString(text)
		for _, r := range text {
			if unicode.IsControl(r) {
				admitted = false
			}
		}
		err := label.Validate()
		if admitted {
			if err != nil || label.String() != text {
				t.Fatalf("admitted label validation=%v, want exact native label", err)
			}
		} else if !errors.Is(err, core.ErrHostFactsContract) {
			t.Fatalf("label validation=%v, want typed refusal", err)
		}
	})
}
