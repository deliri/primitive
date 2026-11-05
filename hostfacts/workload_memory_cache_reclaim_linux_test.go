//go:build linux

package hostfacts

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

// This creates only its own four-MiB cache in a distinct sixteen-MiB native
// scope. The managed outer job still owns its complete admitted reservation.
// No existing fleet group is reclaimed by this kernel fixture.
func TestIdleCgroupCacheReclaimNativeOwnedGroup(t *testing.T) {
	t.Parallel()
	unit := "primitive-cache-" + strconv.Itoa(os.Getpid()) + ".slice"
	output := filepath.Join(t.TempDir(), "owned-cache")
	t.Cleanup(func() {
		duration, err := temporal.DurationFromSeconds(10)
		if err != nil {
			t.Error(err)
			return
		}
		parent, err := temporal.WithoutCancellation(temporal.DetachedContextRequest{Parent: t.Context()})
		if err != nil {
			t.Error(err)
			return
		}
		ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: parent, Duration: duration})
		if err != nil {
			t.Error(err)
			return
		}
		defer cancel()
		if _, err := resourceCommand(ctx, "systemctl", "--user", "stop", "--", unit); err != nil {
			t.Errorf("stop owned empty slice: %v", err)
		}
	})
	_, err := resourceCommand(t.Context(), "systemd-run", "--user", "--scope", "--quiet", "--collect", "--slice="+unit, "-p", "MemoryMax=16777216", "-p", "MemorySwapMax=0", "--", "/usr/bin/dd", "if=/dev/zero", "of="+output, "bs=1048576", "count=4", "conv=fsync", "status=none")
	if err != nil {
		t.Fatalf("owned native cache producer=%v, want joined success", err)
	}
	data, err := resourceCommand(t.Context(), "systemctl", "--user", "show", "--property=ControlGroup", "--value", "--", unit)
	if err != nil {
		t.Fatal(err)
	}
	proc, err := procSelfCgroupPath()
	if err != nil {
		t.Fatal(err)
	}
	mounts, err := procSelfMountsPath()
	if err != nil {
		t.Fatal(err)
	}
	selection, present, err := selectWorkloadMemoryScopeParent(t.Context(), workloadMemoryScopeParentFiles{membership: proc, mounts: mounts}, data)
	if err != nil || !present {
		t.Fatalf("owned group=%t/%v, want present", present, err)
	}
	directory, err := resolveCgroupDirectory(selection.prospective.path, selection.mount)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(directory.String(), "/"+unit) {
		t.Fatalf("owned group=%s, want exact suffix %s", directory, unit)
	}
	root, err := filestore.OpenRoot(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := filestore.CloseRoot(filestore.RootHandleRequest{Root: root}); err != nil {
			t.Error(err)
		}
	})
	const amount = 1 << 20
	before, err := readCacheControl(t.Context(), root, "memory.stat")
	if err != nil {
		t.Fatal(err)
	}
	beforeFile, err := cgroupCacheField(before, "file")
	if err != nil {
		t.Fatal(err)
	}
	beforeCharge, err := readCacheControl(t.Context(), root, "memory.current")
	if err != nil {
		t.Fatal(err)
	}
	beforeCurrent, err := parseWorkloadMemoryCharge(beforeCharge)
	if err != nil {
		t.Fatal(err)
	}
	beforeEvents, err := readCacheControl(t.Context(), root, "memory.events")
	if err != nil {
		t.Fatal(err)
	}
	wrote, err := reclaimEmptyCgroupCache(t.Context(), root, amount)
	if err != nil || !wrote {
		t.Fatalf("native reclaim=%t/%v, want true/nil", wrote, err)
	}
	after, err := readCacheControl(t.Context(), root, "memory.stat")
	if err != nil {
		t.Fatal(err)
	}
	afterFile, err := cgroupCacheField(after, "file")
	if err != nil {
		t.Fatal(err)
	}
	afterCharge, err := readCacheControl(t.Context(), root, "memory.current")
	if err != nil {
		t.Fatal(err)
	}
	afterCurrent, err := parseWorkloadMemoryCharge(afterCharge)
	if err != nil {
		t.Fatal(err)
	}
	afterEvents, err := readCacheControl(t.Context(), root, "memory.events")
	if err != nil {
		t.Fatal(err)
	}
	beforeOOM, err := cgroupCacheField(beforeEvents, "oom_kill")
	if err != nil {
		t.Fatal(err)
	}
	afterOOM, err := cgroupCacheField(afterEvents, "oom_kill")
	if err != nil {
		t.Fatal(err)
	}
	if afterCurrent.Uint64() >= beforeCurrent.Uint64() || beforeCurrent.Uint64()-afterCurrent.Uint64() < amount || beforeOOM != afterOOM {
		t.Fatalf("native current=%d->%d file statistic=%d->%d OOM=%d->%d, want at least %d actually reclaimed from admission charge without kill", beforeCurrent.Uint64(), afterCurrent.Uint64(), beforeFile, afterFile, beforeOOM, afterOOM, amount)
	}
	// The disk bytes remain intact: reclaim is not artifact removal.
	info, err := os.Stat(output)
	if err != nil || info.Size() != 4<<20 {
		t.Fatalf("owned output=%v/%v, want intact four MiB", info, err)
	}
	// A disappeared native control preserves the exact filesystem cause.
	empty := t.TempDir()
	emptyRoot, err := os.OpenRoot(empty)
	if err != nil {
		t.Fatal(err)
	}
	_, missingErr := requestEmptyCgroupCacheReclaim(context.Background(), emptyRoot, amount)
	closeErr := emptyRoot.Close()
	if !errors.Is(missingErr, fs.ErrNotExist) || closeErr != nil {
		t.Fatalf("native missing control=%v/%v, want not-exist", missingErr, closeErr)
	}
}
