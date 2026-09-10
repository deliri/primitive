package github

import (
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type archiveExtentReader struct{ remaining uint64 }

func (r *archiveExtentReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(uint64(len(p)), r.remaining)
	for index := range int(n) {
		p[index] = 0xa5
	}
	r.remaining -= n
	return int(n), nil
}

type archiveExtentObservation struct {
	count  int64
	digest core.SHA256Digest
	err    error
}

func TestArchiveContinuesBeyondFormerGiBCeilingLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		bytes uint64
	}{
		{name: "one scratch window plus a final byte", bytes: 32769},
		{name: "one GiB plus a final byte", bytes: (1 << 30) + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repository := parsedRepository(t, "owner/repository")
			commit := parsedCommit(t)
			const archivePath = "/archive"
			apiPath := repositoryPath(repository) + "/tarball/" + commit.String()
			observed := make(chan archiveExtentObservation, 1)
			var apiCalls, archiveCalls atomic.Uint64
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case apiPath:
					apiCalls.Add(1)
					w.Header().Set(headerLocation, server.URL+archivePath)
					w.WriteHeader(http.StatusFound)
				case archivePath:
					archiveCalls.Add(1)
					w.Header().Set(core.HTTPHeaderContentLength().String(), strconv.FormatUint(tc.bytes, 10))
					digest := sha256.New()
					count, err := io.Copy(io.MultiWriter(w, digest), &archiveExtentReader{remaining: tc.bytes})
					observed <- archiveExtentObservation{count: count, err: err, digest: core.NewSHA256Digest([sha256.Size]byte(digest.Sum(nil)))}
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			client := clientFixture(t, server.URL)
			got, err := client.ReadTarArchive(t.Context(), TarArchiveRequest{
				Repository: repository, Commit: commit, Destination: io.Discard, Buffer: make([]byte, 32<<10),
			})
			if err != nil {
				t.Fatalf("archive %d bytes = %+v/%v, want complete stream", tc.bytes, got, err)
			}
			provider := <-observed
			if provider.err != nil || provider.count != int64(tc.bytes) || got.Validate() != nil || got.State != ArchiveTransferComplete || got.Length.Uint64() != tc.bytes || got.SHA256 != provider.digest || got.Repository != repository || got.Commit != commit || apiCalls.Load() != 1 || archiveCalls.Load() != 1 {
				t.Fatalf("archive=%+v, provider=%+v, calls=%d/%d; want exact %d-byte digest and one call per boundary", got, provider, apiCalls.Load(), archiveCalls.Load(), tc.bytes)
			}
		})
	}
}

// These rows validate representable observations; only the transfer test above
// claims an executed extent. A 100-TB transfer is not materialized or simulated.
func TestArchiveObservationExtentHasNoInventedCeilingLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		length  uint64
		state   ArchiveTransferState
		wantErr error
	}{
		{name: "below former GiB ceiling", length: (1 << 30) - 1, state: ArchiveTransferComplete},
		{name: "at former GiB ceiling", length: 1 << 30, state: ArchiveTransferComplete},
		{name: "above former GiB ceiling", length: (1 << 30) + 1, state: ArchiveTransferComplete},
		{name: "100 TB is a representable completed extent", length: 100_000_000_000_000, state: ArchiveTransferComplete},
		{name: "empty incomplete transfer remains representable", state: ArchiveTransferIncomplete},
		{name: "empty transfer cannot claim completion", state: ArchiveTransferComplete, wantErr: core.ErrGitHubResponse},
		{name: "unknown state cannot borrow a large extent", length: 100_000_000_000_000, state: ArchiveTransferUnknown, wantErr: core.ErrGitHubResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			length, err := core.NewByteLength(tc.length)
			if err != nil {
				t.Fatal(err)
			}
			got := TarArchiveObservation{Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t), Length: length, SHA256: core.SHA256Of(nil), State: tc.state}
			if err := got.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("observation %d bytes/%v = %v, want %v", tc.length, tc.state, err, tc.wantErr)
			}
		})
	}
}
