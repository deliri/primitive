package secretstore

import (
	"bytes"
	"errors"
	"hash/crc32"
	"math"
	"strconv"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const resolvedVersionForTest GoogleVersionNumber = 42
const resolvedProjectForTest GoogleProjectNumber = 123456789

func TestOfficialGoogleResponseHostileTable(t *testing.T) {
	t.Parallel()

	request := accessRequestForTest(t)
	for _, testCase := range officialGoogleResponseCases(t, request) {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := accessResultFromGoogleResponse(request, testCase.response)
			if testCase.wantErr != nil {
				if got != (AccessResult{}) || !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("accessResultFromGoogleResponse() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, testCase.wantErr)
				}
				requireProviderPayloadCleared(t, testCase.response)
				return
			}
			if gotErr != nil || got.Validate() != nil || got.Request != request || got.Reference != testCase.wantReference {
				t.Fatalf("accessResultFromGoogleResponse() = (%v, %v), want reference %v and validated result", got, gotErr, testCase.wantReference)
			}
			gotBytes, gotBytesErr := got.Value.CopyBytes()
			if gotBytesErr != nil || !bytes.Equal(gotBytes, testCase.wantBytes) {
				t.Fatalf("Value.CopyBytes() = (%x, %v), want (%x, nil)", gotBytes, gotBytesErr, testCase.wantBytes)
			}
			gotText, gotTextErr := got.Value.Text()
			if testCase.wantTextErr != nil {
				if gotText != "" || !errors.Is(gotTextErr, testCase.wantTextErr) {
					t.Fatalf("Value.Text() = (%q, %v), want zero and errors.Is(..., %v)", gotText, gotTextErr, testCase.wantTextErr)
				}
			} else if gotTextErr != nil || gotText != string(testCase.wantBytes) {
				t.Fatalf("Value.Text() = (%q, %v), want exact UTF-8 payload", gotText, gotTextErr)
			}
			requireProviderPayloadCleared(t, testCase.response)
			if err := got.Value.Destroy(); err != nil {
				t.Fatalf("Value.Destroy() error = %v, want nil", err)
			}
		})
	}
}

type officialGoogleResponseCase struct {
	name        string
	response    *secretmanagerpb.AccessSecretVersionResponse
	wantErr     error
	wantTextErr error
	wantBytes   []byte

	wantReference ResolvedReference
}

func officialGoogleResponseCases(t *testing.T, request AccessRequest) []officialGoogleResponseCase {
	t.Helper()

	zero := int64(0)
	negative := int64(-1)
	aboveUint32 := int64(math.MaxUint32) + 1
	maximumInt64 := int64(math.MaxInt64)
	wantReference := resolvedReferenceForTest(request, resolvedVersionForTest)
	foreignSecret := request
	secret, err := ParseGoogleSecretID("foreign-secret")
	if err != nil {
		t.Fatalf("ParseGoogleSecretID(foreign fixture) error = %v, want nil", err)
	}
	foreignSecret.Secret = secret
	return []officialGoogleResponseCase{
		// Ten expected-valid cases.
		{name: "ordinary provider payload is admitted", response: officialChecksummedResponse(request, resolvedVersionForTest, []byte("0123456789abcdef")), wantReference: wantReference, wantBytes: []byte("0123456789abcdef")},
		{name: "ordinary password punctuation is admitted", response: officialChecksummedResponse(request, resolvedVersionForTest, []byte("password-punctuation!")), wantReference: wantReference, wantBytes: []byte("password-punctuation!")},
		{name: "ordinary whitespace payload is admitted", response: officialChecksummedResponse(request, resolvedVersionForTest, []byte(" secret value ")), wantReference: wantReference, wantBytes: []byte(" secret value ")},
		{name: "ordinary line ending is admitted", response: officialChecksummedResponse(request, resolvedVersionForTest, []byte("secret\r\n")), wantReference: wantReference, wantBytes: []byte("secret\r\n")},
		{name: "ordinary unicode payload is admitted", response: officialChecksummedResponse(request, resolvedVersionForTest, []byte("sëcret")), wantReference: wantReference, wantBytes: []byte("sëcret")},
		{name: "ordinary binary payload is admitted", response: officialChecksummedResponse(request, resolvedVersionForTest, []byte{0, 1, 2, 3}), wantReference: wantReference, wantBytes: []byte{0, 1, 2, 3}},
		{name: "ordinary invalid utf8 remains opaque", response: officialChecksummedResponse(request, resolvedVersionForTest, []byte{0xff}), wantReference: wantReference, wantBytes: []byte{0xff}, wantTextErr: core.ErrSecretStorePayload},
		{name: "ordinary truncated utf8 remains opaque", response: officialChecksummedResponse(request, resolvedVersionForTest, []byte{0xe2, 0x82}), wantReference: wantReference, wantBytes: []byte{0xe2, 0x82}, wantTextErr: core.ErrSecretStorePayload},
		{name: "ordinary all-zero payload is admitted", response: officialChecksummedResponse(request, resolvedVersionForTest, []byte{0, 0, 0}), wantReference: wantReference, wantBytes: []byte{0, 0, 0}},
		{name: "ordinary long opaque payload is admitted", response: officialChecksummedResponse(request, resolvedVersionForTest, bytes.Repeat([]byte{0xa5}, 128)), wantReference: wantReference, wantBytes: bytes.Repeat([]byte{0xa5}, 128), wantTextErr: core.ErrSecretStorePayload},
		// Hostile provider refusals.
		{name: "nil provider response is rejected", wantErr: core.ErrSecretStorePayload},
		{name: "nil provider payload is rejected", response: &secretmanagerpb.AccessSecretVersionResponse{Name: resolvedNameForTest(request, resolvedVersionForTest)}, wantErr: core.ErrSecretStorePayload},
		{name: "missing resolved name is rejected", response: officialResponse("", []byte("secret"), checksumForTest([]byte("secret"))), wantErr: core.ErrSecretStorePayload},
		{name: "wrong resolved prefix is rejected", response: officialResponse("folders/project1/secrets/runtime-secret/versions/42", []byte("secret"), checksumForTest([]byte("secret"))), wantErr: core.ErrSecretStorePayload},
		{name: "latest is rejected as a resolved version", response: officialResponse(resolvedNameTextForTest(request, googleLatestVersionText), []byte("secret"), checksumForTest([]byte("secret"))), wantErr: core.ErrSecretStorePayload},
		{name: "zero resolved version is rejected", response: officialResponse(resolvedNameTextForTest(request, "0"), []byte("secret"), checksumForTest([]byte("secret"))), wantErr: core.ErrSecretStorePayload},
		{name: "leading-zero resolved version is rejected", response: officialResponse(resolvedNameTextForTest(request, "042"), []byte("secret"), checksumForTest([]byte("secret"))), wantErr: core.ErrSecretStorePayload},
		{name: "project ID in resolved name is rejected", response: officialResponse(resolvedNamePartsForTest(request.Project.String(), request.Secret, "42"), []byte("secret"), checksumForTest([]byte("secret"))), wantErr: core.ErrSecretStorePayload},
		{name: "zero resolved project number is rejected", response: officialResponse(resolvedNamePartsForTest("0", request.Secret, "42"), []byte("secret"), checksumForTest([]byte("secret"))), wantErr: core.ErrSecretStorePayload},
		{name: "leading-zero resolved project number is rejected", response: officialResponse(resolvedNamePartsForTest("0123456789", request.Secret, "42"), []byte("secret"), checksumForTest([]byte("secret"))), wantErr: core.ErrSecretStorePayload},
		{name: "foreign resolved secret is rejected", response: officialChecksummedResponse(foreignSecret, resolvedVersionForTest, []byte("secret")), wantErr: core.ErrSecretStorePayload},
		{name: "missing provider checksum is rejected", response: officialResponse(resolvedNameForTest(request, resolvedVersionForTest), []byte("secret"), nil), wantErr: core.ErrSecretStorePayload},
		{name: "wrong provider checksum is rejected", response: officialResponse(resolvedNameForTest(request, resolvedVersionForTest), []byte("secret"), &zero), wantErr: core.ErrSecretStorePayload},
		{name: "negative provider checksum is rejected", response: officialResponse(resolvedNameForTest(request, resolvedVersionForTest), []byte("secret"), &negative), wantErr: core.ErrSecretStorePayload},
		{name: "provider checksum above uint32 is rejected", response: officialResponse(resolvedNameForTest(request, resolvedVersionForTest), []byte("secret"), &aboveUint32), wantErr: core.ErrSecretStorePayload},
		{name: "provider checksum at maximum int64 is rejected", response: officialResponse(resolvedNameForTest(request, resolvedVersionForTest), []byte("secret"), &maximumInt64), wantErr: core.ErrSecretStorePayload},
		{name: "one above provider maximum is rejected", response: officialChecksummedResponse(request, resolvedVersionForTest, bytes.Repeat([]byte{'a'}, PayloadMaximumBytes+1)), wantErr: core.ErrSecretStorePayload},
		// Twenty hostile payload and version boundaries.
		{name: "exact zero-byte provider floor is admitted", response: officialChecksummedResponse(request, 1, nil), wantReference: resolvedReferenceForTest(request, 1)},
		{name: "exact one-byte text payload is admitted", response: officialChecksummedResponse(request, 1, []byte("a")), wantReference: resolvedReferenceForTest(request, 1), wantBytes: []byte("a")},
		{name: "exact one-byte opaque payload is admitted", response: officialChecksummedResponse(request, 2, []byte{0xff}), wantReference: resolvedReferenceForTest(request, 2), wantBytes: []byte{0xff}, wantTextErr: core.ErrSecretStorePayload},
		{name: "two-byte text payload is admitted", response: officialChecksummedResponse(request, 9, []byte("ab")), wantReference: resolvedReferenceForTest(request, 9), wantBytes: []byte("ab")},
		{name: "minimum nul byte is admitted", response: officialChecksummedResponse(request, 10, []byte{0}), wantReference: resolvedReferenceForTest(request, 10), wantBytes: []byte{0}},
		{name: "minimum space byte is admitted", response: officialChecksummedResponse(request, 99, []byte{' '}), wantReference: resolvedReferenceForTest(request, 99), wantBytes: []byte{' '}},
		{name: "minimum newline byte is admitted", response: officialChecksummedResponse(request, 100, []byte{'\n'}), wantReference: resolvedReferenceForTest(request, 100), wantBytes: []byte{'\n'}},
		{name: "minimum delete byte is admitted", response: officialChecksummedResponse(request, 999, []byte{0x7f}), wantReference: resolvedReferenceForTest(request, 999), wantBytes: []byte{0x7f}},
		{name: "utf8 boundary two-byte rune is admitted", response: officialChecksummedResponse(request, 1000, []byte("é")), wantReference: resolvedReferenceForTest(request, 1000), wantBytes: []byte("é")},
		{name: "utf8 boundary three-byte rune is admitted", response: officialChecksummedResponse(request, 9999, []byte("€")), wantReference: resolvedReferenceForTest(request, 9999), wantBytes: []byte("€")},
		{name: "utf8 boundary four-byte rune is admitted", response: officialChecksummedResponse(request, 10000, []byte("😀")), wantReference: resolvedReferenceForTest(request, 10000), wantBytes: []byte("😀")},
		{name: "invalid utf8 continuation remains opaque", response: officialChecksummedResponse(request, 99999, []byte{0x80}), wantReference: resolvedReferenceForTest(request, 99999), wantBytes: []byte{0x80}, wantTextErr: core.ErrSecretStorePayload},
		{name: "invalid utf8 overlong form remains opaque", response: officialChecksummedResponse(request, 100000, []byte{0xc0, 0x80}), wantReference: resolvedReferenceForTest(request, 100000), wantBytes: []byte{0xc0, 0x80}, wantTextErr: core.ErrSecretStorePayload},
		{name: "invalid utf8 surrogate remains opaque", response: officialChecksummedResponse(request, 999999, []byte{0xed, 0xa0, 0x80}), wantReference: resolvedReferenceForTest(request, 999999), wantBytes: []byte{0xed, 0xa0, 0x80}, wantTextErr: core.ErrSecretStorePayload},
		{name: "one below provider maximum is admitted", response: officialChecksummedResponse(request, 1000000, bytes.Repeat([]byte{'a'}, PayloadMaximumBytes-1)), wantReference: resolvedReferenceForTest(request, 1000000), wantBytes: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes-1)},
		{name: "exact provider maximum is admitted", response: officialChecksummedResponse(request, 9999999, bytes.Repeat([]byte{'a'}, PayloadMaximumBytes)), wantReference: resolvedReferenceForTest(request, 9999999), wantBytes: bytes.Repeat([]byte{'a'}, PayloadMaximumBytes)},
		{name: "maximum opaque bytes are admitted", response: officialChecksummedResponse(request, 10000000, bytes.Repeat([]byte{0xff}, PayloadMaximumBytes)), wantReference: resolvedReferenceForTest(request, 10000000), wantBytes: bytes.Repeat([]byte{0xff}, PayloadMaximumBytes), wantTextErr: core.ErrSecretStorePayload},
		{name: "maximum ending newline is admitted", response: officialChecksummedResponse(request, GoogleVersionNumber(math.MaxUint32), append(bytes.Repeat([]byte{'a'}, PayloadMaximumBytes-1), '\n')), wantReference: resolvedReferenceForTest(request, GoogleVersionNumber(math.MaxUint32)), wantBytes: append(bytes.Repeat([]byte{'a'}, PayloadMaximumBytes-1), '\n')},
		{name: "maximum ending nul is admitted", response: officialChecksummedResponse(request, GoogleVersionNumber(math.MaxInt64), append(bytes.Repeat([]byte{'a'}, PayloadMaximumBytes-1), 0)), wantReference: resolvedReferenceForTest(request, GoogleVersionNumber(math.MaxInt64)), wantBytes: append(bytes.Repeat([]byte{'a'}, PayloadMaximumBytes-1), 0)},
		{name: "maximum uint64 version is admitted", response: officialChecksummedResponse(request, GoogleVersionNumber(math.MaxUint64), []byte("secret")), wantReference: resolvedReferenceForTest(request, GoogleVersionNumber(math.MaxUint64)), wantBytes: []byte("secret")},
		{name: "checksum one below correct is rejected", response: officialResponseWithChecksumDelta(request, resolvedVersionForTest, []byte("secret"), -1), wantErr: core.ErrSecretStorePayload},
	}
}

func accessRequestForTest(t testing.TB) AccessRequest {
	t.Helper()
	project, err := ParseGoogleProjectID("sample-project")
	if err != nil {
		t.Fatalf("ParseGoogleProjectID(fixture) error = %v, want nil", err)
	}
	secret, err := ParseGoogleSecretID("runtime-secret")
	if err != nil {
		t.Fatalf("ParseGoogleSecretID(fixture) error = %v, want nil", err)
	}
	return AccessRequest{Project: project, Secret: secret, Version: GoogleVersionSelectorLatest}
}

func resolvedReferenceForTest(request AccessRequest, version GoogleVersionNumber) ResolvedReference {
	return ResolvedReference{ProjectNumber: resolvedProjectForTest, Secret: request.Secret, Version: version}
}

func resolvedNameForTest(request AccessRequest, version GoogleVersionNumber) string {
	return resolvedNameTextForTest(request, strconv.FormatUint(version.Uint64(), 10))
}

func resolvedNameTextForTest(request AccessRequest, version string) string {
	return resolvedNamePartsForTest(strconv.FormatUint(resolvedProjectForTest.Uint64(), 10), request.Secret, version)
}

func resolvedNamePartsForTest(project string, secret GoogleSecretID, version string) string {
	return googleProjectResourcePrefix + project + googleSecretResourceSegment +
		secret.String() + googleVersionResourceSegment + version
}

func requireProviderPayloadCleared(t *testing.T, response *secretmanagerpb.AccessSecretVersionResponse) {
	t.Helper()
	if response == nil || response.Payload == nil {
		return
	}
	for index, value := range response.Payload.Data {
		if value != 0 {
			t.Fatalf("provider response payload[%d] = %d, want cleared", index, value)
		}
	}
}

func checksumForTest(payload []byte) *int64 {
	checksum := int64(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)))
	return &checksum
}

func officialResponseWithChecksumDelta(request AccessRequest, version GoogleVersionNumber, payload []byte, delta int64) *secretmanagerpb.AccessSecretVersionResponse {
	checksum := int64(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli))) + delta
	return officialResponse(resolvedNameForTest(request, version), payload, &checksum)
}

func TestOfficialPermissionDeniedIdentitySurvivesProjection(t *testing.T) {
	t.Parallel()

	providerErr := status.Error(codes.PermissionDenied, "provider denied access")
	gotErr := googleAccessError(providerErr)
	if !errors.Is(gotErr, core.ErrSecretStoreAccess) {
		t.Fatalf("googleAccessError() error = %v, want errors.Is(..., %v)", gotErr, core.ErrSecretStoreAccess)
	}
	if status.Code(gotErr) != codes.PermissionDenied {
		t.Fatalf("status.Code(googleAccessError()) = %v, want %v", status.Code(gotErr), codes.PermissionDenied)
	}
}

func officialResponse(name string, payload []byte, checksum *int64) *secretmanagerpb.AccessSecretVersionResponse {
	return &secretmanagerpb.AccessSecretVersionResponse{
		Name:    name,
		Payload: &secretmanagerpb.SecretPayload{Data: payload, DataCrc32C: checksum},
	}
}

func officialChecksummedResponse(request AccessRequest, version GoogleVersionNumber, payload []byte) *secretmanagerpb.AccessSecretVersionResponse {
	return officialResponse(resolvedNameForTest(request, version), payload, checksumForTest(payload))
}

func FuzzResolvedGoogleReferenceSemanticClosure(f *testing.F) {
	request := accessRequestForTest(f)
	f.Add(resolvedNameForTest(request, 1))
	f.Add(resolvedNameForTest(request, resolvedVersionForTest))
	f.Add("")
	f.Add(resolvedNameTextForTest(request, googleLatestVersionText))
	f.Fuzz(func(t *testing.T, name string) {
		got, gotErr := parseResolvedReference(name)
		if gotErr != nil {
			if got != (ResolvedReference{}) || !errors.Is(gotErr, core.ErrSecretStorePayload) {
				t.Fatalf("parseResolvedReference(rejected) = (%v, %v), want zero and typed payload error", got, gotErr)
			}
			return
		}
		canonical := resolvedNamePartsForTest(strconv.FormatUint(got.ProjectNumber.Uint64(), 10), got.Secret, strconv.FormatUint(got.Version.Uint64(), 10))
		if got.Validate() != nil || canonical != name {
			t.Fatalf("parseResolvedReference(accepted) = %v, want validated canonical identity for %q", got, name)
		}
		roundTrip, roundTripErr := parseResolvedReference(canonical)
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("parseResolvedReference(round trip) = (%v, %v), want (%v, nil)", roundTrip, roundTripErr, got)
		}
	})
}

func FuzzOfficialGoogleResponseSemanticClosure(f *testing.F) {
	request := accessRequestForTest(f)
	f.Add([]byte("0123456789abcdef"), true)
	f.Add([]byte{}, true)
	f.Add([]byte(" secret"), true)
	f.Add([]byte("secret"), false)
	f.Fuzz(func(t *testing.T, payload []byte, correctChecksum bool) {
		checksum := int64(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)))
		if !correctChecksum {
			checksum = int64(uint32(checksum) ^ 1)
		}
		responsePayload := append([]byte(nil), payload...)
		response := officialResponse(resolvedNameForTest(request, resolvedVersionForTest), responsePayload, &checksum)
		got, gotErr := accessResultFromGoogleResponse(request, response)
		want, wantErr := NewValue(payload)
		wantAccepted := correctChecksum && wantErr == nil
		if !wantAccepted {
			if !errors.Is(gotErr, core.ErrSecretStorePayload) || got != (AccessResult{}) {
				t.Fatalf("accessResultFromGoogleResponse(rejected) = (%v, %v), want zero and payload identity", got, gotErr)
			}
			requireProviderPayloadCleared(t, response)
			return
		}
		if gotErr != nil || got.Validate() != nil || got.Request != request || got.Reference != resolvedReferenceForTest(request, resolvedVersionForTest) {
			t.Fatalf("accessResultFromGoogleResponse(accepted) = (%v, %v), want exact validated result", got, gotErr)
		}
		gotBytes, gotBytesErr := got.Value.CopyBytes()
		wantBytes, wantBytesErr := want.CopyBytes()
		if gotBytesErr != nil || wantBytesErr != nil || !bytes.Equal(gotBytes, wantBytes) {
			t.Fatalf("provider/value projection = (%x, %v), want (%x, %v)", gotBytes, gotBytesErr, wantBytes, wantBytesErr)
		}
		requireProviderPayloadCleared(t, response)
		if destroyErr := got.Value.Destroy(); destroyErr != nil {
			t.Fatalf("provider Value.Destroy() error = %v, want nil", destroyErr)
		}
		if destroyErr := want.Destroy(); destroyErr != nil {
			t.Fatalf("oracle Value.Destroy() error = %v, want nil", destroyErr)
		}
	})
}
