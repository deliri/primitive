package objectstore_test

import (
	"bytes"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

type progressRecorder struct {
	failure   error
	completed uint64
	total     uint64
	direction objectstore.Direction
	calls     uint64
	mu        sync.Mutex
}

func (r *progressRecorder) observe(progress objectstore.TransferProgress) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := progress.Validate(); err != nil {
		r.failure = err
		return err
	}
	if progress.Completed().Uint64() <= r.completed && r.calls != 0 {
		r.failure = core.ErrObjectStoreContract
		return r.failure
	}
	r.completed = progress.Completed().Uint64()
	r.total = progress.Total().Uint64()
	r.direction = progress.Direction()
	r.calls++
	return nil
}

type progressRecord struct {
	failure   error
	completed uint64
	total     uint64
	direction objectstore.Direction
	calls     uint64
}

func (r *progressRecorder) record() progressRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return progressRecord{
		failure: r.failure, completed: r.completed, total: r.total,
		direction: r.direction, calls: r.calls,
	}
}

func TestCapabilityTransferProgressLayerTriadPositiveReportsExactStreamBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		size int
	}{
		{name: "one byte", size: 1},
		{name: "two bytes", size: 2},
		{name: "one below first stream boundary", size: 32<<10 - 1},
		{name: "at first stream boundary", size: 32 << 10},
		{name: "one above first stream boundary", size: 32<<10 + 1},
		{name: "one below second stream boundary", size: 64<<10 - 1},
		{name: "at second stream boundary", size: 64 << 10},
		{name: "one above second stream boundary", size: 64<<10 + 1},
		{name: "at third stream boundary", size: 96 << 10},
		{name: "one above third stream boundary", size: 96<<10 + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name+" upload", func(t *testing.T) {
			t.Parallel()
			payload := bytes.Repeat([]byte{0x5a}, tc.size)
			handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				_ = observeUpload(request)
				setProviderVersion(writer.Header(), objectstore.ProviderGoogleCloudStorage)
				writer.WriteHeader(http.StatusOK)
			})
			targetURL, client := providerServer(t, objectstore.ProviderGoogleCloudStorage, objectstore.DirectionUpload, handler)
			ordinary := uploadRequest(t, objectstore.ProviderGoogleCloudStorage, targetURL, payload)
			capability := receivedUploadCapability(t, objectstore.ProviderGoogleCloudStorage, ordinary.Target)
			recorder := &progressRecorder{}
			got, gotErr := objectstore.Upload(t.Context(), newObjectstoreClient(t, client), objectstore.UploadCapabilityRequest{
				Source: ordinary.Source, ContentType: ordinary.ContentType, Capability: capability,
				Integrity: ordinary.Integrity, Policy: ordinary.Policy, Observer: recorder.observe,
			})
			verifyProgressTransfer(t, progressTransferResult{
				Transfer: got, Err: gotErr, Record: recorder.record(),
				Direction: objectstore.DirectionUpload, Size: uint64(tc.size),
			})
		})

		t.Run(tc.name+" download", func(t *testing.T) {
			t.Parallel()
			payload := bytes.Repeat([]byte{0xa5}, tc.size)
			handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
				setProviderVersion(writer.Header(), objectstore.ProviderGoogleCloudStorage)
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write(payload)
			})
			targetURL, client := providerServer(t, objectstore.ProviderGoogleCloudStorage, objectstore.DirectionDownload, handler)
			var destination bytes.Buffer
			ordinary := downloadRequest(t, objectstore.ProviderGoogleCloudStorage, targetURL, &destination, payload)
			capability := receivedDownloadCapability(t, objectstore.ProviderGoogleCloudStorage, ordinary.Target)
			recorder := &progressRecorder{}
			got, gotErr := objectstore.Download(t.Context(), newObjectstoreClient(t, client), objectstore.DownloadCapabilityRequest{
				Destination: &destination, ContentType: ordinary.ContentType, Capability: capability,
				Integrity: ordinary.Integrity, Policy: ordinary.Policy, Observer: recorder.observe,
			})
			verifyProgressTransfer(t, progressTransferResult{
				Transfer: got, Err: gotErr, Record: recorder.record(),
				Direction: objectstore.DirectionDownload, Size: uint64(tc.size),
			})
			if !bytes.Equal(destination.Bytes(), payload) {
				t.Fatalf("download destination bytes = %d, want exact %d", destination.Len(), len(payload))
			}
		})
	}
}

type progressTransferResult struct {
	Err       error
	Record    progressRecord
	Transfer  objectstore.Transfer
	Size      uint64
	Direction objectstore.Direction
}

func verifyProgressTransfer(t *testing.T, result progressTransferResult) {
	t.Helper()
	if result.Err != nil || result.Transfer.Validate() != nil || result.Record.failure != nil ||
		result.Record.calls == 0 || result.Record.completed != result.Size ||
		result.Record.total != result.Size || result.Record.direction != result.Direction {
		t.Fatalf("progress transfer = (transfer %v, error %v, record %+v), want exact %v %d-byte completion",
			result.Transfer, result.Err, result.Record, result.Direction, result.Size)
	}
}

func TestCapabilityTransferProgressLayerTriadNegativeObserverFailureStopsEffect(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte{0x3c}, 64<<10)
	for _, direction := range []objectstore.Direction{objectstore.DirectionUpload, objectstore.DirectionDownload} {
		t.Run(direction.String(), func(t *testing.T) {
			t.Parallel()
			handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if direction == objectstore.DirectionUpload {
					_ = observeUpload(request)
				} else {
					writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
					writer.WriteHeader(http.StatusOK)
					_, _ = writer.Write(payload)
				}
			})
			targetURL, client := providerServer(t, objectstore.ProviderGoogleCloudStorage, direction, handler)
			observer := objectstore.ProgressObserver(func(objectstore.TransferProgress) error {
				return core.ErrObjectStoreContract
			})
			var gotErr error
			if direction == objectstore.DirectionUpload {
				ordinary := uploadRequest(t, objectstore.ProviderGoogleCloudStorage, targetURL, payload)
				_, gotErr = objectstore.UploadGCS(t.Context(), newObjectstoreClient(t, client), objectstore.UploadRequest{
					Source: ordinary.Source, Target: ordinary.Target, ContentType: ordinary.ContentType,
					Integrity: ordinary.Integrity, Policy: ordinary.Policy, Observer: observer,
				})
				if !errors.Is(gotErr, core.ErrObjectStoreSource) {
					t.Fatalf("upload observer refusal error = %v, want errors.Is %v", gotErr, core.ErrObjectStoreSource)
				}
				return
			}
			ordinary := downloadRequest(t, objectstore.ProviderGoogleCloudStorage, targetURL, bytes.NewBuffer(nil), payload)
			_, gotErr = objectstore.DownloadGCS(t.Context(), newObjectstoreClient(t, client), objectstore.DownloadRequest{
				Destination: ordinary.Destination, Target: ordinary.Target, ContentType: ordinary.ContentType,
				Integrity: ordinary.Integrity, Policy: ordinary.Policy, Observer: observer,
			})
			if !errors.Is(gotErr, core.ErrObjectStoreDestination) {
				t.Fatalf("download observer refusal error = %v, want errors.Is %v", gotErr, core.ErrObjectStoreDestination)
			}
		})
	}
}

func TestCapabilityTransferProgressLayerTriadNeutralAbsentObserverCompletes(t *testing.T) {
	t.Parallel()

	payload := []byte{0x7d}
	for _, direction := range []objectstore.Direction{objectstore.DirectionUpload, objectstore.DirectionDownload} {
		t.Run(direction.String(), func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if direction == objectstore.DirectionUpload {
					_ = observeUpload(request)
					setProviderVersion(writer.Header(), objectstore.ProviderGoogleCloudStorage)
					writer.WriteHeader(http.StatusOK)
					return
				}
				writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
				setProviderVersion(writer.Header(), objectstore.ProviderGoogleCloudStorage)
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write(payload)
			})
			targetURL, client := providerServer(t, objectstore.ProviderGoogleCloudStorage, direction, handler)
			if direction == objectstore.DirectionUpload {
				ordinary := uploadRequest(t, objectstore.ProviderGoogleCloudStorage, targetURL, payload)
				got, gotErr := objectstore.UploadGCS(t.Context(), newObjectstoreClient(t, client), objectstore.UploadRequest{
					Source: ordinary.Source, Target: ordinary.Target, ContentType: ordinary.ContentType,
					Integrity: ordinary.Integrity, Policy: ordinary.Policy,
				})
				if gotErr != nil || got.Validate() != nil {
					t.Fatalf("observer-absent UploadGCS() = (%v, %v), want confirmed transfer and nil", got, gotErr)
				}
				return
			}
			var destination bytes.Buffer
			ordinary := downloadRequest(t, objectstore.ProviderGoogleCloudStorage, targetURL, &destination, payload)
			got, gotErr := objectstore.DownloadGCS(t.Context(), newObjectstoreClient(t, client), objectstore.DownloadRequest{
				Destination: &destination, Target: ordinary.Target, ContentType: ordinary.ContentType,
				Integrity: ordinary.Integrity, Policy: ordinary.Policy,
			})
			if gotErr != nil || got.Validate() != nil || !bytes.Equal(destination.Bytes(), payload) {
				t.Fatalf("observer-absent DownloadGCS() = (%v, %v, %x), want confirmed transfer, nil, and %x",
					got, gotErr, destination.Bytes(), payload)
			}
		})
	}
}
