package filestore

import (
	"errors"
	"io/fs"
	"math"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestInternalDiscriminatorsRefuseUnknownInsteadOfSelectingBehavior(t *testing.T) {
	t.Parallel()

	t.Run("destination identity", func(t *testing.T) {
		t.Parallel()
		native := errors.New("destination refused bytes")
		gotErr := classifyDestinationError(streamDestination(0), native)
		if !errors.Is(gotErr, core.ErrFilestoreContract) ||
			errors.Is(gotErr, core.ErrFilestoreDestination) ||
			errors.Is(gotErr, core.ErrFilestoreActivation) ||
			!errors.Is(gotErr, native) {
			t.Fatalf("classifyDestinationError(unknown) = %v, want contract and native identity only", gotErr)
		}
	})

	t.Run("directory position", func(t *testing.T) {
		t.Parallel()
		rootPath := t.TempDir()
		if err := os.Mkdir(rootPath+"/entry", 0o700); err != nil {
			t.Fatalf("os.Mkdir() error = %v, want nil", err)
		}
		root, err := os.OpenRoot(rootPath)
		if err != nil {
			t.Fatalf("os.OpenRoot() error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = root.Close() })
		path, err := core.ParseRelativePath("entry")
		if err != nil {
			t.Fatalf("core.ParseRelativePath() error = %v, want nil", err)
		}
		gotErr := ensureDirectoryEntry(root, path, fs.FileMode(0o750), directoryPosition(0))
		if !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("ensureDirectoryEntry(unknown position) error = %v, want %v", gotErr, core.ErrFilestoreContract)
		}
	})
}

func TestInternalDiscriminatorDomainsExhaustBackingType(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		destination := streamDestination(raw)
		wantDestination := destination == streamDestinationCaller ||
			destination == streamDestinationFile
		if destination.IsValid() != wantDestination ||
			(destination.String() != unknownEnumDiagnostic) != wantDestination {
			t.Fatalf("streamDestination(%d) validity/text = (%t, %q), want admitted=%t",
				raw, destination.IsValid(), destination.String(), wantDestination)
		}
		position := directoryPosition(raw)
		wantPosition := position == directoryIntermediate || position == directoryFinal
		if position.IsValid() != wantPosition ||
			(position.String() != unknownEnumDiagnostic) != wantPosition {
			t.Fatalf("directoryPosition(%d) validity/text = (%t, %q), want admitted=%t",
				raw, position.IsValid(), position.String(), wantPosition)
		}
	}
}
