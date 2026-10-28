package gcsobjects

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/option"
	storageapi "google.golang.org/api/storage/v1"
)

func TestVisitGCSUsageLogLifetimeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		kind        int
		wantVisits  uint64
		wantClose   bool
		wantVisitor bool
		wantRead    bool
		wantCancel  bool
	}{
		{name: "complete reader closes before final observation", wantVisits: 1},
		{name: "reader close failure refuses final observation", kind: 1, wantVisits: 1, wantClose: true},
		{name: "visitor failure closes reader without final observation", kind: 2, wantVisits: 1, wantVisitor: true},
		{name: "visitor and close failures retain both causes", kind: 3, wantVisits: 1, wantVisitor: true, wantClose: true},
		{name: "caller cancellation after provisional row retains own cause", kind: 4, wantVisits: 1, wantCancel: true},
		{name: "cancelled caller never opens provider", kind: 5, wantCancel: true},
		{name: "mixed EOF source failure cannot release final observation", kind: 6, wantVisits: 1, wantRead: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			names := usageColumnNames()
			data := usageCSVFixture(t, names[:], usageRecordValues(t, usageRecordFixture(t)))
			closeCause, visitorCause, readCause, cancelCause := errors.New("fixture close failed"), errors.New("fixture visitor failed"), errors.New("fixture read failed"), errors.New("fixture caller cancelled")
			var closeErr, readErr error
			if tc.wantClose {
				closeErr = closeCause
			}
			if tc.wantRead {
				readErr = readCause
			}
			var calls, closes atomic.Int32
			client := usageLifetimeClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Query().Get("alt") == "json" {
					writeGCSAttrsResponse(t, w, data)
					return
				}
				writeGCSMediaResponseFraming(t, w, data, false)
			}), closeErr, readErr, &closes)
			listed, err := LookupGCSObject(t.Context(), client, GCSObjectLookupRequest{Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText)})
			if err != nil {
				t.Fatal(err)
			}
			priorCalls := calls.Load()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			if tc.kind == 5 {
				cancel(cancelCause)
			}
			visits := uint64(0)
			got, err := VisitGCSUsageLog(ctx, client, GCSUsageLogRequest{Object: listed, Maximum: gcsProviderMaximum(t, uint64(len(data)))}, func(row GCSUsageRecord) error {
				visits++
				if tc.wantVisitor {
					return visitorCause
				}
				if tc.kind == 4 {
					cancel(cancelCause)
				}
				return row.Validate()
			})
			if visits != tc.wantVisits {
				t.Fatalf("provisional visits=%d,want %d", visits, tc.wantVisits)
			}
			if tc.kind == 0 {
				if err != nil || got.Validate() != nil {
					t.Fatalf("complete observation=%v/%v,want valid/nil", got, err)
				}
			} else {
				if got != (GCSUsageLogObservation{}) || err == nil {
					t.Fatalf("failed observation=%v/%v,want zero/error", got, err)
				}
				for _, cause := range []struct {
					want bool
					err  error
				}{{tc.wantClose, closeCause}, {tc.wantVisitor, visitorCause}, {tc.wantCancel, cancelCause}} {
					if cause.want && !errors.Is(err, cause.err) {
						t.Fatalf("error=%v,want retained cause %v", err, cause.err)
					}
				}
			}
			if tc.kind == 5 {
				if calls.Load() != priorCalls || closes.Load() != 0 {
					t.Fatalf("cancelled provider calls=%d closes=%d,want zero", calls.Load()-priorCalls, closes.Load())
				}
				return
			}
			wantCloses := int32(1)
			if tc.kind == 6 {
				wantCloses = 2
				if !errors.Is(err, core.ErrObjectStoreSource) {
					t.Fatalf("SDK reopen error=%v,want typed source refusal", err)
				}
			}
			if closes.Load() != wantCloses {
				t.Fatalf("native body closes=%d,want %d (including any SDK retry close)", closes.Load(), wantCloses)
			}
		})
	}
}

// Fault injection sits at the native HTTP response body, below the official GCS
// reader and its real CSV consumer; it does not replace an SDK result.
func usageLifetimeClient(t testing.TB, handler http.Handler, closeErr, readErr error, closes *atomic.Int32) *GCSClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	httpClient := boundedGCSTestHTTPClient(t, server.Client())
	httpClient.Transport = usageLifetimeTransport{base: httpClient.Transport, closeErr: closeErr, readErr: readErr, closes: closes}
	options := []option.ClientOption{option.WithEndpoint(server.URL + "/storage/v1/"), option.WithoutAuthentication(), option.WithHTTPClient(httpClient)}
	client, err := storage.NewClient(t.Context(), append(options, storage.WithJSONReads())...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	jsonClient, err := storageapi.NewService(t.Context(), options...)
	if err != nil {
		t.Fatal(err)
	}
	return &GCSClient{client: client, objects: jsonClient.Objects}
}

type usageLifetimeTransport struct {
	base              http.RoundTripper
	closeErr, readErr error
	closes            *atomic.Int32
}

func (t usageLifetimeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err == nil && request.URL.Query().Get("alt") == "media" {
		response.Body = usageLifetimeBody{ReadCloser: response.Body, closeErr: t.closeErr, readErr: t.readErr, closes: t.closes}
	}
	return response, err
}

type usageLifetimeBody struct {
	io.ReadCloser
	closeErr, readErr error
	closes            *atomic.Int32
}

func (b usageLifetimeBody) Read(data []byte) (int, error) {
	n, err := b.ReadCloser.Read(data)
	if err == io.EOF && b.readErr != nil {
		return n, errors.Join(err, b.readErr)
	}
	return n, err
}
func (b usageLifetimeBody) Close() error {
	b.closes.Add(1)
	return errors.Join(b.ReadCloser.Close(), b.closeErr)
}

func TestGCSUsageZeroObservationIsNotEvidence(t *testing.T) {
	t.Parallel()
	var got GCSUsageLogObservation
	metadata, metaErr := got.Metadata()
	count, countErr := got.RecordCount()
	if !errors.Is(got.Validate(), core.ErrObjectStoreContract) || !errors.Is(metaErr, core.ErrObjectStoreContract) || !errors.Is(countErr, core.ErrObjectStoreContract) || metadata != (GCSObjectMetadata{}) || count != 0 {
		t.Fatalf("zero observation metadata=%v/%v count=%d/%v,want no evidence", metadata, metaErr, count, countErr)
	}
}
