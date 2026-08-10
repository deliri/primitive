package objectstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"hash/crc32"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type transferEvidenceFixtureRequest struct {
	Version   string
	Bytes     uint64
	Provider  Provider
	Direction Direction
}

type transferEvidenceProjectionCase struct {
	name    string
	request transferEvidenceFixtureRequest
}

// TestTransferEvidenceProjectionLayerTriad is a direct projection-layer
// ratchet. Its fixture starts from a sealed Transfer; provider execution is
// covered by the transfer entry-point tests, not fabricated here.
func TestTransferEvidenceProjectionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive sealed transfers project every exact boundary fact", func(t *testing.T) {
		t.Parallel()

		cases := []transferEvidenceProjectionCase{
			{name: "zero-byte amazon upload without provider version", request: transferEvidenceFixtureRequest{Provider: ProviderAmazonS3, Direction: DirectionUpload}},
			{name: "one-byte amazon download without provider version", request: transferEvidenceFixtureRequest{Provider: ProviderAmazonS3, Direction: DirectionDownload, Bytes: 1}},
			{name: "two-byte google upload without provider version", request: transferEvidenceFixtureRequest{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Bytes: 2}},
			{name: "stream chunk one below boundary", request: transferEvidenceFixtureRequest{Provider: ProviderGoogleCloudStorage, Direction: DirectionDownload, Bytes: 32<<10 - 1}},
			{name: "stream chunk at boundary", request: transferEvidenceFixtureRequest{Provider: ProviderGoogleCloudStorage, Direction: DirectionDownload, Bytes: 32 << 10}},
			{name: "stream chunk one above boundary", request: transferEvidenceFixtureRequest{Provider: ProviderGoogleCloudStorage, Direction: DirectionDownload, Bytes: 32<<10 + 1}},
			{name: "maximum signed byte length", request: transferEvidenceFixtureRequest{Provider: ProviderAmazonS3, Direction: DirectionUpload, Bytes: math.MaxInt64}},
			{name: "minimum amazon version identifier", request: transferEvidenceFixtureRequest{Provider: ProviderAmazonS3, Direction: DirectionUpload, Bytes: 1, Version: "v"}},
			{name: "maximum amazon version identifier", request: transferEvidenceFixtureRequest{Provider: ProviderAmazonS3, Direction: DirectionDownload, Bytes: 1, Version: strings.Repeat("v", AmazonS3VersionIDMaximumBytes)}},
			{name: "maximum amazon version json expansion", request: transferEvidenceFixtureRequest{Provider: ProviderAmazonS3, Direction: DirectionUpload, Bytes: 1, Version: strings.Repeat("<", AmazonS3VersionIDMaximumBytes)}},
			{name: "maximum google generation", request: transferEvidenceFixtureRequest{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Bytes: 1, Version: "18446744073709551615"}},
			{name: "cloudflare upload without impossible version", request: transferEvidenceFixtureRequest{Provider: ProviderCloudflareImages, Direction: DirectionUpload, Bytes: CloudflareImagesUploadMaximumBytes}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				transfer := sealedTransferEvidenceFixture(t, tc.request)
				projection, gotErr := transfer.Evidence()
				if gotErr != nil {
					t.Fatalf("Transfer.Evidence() error = %v, want nil", gotErr)
				}
				if gotErr := projection.Validate(); gotErr != nil {
					t.Fatalf("TransferEvidenceProjection.Validate() error = %v, want nil", gotErr)
				}
				got, gotErr := transferEvidenceRoundTrip(t, projection)
				if gotErr != nil {
					t.Fatalf("transfer evidence round trip error = %v, want nil", gotErr)
				}
				requireTransferEvidenceFacts(t, transferEvidenceFactsCheck{got: got, want: transfer})
			})
		}
	})

	t.Run("negative unconfirmed and impossible transfers emit no projection", func(t *testing.T) {
		t.Parallel()

		base := sealedTransferEvidenceFixture(t, transferEvidenceFixtureRequest{
			Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Bytes: 1, Version: "42",
		})
		failedStatus := core.HTTPStatusOK()
		if gotErr := failedStatus.AdmitInt(http.StatusInternalServerError); gotErr != nil {
			t.Fatalf("HTTPStatusCode.AdmitInt(%d) setup error = %v, want nil", http.StatusInternalServerError, gotErr)
		}
		cases := []struct {
			mutate  func(Transfer) Transfer
			name    string
			wantErr error
		}{
			{name: "zero transfer has no confirmed source", mutate: func(Transfer) Transfer { return Transfer{} }, wantErr: core.ErrObjectStoreContract},
			{name: "unknown provider has no execution owner", mutate: func(value Transfer) Transfer { value.provider = ProviderUnknown; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "future provider has no execution owner", mutate: func(value Transfer) Transfer { value.provider = providerLimit; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "unknown direction names no operation", mutate: func(value Transfer) Transfer { value.direction = DirectionUnknown; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "future direction names no operation", mutate: func(value Transfer) Transfer { value.direction = directionLimit; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "not-attempted commitment proves no transfer", mutate: func(value Transfer) Transfer { value.commitment = CommitmentNotAttempted; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "rejected commitment proves no accepted object", mutate: func(value Transfer) Transfer { value.commitment = CommitmentRejected; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "indeterminate commitment proves no accepted object", mutate: func(value Transfer) Transfer { value.commitment = CommitmentIndeterminate; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "unset sha256 proves no exact bytes", mutate: func(value Transfer) Transfer { value.sha256 = core.SHA256Digest{}; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "unset crc32c proves no exact bytes", mutate: func(value Transfer) Transfer { value.crc32c = core.CRC32C{}; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "unset status proves no provider acceptance", mutate: func(value Transfer) Transfer { value.status = core.HTTPStatusCode{}; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "failed status contradicts confirmed commitment", mutate: func(value Transfer) Transfer { value.status = failedStatus; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "provider version from another provider is contradictory", mutate: func(value Transfer) Transfer {
				value.version, _ = newProviderVersion(ProviderAmazonS3, "version")
				return value
			}, wantErr: core.ErrObjectStoreContract},
			{name: "cloudflare version is not a published object identity", mutate: func(value Transfer) Transfer { value.provider = ProviderCloudflareImages; return value }, wantErr: core.ErrObjectStoreContract},
			{name: "cloudflare download is not a published operation", mutate: func(value Transfer) Transfer {
				value.provider = ProviderCloudflareImages
				value.direction = DirectionDownload
				value.version = ProviderVersion{}
				return value
			}, wantErr: core.ErrObjectStoreContract},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, gotErr := tc.mutate(base).Evidence()
				if !errors.Is(gotErr, tc.wantErr) || got != (TransferEvidenceProjection{}) {
					t.Fatalf("Transfer.Evidence() = (%v, %v), want zero and errors.Is %v", got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero-byte transfer invents no optional provider version", func(t *testing.T) {
		t.Parallel()

		transfer := sealedTransferEvidenceFixture(t, transferEvidenceFixtureRequest{
			Provider: ProviderAmazonS3, Direction: DirectionUpload,
		})
		projection, gotErr := transfer.Evidence()
		if gotErr != nil {
			t.Fatalf("Transfer.Evidence() error = %v, want nil", gotErr)
		}
		got, gotErr := transferEvidenceRoundTrip(t, projection)
		if gotErr != nil {
			t.Fatalf("transfer evidence round trip error = %v, want nil", gotErr)
		}
		gotVersion, gotPresent := got.Version()
		if gotPresent || gotVersion != (ProviderVersion{}) || got.Bytes().Uint64() != 0 {
			t.Fatalf("neutral evidence version/bytes = (%v, %t, %d), want (zero, false, 0)", gotVersion, gotPresent, got.Bytes().Uint64())
		}
	})
}

type transferEvidenceDocumentCase struct {
	build   func([]byte) []byte
	name    string
	wantErr error
}

func TestTransferEvidenceDecodeLayerTriad(t *testing.T) {
	t.Parallel()

	projection, gotErr := sealedTransferEvidenceFixture(t, transferEvidenceFixtureRequest{
		Provider: ProviderAmazonS3, Direction: DirectionDownload, Bytes: 1, Version: "version-1",
	}).Evidence()
	if gotErr != nil {
		t.Fatalf("Transfer.Evidence() setup error = %v, want nil", gotErr)
	}
	canonical, gotErr := projection.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("TransferEvidenceProjection.MarshalJSON() setup error = %v, want nil", gotErr)
	}

	t.Run("positive strict receiver accepts canonical and hostile-valid framing", func(t *testing.T) {
		t.Parallel()

		cases := []transferEvidenceDocumentCase{
			{name: "canonical issuer document", build: unchangedTransferEvidenceDocument},
			{name: "leading json whitespace", build: func(value []byte) []byte { return append([]byte(" \n\t"), value...) }},
			{name: "trailing json whitespace", build: func(value []byte) []byte { return append(append([]byte(nil), value...), ' ', '\n', '\t') }},
			{name: "both-side json whitespace", build: func(value []byte) []byte { return append(append([]byte(" \n"), value...), '\n', ' ') }},
			{name: "members reordered without changing facts", build: reorderedTransferEvidenceDocument},
			{name: "zero bytes remain explicitly present", build: zeroByteTransferEvidenceDocument},
			{name: "optional version absent", build: versionAbsentTransferEvidenceDocument},
			{name: "one below document ceiling", build: func(value []byte) []byte {
				return padTransferEvidenceDocument(value, TransferEvidenceJSONMaximumBytes-1)
			}},
			{name: "exact document ceiling", build: func(value []byte) []byte { return padTransferEvidenceDocument(value, TransferEvidenceJSONMaximumBytes) }},
			{name: "google generation uses canonical decimal", build: googleTransferEvidenceDocument},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				document := tc.build(canonical)
				var got TransferEvidence
				gotErr := got.UnmarshalJSON(document)
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("TransferEvidence.UnmarshalJSON() = (%v, %v), want valid evidence and nil", got, gotErr)
				}
			})
		}
	})

	t.Run("negative malformed and contradictory documents preserve receiver", func(t *testing.T) {
		t.Parallel()

		var before TransferEvidence
		if gotErr := before.UnmarshalJSON(canonical); gotErr != nil {
			t.Fatalf("TransferEvidence.UnmarshalJSON(valid setup) error = %v, want nil", gotErr)
		}
		cases := []transferEvidenceDocumentCase{
			{name: "empty document", build: func([]byte) []byte { return nil }, wantErr: core.ErrJSONContract},
			{name: "whitespace-only document", build: func([]byte) []byte { return []byte(" \n\t") }, wantErr: core.ErrJSONContract},
			{name: "null document", build: func([]byte) []byte { return []byte("null") }, wantErr: core.ErrJSONContract},
			{name: "array document", build: func([]byte) []byte { return []byte("[]") }, wantErr: core.ErrJSONContract},
			{name: "string document", build: func([]byte) []byte { return []byte(`"evidence"`) }, wantErr: core.ErrJSONContract},
			{name: "boolean document", build: func([]byte) []byte { return []byte("true") }, wantErr: core.ErrJSONContract},
			{name: "truncated before first member", build: func([]byte) []byte { return []byte("{") }, wantErr: core.ErrJSONContract},
			{name: "truncated inside provider", build: func([]byte) []byte { return []byte(`{"provider":"amazon`) }, wantErr: core.ErrJSONContract},
			{name: "truncated before final brace", build: func(value []byte) []byte { return append([]byte(nil), value[:len(value)-1]...) }, wantErr: core.ErrJSONContract},
			{name: "two concatenated documents", build: func(value []byte) []byte { return append(append([]byte(nil), value...), value...) }, wantErr: core.ErrJSONContract},
			{name: "unknown member", build: unknownMemberTransferEvidenceDocument, wantErr: core.ErrJSONContract},
			{name: "duplicate provider member", build: duplicateProviderTransferEvidenceDocument, wantErr: core.ErrJSONContract},
			{name: "missing provider member", build: missingProviderTransferEvidenceDocument, wantErr: core.ErrObjectStoreContract},
			{name: "missing direction member", build: missingDirectionTransferEvidenceDocument, wantErr: core.ErrObjectStoreContract},
			{name: "missing bytes member", build: missingBytesTransferEvidenceDocument, wantErr: core.ErrObjectStoreContract},
			{name: "missing sha256 member", build: missingSHA256TransferEvidenceDocument, wantErr: core.ErrObjectStoreContract},
			{name: "missing crc32c member", build: missingCRC32CTransferEvidenceDocument, wantErr: core.ErrObjectStoreContract},
			{name: "unknown provider token", build: unknownProviderTransferEvidenceDocument, wantErr: core.ErrObjectStoreContract},
			{name: "unknown direction token", build: unknownDirectionTransferEvidenceDocument, wantErr: core.ErrObjectStoreContract},
			{name: "upload-only provider declares impossible download", build: impossibleProviderDirectionTransferEvidenceDocument, wantErr: core.ErrObjectStoreContract},
			{name: "provider version belongs to another provider", build: mismatchedProviderVersionTransferEvidenceDocument, wantErr: core.ErrObjectStoreContract},
			{name: "provider has wrong json type", build: providerTypeWrongTransferEvidenceDocument, wantErr: core.ErrJSONContract},
			{name: "direction has wrong json type", build: directionTypeWrongTransferEvidenceDocument, wantErr: core.ErrJSONContract},
			{name: "bytes have wrong json type", build: bytesTypeWrongTransferEvidenceDocument, wantErr: core.ErrJSONContract},
			{name: "negative bytes are outside domain", build: negativeBytesTransferEvidenceDocument, wantErr: core.ErrJSONContract},
			{name: "bytes one above signed maximum", build: overflowBytesTransferEvidenceDocument, wantErr: core.ErrJSONContract},
			{name: "sha256 uppercase is noncanonical", build: uppercaseSHA256TransferEvidenceDocument, wantErr: core.ErrJSONContract},
			{name: "crc32c has invalid encoded width", build: invalidCRC32CTransferEvidenceDocument, wantErr: core.ErrJSONContract},
			{name: "amazon version exceeds maximum by one", build: oversizedVersionTransferEvidenceDocument, wantErr: core.ErrObjectStoreContract},
			{name: "google version is noncanonical decimal", build: noncanonicalGenerationTransferEvidenceDocument, wantErr: core.ErrObjectStoreContract},
			{name: "document exceeds ceiling by one byte", build: func(value []byte) []byte {
				return padTransferEvidenceDocument(value, TransferEvidenceJSONMaximumBytes+1)
			}, wantErr: core.ErrJSONContract},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := before
				gotErr := got.UnmarshalJSON(tc.build(canonical))
				if !errors.Is(gotErr, tc.wantErr) || got != before {
					t.Fatalf("TransferEvidence.UnmarshalJSON() = (%v, %v), want preserved receiver and errors.Is %v", got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral absent optional version creates no provider version", func(t *testing.T) {
		t.Parallel()

		var got TransferEvidence
		gotErr := got.UnmarshalJSON(versionAbsentTransferEvidenceDocument(canonical))
		gotVersion, gotPresent := got.Version()
		if gotErr != nil || gotPresent || gotVersion != (ProviderVersion{}) {
			t.Fatalf("version-absent decode = (%v, %t, %v), want zero version, false, nil", gotVersion, gotPresent, gotErr)
		}
	})
}

type transferEvidenceFactsCheck struct {
	got  TransferEvidence
	want Transfer
}

func requireTransferEvidenceFacts(t *testing.T, check transferEvidenceFactsCheck) {
	t.Helper()

	gotVersion, gotVersionPresent := check.got.Version()
	wantVersion, wantVersionPresent := check.want.Version()
	if check.got.Provider() != check.want.Provider() ||
		check.got.Direction() != check.want.Direction() ||
		check.got.Bytes() != check.want.Bytes() ||
		check.got.SHA256() != check.want.SHA256() ||
		check.got.CRC32C() != check.want.CRC32C() ||
		gotVersion != wantVersion || gotVersionPresent != wantVersionPresent {
		t.Fatalf("transfer evidence facts = (%v, %v, %v, %v, %v, %v, %t), want exact sealed transfer (%v, %v, %v, %v, %v, %v, %t)",
			check.got.Provider(), check.got.Direction(), check.got.Bytes(), check.got.SHA256(), check.got.CRC32C(), gotVersion, gotVersionPresent,
			check.want.Provider(), check.want.Direction(), check.want.Bytes(), check.want.SHA256(), check.want.CRC32C(), wantVersion, wantVersionPresent)
	}
}

func sealedTransferEvidenceFixture(t *testing.T, request transferEvidenceFixtureRequest) Transfer {
	t.Helper()

	length, gotErr := core.NewByteLength(request.Bytes)
	if gotErr != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", request.Bytes, gotErr)
	}
	payload := []byte{byte(request.Bytes), byte(request.Bytes >> 8), byte(request.Bytes >> 16)}
	transfer := Transfer{
		provider: request.Provider, direction: request.Direction, commitment: CommitmentConfirmed,
		bytes: length, sha256: core.SHA256Of(payload),
		crc32c: core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli))),
		status: core.HTTPStatusOK(),
	}
	if request.Version != "" {
		transfer.version, gotErr = newProviderVersion(request.Provider, request.Version)
		if gotErr != nil {
			t.Fatalf("newProviderVersion(%v) error = %v, want nil", request.Provider, gotErr)
		}
	}
	if gotErr := transfer.Validate(); gotErr != nil {
		t.Fatalf("Transfer.Validate() setup error = %v, want nil", gotErr)
	}
	return transfer
}

func transferEvidenceRoundTrip(t *testing.T, projection TransferEvidenceProjection) (TransferEvidence, error) {
	t.Helper()

	encoded, gotErr := projection.MarshalJSON()
	if gotErr != nil {
		return TransferEvidence{}, gotErr
	}
	var got TransferEvidence
	gotErr = json.Unmarshal(encoded, &got)
	return got, gotErr
}

func unchangedTransferEvidenceDocument(value []byte) []byte { return append([]byte(nil), value...) }

func padTransferEvidenceDocument(value []byte, size int) []byte {
	if len(value) >= size {
		return append([]byte(nil), value...)
	}
	return append(append([]byte(nil), value...), bytes.Repeat([]byte{' '}, size-len(value))...)
}

func reorderedTransferEvidenceDocument([]byte) []byte {
	return []byte(`{"crc32c":"AAAAAA==","sha256":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f","bytes":1,"version":"version-1","direction":"download","provider":"amazon_s3"}`)
}

func zeroByteTransferEvidenceDocument([]byte) []byte {
	return []byte(`{"provider":"amazon_s3","direction":"upload","bytes":0,"sha256":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f","crc32c":"AAAAAA=="}`)
}

func versionAbsentTransferEvidenceDocument([]byte) []byte {
	return zeroByteTransferEvidenceDocument(nil)
}

func googleTransferEvidenceDocument([]byte) []byte {
	return []byte(`{"provider":"google_cloud_storage","direction":"upload","version":"42","bytes":1,"sha256":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f","crc32c":"AAAAAA=="}`)
}

func unknownMemberTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`{"provider"`), []byte(`{"unknown":1,"provider"`), 1)
}

func duplicateProviderTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`{"provider":"amazon_s3"`), []byte(`{"provider":"amazon_s3","provider":"amazon_s3"`), 1)
}

func missingProviderTransferEvidenceDocument([]byte) []byte {
	return []byte(`{"direction":"download","version":"version-1","bytes":1,"sha256":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f","crc32c":"AAAAAA=="}`)
}

func missingDirectionTransferEvidenceDocument([]byte) []byte {
	return []byte(`{"provider":"amazon_s3","version":"version-1","bytes":1,"sha256":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f","crc32c":"AAAAAA=="}`)
}

func missingBytesTransferEvidenceDocument([]byte) []byte {
	return []byte(`{"provider":"amazon_s3","direction":"download","version":"version-1","sha256":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f","crc32c":"AAAAAA=="}`)
}

func missingSHA256TransferEvidenceDocument([]byte) []byte {
	return []byte(`{"provider":"amazon_s3","direction":"download","version":"version-1","bytes":1,"crc32c":"AAAAAA=="}`)
}

func missingCRC32CTransferEvidenceDocument([]byte) []byte {
	return []byte(`{"provider":"amazon_s3","direction":"download","version":"version-1","bytes":1,"sha256":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"}`)
}

func unknownProviderTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`"amazon_s3"`), []byte(`"future_store"`), 1)
}

func unknownDirectionTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`"download"`), []byte(`"future_direction"`), 1)
}

func impossibleProviderDirectionTransferEvidenceDocument(value []byte) []byte {
	value = bytes.Replace(value, []byte(`"amazon_s3"`), []byte(`"cloudflare_images"`), 1)
	return bytes.Replace(value, []byte(`,"version":"version-1"`), nil, 1)
}

func mismatchedProviderVersionTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`"amazon_s3"`), []byte(`"google_cloud_storage"`), 1)
}

func providerTypeWrongTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`"provider":"amazon_s3"`), []byte(`"provider":1`), 1)
}

func directionTypeWrongTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`"direction":"download"`), []byte(`"direction":false`), 1)
}

func bytesTypeWrongTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`"bytes":1`), []byte(`"bytes":"1"`), 1)
}

func negativeBytesTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`"bytes":1`), []byte(`"bytes":-1`), 1)
}

func overflowBytesTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`"bytes":1`), []byte(`"bytes":9223372036854775808`), 1)
}

func uppercaseSHA256TransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`"sha256":"`), []byte(`"sha256":"A`), 1)
}

func invalidCRC32CTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`"crc32c":"`), []byte(`"crc32c":"A`), 1)
}

func oversizedVersionTransferEvidenceDocument(value []byte) []byte {
	return bytes.Replace(value, []byte(`"version":"version-1"`), []byte(`"version":"`+strings.Repeat("v", AmazonS3VersionIDMaximumBytes+1)+`"`), 1)
}

func noncanonicalGenerationTransferEvidenceDocument([]byte) []byte {
	return []byte(`{"provider":"google_cloud_storage","direction":"upload","version":"042","bytes":1,"sha256":"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f","crc32c":"AAAAAA=="}`)
}
