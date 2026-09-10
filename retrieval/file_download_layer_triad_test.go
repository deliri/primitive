package retrieval

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

type retrievalRoundTrip func(*http.Request) (*http.Response, error)

func (f retrievalRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestVerifiedGrantDownloadFileLayerTriadPositiveStreamsAndActivates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		size int
	}{
		{name: "one byte", size: 1},
		{name: "at first stream chunk", size: 32 << 10},
		{name: "one above first stream chunk", size: 32<<10 + 1},
		{name: "many windows", size: 4<<20 + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			payload := bytes.Repeat([]byte{0x6a}, tc.size)
			fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: payload})
			root := retrievalFileRoot(t, directory)
			var completed uint64
			observations := 0
			requests := 0
			request := FileDownloadRequest{
				Client: retrievalObjectstoreClientForResponse(t, retrievalHTTPResponse{payload: payload, onRequest: func() { requests++ }}), Policy: fixture.policy,
				Activation: retrievalActivation(t, retrievalActivationRequest{
					Root: root, Size: uint64(tc.size), Install: filestore.InstallCreate,
				}),
				Observer: func(progress objectstore.TransferProgress) error {
					next := progress.Completed().Uint64()
					if progress.Validate() != nil || progress.Direction() != objectstore.DirectionDownload ||
						progress.Total().Uint64() != uint64(tc.size) || next <= completed {
						return core.ErrObjectStoreContract
					}
					completed = next
					observations++
					return nil
				},
			}
			recovery, transfer, err := fixture.grant.DownloadFile(t.Context(), request)
			evidence := fixture.addition.Entry.Evidence.Payload.Body
			if err != nil || recovery.Validate() == nil || transfer.Validate() != nil ||
				transfer.Provider() != objectstore.ProviderGoogleCloudStorage ||
				transfer.Direction() != objectstore.DirectionDownload ||
				transfer.Bytes() != evidence.Extent || transfer.SHA256() != evidence.SHA256 ||
				transfer.CRC32C() != evidence.CRC32C || completed != uint64(tc.size) || observations == 0 {
				t.Fatalf("VerifiedGrant.DownloadFile(%d) = (recovery %v, transfer %v, error %v, progress %d/%d), want exact authenticated GCS download",
					tc.size, recovery, transfer, err, completed, observations)
			}
			got := readRetrievalTarget(t, root, "target")
			if !bytes.Equal(got, payload) {
				t.Fatalf("activated target bytes = %d, want exact %d", len(got), len(payload))
			}
			info, gotStatErr := retrievalTargetStat(t, root, "target")
			if gotStatErr != nil || info.Size() != int64(tc.size) || info.Mode().Perm() != 0o600 {
				t.Fatalf("activated target Stat() = (%v, %v), want size %d and mode 0600", info, gotStatErr, tc.size)
			}
		})
	}
}

func TestVerifiedGrantDownloadFileLayerTriadNegativePreservesPriorTarget(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	payload := bytes.Repeat([]byte{0x7b}, 64<<10+1)
	fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: payload})
	root := retrievalFileRoot(t, directory)
	prior := []byte("customer-prior-version")
	writeRetrievalTarget(t, root, "target", prior)
	truncated := payload[:len(payload)-1]
	request := FileDownloadRequest{
		Client: retrievalObjectstoreClient(t, truncated), Policy: fixture.policy,
		Activation: retrievalActivation(t, retrievalActivationRequest{
			Root: root, Size: uint64(len(payload)), Install: filestore.InstallReplace,
		}),
	}
	recovery, transfer, err := fixture.grant.DownloadFile(t.Context(), request)
	if !errors.Is(err, core.ErrObjectStoreIntegrity) ||
		!errors.Is(recovery.Validate(), core.ErrFilestoreContract) ||
		transfer.Commitment() != objectstore.CommitmentRejected ||
		transfer.Validate() == nil {
		t.Fatalf(
			"truncated DownloadFile() = (recovery %v, transfer %v, error %v), want zero recovery, rejected transfer, and errors.Is %v",
			recovery,
			transfer,
			err,
			core.ErrObjectStoreIntegrity,
		)
	}
	if got := readRetrievalTarget(t, root, "target"); !bytes.Equal(got, prior) {
		t.Fatalf("target after failed download = %q, want preserved %q", got, prior)
	}
	if _, statErr := retrievalTargetStat(t, root, ".download-stage"); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("temporary after failed download Stat() error = %v, want errors.Is %v", statErr, fs.ErrNotExist)
	}
}

func TestVerifiedGrantDownloadFileLayerTriadDeterminateConflictDiscardsStage(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	payload := bytes.Repeat([]byte{0x2d}, 64<<10+1)
	fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: payload})
	root := retrievalFileRoot(t, directory)
	prior := []byte("customer-prior-version")
	writeRetrievalTarget(t, root, "target", prior)
	request := FileDownloadRequest{
		Client: retrievalObjectstoreClient(t, payload), Policy: fixture.policy,
		Activation: retrievalActivation(t, retrievalActivationRequest{
			Root: root, Size: uint64(len(payload)), Install: filestore.InstallCreate,
		}),
	}
	recovery, transfer, gotErr := fixture.grant.DownloadFile(t.Context(), request)
	if !errors.Is(gotErr, core.ErrFilestoreConflict) || recovery.Validate() == nil || transfer.Validate() != nil {
		t.Fatalf("create-only conflict DownloadFile() = (recovery %v, transfer %v, error %v), want zero recovery, confirmed transfer, and errors.Is %v",
			recovery, transfer, gotErr, core.ErrFilestoreConflict)
	}
	if got := readRetrievalTarget(t, root, "target"); !bytes.Equal(got, prior) {
		t.Fatalf("target after create-only conflict = %q, want preserved %q", got, prior)
	}
	if _, gotStatErr := retrievalTargetStat(t, root, ".download-stage"); !errors.Is(gotStatErr, fs.ErrNotExist) {
		t.Fatalf("temporary after determinate conflict Stat() error = %v, want errors.Is %v", gotStatErr, fs.ErrNotExist)
	}
}

func TestVerifiedGrantDownloadFileLayerTriadFailureMatrixPreservesPriorTarget(t *testing.T) {
	t.Parallel()

	original := bytes.Repeat([]byte{0x53}, 64<<10+1)
	altered := bytes.Repeat([]byte{0x54}, len(original))
	cases := []struct {
		wantErr         error
		observerErr     error
		name            string
		response        retrievalHTTPResponse
		cancelBefore    bool
		cancelDuring    bool
		install         filestore.InstallMode
		wantTransferred bool
	}{
		{name: "one byte short body", response: retrievalHTTPResponse{payload: original[:len(original)-1]}, wantErr: core.ErrObjectStoreIntegrity, install: filestore.InstallReplace},
		{name: "one byte padded body", response: retrievalHTTPResponse{payload: append(append([]byte(nil), original...), 0x55)}, wantErr: core.ErrObjectStoreIntegrity, install: filestore.InstallReplace},
		{name: "same extent different SHA-256 and CRC32C", response: retrievalHTTPResponse{payload: altered}, wantErr: core.ErrObjectStoreIntegrity, install: filestore.InstallReplace},
		{name: "provider reports absent object", response: retrievalHTTPResponse{status: http.StatusNotFound}, wantErr: core.ErrObjectStoreAbsent, install: filestore.InstallReplace},
		{name: "provider reports internal failure", response: retrievalHTTPResponse{status: http.StatusInternalServerError}, wantErr: core.ErrExchangeResponse, install: filestore.InstallReplace},
		{name: "provider omits promised content type", response: retrievalHTTPResponse{payload: original, omitContentType: true}, wantErr: core.ErrExchangeContentType, install: filestore.InstallReplace},
		{name: "transport refuses before response", response: retrievalHTTPResponse{transportErr: io.ErrClosedPipe}, wantErr: core.ErrExchangeTransport, install: filestore.InstallReplace},
		{name: "body close fails after exact authenticated bytes", response: retrievalHTTPResponse{payload: original, closeErr: io.ErrClosedPipe}, wantErr: io.ErrClosedPipe, install: filestore.InstallReplace},
		{name: "response stream fails after partial bytes", response: retrievalHTTPResponse{payload: original[:len(original)/2], bodyErr: io.ErrUnexpectedEOF}, wantErr: core.ErrExchangeResponse, install: filestore.InstallReplace},
		{name: "progress observer refuses streamed bytes", response: retrievalHTTPResponse{payload: original}, observerErr: io.ErrClosedPipe, wantErr: core.ErrObjectStoreDestination, install: filestore.InstallReplace},
		{name: "operation context is cancelled before staging", response: retrievalHTTPResponse{payload: original}, cancelBefore: true, wantErr: context.Canceled, install: filestore.InstallReplace},
		{name: "operation context is cancelled during streaming", response: retrievalHTTPResponse{payload: original, observeContext: true}, cancelDuring: true, wantErr: core.ErrExchangeCancelled, install: filestore.InstallReplace},
		{name: "redirect response cannot move the bearer", response: retrievalHTTPResponse{status: http.StatusTemporaryRedirect}, wantErr: core.ErrExchangeResponse, install: filestore.InstallReplace},
		{name: "create-only target already exists", response: retrievalHTTPResponse{payload: original}, wantErr: core.ErrFilestoreConflict, install: filestore.InstallCreate, wantTransferred: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: original})
			root := retrievalFileRoot(t, directory)
			prior := []byte("customer-prior-version")
			writeRetrievalTarget(t, root, "target", prior)
			ctx := t.Context()
			cancel := func() {}
			if tc.cancelBefore || tc.cancelDuring {
				ctx, cancel = context.WithCancel(ctx)
			}
			defer cancel()
			request := FileDownloadRequest{
				Client: retrievalObjectstoreClientForResponse(t, tc.response), Policy: fixture.policy,
				Activation: retrievalActivation(t, retrievalActivationRequest{
					Root: root, Size: uint64(len(original)), Install: tc.install,
				}),
			}
			if tc.observerErr != nil {
				request.Observer = func(objectstore.TransferProgress) error { return tc.observerErr }
			}
			if tc.cancelBefore {
				cancel()
			}
			if tc.cancelDuring {
				request.Observer = func(objectstore.TransferProgress) error {
					cancel()
					return nil
				}
			}
			recovery, transfer, gotErr := fixture.grant.DownloadFile(ctx, request)
			if !errors.Is(gotErr, tc.wantErr) || recovery.Validate() == nil {
				t.Fatalf("DownloadFile(%s) = (recovery %v, transfer %v, error %v), want zero recovery and errors.Is %v",
					tc.name, recovery, transfer, gotErr, tc.wantErr)
			}
			gotTransferred := transfer.Validate() == nil
			if gotTransferred != tc.wantTransferred {
				t.Fatalf("DownloadFile(%s) transfer confirmed = %t, want %t", tc.name, gotTransferred, tc.wantTransferred)
			}
			if got := readRetrievalTarget(t, root, "target"); !bytes.Equal(got, prior) {
				t.Fatalf("target after %s = %q, want preserved %q", tc.name, got, prior)
			}
			if _, gotStatErr := retrievalTargetStat(t, root, ".download-stage"); !errors.Is(gotStatErr, fs.ErrNotExist) {
				t.Fatalf("temporary after %s Stat() error = %v, want errors.Is %v", tc.name, gotStatErr, fs.ErrNotExist)
			}
		})
	}
}

func FuzzVerifiedGrantDownloadFileExternalBodyAtomicity(f *testing.F) {
	expected := []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80}
	fixture := newDownloadCallFixture(f, downloadCallFixtureRequest{Payload: expected})
	f.Add(expected)
	f.Add([]byte{})
	f.Add(expected[:len(expected)-1])
	f.Add(append(append([]byte(nil), expected...), 0x90))
	f.Add([]byte{0x11, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80})

	f.Fuzz(func(t *testing.T, body []byte) {
		directory := t.TempDir()
		root := retrievalFileRoot(t, directory)
		prior := []byte("customer-prior-version")
		writeRetrievalTarget(t, root, "target", prior)
		request := FileDownloadRequest{
			Client: retrievalObjectstoreClientForResponse(t, retrievalHTTPResponse{payload: body}),
			Policy: fixture.policy,
			Activation: retrievalActivation(t, retrievalActivationRequest{
				Root: root, Size: uint64(len(expected)), Install: filestore.InstallReplace,
			}),
		}
		recovery, transfer, gotErr := fixture.grant.DownloadFile(t.Context(), request)
		if bytes.Equal(body, expected) {
			if gotErr != nil || recovery.Validate() == nil || transfer.Validate() != nil {
				t.Fatalf("DownloadFile(exact external body) = (recovery %v, transfer %v, error %v), want exact success", recovery, transfer, gotErr)
			}
			if got := readRetrievalTarget(t, root, "target"); !bytes.Equal(got, expected) {
				t.Fatalf("activated exact external body = %x, want %x", got, expected)
			}
		} else {
			if !errors.Is(gotErr, core.ErrObjectStoreIntegrity) || recovery.Validate() == nil || transfer.Validate() == nil {
				t.Fatalf("DownloadFile(foreign external body %d bytes) = (recovery %v, transfer %v, error %v), want zero results and errors.Is %v",
					len(body), recovery, transfer, gotErr, core.ErrObjectStoreIntegrity)
			}
			if got := readRetrievalTarget(t, root, "target"); !bytes.Equal(got, prior) {
				t.Fatalf("target after foreign external body = %q, want preserved %q", got, prior)
			}
		}
		if _, gotStatErr := retrievalTargetStat(t, root, ".download-stage"); !errors.Is(gotStatErr, fs.ErrNotExist) {
			t.Fatalf("temporary after external body Stat() error = %v, want errors.Is %v", gotStatErr, fs.ErrNotExist)
		}
	})
}

func TestVerifiedGrantDownloadFileLayerTriadNeutralAbsentObserverLeavesNoStage(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	payload := []byte{0x01}
	fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: payload})
	root := retrievalFileRoot(t, directory)
	request := FileDownloadRequest{
		Client: retrievalObjectstoreClient(t, payload), Policy: fixture.policy,
		Activation: retrievalActivation(t, retrievalActivationRequest{
			Root: root, Size: uint64(len(payload)), Install: filestore.InstallCreate,
		}),
	}
	recovery, transfer, gotErr := fixture.grant.DownloadFile(t.Context(), request)
	if gotErr != nil || recovery.Validate() == nil || transfer.Validate() != nil {
		t.Fatalf("observer-absent DownloadFile() = (recovery %v, transfer %v, error %v), want zero recovery, confirmed transfer, and nil",
			recovery, transfer, gotErr)
	}
	if got := readRetrievalTarget(t, root, "target"); !bytes.Equal(got, payload) {
		t.Fatalf("observer-absent activated target = %v, want %v", got, payload)
	}
	if _, gotStatErr := retrievalTargetStat(t, root, ".download-stage"); !errors.Is(gotStatErr, fs.ErrNotExist) {
		t.Fatalf("observer-absent temporary Stat() error = %v, want errors.Is %v", gotStatErr, fs.ErrNotExist)
	}
}

type fileDownloadMutation uint8

const (
	fileDownloadZeroRequest fileDownloadMutation = iota
	fileDownloadZeroClient
	fileDownloadZeroActivation
	fileDownloadMissingRoot
	fileDownloadRootTemporary
	fileDownloadRootTarget
	fileDownloadExtentMismatch
	fileDownloadZeroPolicy
	fileDownloadZeroOperation
	fileDownloadZeroAttempt
	fileDownloadZeroGrant
)

func TestVerifiedGrantDownloadFileLayerTriadIngressControlsFilesystemEffects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr  error
		name     string
		mutation fileDownloadMutation
	}{
		{name: "zero request", mutation: fileDownloadZeroRequest, wantErr: core.ErrRetrievalContract},
		{name: "zero objectstore client", mutation: fileDownloadZeroClient, wantErr: core.ErrRetrievalContract},
		{name: "zero activation request", mutation: fileDownloadZeroActivation, wantErr: core.ErrRetrievalContract},
		{name: "temporary has no root capability", mutation: fileDownloadMissingRoot, wantErr: core.ErrRetrievalContract},
		{name: "temporary names root entry", mutation: fileDownloadRootTemporary, wantErr: core.ErrRetrievalContract},
		{name: "target names root entry", mutation: fileDownloadRootTarget, wantErr: core.ErrRetrievalContract},
		{name: "activation extent differs from authenticated receipt", mutation: fileDownloadExtentMismatch, wantErr: core.ErrRetrievalBinding},
		{name: "zero transfer policy", mutation: fileDownloadZeroPolicy},
		{name: "zero operation timeout", mutation: fileDownloadZeroOperation},
		{name: "zero attempt timeout", mutation: fileDownloadZeroAttempt},
		{name: "zero verified grant", mutation: fileDownloadZeroGrant, wantErr: core.ErrRetrievalContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			payload := []byte{1, 2, 3}
			fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: payload})
			root := retrievalFileRoot(t, directory)
			requests := 0
			request := FileDownloadRequest{
				Client: retrievalObjectstoreClientForResponse(t, retrievalHTTPResponse{payload: payload, onRequest: func() { requests++ }}), Policy: fixture.policy,
				Activation: retrievalActivation(t, retrievalActivationRequest{
					Root: root, Size: uint64(len(payload)), Install: filestore.InstallCreate,
				}),
			}
			grant := fixture.grant
			switch tc.mutation {
			case fileDownloadZeroRequest:
				request = FileDownloadRequest{}
			case fileDownloadZeroClient:
				request.Client = objectstore.Client{}
			case fileDownloadZeroActivation:
				request.Activation = filestore.ActivationRequest{}
			case fileDownloadMissingRoot:
				request.Activation.Temporary.Root = nil
			case fileDownloadRootTemporary:
				request.Activation.Temporary.Path = retrievalPath(t, ".")
			case fileDownloadRootTarget:
				request.Activation.Target = retrievalPath(t, ".")
			case fileDownloadExtentMismatch:
				request.Activation.ExpectedBytes = new(retrievalLength(t, uint64(len(payload)+1)))
			case fileDownloadZeroPolicy:
				request.Policy = objectstore.Policy{}
			case fileDownloadZeroOperation:
				request.Policy.OperationTimeout = temporal.Duration{}
			case fileDownloadZeroAttempt:
				request.Policy.AttemptTimeout = temporal.Duration{}
			case fileDownloadZeroGrant:
				grant = VerifiedGrant{}
			default:
				t.Fatalf("file download mutation = %d, want published mutation", tc.mutation)
			}
			recovery, transfer, err := grant.DownloadFile(t.Context(), request)
			if tc.wantErr == nil {
				if err != nil || recovery.Validate() == nil || transfer.Validate() != nil || transfer.Bytes().Uint64() != uint64(len(payload)) || transfer.SHA256() != core.SHA256Of(payload) {
					t.Fatalf("caller-owned lifetime transfer=%v/%v/%v, want complete authenticated bytes", recovery, transfer, err)
				}
				if got := readRetrievalTarget(t, root, "target"); !bytes.Equal(got, payload) {
					t.Fatalf("published bytes=%v, want %v", got, payload)
				}
				if _, statErr := retrievalTargetStat(t, root, ".download-stage"); !errors.Is(statErr, fs.ErrNotExist) {
					t.Fatalf("stage lookup=%v, want removed", statErr)
				}
				return
			}
			if requests != 0 {
				t.Fatalf("rejected ingress contacted provider %d times, want zero", requests)
			}
			if _, statErr := retrievalTargetStat(t, root, "target"); !errors.Is(statErr, fs.ErrNotExist) {
				t.Fatalf("rejected ingress target=%v, want absent", statErr)
			}
			if !errors.Is(err, tc.wantErr) || recovery.Validate() == nil || transfer.Validate() == nil {
				t.Fatalf("DownloadFile(mutation %d) = (%v, %v, %v), want zero results and errors.Is %v",
					tc.mutation, recovery, transfer, err, tc.wantErr)
			}
			if _, statErr := retrievalTargetStat(t, root, ".download-stage"); !errors.Is(statErr, fs.ErrNotExist) {
				t.Fatalf("pre-effect refusal temporary Stat() error = %v, want errors.Is %v", statErr, fs.ErrNotExist)
			}
		})
	}
}

func retrievalObjectstoreClient(t testing.TB, payload []byte) objectstore.Client {
	t.Helper()
	return retrievalObjectstoreClientForResponse(t, retrievalHTTPResponse{payload: payload})
}

type retrievalHTTPResponse struct {
	onRequest       func()
	closeErr        error
	transportErr    error
	bodyErr         error
	payload         []byte
	status          int
	omitContentType bool
	observeContext  bool
}

type retrievalErrorReader struct{ err error }

func (r retrievalErrorReader) Read([]byte) (int, error) { return 0, r.err }

type retrievalContextReader struct {
	context context.Context
	source  io.Reader
}

func (r retrievalContextReader) Read(payload []byte) (int, error) {
	if err := r.context.Err(); err != nil {
		return 0, err
	}
	return r.source.Read(payload)
}

func retrievalObjectstoreClientForResponse(t testing.TB, response retrievalHTTPResponse) objectstore.Client {
	t.Helper()
	transport := retrievalRoundTrip(func(request *http.Request) (*http.Response, error) {
		if response.onRequest != nil {
			response.onRequest()
		}
		if response.transportErr != nil {
			return nil, response.transportErr
		}
		headers := make(http.Header)
		if !response.omitContentType {
			headers.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
		}
		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		var body io.Reader = bytes.NewReader(response.payload)
		if response.bodyErr != nil {
			body = io.MultiReader(body, retrievalErrorReader{err: response.bodyErr})
		}
		if response.observeContext {
			body = retrievalContextReader{context: request.Context(), source: body}
		}
		return &http.Response{
			StatusCode: status, Header: headers,
			Body: retrievalResponseBody{Reader: body, closeErr: response.closeErr}, ContentLength: int64(len(response.payload)), Request: request,
		}, nil
	})
	exchangeClient, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	client, err := objectstore.NewClient(exchangeClient)
	if err != nil {
		t.Fatalf("objectstore.NewClient() error = %v, want nil", err)
	}
	return client
}

func retrievalFileRoot(t testing.TB, directory string) *os.Root {
	t.Helper()
	path, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filestore.OpenRoot(t.Context(), path)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", err)
		}
	})
	return root
}

type retrievalActivationRequest struct {
	Root    *os.Root
	Size    uint64
	Install filestore.InstallMode
}

func retrievalActivation(t testing.TB, request retrievalActivationRequest) filestore.ActivationRequest {
	t.Helper()
	return filestore.ActivationRequest{
		Temporary: filestore.Location{Root: request.Root, Path: retrievalPath(t, ".download-stage")},
		Target:    retrievalPath(t, "target"), ExpectedBytes: new(retrievalLength(t, request.Size)),
		Mode: 0o600, Install: request.Install,
	}
}

func retrievalPath(t testing.TB, value string) core.RelativePath {
	t.Helper()
	path, err := core.ParseRelativePath(value)
	if err != nil {
		t.Fatalf("core.ParseRelativePath(%q) error = %v, want nil", value, err)
	}
	return path
}

func retrievalLength(t testing.TB, value uint64) core.ByteLength {
	t.Helper()
	length, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", value, err)
	}
	return length
}

func readRetrievalTarget(t testing.TB, root *os.Root, name string) []byte {
	t.Helper()
	var destination bytes.Buffer
	_, err := filestore.Read(t.Context(), filestore.ReadRequest{
		Destination: &destination, Location: filestore.Location{Root: root, Path: retrievalPath(t, name)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return destination.Bytes()
}

func writeRetrievalTarget(t testing.TB, root *os.Root, name string, content []byte) {
	t.Helper()
	recovery, err := filestore.Write(t.Context(), filestore.WriteRequest{
		Location:  filestore.Location{Root: root, Path: retrievalPath(t, name)},
		Temporary: retrievalPath(t, ".fixture-stage"), Source: bytes.NewReader(content),
		Mode: 0o600, Install: filestore.InstallCreate,
	})
	if err != nil || recovery.Validate() == nil {
		t.Fatalf("fixture write=%v/%v, want complete installation without recovery", recovery, err)
	}
}

func retrievalTargetStat(t testing.TB, root *os.Root, name string) (fs.FileInfo, error) {
	t.Helper()
	file, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{
		Location: filestore.Location{Root: root, Path: retrievalPath(t, name)},
	})
	if err != nil {
		return nil, err
	}
	info, statErr := file.Stat()
	return info, errors.Join(statErr, file.Close())
}

type retrievalResponseBody struct {
	io.Reader
	closeErr error
}

func (body retrievalResponseBody) Close() error { return body.closeErr }

func TestDownloadFileStageOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		payload       []byte
		occupied      bool
		missingExtent bool
		want          error
		requests      int
	}{
		{name: "positive_empty_authenticated_file", requests: 1},
		{name: "negative_preexisting_stage_is_not_ours_to_remove", payload: []byte{1}, occupied: true, want: core.ErrFilestoreConflict},
		{name: "neutral_missing_authenticated_extent_has_no_effect", payload: []byte{1}, missingExtent: true, want: core.ErrRetrievalBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newDownloadCallFixture(t, downloadCallFixtureRequest{Payload: tc.payload})
			root := retrievalFileRoot(t, t.TempDir())
			prior := []byte{0x62, 0x63}
			writeRetrievalTarget(t, root, "target", prior)
			if tc.occupied {
				writeRetrievalTarget(t, root, ".download-stage", prior)
			}
			calls := 0
			request := FileDownloadRequest{
				Client:     retrievalObjectstoreClientForResponse(t, retrievalHTTPResponse{payload: tc.payload, onRequest: func() { calls++ }}),
				Activation: retrievalActivation(t, retrievalActivationRequest{Root: root, Size: uint64(len(tc.payload)), Install: filestore.InstallReplace}),
			}
			if tc.missingExtent {
				request.Activation.ExpectedBytes = nil
			}
			recovery, transfer, err := fixture.grant.DownloadFile(t.Context(), request)
			if !errors.Is(err, tc.want) || calls != tc.requests || recovery.Validate() == nil {
				t.Fatalf("download=%v/%v/%v calls=%d, want %v calls=%d zero recovery", recovery, transfer, err, calls, tc.want, tc.requests)
			}
			want := prior
			if tc.want == nil {
				want = tc.payload
				body := fixture.addition.Entry.Evidence.Payload.Body
				if transfer.Validate() != nil || transfer.Bytes() != body.Extent || transfer.SHA256() != body.SHA256 || transfer.CRC32C() != body.CRC32C {
					t.Fatalf("empty transfer=%v, want exact signed empty integrity", transfer)
				}
			} else if transfer.Validate() == nil {
				t.Fatalf("refused transfer=%v, want no success", transfer)
			}
			if got := readRetrievalTarget(t, root, "target"); !bytes.Equal(got, want) {
				t.Fatalf("target=%x, want %x", got, want)
			}
			if tc.occupied {
				if got := readRetrievalTarget(t, root, ".download-stage"); !bytes.Equal(got, prior) {
					t.Fatalf("foreign stage=%x, want %x", got, prior)
				}
			} else if _, err := retrievalTargetStat(t, root, ".download-stage"); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("stage=%v, want absent", err)
			}
		})
	}
}
