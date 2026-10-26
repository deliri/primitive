//go:build darwin || linux

package filestore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

// The native file-size limit creates a real partial os.File.Write followed by
// EFBIG. Its signal and process limit belong only to the isolated child.
func TestAppendSpoolPartialWriteNative(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessSignal, Scope: core.TestIsolationScopePackageProcess})
	if os.Getenv("PRIMITIVE_APPEND_SPOOL_PARTIAL_CHILD") != "1" {
		ctx, cancel := newNativeBackstop(t.Context(), t, 30*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAppendSpoolPartialWriteNative$", "-test.v")
		command.Env = append(os.Environ(), "PRIMITIVE_APPEND_SPOOL_PARTIAL_CHILD=1")
		output, err := command.CombinedOutput()
		t.Logf("native partial-write child argv = %v; output = %s", command.Args, output)
		if err != nil {
			t.Fatalf("native partial-write child error = %v, want successful native assertions", err)
		}
		return
	}
	location := spoolLocation(t, t.TempDir())
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 4096, Max: 4096}); err != nil {
		t.Fatal(err)
	}
	signal.Ignore(syscall.SIGXFSZ)
	data := bytes.Repeat([]byte{127}, 8192)
	result, err := WithAppendSpoolScope(t.Context(), AppendSpoolScopeRequest{Location: location, Mode: 0o600, Use: func(ctx context.Context, spool *AppendSpool) error {
		written, err := spool.Write(data)
		if written != 4096 || !errors.Is(err, syscall.EFBIG) || !errors.Is(err, core.ErrFilestoreDestination) {
			t.Fatalf("native partial write = (%d,%v), want 4096 and destination/EFBIG", written, err)
		}
		digest, length, err := spool.digest.Digest()
		if err != nil || length.Uint64() != 4096 || digest != core.SHA256Of(data[:4096]) {
			t.Fatalf("accepted prefix = (%v,%v,%v), want digest of native accepted4096bytes", digest, length, err)
		}
		proof, err := spool.Seal(ctx, nil)
		if proof != (ContentIndexEntry{}) || !errors.Is(err, syscall.EFBIG) {
			t.Fatalf("partial producer seal = (%+v,%v), want zero and EFBIG", proof, err)
		}
		return spool.WithReader(ctx, AppendSpoolReadRequest{Use: func(_ context.Context, source io.Reader) error {
			got, err := io.ReadAll(source)
			if !bytes.Equal(got, data[:4096]) || !errors.Is(err, syscall.EFBIG) {
				t.Fatalf("partial native read = (%dbytes,%v), want4096exactbytes and EFBIG", len(got), err)
			}
			return nil
		}})
	}})
	proof, proofErr := result.Content()
	if err != nil || !errors.Is(proofErr, syscall.EFBIG) || proof != (ContentIndexEntry{}) {
		t.Fatalf("native partial scope proof = (%+v,%v,%v), want zero and EFBIG", proof, err, proofErr)
	}
}
