package gcsobjects

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"hash/crc32"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
	storageapi "google.golang.org/api/storage/v1"
)

const gcsObservationSignedQuery = "?X-Goog-Signature=fixture&X-Goog-SignedHeaders=host%3Bx-goog-hash%3Bx-goog-if-generation-match"

func TestGCSUploadObservationLayerTriadAuthenticatesExactProviderGeneration(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name       string
		payload    []byte
		generation int64
	}{
		{name: "empty object at minimum generation", generation: 1},
		{name: "one byte at generation two", payload: []byte{0x01}, generation: 2},
		{name: "two bytes at generation three", payload: []byte{0x01, 0x02}, generation: 3},
		{name: "seven bytes below small boundary", payload: bytes.Repeat([]byte{0x04}, 7), generation: 7},
		{name: "eight bytes at small boundary", payload: bytes.Repeat([]byte{0x05}, 8), generation: 8},
		{name: "nine bytes above small boundary", payload: bytes.Repeat([]byte{0x06}, 9), generation: 9},
		{name: "one below stream buffer", payload: bytes.Repeat([]byte{0x07}, 32<<10-1), generation: 41},
		{name: "at stream buffer", payload: bytes.Repeat([]byte{0x08}, 32<<10), generation: 42},
		{name: "one above stream buffer", payload: bytes.Repeat([]byte{0x09}, 32<<10+1), generation: 43},
		{name: "SDK generation ceiling", payload: []byte{0x0a}, generation: int64(^uint64(0) >> 1)},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			evidence := observedTransferEvidence(t, tc.payload, tc.generation)
			integrity := observationIntegrity(t, tc.payload)
			client := bucketTestClient(t, observationProvider(t, observationProviderRequest{
				generation: tc.generation, integrity: integrity,
			}))
			got, gotErr := ObserveGCSUpload(context.Background(), client, GCSUploadObservationRequest{
				Bucket: parsedGCSBucket(t, gcsProviderBucketText),
				Name:   parsedGCSObjectName(t, gcsProviderObjectText), Evidence: evidence,
			})
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf("ObserveGCSUpload() = (%v, %v), want validated proof and nil", got, gotErr)
			}
			metadata, metadataErr := got.Metadata()
			observed, evidenceErr := got.Evidence()
			providerObservation, providerErr := got.ProviderObservation()
			providerEvidence, providerEvidenceErr := providerObservation.Evidence()
			providerType, providerTypeErr := providerObservation.ContentType()
			providerOccurredAt, providerOccurredAtErr := providerObservation.OccurredAt()
			if metadataErr != nil || evidenceErr != nil || providerErr != nil || providerEvidenceErr != nil ||
				providerTypeErr != nil || providerOccurredAtErr != nil || observed != evidence || providerEvidence != evidence ||
				providerType != metadata.ContentType() || providerOccurredAt != metadata.CreatedAt() ||
				metadata.Length() != evidence.Bytes() || metadata.CRC32C() != evidence.CRC32C() {
				t.Fatalf("observed upload facts = (%v, %v, %v, %v, %v, %v), want exact GCS and provider-neutral evidence",
					metadata, observed, providerEvidence, providerType, providerOccurredAt,
					errors.Join(metadataErr, evidenceErr, providerErr, providerEvidenceErr, providerTypeErr, providerOccurredAtErr))
			}
		})
	}

	t.Run("negative provider contradictions release no proof", func(t *testing.T) {
		t.Parallel()

		payload := []byte("authoritative provider observation")
		evidence := observedTransferEvidence(t, payload, 42)
		integrity := observationIntegrity(t, payload)
		cases := []struct {
			wantErr error
			mutate  func(*storageapi.Object)
			name    string
			status  int
		}{
			{name: "provider says object is absent", status: http.StatusNotFound, wantErr: core.ErrObjectStoreAbsent},
			{name: "provider refuses metadata", status: http.StatusBadRequest, wantErr: core.ErrObjectStoreSource},
			{name: "foreign bucket", mutate: func(v *storageapi.Object) { v.Bucket = "other-bucket" }, wantErr: core.ErrObjectStoreIntegrity},
			{name: "foreign object name", mutate: func(v *storageapi.Object) { v.Name = "other/object.bin" }, wantErr: core.ErrObjectStoreIntegrity},
			{name: "one generation before", mutate: func(v *storageapi.Object) { v.Generation-- }, wantErr: core.ErrObjectStoreIntegrity},
			{name: "one generation after", mutate: func(v *storageapi.Object) { v.Generation++ }, wantErr: core.ErrObjectStoreIntegrity},
			{name: "one byte short", mutate: func(v *storageapi.Object) { v.Size-- }, wantErr: core.ErrObjectStoreIntegrity},
			{name: "one byte long", mutate: func(v *storageapi.Object) { v.Size++ }, wantErr: core.ErrObjectStoreIntegrity},
			{name: "foreign CRC32C", mutate: func(v *storageapi.Object) { v.Crc32c = "AAAAAA==" }, wantErr: core.ErrObjectStoreIntegrity},
			{name: "missing content type", mutate: func(v *storageapi.Object) { v.ContentType = "" }, wantErr: core.ErrObjectStoreSource},
			{name: "missing creation time", mutate: func(v *storageapi.Object) { v.TimeCreated = "" }, wantErr: core.ErrObjectStoreSource},
			{name: "missing update time", mutate: func(v *storageapi.Object) { v.Updated = "" }, wantErr: core.ErrObjectStoreSource},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				client := bucketTestClient(t, observationProvider(t, observationProviderRequest{
					generation: 42, integrity: integrity, mutate: tc.mutate, status: tc.status,
				}))
				got, gotErr := ObserveGCSUpload(context.Background(), client, GCSUploadObservationRequest{
					Bucket: parsedGCSBucket(t, gcsProviderBucketText),
					Name:   parsedGCSObjectName(t, gcsProviderObjectText), Evidence: evidence,
				})
				if !errors.Is(gotErr, tc.wantErr) || got != (VerifiedGCSUpload{}) {
					t.Fatalf("ObserveGCSUpload(%s) = (%v, %v), want zero and errors.Is(..., %v)", tc.name, got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero values cannot claim observation", func(t *testing.T) {
		t.Parallel()

		if err := (GCSUploadObservationRequest{}).Validate(); !errors.Is(err, core.ErrObjectStoreContract) {
			t.Fatalf("GCSUploadObservationRequest{}.Validate() error = %v, want errors.Is(..., %v)", err, core.ErrObjectStoreContract)
		}
		if err := (VerifiedGCSUpload{}).Validate(); !errors.Is(err, core.ErrObjectStoreContract) {
			t.Fatalf("VerifiedGCSUpload{}.Validate() error = %v, want errors.Is(..., %v)", err, core.ErrObjectStoreContract)
		}
	})
}

type observationProviderRequest struct {
	mutate     func(*storageapi.Object)
	integrity  objectstore.Integrity
	generation int64
	status     int
}

func observationProvider(t testing.TB, request observationProviderRequest) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
		if incoming.Method != http.MethodGet {
			t.Errorf("observation provider method = %q, want %q", incoming.Method, http.MethodGet)
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if got := incoming.URL.Query().Get("generation"); got != strconv.FormatInt(request.generation, 10) {
			t.Errorf("observation generation query = %q, want %d", got, request.generation)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if request.status != 0 {
			writeGoogleAPIError(writer, request.status)
			return
		}
		object := observationProviderObject(t, request.integrity, request.generation)
		if request.mutate != nil {
			request.mutate(&object)
		}
		writeProviderJSON(t, writer, object)
	})
}

func observationProviderObject(t testing.TB, integrity objectstore.Integrity, generation int64) storageapi.Object {
	t.Helper()
	checksum, err := integrity.CRC32C.Base64()
	if err != nil {
		t.Fatalf("CRC32C.Base64() error = %v, want nil", err)
	}
	instant := time.Unix(0, gcsProviderInstantNanos).UTC().Format(time.RFC3339Nano)
	return storageapi.Object{
		Bucket: gcsProviderBucketText, Name: gcsProviderObjectText, Generation: generation,
		Size: integrity.Length.Uint64(), Crc32c: checksum, ContentType: gcsProviderMediaTypeText,
		TimeCreated: instant, Updated: instant,
	}
}

func observedTransferEvidence(t testing.TB, payload []byte, generation int64) objectstore.TransferEvidence {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
		got, err := io.ReadAll(incoming.Body)
		if err != nil || !bytes.Equal(got, payload) {
			t.Errorf("objectstore provider body = (%q, %v), want (%q, nil)", got, err, payload)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writer.Header().Set("X-Goog-Generation", strconv.FormatInt(generation, 10))
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	dialer := &net.Dialer{}
	address := server.Listener.Addr().String()
	transport.DialContext = func(ctx context.Context, network string, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, address)
	}
	t.Cleanup(transport.CloseIdleConnections)
	exchangeClient, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	client, err := objectstore.NewClient(exchangeClient)
	if err != nil {
		t.Fatalf("objectstore.NewClient() error = %v, want nil", err)
	}
	headers, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		t.Fatalf("objectstore.NewSignedHeaders() error = %v, want nil", err)
	}
	url, err := objectstore.ParseSignedURL(core.SchemeHTTPS + "://" + core.GoogleCloudStorageHost + "/bucket/object" + gcsObservationSignedQuery)
	if err != nil {
		t.Fatalf("objectstore.ParseSignedURL() error = %v, want nil", err)
	}
	operation, operationErr := temporal.DurationFromSeconds(10)
	attempt, attemptErr := temporal.DurationFromSeconds(5)
	horizon, horizonErr := temporal.DurationFromDays(10 * 365)
	if err := errors.Join(operationErr, attemptErr, horizonErr); err != nil {
		t.Fatalf("observation duration construction error = %v, want nil", err)
	}
	expiresAt, err := temporal.InstantFromNanoseconds(gcsProviderInstantNanos).Add(horizon)
	if err != nil {
		t.Fatalf("observation expiry construction error = %v, want nil", err)
	}
	limit, err := core.NewByteCount(4096)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	upload := objectstore.UploadRequest{
		Source: bytes.NewReader(payload), ContentType: core.HTTPMediaTypeOctetStream(),
		Target: objectstore.UploadTarget{
			URL: url, Headers: headers, ExpiresAt: expiresAt,
		},
		Integrity: observationIntegrity(t, payload),
		Policy:    objectstore.Policy{OperationTimeout: operation, AttemptTimeout: attempt, ErrorBodyLimit: limit},
	}
	if err := upload.Validate(); err != nil {
		t.Fatalf("objectstore.UploadRequest.Validate() error = %v, want nil", err)
	}
	transfer, err := objectstore.UploadGCS(context.Background(), client, upload)
	if err != nil {
		version, present := transfer.Version()
		t.Fatalf("objectstore.UploadGCS() = (commitment %v, version %v/%t, %v), want confirmed exact generation and nil", transfer.Commitment(), version, present, err)
	}
	projection, err := transfer.Evidence()
	if err != nil {
		t.Fatalf("Transfer.Evidence() error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("TransferEvidenceProjection.MarshalJSON() error = %v, want nil", err)
	}
	var evidence objectstore.TransferEvidence
	if err := json.Unmarshal(encoded, &evidence); err != nil {
		t.Fatalf("json.Unmarshal(TransferEvidence) error = %v, want nil", err)
	}
	return evidence
}

func observationIntegrity(t testing.TB, payload []byte) objectstore.Integrity {
	t.Helper()
	length, err := core.NewByteLength(uint64(len(payload)))
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v, want nil", err)
	}
	return objectstore.Integrity{
		Length: length, SHA256: core.SHA256Of(payload),
		CRC32C: core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli))),
	}
}
