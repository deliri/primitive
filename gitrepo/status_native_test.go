package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestStatusObservationConservesNativeGitByteReceipt(t *testing.T) {
	t.Parallel()
	for _, extent := range [...]int{0, 12, 65535, 65536, 65537, 1<<20 + 1} {
		t.Run(strconv.Itoa(extent), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			initializeGitRepository(t, dir)
			full, tail := extent/256, extent%256
			if tail > 0 && tail < 12 {
				full--
				tail += 256
			}
			for i := range full {
				statusFixtureEntry(t, dir, i, 252)
			}
			if tail > 0 {
				statusFixtureEntry(t, dir, full, tail-4)
			}
			c, request := statusFixtureCapability(t, dir)
			observation, err := c.ObserveStatus(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			stdout, stderr := independentGitStatusBytes(t, c, dir)
			if len(stdout) != extent || observation.StdoutBytes.Uint64() != uint64(len(stdout)) || observation.StderrBytes.Uint64() != uint64(len(stderr)) || observation.ExitCode != 0 || observation.StdinBytes.Uint64() != 0 || observation.Validate() != nil {
				t.Fatalf("receipt=%+v source=%d/%d expected=%d", observation, len(stdout), len(stderr), extent)
			}
		})
	}
	for _, name := range [...]string{"space name.go", "line\nbreak.go", "colon:path.go", "unicode-雪.go"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			initializeGitRepository(t, dir)
			writeWorktreeFile(t, dir, name, "")
			c, request := statusFixtureCapability(t, dir)
			observation, err := c.ObserveStatus(t.Context(), request)
			stdout, stderr := independentGitStatusBytes(t, c, dir)
			if err != nil || observation.StdoutBytes.Uint64() != uint64(len(stdout)) || observation.StderrBytes.Uint64() != uint64(len(stderr)) {
				t.Fatalf("receipt=%+v bytes=%d/%d err=%v", observation, len(stdout), len(stderr), err)
			}
		})
	}
}

func TestStatusObservationRefusesNativeFailureWithoutInventedReceipt(t *testing.T) {
	t.Parallel()
	for mode := range 4 {
		t.Run(strconv.Itoa(mode), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			c, request := statusFixtureCapability(t, dir)
			ctx := t.Context()
			var want error = core.ErrGitRepositoryExecution
			switch mode {
			case 0: // Real Git rejects a nonrepository root.
			case 1:
				initializeGitRepository(t, dir)
				cancelCtx, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelCtx
				want = context.Canceled
			case 2:
				request = StatusRequest{}
				want = core.ErrGitRepositoryContract
			case 3:
				c = Capability{}
				want = core.ErrGitRepositoryContract
			}
			observation, err := c.ObserveStatus(ctx, request)
			if !errors.Is(err, want) || observation != (process.ResultObservation{}) {
				t.Fatalf("receipt=%+v err=%v want=%v", observation, err, want)
			}
		})
	}
}

func statusFixtureCapability(t testing.TB, dir string) (Capability, StatusRequest) {
	t.Helper()
	configuration, err := DefaultConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	c, err := Open(t.Context(), configuration)
	if err != nil {
		t.Fatal(err)
	}
	root, err := core.ParseAbsolutePath(dir)
	if err != nil {
		t.Fatal(err)
	}
	return c, StatusRequest{Root: root}
}

func statusFixtureEntry(t testing.TB, dir string, index, length int) {
	t.Helper()
	prefix := fmt.Sprintf("%08x", index)
	name := prefix + strings.Repeat("x", length-len(prefix))
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
		t.Fatal(err)
	}
}

func independentGitStatusBytes(t testing.TB, c Capability, dir string) ([]byte, []byte) {
	t.Helper()
	command := exec.Command("git", "--no-optional-locks", "-c", "core.excludesFile=", "-c", "core.fsmonitor=false", "status", "--porcelain=v1", "-z")
	command.Dir = dir
	env, err := c.environment.Strings()
	if err != nil {
		t.Fatal(err)
	}
	command.Env = env
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("native Git oracle: %v", err)
	}
	return stdout.Bytes(), stderr.Bytes()
}

func FuzzStatusObservationConservesNativeExtent(f *testing.F) {
	for _, count := range [...]uint8{0, 1, 2, 127, 255} {
		f.Add(count, false)
		f.Add(count, true)
	}
	f.Fuzz(func(t *testing.T, count uint8, staged bool) {
		dir := t.TempDir()
		initializeGitRepository(t, dir)
		for i := range int(count) {
			statusFixtureEntry(t, dir, i, 252)
		}
		if staged {
			runGitSetup(t, dir, "add", "--all")
		}
		c, request := statusFixtureCapability(t, dir)
		observation, err := c.ObserveStatus(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		stdout, stderr := independentGitStatusBytes(t, c, dir)
		if observation.StdoutBytes.Uint64() != uint64(len(stdout)) || observation.StderrBytes.Uint64() != uint64(len(stderr)) || observation.ExitCode != 0 {
			t.Fatalf("receipt=%+v native=%d/%d", observation, len(stdout), len(stderr))
		}
	})
}

func BenchmarkStatusObservationStreamsWithoutRetainedOutput(b *testing.B) {
	for _, count := range [...]int{256, 4096} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			dir := b.TempDir()
			initializeGitRepository(b, dir)
			for i := range count {
				statusFixtureEntry(b, dir, i, 252)
			}
			c, request := statusFixtureCapability(b, dir)
			b.ReportAllocs()
			b.SetBytes(int64(count) * 256)
			for b.Loop() {
				observation, err := c.ObserveStatus(b.Context(), request)
				if err != nil || observation.StdoutBytes.Uint64() != uint64(count)*256 || observation.StderrBytes.Uint64() != 0 {
					b.Fatalf("receipt=%+v err=%v", observation, err)
				}
			}
		})
	}
}
