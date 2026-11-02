//go:build darwin || linux

package gotoolchain

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestModuleClosureLockIsNativeGeneratedCoordinate(t *testing.T) {
	t.Parallel()
	fixture := newModuleClosureFixture(t)
	var selected uint64
	request := fixture.request()
	request.VisitSelected = func(_ context.Context, row SelectedModuleObservation) error {
		info, err := os.Lstat(row.Lock.String())
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return core.ErrGoToolchainContract
		}
		missing := row
		missing.Lock = core.AbsolutePath{}
		if err := missing.Validate(); !errors.Is(err, core.ErrPrimitiveContract) {
			return core.ErrGoToolchainContract
		}
		selected++
		return nil
	}
	got, err := fixture.capability.ObserveModuleClosure(t.Context(), request)
	if err != nil || selected != 4 || got.Selected != selected {
		t.Fatalf("native lock coordinates=%d/%+v/%v, want all four native-created carriers and required field validation", selected, got, err)
	}
}

// A real Go download must wait at the observed version carrier. The existing
// Primitive native lock interoperability is exercised at the SDK-derived name;
// neither this consumer nor Anvil synthesizes an escaped cache lock address.
func TestModuleClosureLockExcludesNativeGoDownload(t *testing.T) {
	t.Parallel()
	fixture := newModuleClosureFixture(t)
	t.Cleanup(func() {
		if err := filepath.WalkDir(fixture.cache, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return os.Chmod(path, 0o755)
			}
			return nil
		}); err != nil {
			t.Errorf("owned native cache cleanup permissions=%v, want writable directories", err)
		}
	})
	var selected SelectedModuleObservation
	request := fixture.request()
	request.VisitSelected = func(_ context.Context, row SelectedModuleObservation) error {
		if row.Module.String() == "example.com/a" {
			selected = row
		}
		return nil
	}
	if _, err := fixture.capability.ObserveModuleClosure(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if err := selected.Validate(); err != nil {
		t.Fatal(err)
	}
	// Leave the native cached ZIP in place but remove the complete-directory
	// fast path. This actual download must acquire its version lock to unzip.
	if err := os.Rename(selected.Directory.String(), selected.Directory.String()+".saved"); err != nil {
		t.Fatal(err)
	}
	location, err := filestore.OpenParent(t.Context(), selected.Lock)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := filestore.CloseRoot(filestore.RootHandleRequest{Root: location.Root}); err != nil {
			t.Errorf("carrier parent close=%v, want nil", err)
		}
	}()
	duration, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatal(err)
	}
	wait, err := temporal.DurationFromSeconds(3)
	if err != nil {
		t.Fatal(err)
	}
	arguments, err := process.ParseArguments([]string{"mod", "download", selected.Module.String() + "@" + selected.Version.String()})
	if err != nil {
		t.Fatal(err)
	}
	var native process.ResultObservation
	var nativeErr error
	result, err := filelock.WithScope(t.Context(), filelock.ScopeRequest{Carrier: filestore.LockFileRequest{Location: location, Mode: 0o600}, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate, Use: func(ctx context.Context) error {
		bounded, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: duration})
		if err != nil {
			return err
		}
		defer cancel()
		execution, err := process.Begin(bounded, process.Request{Command: fixture.capability.command, WorkingDirectory: fixture.directory, Arguments: arguments, Environment: fixture.capability.environment, Streams: process.Streams{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard}, OutputPolicy: process.OutputPolicy{Mode: process.OutputModeStreaming}, WaitDelay: wait, Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalKill}})
		if err != nil {
			return err
		}
		identity, identityErr := execution.Identity()
		if identityErr != nil {
			sweepErr := execution.Sweep()
			_, waitErr := execution.Wait()
			return errors.Join(identityErr, sweepErr, waitErr)
		}
		t.Logf("native version-lock waiter PID=%v", identity)
		completed, childErr := execution.Wait()
		observation, observationErr := completed.Observation()
		native, nativeErr = observation, childErr
		return errors.Join(observationErr, execution.Sweep())
	}})
	if err != nil {
		t.Fatal(err)
	}
	held, heldErr := result.Held()
	if err := errors.Join(heldErr, result.OperationError(), result.CleanupError()); err != nil || !held {
		t.Fatalf("native carrier scope held=%t/%v, want acquired and joined cleanup", held, err)
	}
	if !errors.Is(nativeErr, context.DeadlineExceeded) || native.ExitCode == 0 {
		t.Fatalf("native download while held=%+v/%v, want deadline cancellation without a completed download", native, nativeErr)
	}
	// Only after the holder and canceled child have joined may native Go finish
	// the same unexecuted download. It uses the retained ZIP, never a network.
	fixture.goCommand(t, "mod", "download", selected.Module.String()+"@"+selected.Version.String())
	if info, err := os.Stat(selected.Directory.String()); err != nil || !info.IsDir() {
		t.Fatalf("native download after release=%v/%v, want complete extracted directory", info, err)
	}
}
