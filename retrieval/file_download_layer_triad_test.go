package retrieval

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"sync/atomic"
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
		{name: "two bytes", size: 2},
		{name: "one below first stream chunk", size: 32<<10 - 1},
		{name: "at first stream chunk", size: 32 << 10},
		{name: "one above first stream chunk", size: 32<<10 + 1},
		{name: "one below second stream chunk", size: 64<<10 - 1},
		{name: "at second stream chunk", size: 64 << 10},
		{name: "one above second stream chunk", size: 64<<10 + 1},
		{name: "at third stream chunk", size: 96 << 10},
		{name: "one above third stream chunk", size: 96<<10 + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			payload := bytes.Repeat([]byte{0x6a}, tc.size)
			fixture := newDownloadCallFixture(t, payload)
			root := retrievalFileRoot(t, directory)
			var completed atomic.Uint64
			request := FileDownloadRequest{
				Client: retrievalObjectstoreClient(t, payload), Policy: fixture.policy,
				Activation: retrievalActivation(t, retrievalActivationRequest{
					Root: root, Size: uint64(tc.size), Install: filestore.InstallCreate,
				}),
				Observer: func(progress objectstore.TransferProgress) error {
					completed.Store(progress.Completed().Uint64())
					return nil
				},
			}
			recovery, transfer, err := fixture.grant.DownloadFile(t.Context(), request)
			if err != nil || recovery.Validate() == nil || transfer.Validate() != nil ||
				completed.Load() != uint64(tc.size) {
				t.Fatalf("VerifiedGrant.DownloadFile(%d) = (recovery %v, transfer %v, error %v, progress %d), want zero recovery and exact success",
					tc.size, recovery, transfer, err, completed.Load())
			}
			got := readRetrievalTarget(t, root, "target")
			if !bytes.Equal(got, payload) {
				t.Fatalf("activated target bytes = %d, want exact %d", len(got), len(payload))
			}
		})
	}
}

func TestVerifiedGrantDownloadFileLayerTriadNegativePreservesPriorTarget(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	payload := bytes.Repeat([]byte{0x7b}, 64<<10+1)
	fixture := newDownloadCallFixture(t, payload)
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
	if err == nil || recovery.Validate() == nil || transfer.Commitment() == objectstore.CommitmentConfirmed {
		t.Fatalf("truncated DownloadFile() = (recovery %v, commitment %v, error %v), want zero recovery and failed transfer",
			recovery, transfer.Commitment(), err)
	}
	if got := readRetrievalTarget(t, root, "target"); !bytes.Equal(got, prior) {
		t.Fatalf("target after failed download = %q, want preserved %q", got, prior)
	}
	if _, statErr := root.Stat(".download-stage"); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("temporary after failed download Stat() error = %v, want errors.Is %v", statErr, fs.ErrNotExist)
	}
}

func TestVerifiedGrantDownloadFileLayerTriadNeutralAbsentObserverLeavesNoStage(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	payload := []byte{0x01}
	fixture := newDownloadCallFixture(t, payload)
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
	if _, gotStatErr := root.Stat(".download-stage"); !errors.Is(gotStatErr, fs.ErrNotExist) {
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
	fileDownloadZeroErrorLimit
	fileDownloadZeroGrant
)

func TestVerifiedGrantDownloadFileLayerTriadIngressRefusesBeforeFilesystemEffect(t *testing.T) {
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
		{name: "zero transfer policy", mutation: fileDownloadZeroPolicy, wantErr: core.ErrRetrievalContract},
		{name: "zero operation timeout", mutation: fileDownloadZeroOperation, wantErr: core.ErrRetrievalContract},
		{name: "zero attempt timeout", mutation: fileDownloadZeroAttempt, wantErr: core.ErrRetrievalContract},
		{name: "zero error-body limit", mutation: fileDownloadZeroErrorLimit, wantErr: core.ErrRetrievalContract},
		{name: "zero verified grant", mutation: fileDownloadZeroGrant, wantErr: core.ErrRetrievalContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			payload := []byte{1, 2, 3}
			fixture := newDownloadCallFixture(t, payload)
			root := retrievalFileRoot(t, directory)
			request := FileDownloadRequest{
				Client: retrievalObjectstoreClient(t, payload), Policy: fixture.policy,
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
				request.Activation.ExpectedBytes = retrievalLength(t, uint64(len(payload)+1))
			case fileDownloadZeroPolicy:
				request.Policy = objectstore.Policy{}
			case fileDownloadZeroOperation:
				request.Policy.OperationTimeout = temporal.Duration{}
			case fileDownloadZeroAttempt:
				request.Policy.AttemptTimeout = temporal.Duration{}
			case fileDownloadZeroErrorLimit:
				request.Policy.ErrorBodyLimit = core.ByteCount{}
			case fileDownloadZeroGrant:
				grant = VerifiedGrant{}
			default:
				t.Fatalf("file download mutation = %d, want published mutation", tc.mutation)
			}
			recovery, transfer, err := grant.DownloadFile(t.Context(), request)
			if !errors.Is(err, tc.wantErr) || recovery.Validate() == nil || transfer.Validate() == nil {
				t.Fatalf("DownloadFile(mutation %d) = (%v, %v, %v), want zero results and errors.Is %v",
					tc.mutation, recovery, transfer, err, tc.wantErr)
			}
			if _, statErr := root.Stat(".download-stage"); !errors.Is(statErr, fs.ErrNotExist) {
				t.Fatalf("pre-effect refusal temporary Stat() error = %v, want errors.Is %v", statErr, fs.ErrNotExist)
			}
		})
	}
}

func retrievalObjectstoreClient(t *testing.T, payload []byte) objectstore.Client {
	t.Helper()
	transport := retrievalRoundTrip(func(request *http.Request) (*http.Response, error) {
		headers := make(http.Header)
		headers.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
		return &http.Response{
			StatusCode: http.StatusOK, Header: headers,
			Body: io.NopCloser(bytes.NewReader(payload)), ContentLength: int64(len(payload)), Request: request,
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

func retrievalFileRoot(t *testing.T, directory string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(directory)
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

func retrievalActivation(t *testing.T, request retrievalActivationRequest) filestore.ActivationRequest {
	t.Helper()
	return filestore.ActivationRequest{
		Temporary: filestore.Location{Root: request.Root, Path: retrievalPath(t, ".download-stage")},
		Target:    retrievalPath(t, "target"), ExpectedBytes: retrievalLength(t, request.Size),
		Mode: 0o600, Install: request.Install,
	}
}

func retrievalPath(t *testing.T, value string) core.RelativePath {
	t.Helper()
	path, err := core.ParseRelativePath(value)
	if err != nil {
		t.Fatalf("core.ParseRelativePath(%q) error = %v, want nil", value, err)
	}
	return path
}

func retrievalLength(t *testing.T, value uint64) core.ByteLength {
	t.Helper()
	length, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", value, err)
	}
	return length
}

func readRetrievalTarget(t *testing.T, root *os.Root, name string) []byte {
	t.Helper()
	file, err := root.Open(name)
	if err != nil {
		t.Fatalf("os.Root.Open(%q) error = %v, want nil", name, err)
	}
	content, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read and close target error = %v, want nil", err)
	}
	return content
}

func writeRetrievalTarget(t *testing.T, root *os.Root, name string, content []byte) {
	t.Helper()
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatalf("os.Root.OpenFile(%q) error = %v, want nil", name, err)
	}
	written, writeErr := file.Write(content)
	closeErr := file.Close()
	if errors.Join(writeErr, closeErr) != nil || written != len(content) {
		t.Fatalf("write target = (%d, %v, %v), want (%d, nil, nil)", written, writeErr, closeErr, len(content))
	}
}
