package hostfacts

import (
	"errors"
	"path/filepath"
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

	root := t.TempDir()
	separator := string(filepath.Separator)
	volumeRoot := filepath.VolumeName(root) + separator
	maximumComponents := volumeRoot + strings.Repeat("a"+separator, core.FilesystemPathMaximumComponents-1) + "a"
	overMaximumComponents := maximumComponents + string(filepath.Separator) + "a"
	tests := []struct {
		wantErr error
		name    string
		input   string
	}{
		{name: "temporary root is admitted", input: root},
		{name: "single child is admitted", input: filepath.Join(root, "child")},
		{name: "nested child is admitted", input: filepath.Join(root, "one", "two")},
		{name: "space inside component is admitted", input: filepath.Join(root, "two words")},
		{name: "unicode component is admitted", input: filepath.Join(root, "café")},
		{name: "dot inside component is admitted", input: filepath.Join(root, "cache.v1")},
		{name: "dash and underscore component is admitted", input: filepath.Join(root, "cache-v1_data")},
		{name: "ampersand inside component is admitted", input: filepath.Join(root, "cache&state")},
		{name: "brackets inside component are admitted", input: filepath.Join(root, "cache[state]")},
		{name: "emoji component is admitted", input: filepath.Join(root, "cache-🧪")},
		{name: "maximum component count is admitted", input: maximumComponents},
		{name: "tab remains data in a component", input: filepath.Join(root, "tab\tname")},
		{name: "newline remains data in a component", input: filepath.Join(root, "line\nname")},
		{name: "carriage return remains data in a component", input: filepath.Join(root, "line\rname")},
		{name: "empty platform answer is a failed observation", input: "", wantErr: core.ErrHostFactsObservation},
		{name: "relative answer is a failed observation", input: "relative/config", wantErr: core.ErrHostFactsObservation},
		{name: "bare name answer is a failed observation", input: "config", wantErr: core.ErrHostFactsObservation},
		{name: "current-directory answer is a failed observation", input: ".", wantErr: core.ErrHostFactsObservation},
		{name: "parent-relative answer is a failed observation", input: "../config", wantErr: core.ErrHostFactsObservation},
		{name: "embedded current-directory component is refused", input: root + string(filepath.Separator) + "." + string(filepath.Separator) + "child", wantErr: core.ErrHostFactsObservation},
		{name: "embedded parent component is refused", input: root + string(filepath.Separator) + "child" + string(filepath.Separator) + "..", wantErr: core.ErrHostFactsObservation},
		{name: "doubled separator is refused", input: root + string(filepath.Separator) + string(filepath.Separator) + "child", wantErr: core.ErrHostFactsObservation},
		{name: "trailing separator is refused", input: root + string(filepath.Separator), wantErr: core.ErrHostFactsObservation},
		{name: "embedded NUL is refused", input: root + string(filepath.Separator) + "bad\x00name", wantErr: core.ErrHostFactsObservation},
		{name: "invalid UTF-8 lead byte is refused", input: root + string(filepath.Separator) + "bad\xffname", wantErr: core.ErrHostFactsObservation},
		{name: "one component above ceiling is refused", input: overMaximumComponents, wantErr: core.ErrHostFactsObservation},
		{name: "absolute parent escape is refused", input: root + string(filepath.Separator) + ".." + string(filepath.Separator) + "escape", wantErr: core.ErrHostFactsObservation},
		{name: "absolute current-directory suffix is refused", input: root + string(filepath.Separator) + ".", wantErr: core.ErrHostFactsObservation},
		{name: "absolute parent suffix is refused", input: root + string(filepath.Separator) + "..", wantErr: core.ErrHostFactsObservation},
		{name: "component-count overflow remains refused after a long prefix", input: overMaximumComponents + string(filepath.Separator) + "tail", wantErr: core.ErrHostFactsObservation},
		{name: "relative path at component ceiling remains refused", input: strings.Repeat("a"+string(filepath.Separator), core.FilesystemPathMaximumComponents-1) + "a", wantErr: core.ErrHostFactsObservation},
		{name: "relative path above component ceiling remains refused", input: strings.Repeat("a"+string(filepath.Separator), core.FilesystemPathMaximumComponents) + "a", wantErr: core.ErrHostFactsObservation},
		{name: "root followed by three separators is refused", input: root + strings.Repeat(string(filepath.Separator), 3) + "child", wantErr: core.ErrHostFactsObservation},
		{name: "noncanonical child then current directory is refused", input: filepath.Join(root, "child") + string(filepath.Separator) + ".", wantErr: core.ErrHostFactsObservation},
		{name: "noncanonical child then parent is refused", input: filepath.Join(root, "child") + string(filepath.Separator) + "..", wantErr: core.ErrHostFactsObservation},
		{name: "NUL at component start is refused", input: root + string(filepath.Separator) + "\x00bad", wantErr: core.ErrHostFactsObservation},
		{name: "NUL at component end is refused", input: root + string(filepath.Separator) + "bad\x00", wantErr: core.ErrHostFactsObservation},
		{name: "invalid UTF-8 tail is refused", input: root + string(filepath.Separator) + "bad\xc3", wantErr: core.ErrHostFactsObservation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := admitObservedPath(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("admitObservedPath(%q) error = %v, want errors.Is(..., %v)", tc.input, gotErr, tc.wantErr)
				}
				if !errors.Is(gotErr, core.ErrPrimitiveContract) {
					t.Fatalf("admitObservedPath(%q) error = %v, want preserved %v identity", tc.input, gotErr, core.ErrPrimitiveContract)
				}
				if got != (core.AbsolutePath{}) {
					t.Fatalf("admitObservedPath(%q) = %v, want zero path on refusal", tc.input, got)
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
		{name: "space is data rather than a control byte", input: "host name"},
		{name: "unicode label is admitted", input: "hôte"},
		{name: "one byte below ceiling is admitted", input: strings.Repeat("b", hostnameMaximumBytes-1)},
		{name: "two bytes below ceiling is admitted", input: strings.Repeat("c", hostnameMaximumBytes-2)},
		{name: "maximum extent is admitted", input: strings.Repeat("a", hostnameMaximumBytes)},
		{name: "multibyte text exactly at byte ceiling is admitted", input: strings.Repeat("é", 126) + "a"},
		{name: "printable punctuation is admitted", input: "host_name-1.example"},
		{name: "empty answer is a failed observation", input: "", wantErr: core.ErrHostFactsObservation},
		{name: "one byte over the ceiling is a failed observation", input: strings.Repeat("a", hostnameMaximumBytes+1), wantErr: core.ErrHostFactsObservation},
		{name: "two bytes over the ceiling are a failed observation", input: strings.Repeat("a", hostnameMaximumBytes+2), wantErr: core.ErrHostFactsObservation},
		{name: "multibyte text one byte over ceiling is refused", input: strings.Repeat("é", 127), wantErr: core.ErrHostFactsObservation},
		{name: "embedded NUL is a failed observation", input: "host\x00name", wantErr: core.ErrHostFactsObservation},
		{name: "leading NUL is a failed observation", input: "\x00hostname", wantErr: core.ErrHostFactsObservation},
		{name: "trailing NUL is a failed observation", input: "hostname\x00", wantErr: core.ErrHostFactsObservation},
		{name: "start-of-heading control byte is refused", input: "host\x01name", wantErr: core.ErrHostFactsObservation},
		{name: "backspace control byte is refused", input: "host\x08name", wantErr: core.ErrHostFactsObservation},
		{name: "tab control byte is refused", input: "host\tname", wantErr: core.ErrHostFactsObservation},
		{name: "line-feed control byte is refused", input: "host\nname", wantErr: core.ErrHostFactsObservation},
		{name: "vertical-tab control byte is refused", input: "host\vname", wantErr: core.ErrHostFactsObservation},
		{name: "form-feed control byte is refused", input: "host\fname", wantErr: core.ErrHostFactsObservation},
		{name: "carriage-return control byte is refused", input: "host\rname", wantErr: core.ErrHostFactsObservation},
		{name: "unit separator control byte is a failed observation", input: "host\x1fname", wantErr: core.ErrHostFactsObservation},
		{name: "DEL byte is a failed observation", input: "host\x7fname", wantErr: core.ErrHostFactsObservation},
		{name: "invalid utf8 lead byte is a failed observation", input: "host\xffname", wantErr: core.ErrHostFactsObservation},
		{name: "truncated two-byte UTF-8 sequence is refused", input: "host\xc3", wantErr: core.ErrHostFactsObservation},
		{name: "truncated three-byte UTF-8 sequence is refused", input: "host\xe2\x82", wantErr: core.ErrHostFactsObservation},
		{name: "continuation byte without a lead is refused", input: "host\x80name", wantErr: core.ErrHostFactsObservation},
		{name: "overlong UTF-8 NUL is refused", input: "host\xc0\x80name", wantErr: core.ErrHostFactsObservation},
		{name: "surrogate UTF-8 encoding is refused", input: "host\xed\xa0\x80name", wantErr: core.ErrHostFactsObservation},
		{name: "maximum ASCII followed by NUL is refused", input: strings.Repeat("a", hostnameMaximumBytes-1) + "\x00", wantErr: core.ErrHostFactsObservation},
		{name: "maximum ASCII followed by control is refused", input: strings.Repeat("a", hostnameMaximumBytes-1) + "\x1f", wantErr: core.ErrHostFactsObservation},
		{name: "valid prefix plus one over ceiling is refused", input: "host-" + strings.Repeat("a", hostnameMaximumBytes-len("host-")+1), wantErr: core.ErrHostFactsObservation},
		{name: "valid prefix plus two over ceiling is refused", input: "host-" + strings.Repeat("a", hostnameMaximumBytes-len("host-")+2), wantErr: core.ErrHostFactsObservation},
		{name: "control at first byte is refused", input: "\x1fhost", wantErr: core.ErrHostFactsObservation},
		{name: "control at final byte is refused", input: "host\x7f", wantErr: core.ErrHostFactsObservation},
		{name: "only newline is refused", input: "\n", wantErr: core.ErrHostFactsObservation},
		{name: "only DEL is refused", input: "\x7f", wantErr: core.ErrHostFactsObservation},
		{name: "invalid UTF-8 after maximum valid prefix is refused", input: strings.Repeat("a", hostnameMaximumBytes-1) + "\xff", wantErr: core.ErrHostFactsObservation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := admitHostname(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("admitHostname(%q) error = %v, want errors.Is(..., %v)", tc.input, gotErr, tc.wantErr)
				}
				if got != (Hostname{}) {
					t.Fatalf("admitHostname(%q) = %v, want zero hostname on refusal", tc.input, got)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("admitHostname(%q) error = %v, want nil", tc.input, gotErr)
			}
			if got.String() != tc.input {
				t.Fatalf("admitHostname(%q) = %q, want the admitted name", tc.input, got.String())
			}
			if gotErr := got.Validate(); gotErr != nil {
				t.Fatalf("admitHostname(%q).Validate() error = %v, want nil", tc.input, gotErr)
			}
		})
	}
}
