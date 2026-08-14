package gcsobjects_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gcsobjects"
)

func TestNewGCSGenerationExhaustsProviderIntegerBoundaryClasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   int64
	}{
		{name: "minimum signed generation is rejected", value: math.MinInt64, wantErr: core.ErrObjectStoreContract},
		{name: "minimum plus one generation is rejected", value: math.MinInt64 + 1, wantErr: core.ErrObjectStoreContract},
		{name: "ordinary negative generation is rejected", value: -2, wantErr: core.ErrObjectStoreContract},
		{name: "one below provider floor is rejected", value: -1, wantErr: core.ErrObjectStoreContract},
		{name: "unset provider generation is rejected", wantErr: core.ErrObjectStoreContract},
		{name: "provider floor is admitted", value: 1},
		{name: "one above provider floor is admitted", value: 2},
		{name: "maximum minus one provider generation is admitted", value: math.MaxInt64 - 1},
		{name: "maximum provider generation is admitted", value: math.MaxInt64},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := gcsobjects.NewGCSGeneration(test.value)
			if !errors.Is(gotErr, test.wantErr) {
				t.Fatalf("NewGCSGeneration(%d) error = %v, want errors.Is(..., %v)", test.value, gotErr, test.wantErr)
			}
			if test.wantErr != nil {
				if got != (gcsobjects.GCSGeneration{}) {
					t.Fatalf("NewGCSGeneration(%d) generation = %v, want zero after rejection", test.value, got)
				}
				return
			}
			gotValue, gotValueErr := got.Int64()
			if gotValueErr != nil || gotValue != test.value || got.Validate() != nil {
				t.Fatalf("NewGCSGeneration(%d) projection = (%d, %v), Validate() = %v, want (%d, nil, nil)", test.value, gotValue, gotValueErr, got.Validate(), test.value)
			}
		})
	}
}

func FuzzNewGCSGenerationProviderIngressSemanticClosure(f *testing.F) {
	for _, value := range []int64{math.MinInt64, -1, 0, 1, 2, math.MaxInt64} {
		f.Add(value)
	}

	f.Fuzz(func(t *testing.T, value int64) {
		got, gotErr := gcsobjects.NewGCSGeneration(value)
		if value <= 0 {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (gcsobjects.GCSGeneration{}) {
				t.Fatalf("NewGCSGeneration(%d) = (%v, %v), want zero and errors.Is(..., %v)", value, got, gotErr, core.ErrObjectStoreContract)
			}
			return
		}
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf("NewGCSGeneration(%d) = (%v, %v), Validate() = %v, want valid and nil", value, got, gotErr, got.Validate())
		}
		projected, projectedErr := got.Int64()
		if projectedErr != nil || projected != value {
			t.Fatalf("NewGCSGeneration(%d).Int64() = (%d, %v), want (%d, nil)", value, projected, projectedErr, value)
		}
		roundTrip, roundTripErr := gcsobjects.NewGCSGeneration(projected)
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("NewGCSGeneration(projected %d) = (%v, %v), want (%v, nil)", projected, roundTrip, roundTripErr, got)
		}
	})
}

func TestParseGCSBucketPressuresEveryProviderNamingBoundary(t *testing.T) {
	t.Parallel()

	component63 := strings.Repeat("a", gcsobjects.GCSBucketComponentMaximumBytes)
	dotted222 := component63 + "." + component63 + "." + component63 + "." + strings.Repeat("a", 30)
	tests := []struct {
		wantErr error
		name    string
		input   string
		want    string
	}{
		{name: "three byte letter minimum is admitted", input: "abc", want: "abc"},
		{name: "three byte digit minimum is admitted", input: "123", want: "123"},
		{name: "interior hyphen is admitted", input: "a-b", want: "a-b"},
		{name: "interior underscore is admitted", input: "a_b", want: "a_b"},
		{name: "one dotted separator is admitted", input: "photos.example", want: "photos.example"},
		{name: "lowercase mixed provider alphabet is admitted", input: "clean-lift_2026.photos", want: "clean-lift_2026.photos"},
		{name: "one component at sixty three bytes is admitted", input: component63, want: component63},
		{name: "dotted name at two hundred twenty two bytes is admitted", input: dotted222, want: dotted222},
		{name: "digits at both outer edges are admitted", input: "1bucket9", want: "1bucket9"},
		{name: "four nonempty dotted components are admitted", input: "a1.b2.c3.d4", want: "a1.b2.c3.d4"},
		{name: "unset name is rejected", input: "", wantErr: core.ErrObjectStoreContract},
		{name: "one byte name is below minimum", input: "a", wantErr: core.ErrObjectStoreContract},
		{name: "two byte name is below minimum", input: "ab", wantErr: core.ErrObjectStoreContract},
		{name: "uppercase byte is outside provider alphabet", input: "Abc", wantErr: core.ErrObjectStoreContract},
		{name: "hyphen at the first byte is rejected", input: "-ab", wantErr: core.ErrObjectStoreContract},
		{name: "underscore at the final byte is rejected", input: "ab_", wantErr: core.ErrObjectStoreContract},
		{name: "ipv4 shaped name is reserved", input: "192.168.5.4", wantErr: core.ErrObjectStoreContract},
		{name: "google substring is reserved", input: "my-google-bucket", wantErr: core.ErrObjectStoreContract},
		{name: "non ascii byte is outside provider alphabet", input: "café", wantErr: core.ErrObjectStoreContract},
		{name: "slash is outside provider alphabet", input: "abc/def", wantErr: core.ErrObjectStoreContract},
		{name: "minimum plus one remains admitted", input: "a000", want: "a000"},
		{name: "one below component ceiling remains admitted", input: strings.Repeat("a", 62), want: strings.Repeat("a", 62)},
		{name: "component ceiling remains admitted", input: component63, want: component63},
		{name: "one above undotted component ceiling is rejected", input: strings.Repeat("a", 64), wantErr: core.ErrObjectStoreContract},
		{name: "one below dotted component ceiling remains admitted", input: strings.Repeat("a", 62) + ".b", want: strings.Repeat("a", 62) + ".b"},
		{name: "dotted component ceiling remains admitted", input: component63 + ".b", want: component63 + ".b"},
		{name: "one above dotted component ceiling is rejected", input: strings.Repeat("a", 64) + ".b", wantErr: core.ErrObjectStoreContract},
		{name: "one below dotted total ceiling remains admitted", input: dotted222[:221], want: dotted222[:221]},
		{name: "dotted total ceiling remains admitted", input: dotted222, want: dotted222},
		{name: "one above dotted total ceiling is rejected", input: dotted222 + "a", wantErr: core.ErrObjectStoreContract},
		{name: "digit at the first byte remains admitted", input: "0ab", want: "0ab"},
		{name: "digit at the final byte remains admitted", input: "ab0", want: "ab0"},
		{name: "dot at the first byte is rejected", input: ".ab", wantErr: core.ErrObjectStoreContract},
		{name: "dot at the final byte is rejected", input: "ab.", wantErr: core.ErrObjectStoreContract},
		{name: "empty dotted component is rejected", input: "a..b", wantErr: core.ErrObjectStoreContract},
		{name: "hyphen immediately inside the outer bytes is admitted", input: "a-b", want: "a-b"},
		{name: "hyphen at the outer byte is rejected", input: "a-b-", wantErr: core.ErrObjectStoreContract},
		{name: "underscore immediately inside the outer bytes is admitted", input: "a_b", want: "a_b"},
		{name: "underscore at the outer byte is rejected", input: "_ab", wantErr: core.ErrObjectStoreContract},
		{name: "maximum uint text is admitted as an ordinary name", input: "18446744073709551615", want: "18446744073709551615"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := gcsobjects.ParseGCSBucket(test.input)
			if !errors.Is(gotErr, test.wantErr) {
				t.Fatalf("ParseGCSBucket(%q) error = %v, want errors.Is(..., %v)", test.input, gotErr, test.wantErr)
			}
			if got.String() != test.want {
				t.Fatalf("ParseGCSBucket(%q).String() = %q, want %q", test.input, got.String(), test.want)
			}
			if test.wantErr == nil && got.Validate() != nil {
				t.Fatalf("ParseGCSBucket(%q).Validate() error = %v, want nil", test.input, got.Validate())
			}
		})
	}
}

func TestParseGCSObjectNamePressuresExtentEncodingAndReservedNames(t *testing.T) {
	t.Parallel()

	maximum := strings.Repeat("x", gcsobjects.GCSObjectNameMaximumBytes)
	tests := []struct {
		wantErr error
		name    string
		input   string
		want    string
	}{
		{name: "one ascii byte is admitted", input: "a", want: "a"},
		{name: "slash separated product key is admitted", input: "users/01/profile/file.webp", want: "users/01/profile/file.webp"},
		{name: "embedded space is admitted", input: "photos/my image.png", want: "photos/my image.png"},
		{name: "valid utf8 is admitted", input: "photos/体育.webp", want: "photos/体育.webp"},
		{name: "leading slash is an exact provider name", input: "/object", want: "/object"},
		{name: "trailing slash is an exact provider name", input: "directory/", want: "directory/"},
		{name: "repeated slash is an exact provider name", input: "a//b", want: "a//b"},
		{name: "embedded current segment is ordinary object text", input: "a/./b", want: "a/./b"},
		{name: "horizontal tab is admitted by provider name rules", input: "a\tb", want: "a\tb"},
		{name: "exact byte ceiling is admitted", input: maximum, want: maximum},
		{name: "unset object name is rejected", input: "", wantErr: core.ErrObjectStoreContract},
		{name: "single current directory name is reserved", input: ".", wantErr: core.ErrObjectStoreContract},
		{name: "single parent directory name is reserved", input: "..", wantErr: core.ErrObjectStoreContract},
		{name: "carriage return is rejected anywhere", input: "a\rb", wantErr: core.ErrObjectStoreContract},
		{name: "line feed is rejected anywhere", input: "a\nb", wantErr: core.ErrObjectStoreContract},
		{name: "acme challenge namespace is reserved", input: ".well-known/acme-challenge/token", wantErr: core.ErrObjectStoreContract},
		{name: "one byte above extent ceiling is rejected", input: maximum + "x", wantErr: core.ErrObjectStoreContract},
		{name: "invalid leading utf8 byte is rejected", input: string([]byte{0xff}), wantErr: core.ErrObjectStoreContract},
		{name: "orphan utf8 continuation byte is rejected", input: string([]byte{'a', 0x80}), wantErr: core.ErrObjectStoreContract},
		{name: "truncated multibyte utf8 is rejected", input: string([]byte{0xe2, 0x82}), wantErr: core.ErrObjectStoreContract},
		{name: "two ascii bytes remain admitted", input: "ab", want: "ab"},
		{name: "one slash byte is admitted as an object", input: "/", want: "/"},
		{name: "current segment with slash is not the reserved singleton", input: "./", want: "./"},
		{name: "parent segment with slash is not the reserved singleton", input: "../", want: "../"},
		{name: "acme namespace without trailing slash is not reserved", input: ".well-known/acme-challenge", want: ".well-known/acme-challenge"},
		{name: "acme namespace exact trailing slash is reserved", input: ".well-known/acme-challenge/", wantErr: core.ErrObjectStoreContract},
		{name: "acme namespace after a leading byte is not reserved", input: "x.well-known/acme-challenge/token", want: "x.well-known/acme-challenge/token"},
		{name: "one byte below extent ceiling remains admitted", input: maximum[:gcsobjects.GCSObjectNameMaximumBytes-1], want: maximum[:gcsobjects.GCSObjectNameMaximumBytes-1]},
		{name: "extent ceiling remains admitted", input: maximum, want: maximum},
		{name: "extent ceiling plus one remains rejected", input: maximum + "y", wantErr: core.ErrObjectStoreContract},
		{name: "carriage return at first byte is rejected", input: "\ra", wantErr: core.ErrObjectStoreContract},
		{name: "carriage return at final byte is rejected", input: "a\r", wantErr: core.ErrObjectStoreContract},
		{name: "line feed at first byte is rejected", input: "\na", wantErr: core.ErrObjectStoreContract},
		{name: "line feed at final byte is rejected", input: "a\n", wantErr: core.ErrObjectStoreContract},
		{name: "nul byte remains provider-valid object text", input: "a\x00b", want: "a\x00b"},
		{name: "delete control byte remains provider-valid object text", input: "a\x7fb", want: "a\x7fb"},
		{name: "two byte utf8 scalar is admitted", input: "¢", want: "¢"},
		{name: "three byte utf8 scalar is admitted", input: "€", want: "€"},
		{name: "four byte utf8 scalar is admitted", input: "U0001f3cb", want: "U0001f3cb"},
		{name: "multibyte text at exact byte ceiling is admitted", input: strings.Repeat("¢", gcsobjects.GCSObjectNameMaximumBytes/2), want: strings.Repeat("¢", gcsobjects.GCSObjectNameMaximumBytes/2)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := gcsobjects.ParseGCSObjectName(test.input)
			if !errors.Is(gotErr, test.wantErr) {
				t.Fatalf("ParseGCSObjectName(%q) error = %v, want errors.Is(..., %v)", test.input, gotErr, test.wantErr)
			}
			if got.String() != test.want {
				t.Fatalf("ParseGCSObjectName(%q).String() = %q, want %q", test.input, got.String(), test.want)
			}
		})
	}
}

func TestParseGCSObjectPrefixRefusesAmbiguousDestructiveScopes(t *testing.T) {
	t.Parallel()

	maximum := strings.Repeat("a", gcsobjects.GCSObjectNameMaximumBytes-1) + "/"
	tests := []struct {
		wantErr error
		name    string
		input   string
		want    string
	}{
		{name: "one segment subtree is admitted", input: "users/", want: "users/"},
		{name: "two segment subtree is admitted", input: "users/01/", want: "users/01/"},
		{name: "three segment subtree is admitted", input: "users/01/profile/", want: "users/01/profile/"},
		{name: "hyphenated segments are admitted", input: "user-images/a-b/", want: "user-images/a-b/"},
		{name: "underscored segments are admitted", input: "user_images/a_b/", want: "user_images/a_b/"},
		{name: "spaces remain exact within segments", input: "user images/a b/", want: "user images/a b/"},
		{name: "utf8 segments remain exact", input: "users/体育/", want: "users/体育/"},
		{name: "dot within a non-dot segment is admitted", input: "users/example.com/", want: "users/example.com/"},
		{name: "one character subtree is admitted", input: "a/", want: "a/"},
		{name: "exact object-name byte ceiling is admitted", input: maximum, want: maximum},
		{name: "unset prefix cannot select a whole bucket", input: "", wantErr: core.ErrObjectStoreContract},
		{name: "root slash cannot select a whole bucket", input: "/", wantErr: core.ErrObjectStoreContract},
		{name: "leaf-shaped text is not a destructive subtree", input: "users", wantErr: core.ErrObjectStoreContract},
		{name: "leading slash creates an ambiguous root", input: "/users/", wantErr: core.ErrObjectStoreContract},
		{name: "empty interior segment is rejected", input: "users//", wantErr: core.ErrObjectStoreContract},
		{name: "current directory segment is rejected", input: "users/./", wantErr: core.ErrObjectStoreContract},
		{name: "parent directory segment is rejected", input: "users/../", wantErr: core.ErrObjectStoreContract},
		{name: "line feed is rejected before scope construction", input: "users\n/", wantErr: core.ErrObjectStoreContract},
		{name: "reserved acme namespace is rejected", input: ".well-known/acme-challenge/", wantErr: core.ErrObjectStoreContract},
		{name: "one byte above object-name ceiling is rejected", input: maximum + "a", wantErr: core.ErrObjectStoreContract},
		{name: "minimum subtree a slash is admitted", input: "a/", want: "a/"},
		{name: "two single-byte segments are admitted", input: "a/b/", want: "a/b/"},
		{name: "empty middle segment is rejected", input: "a//b/", wantErr: core.ErrObjectStoreContract},
		{name: "current directory at first segment is rejected", input: "./a/", wantErr: core.ErrObjectStoreContract},
		{name: "parent directory at first segment is rejected", input: "../a/", wantErr: core.ErrObjectStoreContract},
		{name: "current directory at middle segment is rejected", input: "a/./b/", wantErr: core.ErrObjectStoreContract},
		{name: "parent directory at middle segment is rejected", input: "a/../b/", wantErr: core.ErrObjectStoreContract},
		{name: "three dots are ordinary segment text", input: "a/.../", want: "a/.../"},
		{name: "one byte below prefix ceiling is admitted", input: strings.Repeat("a", gcsobjects.GCSObjectNameMaximumBytes-2) + "/", want: strings.Repeat("a", gcsobjects.GCSObjectNameMaximumBytes-2) + "/"},
		{name: "prefix ceiling is admitted", input: maximum, want: maximum},
		{name: "prefix ceiling plus one is rejected", input: maximum + "a", wantErr: core.ErrObjectStoreContract},
		{name: "carriage return before trailing slash is rejected", input: "a\r/", wantErr: core.ErrObjectStoreContract},
		{name: "line feed before trailing slash is rejected", input: "a\n/", wantErr: core.ErrObjectStoreContract},
		{name: "nul remains exact segment text", input: "a\x00/", want: "a\x00/"},
		{name: "horizontal tab remains exact segment text", input: "a\t/", want: "a\t/"},
		{name: "two byte utf8 subtree is admitted", input: "¢/", want: "¢/"},
		{name: "three byte utf8 subtree is admitted", input: "€/", want: "€/"},
		{name: "four byte utf8 subtree is admitted", input: "U0001f3cb/", want: "U0001f3cb/"},
		{name: "two trailing slashes expose an empty segment", input: "a//", wantErr: core.ErrObjectStoreContract},
		{name: "nonterminal slash is required for destructive intent", input: "a/b", wantErr: core.ErrObjectStoreContract},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := gcsobjects.ParseGCSObjectPrefix(test.input)
			if !errors.Is(gotErr, test.wantErr) {
				t.Fatalf("ParseGCSObjectPrefix(%q) error = %v, want errors.Is(..., %v)", test.input, gotErr, test.wantErr)
			}
			if got.String() != test.want {
				t.Fatalf("ParseGCSObjectPrefix(%q).String() = %q, want %q", test.input, got.String(), test.want)
			}
		})
	}
}

func TestGCSAuthenticationExhaustsTheClosedCompilerDomain(t *testing.T) {
	t.Parallel()

	credentialFile, gotPathErr := core.ParseAbsolutePath("/credentials/service-account.json")
	if gotPathErr != nil {
		t.Fatalf("ParseAbsolutePath() error = %v, want nil", gotPathErr)
	}
	tests := []struct {
		wantErr error
		name    string
		config  gcsobjects.GCSClientConfig
	}{
		{name: "application default mode needs no path", config: gcsobjects.GCSClientConfig{Authentication: gcsobjects.GCSAuthenticationApplicationDefault}},
		{name: "service account mode requires one absolute path", config: gcsobjects.GCSClientConfig{Authentication: gcsobjects.GCSAuthenticationServiceAccountFile, CredentialFile: credentialFile}},
		{name: "zero authentication mode is rejected", config: gcsobjects.GCSClientConfig{}, wantErr: core.ErrObjectStoreContract},
		{name: "future authentication mode is rejected", config: gcsobjects.GCSClientConfig{Authentication: gcsobjects.GCSAuthentication(math.MaxUint8)}, wantErr: core.ErrObjectStoreContract},
		{name: "application default mode refuses a contradictory file", config: gcsobjects.GCSClientConfig{Authentication: gcsobjects.GCSAuthenticationApplicationDefault, CredentialFile: credentialFile}, wantErr: core.ErrObjectStoreContract},
		{name: "service account mode refuses an unset file", config: gcsobjects.GCSClientConfig{Authentication: gcsobjects.GCSAuthenticationServiceAccountFile}, wantErr: core.ErrObjectStoreContract},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			gotErr := test.config.Validate()
			if !errors.Is(gotErr, test.wantErr) {
				t.Fatalf("GCSClientConfig.Validate() error = %v, want errors.Is(..., %v)", gotErr, test.wantErr)
			}
		})
	}
}

// TestGCSAuthenticationOffWireEnumExhaustsEveryByte proves the closed
// credential-discovery domain across the whole uint8 range: exactly the two
// admitted modes validate and carry diagnostic text, every other byte is
// rejected with the stable contract identity, and the off-wire marker holds.
func TestGCSAuthenticationOffWireEnumExhaustsEveryByte(t *testing.T) {
	t.Parallel()

	admitted := map[gcsobjects.GCSAuthentication]string{
		gcsobjects.GCSAuthenticationApplicationDefault: "application_default",
		gcsobjects.GCSAuthenticationServiceAccountFile: "service_account_file",
	}
	var _ core.OffWireEnum = gcsobjects.GCSAuthenticationApplicationDefault
	for raw := range 256 {
		value := gcsobjects.GCSAuthentication(raw)
		wantText, wantValid := admitted[value]
		gotErr := value.Validate()
		if wantValid && gotErr != nil {
			t.Fatalf("GCSAuthentication(%d).Validate() error = %v, want nil", raw, gotErr)
		}
		if !wantValid && !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf("GCSAuthentication(%d).Validate() error = %v, want %v", raw, gotErr, core.ErrObjectStoreContract)
		}
		if gotValid := value.IsValid(); gotValid != wantValid {
			t.Fatalf("GCSAuthentication(%d).IsValid() = %t, want %t", raw, gotValid, wantValid)
		}
		if gotText := value.String(); gotText != wantText {
			t.Fatalf("GCSAuthentication(%d).String() = %q, want %q", raw, gotText, wantText)
		}
		value.OffWireEnum()
	}
}
