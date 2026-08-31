package runworkspace

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"hash/crc32"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestSourceDownloadEffectLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive receive-only capability streams exact source through filestore into verified checkout", func(t *testing.T) {
		t.Parallel()
		manager, unit := sourceWorkspaceFixture(t)
		defer closeSourceManager(t, manager)
		defer cleanupSourceUnit(t, manager, unit)
		archive, document, grant, trusted := sourceArchiveFixture(t, unit, []byte("package subject\n"))
		request := sourceDownloadRequestFixture(t, unit, grant, document, trusted, archive, archive)

		got, gotErr := manager.AcquireSource(t.Context(), request)
		if gotErr != nil || got.Coordinate != grant.Source || got.Files != 1 || got.Directories != 2 {
			t.Fatalf("Manager.AcquireSource() = (%+v, %v), want exact coordinate, 1 file, 2 directories, nil", got, gotErr)
		}
		archivePath, pathErr := joinLiteral(unit.Root, ".source-archive.download")
		if pathErr != nil {
			t.Fatalf("joinLiteral(source archive stage) error = %v, want nil", pathErr)
		}
		_, openErr := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: filestore.Location{Root: manager.root, Path: archivePath}})
		if !errors.Is(openErr, core.ErrFilestoreSource) {
			t.Fatalf("filestore.OpenRead(removed source stage) error = %v, want errors.Is(..., %v)", openErr, core.ErrFilestoreSource)
		}
	})

	t.Run("negative provider byte substitution is rejected and stage plus checkout remain absent", func(t *testing.T) {
		t.Parallel()
		manager, unit := sourceWorkspaceFixture(t)
		defer closeSourceManager(t, manager)
		defer cleanupSourceUnit(t, manager, unit)
		archive, document, grant, trusted := sourceArchiveFixture(t, unit, []byte("package subject\n"))
		mutated := append([]byte(nil), archive...)
		mutated[len(mutated)/2] ^= 1
		request := sourceDownloadRequestFixture(t, unit, grant, document, trusted, archive, mutated)

		got, gotErr := manager.AcquireSource(t.Context(), request)
		if got != (VerifiedSource{}) || !errors.Is(gotErr, core.ErrObjectStoreIntegrity) {
			t.Fatalf("Manager.AcquireSource(substituted provider bytes) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrObjectStoreIntegrity)
		}
		observation, observeErr := manager.Observe(t.Context(), temporal.InstantFromNanoseconds(11), Residue{})
		if observeErr != nil || observation.Entries != 1 {
			t.Fatalf("Manager.Observe(after download refusal) = (%+v, %v), want only scheduling unit and nil", observation, observeErr)
		}
	})

	t.Run("neutral expired source grant refuses before provider or filesystem effects", func(t *testing.T) {
		t.Parallel()
		manager, unit := sourceWorkspaceFixture(t)
		defer closeSourceManager(t, manager)
		defer cleanupSourceUnit(t, manager, unit)
		archive, document, grant, trusted := sourceArchiveFixture(t, unit, []byte("package subject\n"))
		var invoked atomic.Bool
		request := sourceDownloadRequestFixtureWithObservation(t, unit, grant, document, trusted, archive, archive, &invoked)
		request.ObservedAt = grant.ExpiresAt

		got, gotErr := manager.AcquireSource(t.Context(), request)
		if got != (VerifiedSource{}) || !errors.Is(gotErr, core.ErrPrimitiveContract) || invoked.Load() {
			t.Fatalf("Manager.AcquireSource(expired grant) = (%+v, %v, provider invoked %t), want zero, errors.Is(..., %v), false", got, gotErr, invoked.Load(), core.ErrPrimitiveContract)
		}
		observation, observeErr := manager.Observe(t.Context(), temporal.InstantFromNanoseconds(101), Residue{})
		if observeErr != nil || observation.Entries != 1 {
			t.Fatalf("Manager.Observe(after pre-effect source refusal) = (%+v, %v), want only scheduling unit and nil", observation, observeErr)
		}
	})
}

func sourceDownloadRequestFixture(t testing.TB, unit Unit, grant runnercontrol.SourceGrant, document runnercontrol.SourceArchiveDocument, trusted attest.TrustedKeys, expected, served []byte) SourceDownloadRequest {
	t.Helper()
	return sourceDownloadRequestFixtureWithObservation(t, unit, grant, document, trusted, expected, served, nil)
}

func sourceDownloadRequestFixtureWithObservation(t testing.TB, unit Unit, grant runnercontrol.SourceGrant, document runnercontrol.SourceArchiveDocument, trusted attest.TrustedKeys, expected, served []byte, invoked *atomic.Bool) SourceDownloadRequest {
	t.Helper()
	checksum := core.NewCRC32C(crc32.Checksum(expected, crc32.MakeTable(crc32.Castagnoli)))
	checksumText, checksumErr := checksum.Base64()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if invoked != nil {
			invoked.Store(true)
		}
		writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
		writer.Header().Set("X-Goog-Hash", "crc32c="+checksumText)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write(served); err != nil {
			t.Errorf("provider response Write() error = %v, want nil", err)
		}
	}))
	t.Cleanup(server.Close)
	client := sourceObjectstoreClient(t, server)
	signedURL, signedErr := objectstore.ParseSignedURL("https://storage.googleapis.com/bucket/object?X-Goog-Signature=signature&X-Goog-SignedHeaders=host")
	headers, headersErr := objectstore.NewSignedHeaders(nil)
	target := objectstore.DownloadTarget{URL: signedURL, Headers: headers, ExpiresAt: temporal.InstantFromNanoseconds(2_051_222_400_000_000_000)}
	projection, projectionErr := objectstore.NewDownloadCapabilityProjection(objectstore.ProviderGoogleCloudStorage, target)
	encoded, encodedErr := projection.MarshalJSON()
	var capability objectstore.DownloadCapability
	decodeErr := json.Unmarshal(encoded, &capability)
	errorLimit, errorLimitErr := core.NewByteCount(4096)
	operation, operationErr := temporal.DurationFromSeconds(10)
	attempt, attemptErr := temporal.DurationFromSeconds(5)
	integrity := objectstore.Integrity{SHA256: core.SHA256Of(expected), Length: sourceByteLength(t, uint64(len(expected))), CRC32C: checksum}
	if err := errors.Join(checksumErr, signedErr, headersErr, projectionErr, encodedErr, decodeErr, errorLimitErr, operationErr, attemptErr); err != nil {
		t.Fatalf("source download request fixture error = %v, want nil", err)
	}
	return SourceDownloadRequest{
		Unit: unit, Grant: grant, Document: document, TrustedKeys: trusted,
		ObservedAt: temporal.InstantFromNanoseconds(10), Client: client, Capability: capability,
		Integrity: integrity, ContentType: core.HTTPMediaTypeOctetStream(),
		Policy: objectstore.Policy{OperationTimeout: operation, AttemptTimeout: attempt, ErrorBodyLimit: errorLimit},
	}
}

func sourceObjectstoreClient(t testing.TB, server *httptest.Server) objectstore.Client {
	t.Helper()
	serverAddress := strings.TrimPrefix(server.URL, "https://")
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, serverAddress)
	}
	httpClient := &http.Client{Transport: transport}
	t.Cleanup(transport.CloseIdleConnections)
	exchangeClient, exchangeErr := exchange.NewClient(httpClient)
	client, clientErr := objectstore.NewClient(exchangeClient)
	if err := errors.Join(exchangeErr, clientErr); err != nil {
		t.Fatalf("objectstore client fixture error = %v, want nil", err)
	}
	return client
}
