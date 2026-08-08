package hostfacts

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestAdmitObservedPathRejectsUnusablePlatformAnswers pins the observation
// taxonomy the happy-path oracle test cannot reach: a base path the platform
// reports that is not absolute is a failed observation of the host, never a
// contract breach, so a product can tell "this host cannot answer" from "the
// caller passed something wrong."
func TestAdmitObservedPathRejectsUnusablePlatformAnswers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		input   string
	}{
		{name: "absolute base is admitted", input: "/home/ase/.config"},
		{name: "filesystem root is admitted", input: "/"},
		{name: "nested absolute base is admitted", input: "/var/folders/tmp"},
		{name: "empty platform answer is a failed observation", input: "", wantErr: core.ErrHostFactsObservation},
		{name: "relative answer is a failed observation", input: "relative/config", wantErr: core.ErrHostFactsObservation},
		{name: "bare name answer is a failed observation", input: "config", wantErr: core.ErrHostFactsObservation},
		{name: "current-directory answer is a failed observation", input: ".", wantErr: core.ErrHostFactsObservation},
		{name: "parent-relative answer is a failed observation", input: "../config", wantErr: core.ErrHostFactsObservation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := admitObservedPath(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("admitObservedPath(%q) error = %v, want errors.Is(..., %v)", tc.input, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("admitObservedPath(%q) error = %v, want nil", tc.input, gotErr)
			}
			if got.String() != tc.input {
				t.Fatalf("admitObservedPath(%q) = %q, want the admitted path", tc.input, got.String())
			}
		})
	}
}

// TestAdmitHostnameRejectsAnswersNoLabelCanCarry pins the hostname observation
// boundary the live oracle cannot force: the platform's own name is carried
// when it fits a device record, and every unusable answer is a failed
// observation rather than a value.
func TestAdmitHostnameRejectsAnswersNoLabelCanCarry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		input   string
	}{
		{name: "ordinary host label is admitted", input: "ase-mbp"},
		{name: "dotted fqdn is admitted", input: "host.example.internal"},
		{name: "single byte is admitted", input: "a"},
		{name: "maximum extent is admitted", input: strings.Repeat("a", hostnameMaximumBytes)},
		{name: "empty answer is a failed observation", input: "", wantErr: core.ErrHostFactsObservation},
		{name: "one byte over the ceiling is a failed observation", input: strings.Repeat("a", hostnameMaximumBytes+1), wantErr: core.ErrHostFactsObservation},
		{name: "embedded NUL is a failed observation", input: "host\x00name", wantErr: core.ErrHostFactsObservation},
		{name: "unit separator control byte is a failed observation", input: "host\x1fname", wantErr: core.ErrHostFactsObservation},
		{name: "DEL byte is a failed observation", input: "host\x7fname", wantErr: core.ErrHostFactsObservation},
		{name: "newline is a failed observation", input: "host\nname", wantErr: core.ErrHostFactsObservation},
		{name: "invalid utf8 lead byte is a failed observation", input: "host\xffname", wantErr: core.ErrHostFactsObservation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := admitHostname(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("admitHostname(%q) error = %v, want errors.Is(..., %v)", tc.input, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("admitHostname(%q) error = %v, want nil", tc.input, gotErr)
			}
			if got.String() != tc.input {
				t.Fatalf("admitHostname(%q) = %q, want the admitted name", tc.input, got.String())
			}
		})
	}
}
