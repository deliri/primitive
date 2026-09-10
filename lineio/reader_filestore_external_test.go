package lineio_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
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

func TestReaderFilestoreLayerTriad(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		body    string
		closed  bool
		wantErr error
	}{
		{name: "large real file line crosses fixed buffer", body: string(bytes.Repeat([]byte{'x'}, 4096)) + "\r\n"},
		{name: "closed real file preserves native failure", body: "kept\n", closed: true, wantErr: fs.ErrClosed},
		{name: "empty real file remains neutral"},
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
			_, err = filestore.Write(t.Context(), filestore.WriteRequest{
				Source:    bytes.NewReader([]byte(tc.body)),
				Location:  filestore.Location{Root: root, Path: target},
				Temporary: mustRelativePath(t, lineioProofTemporaryName),
				Mode:      lineioProofFileMode,
				Install:   filestore.InstallCreate,
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
				if closeErr := file.Close(); closeErr != nil && !tc.closed {
					t.Errorf("proof file Close() error = %v, want nil", closeErr)
				}
			})

			if tc.closed {
				if err := file.Close(); err != nil {
					t.Fatalf("file.Close() = %v, want nil", err)
				}
			}
			reader, err := lineio.New(lineio.Request{Source: file, BufferBytes: mustByteCount(t, streamBufferFixture)})
			if err != nil {
				t.Fatalf("New(real file) = %v, want nil", err)
			}
			var got []byte
			var terminal error
			for {
				fragment, err := reader.ReadFragment()
				got = append(got, fragment.Bytes...)
				if err != nil {
					terminal = err
					break
				}
			}
			wantBody := tc.body
			wantErr := tc.wantErr
			if tc.closed {
				wantBody = ""
			} else {
				wantErr = io.EOF
			}
			if !bytes.Equal(got, []byte(wantBody)) || !errors.Is(terminal, wantErr) || errors.Is(terminal, core.ErrLineIOScan) != tc.closed {
				t.Fatalf("real file = (%q,%v), want (%q,%v), scan failure %t", got, terminal, wantBody, wantErr, tc.closed)
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
