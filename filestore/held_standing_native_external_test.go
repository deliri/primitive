package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestHeldStandingNativeIdentityLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name               string
		mutation           heldNativeMutation
		want               filestore.HeldStanding
		wantErr, wantCause error
	}{
		{name: "same inode survives observation without consuming held bytes", want: filestore.HeldStandingSame},
		{name: "hard-link spelling retains identical native identity", mutation: heldNativeHardLink, want: filestore.HeldStandingSame},
		{name: "renamed candidate still identifies original held entry", mutation: heldNativeRenamedCandidate, want: filestore.HeldStandingSame},
		{name: "same bytes and mode cannot impersonate original inode", mutation: heldNativeForeignSameBytes, want: filestore.HeldStandingReplaced},
		{name: "replacement directory cannot inherit held-file identity", mutation: heldNativeReplacementDirectory, want: filestore.HeldStandingReplaced},
		{name: "final symlink to owned inode remains a replacement", mutation: heldNativeLinkToOwned, want: filestore.HeldStandingReplaced},
		{name: "dangling final link is occupied rather than absent", mutation: heldNativeDanglingLink, want: filestore.HeldStandingReplaced},
		{name: "outside final link cannot lend referent identity", mutation: heldNativeOutsideLink, want: filestore.HeldStandingReplaced},
		{name: "unlinked inode stays readable while name is absent", mutation: heldNativeUnlinked, want: filestore.HeldStandingAbsent},
		{name: "renamed-away inode preserves its retained namespace", mutation: heldNativeRenamedAway, want: filestore.HeldStandingAbsent},
		{name: "missing parent is observed as absence", mutation: heldNativeMissingParent, want: filestore.HeldStandingAbsent},
		{name: "regular-file parent is observed as absence", mutation: heldNativeFileParent, want: filestore.HeldStandingAbsent},
		{name: "ancestor alias resolves to exact held inode", mutation: heldNativeAncestorAlias, want: filestore.HeldStandingSame},
		{name: "absolute ancestor traversal observes actual foreign identity", mutation: heldNativeAncestorOutside, want: filestore.HeldStandingReplaced},
		{name: "ancestor cycle remains a native source refusal", mutation: heldNativeAncestorCycle, want: filestore.HeldStandingUnknown, wantErr: core.ErrFilestoreSource},
		{name: "nil handle cannot become an absent observation", mutation: heldNativeNilHandle, want: filestore.HeldStandingUnknown, wantErr: core.ErrFilestoreContract},
		{name: "closed handle retains contract and native closed identities", mutation: heldNativeClosedHandle, want: filestore.HeldStandingUnknown, wantErr: core.ErrFilestoreContract, wantCause: os.ErrClosed},
		{name: "zero path cannot become a working-directory observation", mutation: heldNativeZeroPath, want: filestore.HeldStandingUnknown, wantErr: core.ErrFilestoreContract},
		{name: "nil context refuses before native observation", mutation: heldNativeNilContext, want: filestore.HeldStandingUnknown, wantErr: core.ErrNilContext},
		{name: "canceled context cannot publish a standing", mutation: heldNativeCanceledContext, want: filestore.HeldStandingUnknown, wantErr: context.Canceled},
		{name: "held directory is admitted without consuming child iteration", mutation: heldNativeDirectory, want: filestore.HeldStandingSame},
		{name: "held native pipe is compared without inventing a file-kind gate", mutation: heldNativePipe, want: filestore.HeldStandingReplaced},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			container := t.TempDir()
			payload := []byte{0, 255, 1, 127}
			fixture, err := createHeldNativeFixture(container, tc.mutation, payload)
			if err != nil {
				t.Fatal(err)
			}
			if !fixture.closed {
				t.Cleanup(func() {
					if err := fixture.file.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			path, err := core.ParseAbsolutePath(fixture.path)
			if err != nil {
				t.Fatal(err)
			}
			ctx := t.Context()
			held := fixture.file
			switch tc.mutation {
			case heldNativeNilHandle:
				held = nil
			case heldNativeZeroPath:
				path = core.AbsolutePath{}
			case heldNativeNilContext:
				ctx = nil
			case heldNativeCanceledContext:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			heldBefore, heldNativeErr := fixture.file.Stat()
			native, nativeErr := os.Lstat(fixture.path)
			before, err := removalFixtureSnapshot(container)
			if err != nil {
				t.Fatal(err)
			}
			metadata := make([]fs.FileInfo, len(before))
			for i, entry := range before {
				metadata[i], err = os.Lstat(filepath.Join(container, entry.name))
				if err != nil {
					t.Fatal(err)
				}
			}
			got, gotErr := filestore.ObserveHeldStanding(ctx, held, path)
			if got != tc.want || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ObserveHeldStanding = (%v,%v), want (%v,%v)", got, gotErr, tc.want, tc.wantErr)
			}
			if !errors.Is(tc.wantErr, core.ErrFilestoreSource) && errors.Is(gotErr, core.ErrFilestoreSource) {
				t.Fatalf("standing refusal = %v, want no source classification", gotErr)
			}
			if tc.wantCause != nil && !errors.Is(gotErr, tc.wantCause) {
				t.Fatalf("standing cause = %v, want %v", gotErr, tc.wantCause)
			}
			if errors.Is(tc.wantErr, core.ErrFilestoreSource) {
				var nativePath, gotPath *os.PathError
				if !errors.As(nativeErr, &nativePath) || !errors.As(gotErr, &gotPath) || !errors.Is(gotErr, nativePath.Err) || gotPath.Op != nativePath.Op || gotPath.Path != nativePath.Path {
					t.Fatalf("native standing refusal = (%v,%v), want exact PathError", gotErr, nativeErr)
				}
			}
			if tc.want == filestore.HeldStandingSame || tc.want == filestore.HeldStandingReplaced {
				if heldNativeErr != nil || nativeErr != nil || os.SameFile(heldBefore, native) != (tc.want == filestore.HeldStandingSame) {
					t.Fatalf("native identity = (%v,%v,%v,%v), want same %t", heldBefore, heldNativeErr, native, nativeErr, tc.want == filestore.HeldStandingSame)
				}
			}
			if tc.wantErr == nil && got.Validate() != nil {
				t.Fatalf("admitted standing = %v, want valid closed value", got)
			}
			if fixture.closed {
				var data [1]byte
				if n, err := fixture.file.Read(data[:]); n != 0 || !errors.Is(err, os.ErrClosed) {
					t.Fatalf("closed held read = (%d,%v), want native closed identity", n, err)
				}
			} else {
				heldAfter, err := fixture.file.Stat()
				if err != nil || !os.SameFile(heldBefore, heldAfter) || heldBefore.Mode() != heldAfter.Mode() || heldBefore.ModTime().UnixNano() != heldAfter.ModTime().UnixNano() {
					t.Fatalf("retained held identity = (%v,%v), want %v", heldAfter, err, heldBefore)
				}
				if tc.mutation == heldNativeDirectory {
					children, err := fixture.file.ReadDir(1)
					if err != nil || len(children) != 1 || children[0].Name() != "child" {
						t.Fatalf("retained directory cursor = (%v,%v), want first child", children, err)
					}
					rest, err := fixture.file.ReadDir(1)
					if len(rest) != 0 || !errors.Is(err, io.EOF) {
						t.Fatalf("directory completion = (%v,%v), want exact EOF", rest, err)
					}
				} else {
					data := make([]byte, len(payload))
					if n, err := io.ReadFull(fixture.file, data); err != nil || n != len(payload) || !bytes.Equal(data, payload) {
						t.Fatalf("retained held stream = (%v,%d,%v), want %v", data, n, err, payload)
					}
					var extra [1]byte
					if n, err := fixture.file.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
						t.Fatalf("held stream completion = (%d,%v), want exact EOF", n, err)
					}
				}
			}
			after, err := removalFixtureSnapshot(container)
			if err != nil || len(after) != len(before) {
				t.Fatalf("namespace = (%v,%v), want %v", after, err, before)
			}
			for i, want := range before {
				got := after[i]
				if got.name != want.name || got.mode != want.mode || got.target != want.target || !bytes.Equal(got.data, want.data) {
					t.Fatalf("namespace entry = %+v, want %+v", got, want)
				}
				info, err := os.Lstat(filepath.Join(container, want.name))
				if err != nil || !os.SameFile(metadata[i], info) || metadata[i].ModTime().UnixNano() != info.ModTime().UnixNano() {
					t.Fatalf("retained namespace identity = (%v,%v), want %v", info, err, metadata[i])
				}
			}
		})
	}
}
