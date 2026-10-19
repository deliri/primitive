package filestore

import (
	"bufio"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAppendSynchronizationReceiptRequiresNativeSuccess(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		buffered bool
	}{
		{name: "body sync refusal"},
		{name: "buffer flush refusal", buffered: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := os.CreateTemp(t.TempDir(), "native")
			if err != nil {
				t.Fatal(err)
			}
			d := &AppendDestination{file: file, writer: bufio.NewWriter(file)}
			d.self = d
			if tc.buffered {
				if _, err := d.Write([]byte("accepted but not synchronized")); err != nil {
					t.Fatal(err)
				}
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			observation, err := SyncAppend(t.Context(), AppendSyncRequest{Destination: d})
			if !errors.Is(err, os.ErrClosed) || observation != (AppendSyncObservation{}) || !errors.Is(observation.Validate(), core.ErrFilestoreContract) {
				t.Fatalf("native refusal=%+v/%v, want no receipt and native closed cause", observation, err)
			}
			if err := d.Close(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("cleanup=%v, want original closed cause", err)
			}
		})
	}
}

func TestAppendSynchronizationReceiptSurvivesLaterCleanupFailure(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(t.TempDir(), "native")
	if err != nil {
		t.Fatal(err)
	}
	d := &AppendDestination{file: file, writer: bufio.NewWriter(file)}
	d.self = d
	if _, err := d.Write([]byte("sealed")); err != nil {
		t.Fatal(err)
	}
	receipt, err := SyncAppend(t.Context(), AppendSyncRequest{Destination: d})
	if err != nil || receipt.Validate() != nil || receipt.AcceptedBytes().Uint64() != 6 {
		t.Fatalf("sync=%+v/%v", receipt, err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("cleanup=%v, want native closed failure", err)
	}
	if receipt.Validate() != nil || receipt.AcceptedBytes().Uint64() != 6 {
		t.Fatalf("cleanup changed receipt=%+v", receipt)
	}
}

func TestAppendSynchronizationAdmissionCannotFlush(t *testing.T) {
	t.Parallel()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	path, err := core.ParseRelativePath("subject")
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenAppendDestination(t.Context(), AppendRequest{Location: Location{Root: root, Path: path}, Mode: 0o600, Append: AppendCreate})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.Write([]byte("buffered")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(os.ErrPermission)
	receipt, err := SyncAppend(ctx, AppendSyncRequest{Destination: d})
	if receipt != (AppendSyncObservation{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("cancelled sync=%+v/%v", receipt, err)
	}
	if n, err := d.Buffered(); err != nil || n != 8 {
		t.Fatalf("cancelled admission flushed=%v/%v", n, err)
	}
	copied := *d
	for _, refused := range []*AppendDestination{nil, {}, &copied} {
		receipt, err := SyncAppend(t.Context(), AppendSyncRequest{Destination: refused})
		if !errors.Is(err, core.ErrFilestoreContract) || receipt != (AppendSyncObservation{}) {
			t.Fatalf("foreign sync=%+v/%v", receipt, err)
		}
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	receipt, err = SyncAppend(t.Context(), AppendSyncRequest{Destination: d})
	if !errors.Is(err, core.ErrFilestoreContract) || receipt != (AppendSyncObservation{}) {
		t.Fatalf("settled sync=%+v/%v", receipt, err)
	}
}
