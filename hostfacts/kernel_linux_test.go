//go:build linux

package hostfacts

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

func TestKernelObservationMatchesNativeUnameWithoutHostIdentity(t *testing.T) {
	t.Parallel()
	got, err := ObserveKernel(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	output, err := exec.CommandContext(t.Context(), "uname", "-srm").Output()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Fields(string(output))
	if len(want) != 3 || got.System.String() != want[0] || got.Release.String() != want[1] || got.Machine.String() != want[2] || got.Validate() != nil {
		t.Fatalf("observation=%+v native=%q, want exact native system/release/machine", got, output)
	}
	var native syscall.Utsname
	if err := syscall.Uname(&native); err != nil {
		t.Fatal(err)
	}
	var release [65]byte
	for i, value := range native.Release {
		release[i] = byte(value)
	}
	wantRelease := release[:bytes.IndexByte(release[:], 0)]
	if got.Release.String() != string(wantRelease) {
		t.Fatalf("release=%q, want direct syscall %q", got.Release.String(), wantRelease)
	}
}

func FuzzKernelNativeFieldAdmissionMatchesIndependentCStringOracle(f *testing.F) {
	observation, err := ObserveKernel(f.Context())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(append([]byte(observation.Release.String()), 0))
	f.Add([]byte{})
	f.Add(bytes.Repeat([]byte{'x'}, 65))
	f.Add([]byte("Linux\x00\xffignored"))
	f.Fuzz(func(t *testing.T, data []byte) {
		// uname's platform schema is exactly 65 bytes. Copy mutated bytes into
		// that real native field; bytes beyond its extent cannot become a field.
		var field [65]byte
		copy(field[:], data)
		got, err := admitKernelField(field[:])
		var prefix []byte
		terminated := false
		for _, value := range field {
			if value == 0 {
				terminated = true
				break
			}
			prefix = append(prefix, value)
		}
		admitted := terminated && len(prefix) > 0 && utf8.Valid(prefix)
		for _, value := range string(prefix) {
			if unicode.IsSpace(value) || unicode.IsControl(value) {
				admitted = false
			}
		}
		if !admitted {
			if !errors.Is(err, core.ErrHostFactsObservation) || got != (KernelText{}) {
				t.Fatalf("native=%x observation=%q/%v, want zero typed refusal", field, got.String(), err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.String() != string(prefix) {
			t.Fatalf("native=%x observation=%q/%v, want exact independently terminated %q", field, got.String(), err, prefix)
		}
	})
}
