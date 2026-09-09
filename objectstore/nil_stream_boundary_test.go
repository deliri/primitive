package objectstore_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

func TestStreamRequestAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                              string
		source                            func() io.Reader
		destination                       func() io.Writer
		wantSourceErr, wantDestinationErr error
	}{
		{name: "nonempty native streams admit typed boundaries", source: func() io.Reader { return bytes.NewReader([]byte{1}) }, destination: func() io.Writer { return new(bytes.Buffer) }},
		{name: "empty native streams remain present capabilities", source: func() io.Reader { return bytes.NewReader(nil) }, destination: func() io.Writer { return new(bytes.Buffer) }},
		{name: "nil interfaces refuse before stream use", source: func() io.Reader { return nil }, destination: func() io.Writer { return nil }, wantSourceErr: core.ErrObjectStoreSource, wantDestinationErr: core.ErrObjectStoreDestination},
		{name: "typed nil pointers cannot cross either stream boundary", source: func() io.Reader { var r *bytes.Reader; return r }, destination: func() io.Writer { var w *bytes.Buffer; return w }, wantSourceErr: core.ErrObjectStoreSource, wantDestinationErr: core.ErrObjectStoreDestination},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := []byte{1}
			source := tc.source()
			destination := tc.destination()
			upload := uploadRequest(t, objectstore.ProviderGoogleCloudStorage, signedProviderURL(providerEndpoint(objectstore.ProviderGoogleCloudStorage), objectstore.ProviderGoogleCloudStorage, objectstore.DirectionUpload), payload)
			upload.Source = source
			download := downloadRequest(t, objectstore.ProviderGoogleCloudStorage, signedProviderURL(providerEndpoint(objectstore.ProviderGoogleCloudStorage), objectstore.ProviderGoogleCloudStorage, objectstore.DirectionDownload), destination, payload)
			maximum, err := core.NewByteCount(1)
			if err != nil {
				t.Fatalf("maximum error = %v, want nil", err)
			}
			inspection := objectstore.InspectionRequest{Source: source, MaximumBytes: maximum}
			for _, door := range []struct {
				name     string
				validate func() error
				wantErr  error
			}{
				{name: "upload", validate: upload.Validate, wantErr: tc.wantSourceErr},
				{name: "inspection", validate: inspection.Validate, wantErr: tc.wantSourceErr},
				{name: "download", validate: download.Validate, wantErr: tc.wantDestinationErr},
			} {
				gotErr := door.validate()
				if !errors.Is(gotErr, door.wantErr) || (door.wantErr != nil && !errors.Is(gotErr, core.ErrObjectStoreContract)) {
					t.Errorf("%s admission error = %v, want %v with Objectstore identity on refusal", door.name, gotErr, door.wantErr)
				}
			}
		})
	}
}

type refusedStreamTransport struct{ calls atomic.Int64 }

func (s *refusedStreamTransport) RoundTrip(*http.Request) (*http.Response, error) {
	s.calls.Add(1)
	return nil, io.ErrClosedPipe
}

func TestAbsentStreamsRefuseBeforeEffects(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		source      io.Reader
		destination io.Writer
	}{
		{name: "absent interfaces cannot initiate effects"},
		{name: "typed nil streams cannot initiate effects", source: (*bytes.Reader)(nil), destination: (*bytes.Buffer)(nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			transport := new(refusedStreamTransport)
			client := newObjectstoreClient(t, &http.Client{Transport: transport})
			provider := objectstore.ProviderGoogleCloudStorage
			payload := []byte{1}
			upload := uploadRequest(t, provider, signedProviderURL(providerEndpoint(provider), provider, objectstore.DirectionUpload), payload)
			upload.Source = tc.source
			download := downloadRequest(t, provider, signedProviderURL(providerEndpoint(provider), provider, objectstore.DirectionDownload), tc.destination, payload)
			for _, door := range []struct {
				name      string
				run       func() (objectstore.Transfer, error)
				want      error
				direction objectstore.Direction
			}{
				{name: "upload", run: func() (objectstore.Transfer, error) { return objectstore.UploadGCS(t.Context(), client, upload) }, want: core.ErrObjectStoreSource, direction: objectstore.DirectionUpload},
				{name: "download", run: func() (objectstore.Transfer, error) { return objectstore.DownloadGCS(t.Context(), client, download) }, want: core.ErrObjectStoreDestination, direction: objectstore.DirectionDownload},
			} {
				got, err := door.run()
				_, hasStatus := got.Status()
				if !errors.Is(err, door.want) || !errors.Is(err, core.ErrObjectStoreContract) || got.Commitment() != objectstore.CommitmentNotAttempted || got.Provider() != provider || got.Direction() != door.direction || got.Bytes().Uint64() != 0 || hasStatus {
					t.Errorf("%s refusal = (%v, %v), want exact unattempted transfer and %v", door.name, got, err, door.want)
				}
			}
			maximum, err := core.NewByteCount(1)
			if err != nil {
				t.Fatal(err)
			}
			got, err := objectstore.Inspect(t.Context(), objectstore.InspectionRequest{Source: tc.source, MaximumBytes: maximum})
			if got != (objectstore.Inspection{}) || !errors.Is(err, core.ErrObjectStoreSource) || !errors.Is(err, core.ErrObjectStoreContract) {
				t.Errorf("inspection = (%v, %v), want no proof and source contract refusal", got, err)
			}
			if calls := transport.calls.Load(); calls != 0 {
				t.Fatalf("HTTP calls = %d, want zero", calls)
			}
		})
	}
}
