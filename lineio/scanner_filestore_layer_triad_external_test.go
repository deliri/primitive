package lineio_test

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

const (
	lineioProofTargetName    = "line-proof.txt"
	lineioProofTemporaryName = ".line-proof.stage"
	lineioProofFileMode      = fs.FileMode(0o600)
)

func TestScannerFilestoreLayerTriad(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		body    string
		want    []string
	}{
		{name: "positive real file accepts exact CRLF boundary", body: stringsOfLength(hostileMaximumLineBytes) + "\r\n", want: []string{stringsOfLength(hostileMaximumLineBytes)}},
		{name: "negative real file rejects one byte beyond boundary", body: stringsOfLength(hostileMaximumLineBytes+1) + "\n", wantErr: bufio.ErrTooLong},
		{name: "neutral real empty file emits no line", body: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rootDirectory := t.TempDir()
			rootPath, err := core.ParseAbsolutePath(rootDirectory)
			if err != nil {
				t.Fatalf("core.ParseAbsolutePath(temp directory) error = %v, want nil", err)
			}
			root, err := filestore.OpenRoot(t.Context(), rootPath)
			if err != nil {
				t.Fatalf("filestore.OpenRoot() error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if closeErr := root.Close(); closeErr != nil {
					t.Errorf("proof root Close() error = %v, want nil", closeErr)
				}
			})
			target := mustRelativePath(t, lineioProofTargetName)
			maximum := max(uint64(len(tc.body)), 1)
			_, err = filestore.Write(t.Context(), filestore.WriteRequest{
				Source:       bytes.NewReader([]byte(tc.body)),
				Location:     filestore.Location{Root: root, Path: target},
				Temporary:    mustRelativePath(t, lineioProofTemporaryName),
				Mode:         lineioProofFileMode,
				Install:      filestore.InstallCreate,
				MaximumBytes: mustByteCount(t, maximum),
			})
			if err != nil {
				t.Fatalf("filestore.Write(proof source) error = %v, want nil", err)
			}
			file, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{
				Location: filestore.Location{Root: root, Path: target},
			})
			if err != nil {
				t.Fatalf("filestore.OpenRead(proof source) error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if closeErr := file.Close(); closeErr != nil {
					t.Errorf("proof file Close() error = %v, want nil", closeErr)
				}
			})

			scanner, err := lineio.New(lineio.Request{
				Source: file,
				Buffer: lineio.BufferPolicy{
					InitialBytes:     mustByteCount(t, hostileInitialBytes),
					MaximumLineBytes: mustByteCount(t, hostileMaximumLineBytes),
				},
			})
			if err != nil {
				t.Fatalf("lineio.New(real file) error = %v, want nil", err)
			}
			got := scanStrings(scanner)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("real file lines = %q, want %q", got, tc.want)
			}
			gotErr := scanner.Err()
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf("Scanner.Err(real file) = %v, want nil", gotErr)
				}
				return
			}
			if !errors.Is(gotErr, core.ErrLineIOScan) || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Scanner.Err(real file) = %v, want %v and %v", gotErr, core.ErrLineIOScan, tc.wantErr)
			}
		})
	}
}

func mustRelativePath(t *testing.T, value string) core.RelativePath {
	t.Helper()
	path, err := core.ParseRelativePath(value)
	if err != nil {
		t.Fatalf("core.ParseRelativePath(%q) error = %v, want nil", value, err)
	}
	return path
}

func stringsOfLength(length int) string {
	return string(bytes.Repeat([]byte{'x'}, length))
}
