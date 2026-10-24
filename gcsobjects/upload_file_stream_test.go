package gcsobjects

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/background"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
	storageapi "google.golang.org/api/storage/v1"
)

type streamProgressBody struct {
	body     io.ReadCloser
	read     int
	progress chan struct{}
	notified bool
}

func (r *streamProgressBody) Read(data []byte) (int, error) {
	n, err := r.body.Read(data)
	r.read += n
	if r.read >= gcsFileStreamBufferBytes && !r.notified {
		r.notified = true
		close(r.progress)
	}
	return n, err
}

func (r *streamProgressBody) Close() error { return r.body.Close() }

type streamEOFLoan struct {
	ctx    context.Context
	sealed <-chan struct{}
}

func (r streamEOFLoan) Read([]byte) (int, error) {
	select {
	case <-r.sealed:
		return 0, io.EOF
	case <-r.ctx.Done():
		return 0, context.Cause(r.ctx)
	}
}

// Provider bytes must arrive while the producer still owns its final EOF.
// A buffered implementation that waits for the complete source deadlocks this
// causal handshake and fails its backstop; elapsed time is not the oracle.
func TestGCSFileStreamSendsBeforeProducerEOF(t *testing.T) {
	t.Parallel()
	duration, err := temporal.DurationFromSeconds(30)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	progress := make(chan struct{})
	sealed := make(chan struct{})
	payload := bytes.Repeat([]byte{'s'}, 2*gcsFileStreamBufferBytes)
	client := bucketTestClient(t, http.HandlerFunc(func(out http.ResponseWriter, request *http.Request) {
		request.Body = &streamProgressBody{body: request.Body, progress: progress}
		metadata, integrity, err := receiveGCSMultipart(request)
		if err != nil {
			out.WriteHeader(http.StatusBadRequest)
			return
		}
		writeGCSObjectResponse(gcsObjectResponse{t: t, writer: out, metadata: metadata, integrity: integrity})
	}))
	request := GCSFileStreamUpload{
		Source: io.MultiReader(bytes.NewReader(payload), streamEOFLoan{ctx: ctx, sealed: sealed}),
		Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText),
		CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos),
	}
	var got GCSFileStreamResult
	task, err := background.StartTask(background.TaskRequest{Parent: ctx, Work: background.Work{
		Run: func(ctx context.Context) error {
			var err error
			got, err = UploadFileStream(ctx, client, request)
			return err
		},
		Complete: func(error) {},
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		if _, err := task.Close(); err != nil {
			t.Errorf("stream task join=%v, want nil", err)
		}
	})
	select {
	case <-progress:
		close(sealed)
	case <-ctx.Done():
		t.Fatalf("provider progress=%v before producer EOF, want streamed bytes", context.Cause(ctx))
	}
	result, err := task.Wait(ctx)
	if err != nil || result.ActionError != nil || result.CompletionError != nil {
		t.Fatalf("stream task result=%+v/%v, want completed upload and joined observer", result, err)
	}
	integrity, err := got.Integrity()
	if err != nil || integrity != gcsProviderIntegrity(t, payload, payload) {
		t.Fatalf("streamed integrity=%v/%v, want exact bytes after producer seal", integrity, err)
	}
}

// streamTestReader makes EOF/data/error ordering observable without a seek
// method, a declared length, a retry buffer or a second source pass.
type streamTestReader struct {
	data       []byte
	terminal   error
	step       int
	emptyFirst bool
	ended      bool
}

func (r *streamTestReader) Read(out []byte) (int, error) {
	if r.emptyFirst {
		r.emptyFirst = false
		return 0, nil
	}
	if len(r.data) == 0 {
		r.ended = true
		if r.terminal != nil {
			return 0, r.terminal
		}
		return 0, io.EOF
	}
	if r.step > 0 && len(out) > r.step {
		out = out[:r.step]
	}
	n := copy(out, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 && r.terminal != nil {
		r.ended = true
		return n, r.terminal
	}
	return n, nil
}

func TestGCSFileStreamUploadLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		payload   []byte
		step      int
		terminal  error
		emptyRead bool
		status    int
		alter     func(*storageapi.Object)
		wantErr   error
	}{
		{name: "empty stream earns a verified zero extent"},
		{name: "one byte is neither lost to peek nor duplicated", payload: []byte{'a'}},
		{name: "last byte before read buffer boundary", payload: bytes.Repeat([]byte{'b'}, gcsFileStreamBufferBytes-1)},
		{name: "exact read buffer boundary", payload: bytes.Repeat([]byte{'c'}, gcsFileStreamBufferBytes)},
		{name: "first byte after read buffer boundary", payload: bytes.Repeat([]byte{'d'}, gcsFileStreamBufferBytes+1)},
		{name: "two complete read buffers", payload: bytes.Repeat([]byte{'e'}, 2*gcsFileStreamBufferBytes)},
		{name: "one-byte reads preserve the complete stream", payload: []byte("fragmented stream"), step: 1},
		{name: "data with terminal EOF is retained once", payload: []byte("terminal payload"), terminal: io.EOF},
		{name: "zero bytes are data", payload: []byte{0, 1, 0, 255}},
		{name: "one empty nonterminal read creates no byte", payload: []byte("after empty read"), emptyRead: true},
		{name: "source error before first byte produces no result", terminal: io.ErrUnexpectedEOF, wantErr: core.ErrObjectStoreSource},
		{name: "source error after bytes cannot commit a prefix", payload: []byte("partial"), terminal: io.ErrUnexpectedEOF, wantErr: core.ErrObjectStoreSource},
		{name: "wrapped EOF is a refusal not an empty success", terminal: errors.Join(io.EOF, io.ErrUnexpectedEOF), wantErr: io.ErrUnexpectedEOF},
		{name: "create-only conflict never overwrites", payload: []byte("conflict"), status: http.StatusPreconditionFailed, wantErr: core.ErrObjectStoreConflict},
		{name: "provider checksum disagreement produces no proof", payload: []byte("crc"), alter: func(o *storageapi.Object) { o.Crc32c = "AAAAAA==" }, wantErr: core.ErrObjectStoreDestination},
		{name: "provider extent disagreement produces no proof", payload: []byte("extent"), alter: func(o *storageapi.Object) { o.Size++ }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "foreign bucket cannot own this upload", payload: []byte("bucket"), alter: func(o *storageapi.Object) { o.Bucket = "another-valid-bucket" }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "foreign object cannot own this upload", payload: []byte("name"), alter: func(o *storageapi.Object) { o.Name = "another/object" }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "served media cannot impersonate stored bytes", payload: []byte("type"), alter: func(o *storageapi.Object) { o.ContentType = "text/plain" }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "missing generation cannot produce provider proof", payload: []byte("generation"), alter: func(o *storageapi.Object) { o.Generation = 0 }, wantErr: core.ErrObjectStoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			observed := make(chan objectstore.Integrity, 1)
			client := bucketTestClient(t, http.HandlerFunc(func(out http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodPost || request.URL.Query().Get("ifGenerationMatch") != "0" {
					t.Errorf("upload request=%s %s, want create-only POST", request.Method, request.URL)
					out.WriteHeader(http.StatusBadRequest)
					return
				}
				metadata, integrity, err := receiveGCSMultipart(request)
				if err != nil {
					if tc.wantErr == nil {
						t.Errorf("complete multipart upload error=%v, want nil", err)
					}
					out.WriteHeader(http.StatusBadRequest)
					return
				}
				if metadata.Name != gcsProviderObjectText || metadata.ContentType != gcsProviderMediaTypeText || metadata.CacheControl != "" || metadata.Crc32c != "" {
					t.Errorf("stream upload metadata=%+v, want private stored bytes with integrity observed at EOF", metadata)
				}
				select {
				case observed <- integrity:
				default:
					t.Errorf("provider completion queue already holds %d results, want no earlier upload", len(observed))
				}
				if tc.status != 0 {
					out.WriteHeader(tc.status)
					return
				}
				stamp, stampErr := temporal.InstantFromNanoseconds(gcsProviderInstantNanos).RFC3339Nano()
				if stampErr != nil {
					t.Error(stampErr)
					out.WriteHeader(http.StatusInternalServerError)
					return
				}
				checksum, checksumErr := integrity.CRC32C.Base64()
				if checksumErr != nil {
					t.Error(checksumErr)
					out.WriteHeader(http.StatusInternalServerError)
					return
				}
				metadata.Bucket, metadata.Generation = gcsProviderBucketText, gcsProviderGeneration
				metadata.Size, metadata.Crc32c = integrity.Length.Uint64(), checksum
				metadata.TimeCreated, metadata.Updated = stamp, stamp
				if tc.alter != nil {
					tc.alter(&metadata)
				}
				writeProviderJSON(t, out, metadata)
			}))
			source := &streamTestReader{data: bytes.Clone(tc.payload), terminal: tc.terminal, step: tc.step, emptyFirst: tc.emptyRead}
			request := GCSFileStreamUpload{Source: source, Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos)}
			got, err := UploadFileStream(t.Context(), client, request)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (GCSFileStreamResult{}) {
					t.Fatalf("stream upload=%+v/%v, want zero result and %v", got, err, tc.wantErr)
				}
				return
			}
			metadata, metadataErr := got.Metadata()
			integrity, integrityErr := got.Integrity()
			want := gcsProviderIntegrity(t, tc.payload, tc.payload)
			if err != nil || got.Validate() != nil || metadataErr != nil || integrityErr != nil || integrity != want || metadata.Length() != want.Length || metadata.CRC32C() != want.CRC32C || !source.ended {
				t.Fatalf("stream upload=%+v/%v, metadata=%v, integrity=%v/%v, ended=%t, want exact %v after source EOF", got, err, metadataErr, integrity, integrityErr, source.ended, want)
			}
			select {
			case provider := <-observed:
				if provider != want {
					t.Fatalf("provider received=%v, want %v", provider, want)
				}
			default:
				t.Fatalf("provider completion queue length=%d, want one completed object", len(observed))
			}
		})
	}
}

func TestGCSFileStreamRequestRefusesAbsentOwners(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(*GCSFileStreamUpload)
	}{
		{name: "missing reader", change: func(r *GCSFileStreamUpload) { r.Source = nil }},
		{name: "typed nil reader", change: func(r *GCSFileStreamUpload) { r.Source = (*strings.Reader)(nil) }},
		{name: "missing bucket", change: func(r *GCSFileStreamUpload) { r.Bucket = GCSBucket{} }},
		{name: "missing object", change: func(r *GCSFileStreamUpload) { r.Name = GCSObjectName{} }},
		{name: "missing time", change: func(r *GCSFileStreamUpload) { r.CustomTime = temporal.Instant{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := GCSFileStreamUpload{Source: strings.NewReader("source"), Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos)}
			tc.change(&request)
			if err := request.Validate(); !errors.Is(err, core.ErrObjectStoreContract) {
				t.Fatalf("request validation=%v, want %v", err, core.ErrObjectStoreContract)
			}
		})
	}
}

func TestGCSStreamDestinationNativeLimitLayerTriad(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 2, 3} {
		var destination bytes.Buffer
		limited := gcsStreamDestination{destination: &destination, remaining: 2}
		data := bytes.Repeat([]byte{'x'}, size)
		n, err := limited.Write(data)
		if size > 2 {
			if !errors.Is(err, core.ErrObjectStoreSize) || n != 0 || destination.Len() != 0 || limited.remaining != 2 {
				t.Fatalf("oversized write=%d/%v bytes=%d remaining=%d, want no effect and size refusal", n, err, destination.Len(), limited.remaining)
			}
		} else if err != nil || n != size || !bytes.Equal(destination.Bytes(), data) || limited.remaining != uint64(2-size) {
			t.Fatalf("bounded write=%d/%v bytes=%q remaining=%d, want %d/nil and exact bytes", n, err, destination.Bytes(), limited.remaining, size)
		}
	}
	var zero GCSFileStreamResult
	if _, err := zero.Metadata(); !errors.Is(err, core.ErrObjectStoreContract) {
		t.Fatalf("zero result metadata=%v, want contract refusal", err)
	}
	if _, err := zero.Integrity(); !errors.Is(err, core.ErrObjectStoreContract) {
		t.Fatalf("zero result integrity=%v, want contract refusal", err)
	}
}

// This is the local sealed-result schema triad. The separate SDK tests prove
// the producer path; these one-fact mutations challenge both evidence readers.
func TestGCSFileStreamResultSchemaLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		change  func(*GCSFileStreamResult)
		wantErr error
	}{
		{name: "matching provider and stream facts are readable"},
		{name: "unreleased result has no readable proof", change: func(r *GCSFileStreamResult) { r.set = false }, wantErr: core.ErrObjectStoreContract},
		{name: "changed local extent contradicts provider", change: func(r *GCSFileStreamResult) {
			r.integrity.Length, _ = core.NewByteLength(r.integrity.Length.Uint64() + 1)
		}, wantErr: core.ErrObjectStoreIntegrity},
		{name: "changed local checksum contradicts provider", change: func(r *GCSFileStreamResult) {
			checksum, _ := r.integrity.CRC32C.Uint32()
			r.integrity.CRC32C = core.NewCRC32C(checksum ^ 1)
		}, wantErr: core.ErrObjectStoreIntegrity},
		{name: "absent local digest cannot release metadata", change: func(r *GCSFileStreamResult) { r.integrity.SHA256 = core.SHA256Digest{} }, wantErr: core.ErrObjectStoreContract},
		{name: "absent generation cannot release stream facts", change: func(r *GCSFileStreamResult) { r.metadata.generation = GCSGeneration{} }, wantErr: core.ErrObjectStoreContract},
		{name: "zero result preserves absence", change: func(r *GCSFileStreamResult) { *r = GCSFileStreamResult{} }, wantErr: core.ErrObjectStoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			integrity := gcsProviderIntegrity(t, []byte("schema"), []byte("schema"))
			stamp := temporal.InstantFromNanoseconds(gcsProviderInstantNanos)
			base := GCSFileStreamResult{
				metadata: GCSObjectMetadata{
					bucket: parsedGCSBucket(t, gcsProviderBucketText), name: parsedGCSObjectName(t, gcsProviderObjectText),
					contentType: core.HTTPMediaTypeOctetStream(), generation: gcsReadGeneration(t),
					length: integrity.Length, crc32c: integrity.CRC32C, createdAt: stamp, updatedAt: stamp,
				},
				integrity: integrity, set: true,
			}
			got := base
			if tc.change != nil {
				tc.change(&got)
				if got == base {
					t.Fatalf("schema mutation=%+v, want a different value from %+v", got, base)
				}
			}
			metadata, metadataErr := got.Metadata()
			observed, integrityErr := got.Integrity()
			if tc.wantErr != nil {
				if !errors.Is(got.Validate(), tc.wantErr) || !errors.Is(metadataErr, tc.wantErr) || !errors.Is(integrityErr, tc.wantErr) || metadata != (GCSObjectMetadata{}) || observed != (objectstore.Integrity{}) {
					t.Fatalf("mutated result readers=%v/%v, %v/%v, want zero facts and %v", metadata, metadataErr, observed, integrityErr, tc.wantErr)
				}
			} else if got.Validate() != nil || metadataErr != nil || integrityErr != nil || metadata != base.metadata || observed != integrity {
				t.Fatalf("sealed result readers=%v/%v, %v/%v, want exact validated facts", metadata, metadataErr, observed, integrityErr)
			}
		})
	}
}

func TestGCSFileStreamCanceledBeforeReadMakesNoRequest(t *testing.T) {
	t.Parallel()
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel(nil)
	cancel(context.Canceled)
	client := bucketTestClient(t, http.HandlerFunc(func(out http.ResponseWriter, request *http.Request) {
		t.Errorf("provider received canceled request %s, want no request", request.URL)
		out.WriteHeader(http.StatusBadRequest)
	}))
	payload := []byte("unread")
	source := &streamTestReader{data: bytes.Clone(payload)}
	got, err := UploadFileStream(ctx, client, GCSFileStreamUpload{Source: source, Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos)})
	if !errors.Is(err, context.Canceled) || got != (GCSFileStreamResult{}) || !bytes.Equal(source.data, payload) || source.ended {
		t.Fatalf("canceled upload=%v/%v, remaining=%q ended=%t, want zero proof and unread source", got, err, source.data, source.ended)
	}
}

func TestGCSFileStreamCancellationAfterProviderProgressJoins(t *testing.T) {
	t.Parallel()
	duration, err := temporal.DurationFromSeconds(30)
	if err != nil {
		t.Fatal(err)
	}
	parent, endBackstop, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
	if err != nil {
		t.Fatal(err)
	}
	defer endBackstop()
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: parent})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel(nil)
	progress := make(chan struct{})
	neverSealed := make(chan struct{})
	client := bucketTestClient(t, http.HandlerFunc(func(out http.ResponseWriter, request *http.Request) {
		request.Body = &streamProgressBody{body: request.Body, progress: progress}
		metadata, integrity, err := receiveGCSMultipart(request)
		if err != nil {
			out.WriteHeader(http.StatusBadRequest)
			return
		}
		writeGCSObjectResponse(gcsObjectResponse{t: t, writer: out, metadata: metadata, integrity: integrity})
	}))
	request := GCSFileStreamUpload{
		Source: io.MultiReader(bytes.NewReader(bytes.Repeat([]byte{'c'}, 2*gcsFileStreamBufferBytes)), streamEOFLoan{ctx: ctx, sealed: neverSealed}),
		Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText),
		CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos),
	}
	var got GCSFileStreamResult
	task, err := background.StartTask(background.TaskRequest{Parent: ctx, Work: background.Work{
		Run: func(ctx context.Context) error {
			var err error
			got, err = UploadFileStream(ctx, client, request)
			return err
		},
		Complete: func(error) {},
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel(context.Canceled)
		if _, err := task.Close(); err != nil {
			t.Errorf("canceled stream task join=%v, want nil", err)
		}
	})
	select {
	case <-progress:
		cancel(context.Canceled)
	case <-parent.Done():
		t.Fatalf("provider progress=%v, want bytes before cancellation", context.Cause(parent))
	}
	result, err := task.Wait(parent)
	if err != nil || !errors.Is(result.ActionError, context.Canceled) || result.CompletionError != nil || got != (GCSFileStreamResult{}) {
		t.Fatalf("canceled stream=%v, task=%+v/%v, want joined native cancellation and no proof", got, result, err)
	}
}
