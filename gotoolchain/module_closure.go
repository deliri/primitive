package gotoolchain

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/process"
)

// ModuleChecksum is Go's h1 directory/file-list checksum, not a raw-file SHA.
type ModuleChecksum struct{ value string }

func ParseModuleChecksum(text string) (ModuleChecksum, error) {
	if len(text) != 47 || !strings.HasPrefix(text, "h1:") {
		return ModuleChecksum{}, core.ErrGoToolchainContract
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(text[3:])
	if err != nil || len(raw) != 32 || base64.StdEncoding.EncodeToString(raw) != text[3:] {
		return ModuleChecksum{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	return ModuleChecksum{value: text}, nil
}

func (s ModuleChecksum) String() string { return s.value }
func (s ModuleChecksum) Validate() error {
	_, err := ParseModuleChecksum(s.value)
	return err
}
func (ModuleChecksum) goToolchainSealedValue() {}

// SelectedModuleObservation borrows one complete native cache entry. Paths are
// derived from Go's observed cache root and canonical coordinates, never from
// an unchecked native output path. Info has identity, not an h1 commitment.
type SelectedModuleObservation struct {
	Module    gomodule.Path
	Version   ModuleVersion
	Directory core.AbsolutePath
	GoMod     core.AbsolutePath
	Info      core.AbsolutePath
	ZipHash   core.AbsolutePath
	Sum       ModuleChecksum
	GoModSum  ModuleChecksum
}

func (o SelectedModuleObservation) Validate() error {
	return errors.Join(o.Module.Validate(), o.Version.Validate(), o.Directory.Validate(), o.GoMod.Validate(), o.Info.Validate(), o.ZipHash.Validate(), o.Sum.Validate(), o.GoModSum.Validate())
}
func (SelectedModuleObservation) goToolchainProtocolFact() {}

// GoModMetadataObservation is an available go.sum-pinned .mod, including a
// version that did not win MVS. Absent historical .mod files are not invented.
type GoModMetadataObservation struct {
	Module  gomodule.Path
	Version ModuleVersion
	File    core.AbsolutePath
	Sum     ModuleChecksum
}

func (o GoModMetadataObservation) Validate() error {
	return errors.Join(o.Module.Validate(), o.Version.Validate(), o.File.Validate(), o.Sum.Validate())
}
func (GoModMetadataObservation) goToolchainProtocolFact() {}

type ModuleClosureRequest struct {
	WorkingDirectory core.AbsolutePath
	// GoSum is the caller's frozen original go.sum, or an empty reader when
	// absent. This door neither edits it nor supplies missing expected sums.
	GoSum         io.Reader
	VisitSelected func(context.Context, SelectedModuleObservation) error
	VisitGoMod    func(context.Context, GoModMetadataObservation) error
}

func (r ModuleClosureRequest) Validate() error {
	if core.ReaderIsNil(r.GoSum) || r.VisitSelected == nil || r.VisitGoMod == nil {
		return core.ErrGoToolchainContract
	}
	return r.WorkingDirectory.Validate()
}
func (ModuleClosureRequest) goToolchainProtocolFact() {}

type ModuleClosureObservation struct {
	ModuleCache    core.AbsolutePath
	Environment    process.ResultObservation
	Modules        process.ResultObservation
	Verification   process.ResultObservation
	Selected       uint64
	AvailableGoMod uint64
}

func (o ModuleClosureObservation) Validate() error {
	if err := errors.Join(o.ModuleCache.Validate(), o.Environment.Validate(), o.Modules.Validate(), o.Verification.Validate()); err != nil {
		return err
	}
	if o.Environment.ExitCode != 0 || o.Modules.ExitCode != 0 || o.Verification.ExitCode != 0 {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (ModuleClosureObservation) goToolchainProtocolFact() {}

// ObserveModuleClosure runs the exact configured Go environment, selected
// module query and native cache verifier, then borrows selected entries and
// available expected .mod files synchronously. It refuses replacements, missing
// selected entries and checksum disagreement. Native output is streamed from
// the existing completed-output scratch loan; go.sum indexing is SDK-private.
//
// Callbacks are provisional until success, may not outlive this call, and do
// not certify transport or offline closure. The caller must preserve its module
// files, freeze all borrowed content, account for its bytes and prove isolated
// offline graph replay. No network/cache repair or mutation retry is added.
func (c Capability) ObserveModuleClosure(ctx context.Context, request ModuleClosureRequest) (ModuleClosureObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return ModuleClosureObservation{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	if c.configuration.Workspace != WorkspaceModeDisabled {
		return ModuleClosureObservation{}, core.ErrGoToolchainContract
	}
	sums, err := readModuleSums(ctx, request.GoSum)
	if err != nil {
		return ModuleClosureObservation{}, err
	}
	cacheText, environment, err := c.execute(ctx, request.WorkingDirectory, "env", "GOMODCACHE")
	if err != nil {
		return ModuleClosureObservation{}, err
	}
	cache, err := core.ParseAbsolutePath(strings.TrimSuffix(string(cacheText), "\n"))
	if err != nil || strings.ContainsAny(cache.String(), "\r\n") {
		return ModuleClosureObservation{}, errors.Join(core.ErrGoToolchainOutput, err)
	}
	environmentObservation, err := environment.Observation()
	if err != nil {
		return ModuleClosureObservation{}, err
	}
	observed := ModuleClosureObservation{ModuleCache: cache, Environment: environmentObservation}
	err = c.withCompletedCommandOutput(ctx, request.WorkingDirectory, []string{"list", "-mod=readonly", "-m", "-json", "all"}, func(source io.Reader, native process.ResultObservation) error {
		_, verified, verifyErr := c.execute(ctx, request.WorkingDirectory, "mod", "verify")
		if verifyErr != nil {
			return verifyErr
		}
		verification, err := verified.Observation()
		if err != nil {
			return err
		}
		observed.Modules, observed.Verification = native, verification
		selected, err := visitSelectedModules(ctx, moduleSelectionVisit{source: source, cache: cache, sums: sums, visit: request.VisitSelected})
		if err != nil {
			return err
		}
		observed.Selected = selected
		observed.AvailableGoMod, err = visitGoModMetadata(ctx, goModMetadataVisit{cache: cache, sums: sums, visit: request.VisitGoMod})
		return err
	})
	if err := errors.Join(err, contextstate.Validate(ctx)); err != nil {
		return ModuleClosureObservation{}, err
	}
	return observed, observed.Validate()
}
