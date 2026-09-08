package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type sharingIngress uint8

const (
	sharingIngressActive sharingIngress = iota
	sharingIngressNil
	sharingIngressPanickingNil
	sharingIngressSafeNil
	sharingIngressCanceled
	sharingIngressExpired
	sharingIngressZeroPath
	sharingIngressLimit
)

type sharingNilContext struct{ context.Context }
type sharingSafeNilContext struct{ context.Context }

func (*sharingSafeNilContext) Err() error { return nil }

// The unsupported-platform result is separate from ingress refusal. Each
// admitted row reaches the platform probe; every refusal must publish Unknown.
func TestSharingAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		ingress sharingIngress
		wantErr error
	}{
		{name: "active context reaches native probe", ingress: sharingIngressActive},
		{name: "nil context cannot issue native observation", ingress: sharingIngressNil, wantErr: core.ErrNilContext},
		{name: "panicking typed nil cannot bypass context observation", ingress: sharingIngressPanickingNil, wantErr: core.ErrContextObservation},
		{name: "nil safe context remains admitted by its behavior", ingress: sharingIngressSafeNil},
		{name: "canceled context cannot publish availability", ingress: sharingIngressCanceled, wantErr: context.Canceled},
		{name: "expired context retains deadline identity", ingress: sharingIngressExpired, wantErr: context.DeadlineExceeded},
		{name: "zero path cannot become an execution failure", ingress: sharingIngressZeroPath, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			name := filepath.Join(directory, "entry")
			payload := []byte{0, 255, 1, 127}
			if err := os.WriteFile(name, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(name)
			if err != nil {
				t.Fatal(err)
			}
			path := mustAbsolute(t, name)
			ctx := t.Context()
			switch tc.ingress {
			case sharingIngressActive:
			case sharingIngressNil:
				ctx = nil
			case sharingIngressPanickingNil:
				ctx = (*sharingNilContext)(nil)
			case sharingIngressSafeNil:
				ctx = (*sharingSafeNilContext)(nil)
			case sharingIngressCanceled:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case sharingIngressExpired:
				var cancel context.CancelFunc
				ctx, cancel = newFilesystemBackstop(ctx, t, 0)
				defer cancel()
			case sharingIngressZeroPath:
				path = core.AbsolutePath{}
			default:
				t.Fatalf("ingress = %v, want declared fixture", tc.ingress)
			}
			want, wantErr := filestore.SharingUnknown, tc.wantErr
			if wantErr == nil {
				want, wantErr = nativeSharingProbe(name)
			}
			got, gotErr := filestore.ObserveSharing(ctx, path)
			if got != want || !errors.Is(gotErr, wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) {
				t.Fatalf("ObserveSharing = (%v,%v), want (%v,%v) without Source", got, gotErr, want, wantErr)
			}
			after, err := os.Stat(name)
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
				t.Fatalf("observed entry = (%v,%v), want unchanged inode and metadata", after, err)
			}
			data, err := os.ReadFile(name)
			if err != nil || !bytes.Equal(data, payload) {
				t.Fatalf("entry bytes = (%v,%v), want %v", data, err, payload)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 || entries[0].Name() != "entry" {
				t.Fatalf("namespace = (%v,%v), want only original entry", entries, err)
			}
		})
	}
}
