package objectstore

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"hash/crc32"
	"math"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type transferEvidenceFixtureRequest struct {
	Version   string
	Bytes     uint64
	Provider  Provider
	Direction Direction
}

// TestProviderUploadObservationLayerTriad proves the provider-neutral custody
// handoff accepts exact upload facts and refuses every incomplete or
// contradictory observation without releasing a partial sealed value.
func TestProviderUploadObservationLayerTriad(t *testing.T) {
	t.Parallel()

	occurredAt := temporal.InstantFromNanoseconds(1_786_183_200_000_000_000)
	valid := []transferEvidenceFixtureRequest{
		{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Version: "1"},
		{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Version: "2", Bytes: 1},
		{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Version: "3", Bytes: 2},
		{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Version: "7", Bytes: 7},
		{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Version: "8", Bytes: 8},
		{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Version: "9", Bytes: 9},
		{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Version: "41", Bytes: 32<<10 - 1},
		{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Version: "42", Bytes: 32 << 10},
		{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Version: "43", Bytes: 32<<10 + 1},
		{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Version: "9223372036854775807", Bytes: math.MaxInt64},
	}
	for index, fixture := range valid {
		t.Run("exact provider upload "+strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()
			evidence := transferEvidenceFromFixture(t, fixture)
			version, present := evidence.Version()
			if !present {
				t.Fatal("TransferEvidence.Version() present = false, want true")
			}
			request := ProviderUploadObservationRequest{
				Evidence: evidence, Version: version, Bytes: evidence.Bytes(), CRC32C: evidence.CRC32C(),
				ContentType: core.HTTPMediaTypeOctetStream(), OccurredAt: occurredAt,
			}
			got, gotErr := VerifyProviderUpload(request)
			gotEvidence, evidenceErr := got.Evidence()
			gotType, typeErr := got.ContentType()
			gotOccurredAt, occurredAtErr := got.OccurredAt()
			if gotErr != nil || got.Validate() != nil || evidenceErr != nil || typeErr != nil || occurredAtErr != nil ||
				gotEvidence != evidence || gotType != request.ContentType || gotOccurredAt != occurredAt {
				t.Fatalf("VerifyProviderUpload(%d) closure = (%v, %v, %v, %v, %v, %v), want exact validated facts",
					index, got, gotErr, gotEvidence, gotType, gotOccurredAt, errors.Join(evidenceErr, typeErr, occurredAtErr))
			}
		})
	}

	baseEvidence := transferEvidenceFromFixture(t, transferEvidenceFixtureRequest{
		Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Version: "42", Bytes: 8,
	})
	baseVersion, present := baseEvidence.Version()
	if !present {
		t.Fatal("base TransferEvidence.Version() present = false, want true")
	}
	base := ProviderUploadObservationRequest{
		Evidence: baseEvidence, Version: baseVersion, Bytes: baseEvidence.Bytes(), CRC32C: baseEvidence.CRC32C(),
		ContentType: core.HTTPMediaTypeOctetStream(), OccurredAt: occurredAt,
	}
	otherGCSVersion, err := newProviderVersion(ProviderGoogleCloudStorage, "43")
	if err != nil {
		t.Fatalf("newProviderVersion(GCS) error = %v, want nil", err)
	}
	otherS3Version, err := newProviderVersion(ProviderAmazonS3, "version")
	if err != nil {
		t.Fatalf("newProviderVersion(S3) error = %v, want nil", err)
	}
	downloadEvidence := transferEvidenceFromFixture(t, transferEvidenceFixtureRequest{
		Provider: ProviderGoogleCloudStorage, Direction: DirectionDownload, Version: "42", Bytes: 8,
	})
	shortBytes, shortErr := core.NewByteLength(7)
	longBytes, longErr := core.NewByteLength(9)
	if err := errors.Join(shortErr, longErr); err != nil {
		t.Fatalf("observation boundary ByteLength setup error = %v, want nil", err)
	}
	negative := []struct {
		wantErr error
		mutate  func(*ProviderUploadObservationRequest)
		name    string
	}{
		{name: "zero request", mutate: func(v *ProviderUploadObservationRequest) { *v = ProviderUploadObservationRequest{} }, wantErr: core.ErrObjectStoreContract},
		{name: "evidence absent", mutate: func(v *ProviderUploadObservationRequest) { v.Evidence = TransferEvidence{} }, wantErr: core.ErrObjectStoreContract},
		{name: "version absent", mutate: func(v *ProviderUploadObservationRequest) { v.Version = ProviderVersion{} }, wantErr: core.ErrObjectStoreContract},
		{name: "download is not an upload observation", mutate: func(v *ProviderUploadObservationRequest) { v.Evidence = downloadEvidence }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "one provider generation later", mutate: func(v *ProviderUploadObservationRequest) { v.Version = otherGCSVersion }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "version belongs to another provider", mutate: func(v *ProviderUploadObservationRequest) { v.Version = otherS3Version }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "one byte short", mutate: func(v *ProviderUploadObservationRequest) { v.Bytes = shortBytes }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "one byte long", mutate: func(v *ProviderUploadObservationRequest) { v.Bytes = longBytes }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "foreign CRC32C", mutate: func(v *ProviderUploadObservationRequest) { v.CRC32C = core.NewCRC32C(0x01020304) }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "content type absent", mutate: func(v *ProviderUploadObservationRequest) { v.ContentType = core.HTTPMediaType{} }, wantErr: core.ErrObjectStoreContract},
		{name: "provider time absent", mutate: func(v *ProviderUploadObservationRequest) { v.OccurredAt = temporal.Instant{} }, wantErr: core.ErrObjectStoreContract},
	}
	for _, tc := range negative {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := base
			tc.mutate(&request)
			got, gotErr := VerifyProviderUpload(request)
			if !errors.Is(gotErr, tc.wantErr) || got != (VerifiedProviderUpload{}) {
				t.Fatalf("VerifyProviderUpload(%s) = (%v, %v), want zero and errors.Is(..., %v)", tc.name, got, gotErr, tc.wantErr)
			}
		})
	}

	zero := VerifiedProviderUpload{}
	gotEvidence, evidenceErr := zero.Evidence()
	gotType, typeErr := zero.ContentType()
	gotOccurredAt, occurredAtErr := zero.OccurredAt()
	if gotEvidence != (TransferEvidence{}) || gotType != (core.HTTPMediaType{}) || gotOccurredAt != (temporal.Instant{}) ||
		!errors.Is(evidenceErr, core.ErrObjectStoreContract) || !errors.Is(typeErr, core.ErrObjectStoreContract) ||
		!errors.Is(occurredAtErr, core.ErrObjectStoreContract) {
		t.Fatalf("zero VerifiedProviderUpload projections = (%v, %v, %v, %v), want zero facts and typed refusals",
			gotEvidence, gotType, gotOccurredAt, errors.Join(evidenceErr, typeErr, occurredAtErr))
	}
}

func transferEvidenceFromFixture(t testing.TB, request transferEvidenceFixtureRequest) TransferEvidence {
	t.Helper()
	projection, err := sealedTransferEvidenceFixture(t, request).Evidence()
	if err != nil {
		t.Fatalf("Transfer.Evidence() error = %v, want nil", err)
	}
	evidence, err := transferEvidenceRoundTrip(t, projection)
	if err != nil {
		t.Fatalf("transfer evidence round trip error = %v, want nil", err)
	}
	return evidence
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
			{name: "two-byte google upload with exact generation", request: transferEvidenceFixtureRequest{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Bytes: 2, Version: "1"}},
			{name: "stream chunk one below boundary without optional download version", request: transferEvidenceFixtureRequest{Provider: ProviderGoogleCloudStorage, Direction: DirectionDownload, Bytes: 32<<10 - 1}},
			{name: "stream chunk at boundary", request: transferEvidenceFixtureRequest{Provider: ProviderGoogleCloudStorage, Direction: DirectionDownload, Bytes: 32 << 10, Version: "3"}},
			{name: "stream chunk one above boundary", request: transferEvidenceFixtureRequest{Provider: ProviderGoogleCloudStorage, Direction: DirectionDownload, Bytes: 32<<10 + 1, Version: "4"}},
			{name: "maximum signed byte length", request: transferEvidenceFixtureRequest{Provider: ProviderAmazonS3, Direction: DirectionUpload, Bytes: math.MaxInt64}},
			{name: "minimum amazon version identifier", request: transferEvidenceFixtureRequest{Provider: ProviderAmazonS3, Direction: DirectionUpload, Bytes: 1, Version: "v"}},
			{name: "maximum amazon version identifier", request: transferEvidenceFixtureRequest{Provider: ProviderAmazonS3, Direction: DirectionDownload, Bytes: 1, Version: strings.Repeat("v", AmazonS3VersionIDMaximumBytes)}},
			{name: "maximum amazon version json expansion", request: transferEvidenceFixtureRequest{Provider: ProviderAmazonS3, Direction: DirectionUpload, Bytes: 1, Version: strings.Repeat("<", AmazonS3VersionIDMaximumBytes)}},
			{name: "maximum SDK-representable google generation", request: transferEvidenceFixtureRequest{Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload, Bytes: 1, Version: "9223372036854775807"}},
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
		foreignVersion, gotErr := newProviderVersion(ProviderAmazonS3, "version")
		if gotErr != nil {
			t.Fatalf("newProviderVersion(ProviderAmazonS3) setup error = %v, want nil", gotErr)
		}
		cases := []struct {
			wantErr error
			mutate  func(Transfer) Transfer
			name    string
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
				value.version = foreignVersion
				return value
			}, wantErr: core.ErrObjectStoreContract},
			{name: "google upload without an exact generation cannot be reconciled", mutate: func(value Transfer) Transfer {
				value.version = ProviderVersion{}
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
	wantErr error
	build   func([]byte) []byte
	name    string
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

func sealedTransferEvidenceFixture(t testing.TB, request transferEvidenceFixtureRequest) Transfer {
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

func transferEvidenceRoundTrip(t testing.TB, projection TransferEvidenceProjection) (TransferEvidence, error) {
	t.Helper()

	issued, gotErr := projection.MarshalJSON()
	if gotErr != nil {
		return TransferEvidence{}, gotErr
	}
	var received TransferEvidence
	if gotErr = json.Unmarshal(issued, &received); gotErr != nil {
		return TransferEvidence{}, gotErr
	}
	reemitted, gotErr := json.Marshal(received)
	if gotErr != nil {
		return TransferEvidence{}, gotErr
	}
	if !bytes.Equal(reemitted, issued) {
		t.Fatalf("TransferEvidence receive-side canonical bytes = %q, want issuer bytes %q", reemitted, issued)
	}
	var verified TransferEvidence
	if gotErr = json.Unmarshal(reemitted, &verified); gotErr != nil {
		return TransferEvidence{}, gotErr
	}
	return verified, nil
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
