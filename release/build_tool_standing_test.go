package release

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
)

// Direct held-file ratchet: the public opener cannot deterministically race a
// replacement. A real compiler handle and a separately selected namespace fact
// reproduce that seam without scheduling a writer or substituting a parser.
func TestOpenedBuildToolStandingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                    string
		absent, foreign, link, canceled, closed bool
		wantErr                                 error
	}{
		{name: "held compiler at its own path yields exact facts"},
		{name: "absent name cannot borrow compiler proof", absent: true, wantErr: core.ErrReleaseContract},
		{name: "foreign entry cannot borrow compiler proof", foreign: true, wantErr: core.ErrReleaseContract},
		{name: "final link cannot borrow regular entry standing", link: true, wantErr: core.ErrReleaseContract},
		{name: "canceled standing refuses before closed handle stat", canceled: true, closed: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			name, err := core.ParsePathComponent("go")
			if err != nil {
				t.Fatalf("compiler name error = %v, want nil", err)
			}
			path, err := process.Resolve(t.Context(), name)
			if err != nil {
				t.Fatalf("Resolve compiler error = %v, want nil", err)
			}
			path, err = filestore.Canonicalize(t.Context(), path)
			if err != nil {
				t.Fatalf("Canonicalize compiler error = %v, want nil", err)
			}
			location, err := filestore.OpenParent(t.Context(), path)
			if err != nil {
				t.Fatalf("OpenParent compiler error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := location.Root.Close(); err != nil {
					t.Errorf("compiler root Close error = %v, want nil", err)
				}
			})
			held, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: location})
			if err != nil {
				t.Fatalf("OpenRead compiler error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if !tc.closed {
					if err := held.Close(); err != nil {
						t.Errorf("compiler Close error = %v, want nil", err)
					}
				}
			})
			info, err := held.Stat()
			if err != nil {
				t.Fatalf("compiler Stat error = %v, want nil", err)
			}
			oracle := sha256.New()
			n, err := io.Copy(oracle, io.NewSectionReader(held, 0, info.Size()))
			if err != nil || n != info.Size() {
				t.Fatalf("oracle read = (%d, %v), want (%d, nil)", n, err, info.Size())
			}
			wantDigest := core.NewSHA256Digest([core.SHA256DigestBytes]byte(oracle.Sum(nil)))
			subject := path
			if tc.absent || tc.link {
				subject, err = core.ParseAbsolutePath(filepath.Join(directory, "entry"))
				if err != nil {
					t.Fatalf("subject path error = %v, want nil", err)
				}
			}
			if tc.foreign {
				subject = writeInspectionContent(t, inspectionContentRequest{Directory: directory, Content: []byte{1}, Extent: 1, Mode: 0o700})
			}
			if tc.link {
				parent, err := filestore.OpenParent(t.Context(), subject)
				if err != nil {
					t.Fatalf("OpenParent link error = %v, want nil", err)
				}
				t.Cleanup(func() {
					if err := parent.Root.Close(); err != nil {
						t.Errorf("link root Close error = %v, want nil", err)
					}
				})
				if err := parent.Root.Symlink(path.String(), parent.Path.String()); err != nil {
					t.Fatalf("root.Symlink error = %v, want nil", err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			if tc.closed {
				if err := held.Close(); err != nil {
					t.Fatalf("fixture Close error = %v, want nil", err)
				}
			}
			got, digest, gotErr := inspectOpenedBuildTool(ctx, held, subject)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("opened inspection error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !errors.Is(gotErr, core.ErrReleaseContract) || got != nil || digest != (core.SHA256Digest{}) {
					t.Fatalf("refused inspection = (%v, %v, %v), want zero facts and Release refusal", got, digest, gotErr)
				}
				if tc.canceled && errors.Is(gotErr, os.ErrClosed) {
					t.Fatalf("cancellation error = %v, want refusal before reading", gotErr)
				}
				return
			}
			version, err := CurrentGoToolchain().Version()
			if err != nil {
				t.Fatalf("toolchain Version error = %v, want nil", err)
			}
			if got == nil {
				t.Fatal("build info = nil, want exact compiler identity")
			}
			if got.Path != goCommandModulePath || got.GoVersion != version || digest != wantDigest {
				t.Fatalf("inspection = (%q, %q, %v), want (%q, %q, %v)", got.Path, got.GoVersion, digest, goCommandModulePath, version, wantDigest)
			}
		})
	}
}
