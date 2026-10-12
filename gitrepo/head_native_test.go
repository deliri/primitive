package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestHeadObservationConservesNativeGitIdentityAndReceipt(t *testing.T) {
	t.Parallel()
	for _, format := range [...]string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			runGitSetup(t, dir, "init", "--object-format="+format)
			runGitSetup(t, dir, "config", "user.name", "Native proof")
			runGitSetup(t, dir, "config", "user.email", "native@example.invalid")
			if err := os.WriteFile(filepath.Join(dir, "source"), []byte("native bytes"), 0o600); err != nil {
				t.Fatal(err)
			}
			runGitSetup(t, dir, "add", "source")
			runGitSetup(t, dir, "commit", "-m", "native source")
			capability, status := statusFixtureCapability(t, dir)
			got, err := capability.ObserveHead(t.Context(), HeadRequest{Root: status.Root})
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(capability.command.String(), "rev-parse", "--verify", "HEAD")
			command.Dir = dir
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			if err := command.Run(); err != nil {
				t.Fatal(err)
			}
			want := append([]byte(got.Commit.String()), '\n')
			if !bytes.Equal(stdout.Bytes(), want) || stderr.Len() != 0 || got.Process.StdoutBytes.Uint64() != uint64(stdout.Len()) || got.Process.StderrBytes.Uint64() != 0 || got.Validate() != nil {
				t.Fatalf("identity=%q native=%q receipt=%+v", want, stdout.Bytes(), got.Process)
			}
		})
	}
}

func TestHeadObservationRefusesUnavailableNativeIdentity(t *testing.T) {
	t.Parallel()
	for mode := range 4 {
		t.Run(strconv.Itoa(mode), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			capability, status := statusFixtureCapability(t, dir)
			ctx := t.Context()
			var want error = core.ErrGitRepositoryExecution
			switch mode {
			case 1:
				initializeGitRepository(t, dir)
			case 2:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				want = context.Canceled
			case 3:
				status.Root = core.AbsolutePath{}
				want = core.ErrGitRepositoryContract
			}
			got, err := capability.ObserveHead(ctx, HeadRequest{Root: status.Root})
			if got != (HeadObservation{}) || !errors.Is(err, want) {
				t.Fatalf("observation=%+v error=%v want=%v", got, err, want)
			}
		})
	}
}

func TestHeadOutputDrainsPastSchemaWindowWithoutRetainingExtent(t *testing.T) {
	t.Parallel()
	for _, extent := range [...]int{0, 40, 41, 64, 65, 66, 65537, 1<<20 + 1} {
		t.Run(strconv.Itoa(extent), func(t *testing.T) {
			t.Parallel()
			var output headOutput
			source := bytes.Repeat([]byte{'a'}, extent)
			for offset := 0; offset < len(source); {
				end := min(len(source), offset+17)
				n, err := output.Write(source[offset:end])
				if n != end-offset || err != nil {
					t.Fatalf("acknowledgment=%d want=%d err=%v", n, end-offset, err)
				}
				offset = end
			}
			want := min(extent, len(output.bytes))
			if output.used != want || !bytes.Equal(output.bytes[:output.used], source[:want]) {
				t.Fatalf("retained=%d want=%d", output.used, want)
			}
		})
	}
}

func FuzzHeadOutputConservesNativePrefixAndAcceptedByteDomain(f *testing.F) {
	f.Add([]byte("native bytes"), uint8(3))
	f.Add([]byte{}, uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, width uint8) {
		var output headOutput
		stride := int(width) + 1
		for offset := 0; offset < len(data); {
			end := min(len(data), offset+stride)
			n, err := output.Write(data[offset:end])
			if n != end-offset || err != nil {
				t.Fatalf("accepted=%d expected=%d err=%v", n, end-offset, err)
			}
			offset = end
		}
		want := min(len(data), len(output.bytes))
		if output.used != want || !bytes.Equal(output.bytes[:output.used], data[:want]) {
			t.Fatalf("prefix=%d want=%d", output.used, want)
		}
	})
}
