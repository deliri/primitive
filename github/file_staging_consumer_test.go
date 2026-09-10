package github

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestGitHubStreamPublishesOnlyCompletedStagedFilesLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		size         uint64
		failure      fileStreamFailure
		cancelFinish bool
		wantErr      error
	}{
		{name: "complete empty file replaces old bytes"},
		{name: "complete byte replaces old bytes", size: 1},
		{name: "unknown length crosses scratch window", size: 32769},
		{name: "directory refuses publication", size: 31, failure: fileStreamDirectory, wantErr: core.ErrExchangeContentType},
		{name: "truncated response discards partial file", size: 31, failure: fileStreamTruncated, wantErr: io.ErrUnexpectedEOF},
		{name: "canceled finalization leaves old file intact", size: 31, cancelFinish: true, wantErr: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			directory, pathErr := core.ParseAbsolutePath(t.TempDir())
			if pathErr != nil {
				t.Fatal(pathErr)
			}
			root, err := filestore.OpenRoot(t.Context(), directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Errorf("root Close=%v", err)
				}
			})
			target, targetErr := core.ParseRelativePath("published")
			seed, seedErr := core.ParseRelativePath("seed")
			stage, stageErr := core.ParseRelativePath("stage")
			if err := errors.Join(targetErr, seedErr, stageErr); err != nil {
				t.Fatal(err)
			}
			old := []byte("old")
			if _, err := filestore.Write(t.Context(), filestore.WriteRequest{Location: filestore.Location{Root: root, Path: target}, Temporary: seed, Source: bytes.NewReader(old), Mode: 0600, Install: filestore.InstallCreate}); err != nil {
				t.Fatal(err)
			}
			plan := filestore.ActivationRequest{Temporary: filestore.Location{Root: root, Path: stage}, Target: target, Mode: 0600, Install: filestore.InstallReplace}
			destination, err := filestore.OpenStageDestination(ctx, plan.StageDestination())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if destination.Validate() == nil {
					if err := filestore.AbandonStageDestination(destination); err != nil {
						t.Errorf("abandon=%v", err)
					}
				}
			})
			file, err := destination.File()
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				media := core.GitHubRawContentMediaType
				if tc.failure == fileStreamDirectory {
					media = core.HTTPMediaTypeJSON().String()
				}
				w.Header().Set(core.HTTPHeaderContentType().String(), media)
				if tc.failure == fileStreamTruncated {
					w.Header().Set(core.HTTPHeaderContentLength().String(), strconv.FormatUint(tc.size+1, 10))
				}
				n, err := io.Copy(w, &archiveExtentReader{remaining: tc.size})
				if tc.failure == fileStreamOK && (err != nil || uint64(n) != tc.size) {
					t.Errorf("provider copy=%d/%v, want %d/nil", n, err, tc.size)
				}
			}))
			t.Cleanup(server.Close)
			observation, transferErr := clientFixture(t, server.URL).ReadFile(ctx, FileRequest{Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t), Path: parsedPath(t, "source.bin"), Destination: file, Buffer: make([]byte, 32768)})
			resultErr := transferErr
			if transferErr != nil {
				resultErr = errors.Join(transferErr, filestore.AbandonStageDestination(destination))
			} else {
				if tc.cancelFinish {
					cancel()
				}
				staged, finishErr := filestore.FinishStageDestination(ctx, destination)
				resultErr = finishErr
				if finishErr == nil {
					if staged.BytesWritten() != observation.Length {
						t.Fatalf("stage extent=%v, want acknowledged %v", staged.BytesWritten(), observation.Length)
					}
					commit, commitErr := plan.CommitRequest(staged)
					if commitErr != nil {
						t.Fatal(errors.Join(commitErr, filestore.Discard(t.Context(), staged)))
					}
					resultErr = filestore.Commit(ctx, commit)
					if resultErr != nil {
						t.Fatal(resultErr)
					}
				}
			}
			if !errors.Is(resultErr, tc.wantErr) {
				t.Fatalf("consumer result=%v, want %v", resultErr, tc.wantErr)
			}
			digest := core.NewDigestWriter()
			read, readErr := filestore.Read(t.Context(), filestore.ReadRequest{Location: filestore.Location{Root: root, Path: target}, Destination: digest, Buffer: make([]byte, 32768)})
			actual, extent, sealErr := digest.Seal()
			wantExtent, wantDigest := tc.size, fileExpectedDigest(t, tc.size)
			if tc.wantErr != nil {
				wantExtent = uint64(len(old))
				wantDigest = core.SHA256Of(old)
			}
			if readErr != nil || sealErr != nil || read.Uint64() != wantExtent || extent != read || actual != wantDigest {
				t.Fatalf("published count/hash/errors=%d/%v/%v/%v, want exact %d-byte committed content", read.Uint64(), actual, readErr, sealErr, wantExtent)
			}
			_, missingErr := filestore.Read(t.Context(), filestore.ReadRequest{Location: filestore.Location{Root: root, Path: stage}, Destination: io.Discard})
			if !errors.Is(missingErr, fs.ErrNotExist) {
				t.Fatalf("temporary lookup=%v, want removed temporary", missingErr)
			}
		})
	}
}
