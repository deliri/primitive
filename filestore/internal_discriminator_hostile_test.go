package filestore

import (
	"encoding/json"
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
			(destination.String() != core.UnknownEnumDiagnostic) != wantDestination {
			t.Fatalf("streamDestination(%d) validity/text = (%t, %q), want admitted=%t",
				raw, destination.IsValid(), destination.String(), wantDestination)
		}
		position := directoryPosition(raw)
		wantPosition := position == directoryIntermediate || position == directoryFinal
		if position.IsValid() != wantPosition ||
			(position.String() != core.UnknownEnumDiagnostic) != wantPosition {
			t.Fatalf("directoryPosition(%d) validity/text = (%t, %q), want admitted=%t",
				raw, position.IsValid(), position.String(), wantPosition)
		}
		if destination.IsValid() && (destination.Validate() != nil) {
			t.Fatalf("streamDestination(%d).Validate() error = %v, want nil",
				raw, destination.Validate())
		}
		if !destination.IsValid() && !errors.Is(destination.Validate(), core.ErrFilestoreContract) {
			t.Fatalf("streamDestination(%d).Validate() error = %v, want %v",
				raw, destination.Validate(), core.ErrFilestoreContract)
		}
		if position.IsValid() && (position.Validate() != nil) {
			t.Fatalf("directoryPosition(%d).Validate() error = %v, want nil",
				raw, position.Validate())
		}
		if !position.IsValid() && !errors.Is(position.Validate(), core.ErrFilestoreContract) {
			t.Fatalf("directoryPosition(%d).Validate() error = %v, want %v",
				raw, position.Validate(), core.ErrFilestoreContract)
		}
	}
	proveInternalDiscriminatorsStayOffWire(t, streamDestinationFile, directoryFinal)
}

// proveInternalDiscriminatorsStayOffWire proves the claim each internal
// discriminator's off-wire marker makes. Both select filesystem behavior:
// streamDestination picks which failure identity a bounded copy reports, and
// directoryPosition picks whether a chain element may already exist. Giving
// either a JSON encoding would let a decoded document choose that behavior. The
// package's external off-wire sweep cannot name these two, so the same proof is
// extended to them here. Adding MarshalJSON or UnmarshalJSON turns this red.
func proveInternalDiscriminatorsStayOffWire(
	t *testing.T,
	destination streamDestination,
	position directoryPosition,
) {
	t.Helper()

	destination.OffWireEnum()
	position.OffWireEnum()
	if _, encodes := any(destination).(json.Marshaler); encodes {
		t.Fatalf("streamDestination(%d) implements json.Marshaler, want an off-wire enum", destination)
	}
	if _, decodes := any(&destination).(json.Unmarshaler); decodes {
		t.Fatalf("*streamDestination(%d) implements json.Unmarshaler, want an off-wire enum", destination)
	}
	if _, encodes := any(position).(json.Marshaler); encodes {
		t.Fatalf("directoryPosition(%d) implements json.Marshaler, want an off-wire enum", position)
	}
	if _, decodes := any(&position).(json.Unmarshaler); decodes {
		t.Fatalf("*directoryPosition(%d) implements json.Unmarshaler, want an off-wire enum", position)
	}
}
