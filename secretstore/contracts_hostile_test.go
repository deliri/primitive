package secretstore

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

type textBoundaryCase struct {
	wantErr error
	name    string
	value   string
}

func TestParseGoogleProjectIDHostileTable(t *testing.T) {
	t.Parallel()

	for _, testCase := range googleProjectIDCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseGoogleProjectID(testCase.value)
			if testCase.wantErr != nil {
				if got != (GoogleProjectID{}) || !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("ParseGoogleProjectID() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, testCase.wantErr)
				}
				return
			}
			if gotErr != nil || got.String() != testCase.value || got.Validate() != nil {
				t.Fatalf("ParseGoogleProjectID() = (%q, %v), want exact validated %q", got.String(), gotErr, testCase.value)
			}
		})
	}
}

func googleProjectIDCases() []textBoundaryCase {
	return []textBoundaryCase{
		// Ten expected-valid cases.
		{name: "ordinary hyphenated project is admitted", value: "sample-project"},
		{name: "ordinary numeric suffix is admitted", value: "project-2026"},
		{name: "ordinary internal digit is admitted", value: "p1roject"},
		{name: "ordinary repeated internal hyphens are admitted", value: "project--one"},
		{name: "ordinary all-letter project is admitted", value: "projectalpha"},
		{name: "ordinary digit tail is admitted", value: "project1"},
		{name: "ordinary letter tail after hyphen is admitted", value: "project-a"},
		{name: "ordinary long project is admitted", value: "a123456789-b123456789-c12345"},
		{name: "ordinary six-letter project is admitted", value: "abcdef"},
		{name: "ordinary mixed lowercase project is admitted", value: "a1-b2-c3"},
		// Ten expected-rejection cases.
		{name: "uppercase prefix is rejected", value: "Project1", wantErr: core.ErrSecretStoreContract},
		{name: "uppercase interior is rejected", value: "proJect1", wantErr: core.ErrSecretStoreContract},
		{name: "underscore is rejected", value: "project_one", wantErr: core.ErrSecretStoreContract},
		{name: "period is rejected", value: "project.one", wantErr: core.ErrSecretStoreContract},
		{name: "slash is rejected", value: "project/one", wantErr: core.ErrSecretStoreContract},
		{name: "space is rejected", value: "project one", wantErr: core.ErrSecretStoreContract},
		{name: "newline is rejected", value: "project\none", wantErr: core.ErrSecretStoreContract},
		{name: "non-ascii letter is rejected", value: "prøject1", wantErr: core.ErrSecretStoreContract},
		{name: "leading punctuation is rejected", value: "-project", wantErr: core.ErrSecretStoreContract},
		{name: "trailing punctuation is rejected", value: "project-", wantErr: core.ErrSecretStoreContract},
		// Twenty hostile boundary cases.
		{name: "zero-byte project is rejected", wantErr: core.ErrSecretStoreContract},
		{name: "one-byte project is rejected", value: "a", wantErr: core.ErrSecretStoreContract},
		{name: "four-byte project is rejected", value: "a123", wantErr: core.ErrSecretStoreContract},
		{name: "one below project minimum is rejected", value: "a1234", wantErr: core.ErrSecretStoreContract},
		{name: "exact project minimum is admitted", value: "a12345"},
		{name: "one above project minimum is admitted", value: "a123456"},
		{name: "two below project maximum is admitted", value: "a" + strings.Repeat("1", GoogleProjectIDMaximumBytes-3) + "z"},
		{name: "one below project maximum is admitted", value: "a" + strings.Repeat("1", GoogleProjectIDMaximumBytes-2)},
		{name: "exact project maximum is admitted", value: "a" + strings.Repeat("1", GoogleProjectIDMaximumBytes-2) + "z"},
		{name: "one above project maximum is rejected", value: "a" + strings.Repeat("1", GoogleProjectIDMaximumBytes-1) + "z", wantErr: core.ErrSecretStoreContract},
		{name: "two above project maximum is rejected", value: "a" + strings.Repeat("1", GoogleProjectIDMaximumBytes) + "z", wantErr: core.ErrSecretStoreContract},
		{name: "maximum ending hyphen is rejected", value: "a" + strings.Repeat("1", GoogleProjectIDMaximumBytes-2) + "-", wantErr: core.ErrSecretStoreContract},
		{name: "minimum ending hyphen is rejected", value: "a1234-", wantErr: core.ErrSecretStoreContract},
		{name: "minimum beginning digit is rejected", value: "112345", wantErr: core.ErrSecretStoreContract},
		{name: "minimum beginning hyphen is rejected", value: "-12345", wantErr: core.ErrSecretStoreContract},
		{name: "embedded nul is rejected", value: "pro\x00ject", wantErr: core.ErrSecretStoreContract},
		{name: "carriage return is rejected", value: "project\rone", wantErr: core.ErrSecretStoreContract},
		{name: "tab is rejected", value: "project\tone", wantErr: core.ErrSecretStoreContract},
		{name: "utf8 expansion cannot evade byte maximum", value: "a" + strings.Repeat("é", GoogleProjectIDMaximumBytes), wantErr: core.ErrSecretStoreContract},
		{name: "invalid utf8 string is rejected", value: string([]byte{'a', '1', '2', '3', '4', 0xff}), wantErr: core.ErrSecretStoreContract},
	}
}

func TestParseGoogleSecretIDHostileTable(t *testing.T) {
	t.Parallel()

	for _, testCase := range googleSecretIDCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseGoogleSecretID(testCase.value)
			if testCase.wantErr != nil {
				if got != (GoogleSecretID{}) || !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("ParseGoogleSecretID() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, testCase.wantErr)
				}
				return
			}
			if gotErr != nil || got.String() != testCase.value || got.Validate() != nil {
				t.Fatalf("ParseGoogleSecretID() = (%q, %v), want exact validated %q", got.String(), gotErr, testCase.value)
			}
		})
	}
}

func googleSecretIDCases() []textBoundaryCase {
	return []textBoundaryCase{
		// Ten expected-valid cases.
		{name: "ordinary lowercase secret is admitted", value: "runtime-secret"},
		{name: "ordinary uppercase secret is admitted", value: "RuntimeSecret"},
		{name: "ordinary underscore secret is admitted", value: "runtime_secret"},
		{name: "ordinary numeric suffix is admitted", value: "runtime-secret-1"},
		{name: "ordinary leading digit is admitted", value: "1-runtime-secret"},
		{name: "ordinary leading hyphen is admitted", value: "-runtime-secret"},
		{name: "ordinary trailing hyphen is admitted", value: "runtime-secret-"},
		{name: "ordinary leading underscore is admitted", value: "_runtime_secret"},
		{name: "ordinary trailing underscore is admitted", value: "runtime_secret_"},
		{name: "ordinary mixed provider alphabet is admitted", value: "A1_b-2_C3"},
		// Ten expected-rejection cases.
		{name: "space is rejected", value: "runtime secret", wantErr: core.ErrSecretStoreContract},
		{name: "period is rejected", value: "runtime.secret", wantErr: core.ErrSecretStoreContract},
		{name: "slash is rejected", value: "runtime/secret", wantErr: core.ErrSecretStoreContract},
		{name: "backslash is rejected", value: `runtime\secret`, wantErr: core.ErrSecretStoreContract},
		{name: "colon is rejected", value: "runtime:secret", wantErr: core.ErrSecretStoreContract},
		{name: "question mark is rejected", value: "runtime?secret", wantErr: core.ErrSecretStoreContract},
		{name: "hash is rejected", value: "runtime#secret", wantErr: core.ErrSecretStoreContract},
		{name: "non-ascii letter is rejected", value: "sëcret", wantErr: core.ErrSecretStoreContract},
		{name: "newline is rejected", value: "runtime\nsecret", wantErr: core.ErrSecretStoreContract},
		{name: "nul is rejected", value: "runtime\x00secret", wantErr: core.ErrSecretStoreContract},
		// Twenty hostile boundary cases.
		{name: "zero-byte secret is rejected", wantErr: core.ErrSecretStoreContract},
		{name: "exact one-byte minimum letter is admitted", value: "a"},
		{name: "exact one-byte minimum digit is admitted", value: "1"},
		{name: "exact one-byte minimum hyphen is admitted", value: "-"},
		{name: "exact one-byte minimum underscore is admitted", value: "_"},
		{name: "two-byte secret is admitted", value: "a1"},
		{name: "one below secret maximum is admitted", value: strings.Repeat("a", GoogleSecretIDMaximumBytes-1)},
		{name: "exact secret maximum is admitted", value: strings.Repeat("a", GoogleSecretIDMaximumBytes)},
		{name: "one above secret maximum is rejected", value: strings.Repeat("a", GoogleSecretIDMaximumBytes+1), wantErr: core.ErrSecretStoreContract},
		{name: "two above secret maximum is rejected", value: strings.Repeat("a", GoogleSecretIDMaximumBytes+2), wantErr: core.ErrSecretStoreContract},
		{name: "maximum mixed bytes are admitted", value: strings.Repeat("A_", (GoogleSecretIDMaximumBytes-1)/2) + "1"},
		{name: "maximum ending slash is rejected", value: strings.Repeat("a", GoogleSecretIDMaximumBytes-1) + "/", wantErr: core.ErrSecretStoreContract},
		{name: "tab is rejected", value: "runtime\tsecret", wantErr: core.ErrSecretStoreContract},
		{name: "carriage return is rejected", value: "runtime\rsecret", wantErr: core.ErrSecretStoreContract},
		{name: "delete control is rejected", value: "runtime" + string(rune(0x7f)), wantErr: core.ErrSecretStoreContract},
		{name: "invalid utf8 is rejected", value: string([]byte{0xff}), wantErr: core.ErrSecretStoreContract},
		{name: "utf8 expansion cannot evade maximum", value: strings.Repeat("é", GoogleSecretIDMaximumBytes), wantErr: core.ErrSecretStoreContract},
		{name: "equals is rejected", value: "runtime=secret", wantErr: core.ErrSecretStoreContract},
		{name: "percent is rejected", value: "runtime%secret", wantErr: core.ErrSecretStoreContract},
		{name: "path traversal pair is rejected", value: "..", wantErr: core.ErrSecretStoreContract},
	}
}

func TestAccessRequestValidationExhaustsFieldAndEnumStates(t *testing.T) {
	t.Parallel()

	project, err := ParseGoogleProjectID("project1")
	if err != nil {
		t.Fatalf("ParseGoogleProjectID() error = %v, want nil", err)
	}
	secret, err := ParseGoogleSecretID("runtime_secret-1")
	if err != nil {
		t.Fatalf("ParseGoogleSecretID() error = %v, want nil", err)
	}
	tests := []struct {
		wantErr  error
		name     string
		wantName string
		request  AccessRequest
	}{
		{name: "all explicit fields are admitted", request: AccessRequest{Project: project, Secret: secret, Version: GoogleVersionSelectorLatest}, wantName: "projects/project1/secrets/runtime_secret-1/versions/latest"},
		{name: "zero request is rejected", request: AccessRequest{}, wantErr: core.ErrSecretStoreContract},
		{name: "project alone is rejected", request: AccessRequest{Project: project}, wantErr: core.ErrSecretStoreContract},
		{name: "secret alone is rejected", request: AccessRequest{Secret: secret}, wantErr: core.ErrSecretStoreContract},
		{name: "version alone is rejected", request: AccessRequest{Version: GoogleVersionSelectorLatest}, wantErr: core.ErrSecretStoreContract},
		{name: "project and secret without version are rejected", request: AccessRequest{Project: project, Secret: secret}, wantErr: core.ErrSecretStoreContract},
		{name: "project and version without secret are rejected", request: AccessRequest{Project: project, Version: GoogleVersionSelectorLatest}, wantErr: core.ErrSecretStoreContract},
		{name: "secret and version without project are rejected", request: AccessRequest{Secret: secret, Version: GoogleVersionSelectorLatest}, wantErr: core.ErrSecretStoreContract},
		{name: "one above latest version is rejected", request: AccessRequest{Project: project, Secret: secret, Version: GoogleVersionSelectorLatest + 1}, wantErr: core.ErrSecretStoreContract},
		{name: "midrange future version is rejected", request: AccessRequest{Project: project, Secret: secret, Version: GoogleVersionSelector(127)}, wantErr: core.ErrSecretStoreContract},
		{name: "maximum future version is rejected", request: AccessRequest{Project: project, Secret: secret, Version: ^GoogleVersionSelector(0)}, wantErr: core.ErrSecretStoreContract},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotErr := testCase.request.Validate()
			if testCase.wantErr != nil {
				if !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("AccessRequest.Validate() error = %v, want errors.Is(..., %v)", gotErr, testCase.wantErr)
				}
				gotName, gotNameErr := testCase.request.resourceName()
				if gotName != "" || !errors.Is(gotNameErr, testCase.wantErr) {
					t.Fatalf("AccessRequest.resourceName(rejected) = (%q, %v), want zero and errors.Is(..., %v)", gotName, gotNameErr, testCase.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("AccessRequest.Validate() error = %v, want nil", gotErr)
			}
			gotName, gotNameErr := testCase.request.resourceName()
			if gotNameErr != nil || gotName != testCase.wantName {
				t.Fatalf("AccessRequest.resourceName() = (%q, %v), want (%q, nil)", gotName, gotNameErr, testCase.wantName)
			}
		})
	}
}

func TestNewGoogleVersionNumberExhaustsBoundaryClasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   uint64
	}{
		{name: "zero version is rejected", wantErr: core.ErrSecretStoreContract},
		{name: "exact positive floor is admitted", value: 1},
		{name: "one above positive floor is admitted", value: 2},
		{name: "one below uint64 maximum is admitted", value: ^uint64(0) - 1},
		{name: "exact uint64 maximum is admitted", value: ^uint64(0)},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewGoogleVersionNumber(testCase.value)
			if testCase.wantErr != nil {
				if got != 0 || !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("NewGoogleVersionNumber() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, testCase.wantErr)
				}
				return
			}
			if gotErr != nil || got.Validate() != nil || got.Uint64() != testCase.value {
				t.Fatalf("NewGoogleVersionNumber() = (%v, %v), want validated %d", got, gotErr, testCase.value)
			}
		})
	}
}

func TestNewGoogleProjectNumberExhaustsBoundaryClasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   uint64
	}{
		{name: "zero project number is rejected", wantErr: core.ErrSecretStoreContract},
		{name: "exact positive project floor is admitted", value: 1},
		{name: "one above positive project floor is admitted", value: 2},
		{name: "one below uint64 project maximum is admitted", value: ^uint64(0) - 1},
		{name: "exact uint64 project maximum is admitted", value: ^uint64(0)},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewGoogleProjectNumber(testCase.value)
			if testCase.wantErr != nil {
				if got != 0 || !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("NewGoogleProjectNumber() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, testCase.wantErr)
				}
				return
			}
			if gotErr != nil || got.Validate() != nil || got.Uint64() != testCase.value {
				t.Fatalf("NewGoogleProjectNumber() = (%v, %v), want validated %d", got, gotErr, testCase.value)
			}
		})
	}
}

type payloadBoundaryCase struct {
	wantErr     error
	wantTextErr error
	name        string
	payload     []byte
}

func TestNewValueHostileTable(t *testing.T) {
	t.Parallel()

	for _, testCase := range secretPayloadCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewValue(testCase.payload)
			if testCase.wantErr != nil {
				if got != (Value{}) || !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("NewValue() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, testCase.wantErr)
				}
				return
			}
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf("NewValue() = (%v, %v), want validated value", got, gotErr)
			}
			gotBytes, gotBytesErr := got.CopyBytes()
			if gotBytesErr != nil || !bytes.Equal(gotBytes, testCase.payload) {
				t.Fatalf("Value.CopyBytes() = (%x, %v), want exact payload %x", gotBytes, gotBytesErr, testCase.payload)
			}
			gotText, gotTextErr := got.Text()
			if testCase.wantTextErr != nil {
				if gotText != "" || !errors.Is(gotTextErr, testCase.wantTextErr) {
					t.Fatalf("Value.Text() = (%q, %v), want zero and errors.Is(..., %v)", gotText, gotTextErr, testCase.wantTextErr)
				}
			} else if gotTextErr != nil || gotText != string(testCase.payload) {
				t.Fatalf("Value.Text() = (%q, %v), want exact UTF-8 payload", gotText, gotTextErr)
			}
			if gotFormatted := fmt.Sprintf("%v", got); gotFormatted != core.RedactedValueText {
				t.Fatalf("formatted Value = %q, want %q", gotFormatted, core.RedactedValueText)
			}
			if err := got.Destroy(); err != nil {
				t.Fatalf("Value.Destroy() error = %v, want nil", err)
			}
		})
	}
}

func secretPayloadCases() []payloadBoundaryCase {
	return []payloadBoundaryCase{
		// Ten expected-valid cases.
		{name: "provider minimum nil payload is admitted as empty bytes"},
		{name: "provider minimum zero-length payload is admitted", payload: []byte{}},
		{name: "ordinary password is admitted", payload: []byte("ordinary-password!")},
		{name: "ordinary opaque binary is admitted", payload: []byte{0, 1, 2, 3}},
		{name: "ordinary whitespace is admitted", payload: []byte(" secret value ")},
		{name: "ordinary line ending is admitted", payload: []byte("secret\r\n")},
		{name: "ordinary tab is admitted", payload: []byte("secret\tvalue")},
		{name: "ordinary unicode text is admitted", payload: []byte("sëcret")},
		{name: "ordinary invalid utf8 remains opaque", payload: []byte{0xff}, wantTextErr: core.ErrSecretStorePayload},
		{name: "ordinary truncated utf8 remains opaque", payload: []byte{0xe2, 0x82}, wantTextErr: core.ErrSecretStorePayload},
		{name: "ordinary all-zero bytes are admitted", payload: []byte{0, 0, 0}},
		// Ten expected-rejection cases.
		{name: "one above provider maximum is rejected", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes+1), wantErr: core.ErrSecretStorePayload},
		{name: "two above provider maximum are rejected", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes+2), wantErr: core.ErrSecretStorePayload},
		{name: "three above provider maximum are rejected", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes+3), wantErr: core.ErrSecretStorePayload},
		{name: "four above provider maximum are rejected", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes+4), wantErr: core.ErrSecretStorePayload},
		{name: "eight above provider maximum are rejected", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes+8), wantErr: core.ErrSecretStorePayload},
		{name: "sixteen above provider maximum are rejected", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes+16), wantErr: core.ErrSecretStorePayload},
		{name: "one kibibyte above provider maximum is rejected", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes+1024), wantErr: core.ErrSecretStorePayload},
		{name: "one provider maximum above ceiling is rejected", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes*2), wantErr: core.ErrSecretStorePayload},
		{name: "two provider maxima above ceiling are rejected", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes*3), wantErr: core.ErrSecretStorePayload},
		{name: "four provider maxima above ceiling are rejected", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes*5), wantErr: core.ErrSecretStorePayload},
		// Twenty hostile boundary cases.
		{name: "exact provider zero-byte floor is admitted", payload: []byte{}},
		{name: "exact one-byte minimum is admitted", payload: []byte("a")},
		{name: "exact one-byte opaque minimum is admitted", payload: []byte{0xff}, wantTextErr: core.ErrSecretStorePayload},
		{name: "two-byte payload is admitted", payload: []byte("ab")},
		{name: "three-byte payload is admitted", payload: []byte("abc")},
		{name: "minimum nul byte is admitted", payload: []byte{0}},
		{name: "minimum space byte is admitted", payload: []byte{' '}},
		{name: "minimum newline byte is admitted", payload: []byte{'\n'}},
		{name: "minimum tab byte is admitted", payload: []byte{'\t'}},
		{name: "minimum delete byte is admitted", payload: []byte{0x7f}},
		{name: "utf8 boundary two-byte rune is admitted", payload: []byte("é")},
		{name: "utf8 boundary three-byte rune is admitted", payload: []byte("€")},
		{name: "utf8 boundary four-byte rune is admitted", payload: []byte("😀")},
		{name: "invalid utf8 continuation is admitted as opaque", payload: []byte{0x80}, wantTextErr: core.ErrSecretStorePayload},
		{name: "invalid utf8 overlong form is admitted as opaque", payload: []byte{0xc0, 0x80}, wantTextErr: core.ErrSecretStorePayload},
		{name: "invalid utf8 surrogate is admitted as opaque", payload: []byte{0xed, 0xa0, 0x80}, wantTextErr: core.ErrSecretStorePayload},
		{name: "one below provider maximum is admitted", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes-1)},
		{name: "exact provider maximum is admitted", payload: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes)},
		{name: "maximum opaque bytes are admitted", payload: bytes.Repeat([]byte{0xff}, PayloadMaximumBytes), wantTextErr: core.ErrSecretStorePayload},
		{name: "maximum ending newline is admitted", payload: append(bytes.Repeat([]byte{'a'}, PayloadMaximumBytes-1), '\n')},
		{name: "maximum ending nul is admitted", payload: append(bytes.Repeat([]byte{'a'}, PayloadMaximumBytes-1), 0)},
	}
}

func TestValueCopiesShareDestructionState(t *testing.T) {
	t.Parallel()

	value, err := NewValue([]byte("copy-sensitive-secret"))
	if err != nil {
		t.Fatalf("NewValue() error = %v, want nil", err)
	}
	copyOfValue := value
	if err := value.Destroy(); err != nil {
		t.Fatalf("Value.Destroy(first) error = %v, want nil", err)
	}
	if gotErr := copyOfValue.Validate(); !errors.Is(gotErr, core.ErrSecretStorePayload) {
		t.Fatalf("copied Value.Validate() error = %v, want errors.Is(..., %v)", gotErr, core.ErrSecretStorePayload)
	}
	if got, gotErr := copyOfValue.CopyBytes(); got != nil || !errors.Is(gotErr, core.ErrSecretStorePayload) {
		t.Fatalf("copied Value.CopyBytes(after destroy) = (%x, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrSecretStorePayload)
	}
	if got, gotErr := copyOfValue.Text(); got != "" || !errors.Is(gotErr, core.ErrSecretStorePayload) {
		t.Fatalf("copied Value.Text(after destroy) = (%q, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrSecretStorePayload)
	}
	if err := copyOfValue.Destroy(); err != nil {
		t.Fatalf("Value.Destroy(repeated) error = %v, want nil", err)
	}
	if got := fmt.Sprintf("%#v", copyOfValue); got != core.RedactedValueText {
		t.Fatalf("formatted destroyed Value = %q, want %q", got, core.RedactedValueText)
	}
}

func FuzzParseGoogleProjectIDSemanticClosure(f *testing.F) {
	f.Add("project1")
	f.Add("a12345")
	f.Add("")
	f.Add("Project1")
	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := ParseGoogleProjectID(value)
		if gotErr != nil {
			if got != (GoogleProjectID{}) || !errors.Is(gotErr, core.ErrSecretStoreContract) {
				t.Fatalf("ParseGoogleProjectID(rejected) = (%v, %v), want zero and typed contract error", got, gotErr)
			}
			return
		}
		if got.String() != value || got.Validate() != nil {
			t.Fatalf("ParseGoogleProjectID(accepted) = %q, want exact validated %q", got.String(), value)
		}
		roundTrip, roundTripErr := ParseGoogleProjectID(got.String())
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("ParseGoogleProjectID(round trip) = (%v, %v), want (%v, nil)", roundTrip, roundTripErr, got)
		}
	})
}

func FuzzParseGoogleSecretIDSemanticClosure(f *testing.F) {
	f.Add("runtime_secret-1")
	f.Add("a")
	f.Add("")
	f.Add("runtime/secret")
	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := ParseGoogleSecretID(value)
		if gotErr != nil {
			if got != (GoogleSecretID{}) || !errors.Is(gotErr, core.ErrSecretStoreContract) {
				t.Fatalf("ParseGoogleSecretID(rejected) = (%v, %v), want zero and typed contract error", got, gotErr)
			}
			return
		}
		if got.String() != value || got.Validate() != nil {
			t.Fatalf("ParseGoogleSecretID(accepted) = %q, want exact validated %q", got.String(), value)
		}
		roundTrip, roundTripErr := ParseGoogleSecretID(got.String())
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("ParseGoogleSecretID(round trip) = (%v, %v), want (%v, nil)", roundTrip, roundTripErr, got)
		}
	})
}

func FuzzNewValueSemanticClosure(f *testing.F) {
	f.Add([]byte("0123456789abcdef"))
	f.Add([]byte("a"))
	f.Add([]byte{})
	f.Add([]byte{0xff})
	f.Fuzz(func(t *testing.T, payload []byte) {
		got, gotErr := NewValue(payload)
		wantAccepted := len(payload) <= PayloadMaximumBytes
		if !wantAccepted {
			if got != (Value{}) || !errors.Is(gotErr, core.ErrSecretStorePayload) {
				t.Fatalf("NewValue(rejected) = (%v, %v), want zero and typed payload error", got, gotErr)
			}
			return
		}
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf("NewValue(accepted) = (%v, %v), want validated value", got, gotErr)
		}
		gotBytes, gotBytesErr := got.CopyBytes()
		if gotBytesErr != nil || !bytes.Equal(gotBytes, payload) {
			t.Fatalf("Value.CopyBytes() = (%x, %v), want exact accepted payload %x", gotBytes, gotBytesErr, payload)
		}
		gotText, gotTextErr := got.Text()
		if utf8.Valid(payload) {
			if gotTextErr != nil || gotText != string(payload) {
				t.Fatalf("Value.Text(valid UTF-8) = (%q, %v), want exact payload", gotText, gotTextErr)
			}
		} else if gotText != "" || !errors.Is(gotTextErr, core.ErrSecretStorePayload) {
			t.Fatalf("Value.Text(opaque bytes) = (%q, %v), want zero and typed payload error", gotText, gotTextErr)
		}
		if err := got.Destroy(); err != nil {
			t.Fatalf("Value.Destroy() error = %v, want nil", err)
		}
		if gotErr := got.Validate(); !errors.Is(gotErr, core.ErrSecretStorePayload) {
			t.Fatalf("Value.Validate(after destroy) error = %v, want %v", gotErr, core.ErrSecretStorePayload)
		}
	})
}
