package upgrade

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/deliri/primitive/v2026/core"
)

const (
	selectionFilename                      = ".primitive-primary.json"
	selectionTemporaryFilename             = ".primitive-primary.next"
	trialFilename                          = ".primitive-trial.json"
	trialTemporaryFilename                 = ".primitive-trial.next"
	directoryMode              fs.FileMode = 0o700
	executableMode             fs.FileMode = 0o700
	documentMode               fs.FileMode = 0o600
	// selectionDocumentMaximumBytes bounds the only document Upgrade owns. The
	// encoded selection is one revision, one slot token, and one Release
	// artifact. Upgrade states its own bound rather than borrowing Release's
	// document bound for a document Release does not own.
	selectionDocumentMaximumBytes = 8 << 10
	// trialDocumentMaximumBytes bounds one prior selection and one candidate
	// artifact. It is independent from Release's signed document bounds.
	trialDocumentMaximumBytes = 16 << 10
	// selectionArrayItemMaximum is the smallest array bound the strict decoder
	// admits. The selection document contains no array at any nesting depth.
	selectionArrayItemMaximum = 1
)

func validateRoot(root *os.Root, directory core.AbsolutePath) error {
	if root == nil {
		return contractError(diagnosticRoot)
	}
	if err := directory.Validate(); err != nil {
		return contractError(diagnosticRoot, err)
	}
	rootPath, err := core.ParseAbsolutePath(root.Name())
	if err != nil || rootPath != directory {
		return contractError(diagnosticRoot, err)
	}
	return nil
}

func binaryPath(slot Slot, build core.BuildIdentity) (core.RelativePath, error) {
	if err := slot.Validate(); err != nil {
		return core.RelativePath{}, err
	}
	component, err := binaryComponent(build)
	if err != nil {
		return core.RelativePath{}, err
	}
	directory, err := core.ParseRelativePath(slot.String())
	if err != nil {
		return core.RelativePath{}, contractError(err)
	}
	return directory.Join(component)
}

func binaryComponent(build core.BuildIdentity) (core.PathComponent, error) {
	if err := build.Validate(); err != nil {
		return core.PathComponent{}, contractError(err)
	}
	name := build.Offering().String()
	if build.Platform().OperatingSystem == core.OperatingSystemWindows {
		name += ".exe"
	}
	component, err := core.ParsePathComponent(name)
	if err != nil {
		return core.PathComponent{}, contractError(err)
	}
	return component, nil
}

func absoluteBinaryPath(
	directory core.AbsolutePath,
	slot Slot,
	build core.BuildIdentity,
) (core.AbsolutePath, error) {
	if err := slot.Validate(); err != nil {
		return core.AbsolutePath{}, err
	}
	slotComponent, err := core.ParsePathComponent(slot.String())
	if err != nil {
		return core.AbsolutePath{}, contractError(err)
	}
	slotDirectory, err := directory.Join(slotComponent)
	if err != nil {
		return core.AbsolutePath{}, contractError(err)
	}
	binary, err := binaryComponent(build)
	if err != nil {
		return core.AbsolutePath{}, err
	}
	command, err := slotDirectory.Join(binary)
	if err != nil {
		return core.AbsolutePath{}, contractError(err)
	}
	return command, nil
}

func slotPath(slot Slot) (core.RelativePath, error) {
	if err := slot.Validate(); err != nil {
		return core.RelativePath{}, err
	}
	return core.ParseRelativePath(slot.String())
}

func selectionPath() (core.RelativePath, error) {
	path, err := core.ParseRelativePath(selectionFilename)
	if err != nil {
		return core.RelativePath{}, contractError(err)
	}
	return path, nil
}

func selectionTemporaryPath() (core.RelativePath, error) {
	path, err := core.ParseRelativePath(selectionTemporaryFilename)
	if err != nil {
		return core.RelativePath{}, contractError(err)
	}
	return path, nil
}

func trialPath(slot Slot) (core.RelativePath, error) {
	if err := slot.Validate(); err != nil {
		return core.RelativePath{}, err
	}
	path, err := core.ParseRelativePath(
		filepath.Join(slot.String(), trialFilename),
	)
	if err != nil {
		return core.RelativePath{}, contractError(err)
	}
	return path, nil
}

func trialTemporaryPath(slot Slot) (core.RelativePath, error) {
	if err := slot.Validate(); err != nil {
		return core.RelativePath{}, err
	}
	path, err := core.ParseRelativePath(
		filepath.Join(slot.String(), trialTemporaryFilename),
	)
	if err != nil {
		return core.RelativePath{}, contractError(err)
	}
	return path, nil
}

// recoveryContext preserves only the authority needed to settle an effect
// that already created package-owned filesystem state.
func recoveryContext(ctx context.Context) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithoutCancel(ctx)
}
