package github

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type fileStreamFailure uint8

const (
	fileStreamOK fileStreamFailure = iota
	fileStreamDirectory
	fileStreamMissing
	fileStreamTruncated
	fileStreamDestinationFailure
)

type fileCountingDestination struct {
	count        uint64
	maximumWrite int
	fail         bool
	digest       *core.DigestWriter
}

func (w *fileCountingDestination) Write(p []byte) (int, error) {
	w.maximumWrite = max(w.maximumWrite, len(p))
	accepted := len(p)
	if w.fail {
		accepted = min(3, len(p))
	}
	for _, value := range p[:accepted] {
		if value != 0xa5 {
			return 0, core.ErrGitHubBinding
		}
	}
	n, err := w.digest.Write(p[:accepted])
	w.count += uint64(n)
	if w.fail {
		return n, errors.Join(err, io.ErrClosedPipe)
	}
	return n, err
}
func fileExpectedDigest(t testing.TB, length uint64) core.SHA256Digest {
	t.Helper()
	hash := sha256.New()
	n, err := io.Copy(hash, &archiveExtentReader{remaining: length})
	if err != nil || uint64(n) != length {
		t.Fatalf("reference digest count/error=%d/%v, want %d/nil", n, err, length)
	}
	return core.NewSHA256Digest([sha256.Size]byte(hash.Sum(nil)))
}

func TestGitHubRawFileStreamingLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		size    uint64
		window  int
		failure fileStreamFailure
		wantErr error
	}{
		{name: "empty file has an empty digest", window: 1},
		{name: "one byte crosses one byte window", size: 1, window: 1},
		{name: "below native copy window", size: 32767, window: 32768},
		{name: "exact native copy window", size: 32768, window: 32768},
		{name: "above native copy window", size: 32769, window: 32768},
		{name: "below deleted inline ceiling", size: 999999, window: 32768},
		{name: "at deleted inline ceiling", size: 1000000, window: 32768},
		{name: "above deleted inline ceiling", size: 1000001, window: 32768},
		{name: "sixteen MiB plus final byte", size: (16 << 20) + 1, window: 32768},
		{name: "empty scratch delegates allocation to Go", size: 32769},
		{name: "directory JSON cannot become a file", size: 31, window: 3, failure: fileStreamDirectory, wantErr: core.ErrExchangeContentType},
		{name: "not found cannot become an empty success", size: 31, window: 3, failure: fileStreamMissing, wantErr: core.ErrGitHubNotFound},
		{name: "short body preserves acknowledged prefix", size: 31, window: 3, failure: fileStreamTruncated, wantErr: io.ErrUnexpectedEOF},
		{name: "destination failure preserves only its acknowledged prefix", size: 31, window: 8, failure: fileStreamDestinationFailure, wantErr: io.ErrClosedPipe},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repository, commit, path := parsedRepository(t, "owner/repository"), parsedCommit(t), parsedPath(t, "source/raw file.bin")
			var calls atomic.Uint64
			authorization, nameErr := exchange.StandardHeaderAuthorization.Name()
			if nameErr != nil {
				t.Fatal(nameErr)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodGet || r.URL.Path != repositoryPath(repository)+"/contents/"+path.String() ||
					r.URL.Query().Get("ref") != commit.String() || r.Header.Get(core.HTTPHeaderAccept().String()) != core.GitHubRawContentMediaType ||
					r.Header.Get(headerAPIVersion) != core.GitHubAPIVersion || r.Header.Get(headerUserAgent) != "primitive-test" ||
					r.Header.Get(authorization.String()) != "" {
					t.Errorf("wire method/path/query/headers=%s/%s/%s/%v, want exact immutable raw file request", r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header)
				}
				media := core.GitHubRawContentMediaType
				if tc.failure == fileStreamDirectory {
					media = core.HTTPMediaTypeJSON().String()
				}
				w.Header().Set(core.HTTPHeaderContentType().String(), media)
				declared := tc.size
				if tc.failure == fileStreamTruncated {
					declared++
				}
				w.Header().Set(core.HTTPHeaderContentLength().String(), strconv.FormatUint(declared, 10))
				if tc.failure == fileStreamMissing {
					w.WriteHeader(http.StatusNotFound)
				}
				_, err := io.Copy(w, &archiveExtentReader{remaining: tc.size})
				if err != nil && tc.failure == fileStreamOK {
					t.Errorf("provider copy=%v, want nil", err)
				}
			}))
			t.Cleanup(server.Close)
			sink := &fileCountingDestination{digest: core.NewDigestWriter(), fail: tc.failure == fileStreamDestinationFailure}
			request := FileRequest{Repository: repository, Commit: commit, Path: path, Destination: sink, Buffer: make([]byte, tc.window)}
			got, err := clientFixture(t, server.URL).ReadFile(t.Context(), request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ReadFile=%+v/%v, want %v", got, err, tc.wantErr)
			}
			want := tc.size
			if tc.failure == fileStreamDirectory || tc.failure == fileStreamMissing {
				want = 0
			}
			if tc.failure == fileStreamDestinationFailure {
				want = min(tc.size, 3)
			}
			if got.Validate() != nil || got.Repository != repository || got.Commit != commit || got.Path != path ||
				got.Length.Uint64() != want || sink.count != want || got.SHA256 != fileExpectedDigest(t, want) || calls.Load() != 1 {
				t.Fatalf("file=%+v sink=%d calls=%d, want exact coordinates, digest of %d acknowledged bytes and one HTTP request", got, sink.count, calls.Load(), want)
			}
			if tc.window > 0 && sink.maximumWrite > tc.window {
				t.Fatalf("write window=%d exceeds borrowed scratch=%d", sink.maximumWrite, tc.window)
			}
		})
	}
}

func TestFileInvalidIngressCannotReachHTTP(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(*FileRequest)
	}{
		{name: "nil interface", mutate: func(r *FileRequest) { r.Destination = nil }},
		{name: "typed nil buffer", mutate: func(r *FileRequest) { r.Destination = (*bytes.Buffer)(nil) }},
		{name: "typed nil native file", mutate: func(r *FileRequest) { r.Destination = (*os.File)(nil) }},
		{name: "unvalidated repository", mutate: func(r *FileRequest) { r.Repository = Repository{} }},
		{name: "unvalidated commit", mutate: func(r *FileRequest) { r.Commit = core.BuildCommit{} }},
		{name: "unvalidated path", mutate: func(r *FileRequest) { r.Path = core.SourcePath{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Uint64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			t.Cleanup(server.Close)
			r := FileRequest{Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t), Path: parsedPath(t, "data"), Destination: io.Discard}
			tc.mutate(&r)
			validationErr := r.Validate()
			got, err := clientFixture(t, server.URL).ReadFile(t.Context(), r)
			if !errors.Is(validationErr, core.ErrGitHubContract) || !errors.Is(err, core.ErrGitHubContract) || got != (FileObservation{}) || calls.Load() != 0 {
				t.Fatalf("validation/result/error/calls=%v/%+v/%v/%d, want invalid contract and no effect", validationErr, got, err, calls.Load())
			}
		})
	}
}

func FuzzGitHubFileResponseSemanticClosure(f *testing.F) {
	f.Add([]byte{}, uint16(1), uint8(fileStreamOK))
	f.Add([]byte{0, 0xff, 0, 0xfe}, uint16(3), uint8(fileStreamOK))
	f.Add([]byte("[]"), uint16(2), uint8(fileStreamDirectory))
	f.Add([]byte("incomplete"), uint16(7), uint8(fileStreamTruncated))
	f.Fuzz(func(t *testing.T, payload []byte, fragment uint16, mode uint8) {
		if len(payload) > 65536 {
			t.Skip("fuzz generation bound: transfer production has no byte quota")
		}
		failure := fileStreamFailure(mode % uint8(fileStreamTruncated+1))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			media := core.GitHubRawContentMediaType
			if failure == fileStreamDirectory {
				media = core.HTTPMediaTypeJSON().String()
			}
			w.Header().Set(core.HTTPHeaderContentType().String(), media)
			declared := len(payload)
			if failure == fileStreamTruncated {
				declared++
			}
			w.Header().Set(core.HTTPHeaderContentLength().String(), strconv.Itoa(declared))
			if failure == fileStreamMissing {
				w.WriteHeader(http.StatusNotFound)
			}
			if _, err := w.Write(payload); err != nil && failure == fileStreamOK {
				t.Errorf("provider write=%v", err)
			}
		}))
		t.Cleanup(server.Close)
		var destination bytes.Buffer
		request := FileRequest{Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t), Path: parsedPath(t, "data"), Destination: &destination, Buffer: make([]byte, int(fragment)+1)}
		got, err := clientFixture(t, server.URL).ReadFile(t.Context(), request)
		want := payload
		var wantErr error
		switch failure {
		case fileStreamDirectory:
			want = nil
			wantErr = core.ErrExchangeContentType
		case fileStreamMissing:
			want = nil
			wantErr = core.ErrGitHubNotFound
		case fileStreamTruncated:
			wantErr = io.ErrUnexpectedEOF
		}
		if !errors.Is(err, wantErr) || got.Validate() != nil || !bytes.Equal(destination.Bytes(), want) || got.Length.Uint64() != uint64(len(want)) ||
			got.SHA256 != core.SHA256Of(want) || got.Repository != request.Repository || got.Path != request.Path || got.Commit != request.Commit {
			t.Fatalf("stream=%+v/%v captured=%d, want exact %d-byte prefix and %v", got, err, destination.Len(), len(want), wantErr)
		}
	})
}

var _ io.Writer = (*fileCountingDestination)(nil)

type fileInvalidCountWriter struct{ excess bool }

func (w fileInvalidCountWriter) Write(p []byte) (int, error) {
	if w.excess {
		return len(p) + 1, io.ErrClosedPipe
	}
	return -1, io.ErrClosedPipe
}

func TestFileNativeFailureSurvivesInvalidWriteCount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		writer io.Writer
	}{
		{name: "negative acknowledgment", writer: fileInvalidCountWriter{}},
		{name: "acknowledgment exceeds offered bytes", writer: fileInvalidCountWriter{excess: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(core.HTTPHeaderContentType().String(), core.GitHubRawContentMediaType)
				if _, err := w.Write([]byte{0xa5}); err != nil {
					t.Errorf("provider Write=%v", err)
				}
			}))
			t.Cleanup(server.Close)
			got, err := clientFixture(t, server.URL).ReadFile(t.Context(), FileRequest{Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t), Path: parsedPath(t, "file"), Destination: tc.writer})
			if !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrGitHubResponse) || got.Length.Uint64() != 0 || got.SHA256 != core.SHA256Of(nil) {
				t.Fatalf("file=%+v/%v, want native and contract identities with zero acknowledged bytes", got, err)
			}
		})
	}
}
