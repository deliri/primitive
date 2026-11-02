package gotoolchain

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/gomodule"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
)

type moduleSelectionWire struct {
	path, version, directory, goMod, sum, goModSum string
	main, refusal                                  bool
}

func (moduleSelectionWire) goToolchainInternalFlow() {}

type moduleJSONCursor struct {
	ctx     context.Context
	decoder *jsontext.Decoder
}

func (moduleJSONCursor) goToolchainInternalFlow() {}

func (c moduleJSONCursor) token() (jsontext.Token, error) {
	if err := contextstate.Validate(c.ctx); err != nil {
		return jsontext.Token{}, err
	}
	token, err := c.decoder.ReadToken()
	if stateErr := contextstate.Validate(c.ctx); stateErr != nil {
		return token, errors.Join(err, stateErr)
	}
	return token, err
}

// Skip ignored native populations token by token; no Versions/Origin/Error
// object or collection becomes a product-side graph or retained JSON value.
func (c moduleJSONCursor) skip() error {
	depth := c.decoder.StackDepth()
	for {
		if _, err := c.token(); err != nil {
			return err
		}
		if c.decoder.StackDepth() == depth {
			return nil
		}
	}
}

func readModuleSelection(c moduleJSONCursor) (moduleSelectionWire, error) {
	first, err := c.token()
	if err != nil {
		return moduleSelectionWire{}, err
	}
	if first.Kind() != jsontext.KindBeginObject {
		return moduleSelectionWire{}, core.ErrGoToolchainOutput
	}
	var wire moduleSelectionWire
	for {
		key, keyErr := c.token()
		if keyErr != nil {
			return moduleSelectionWire{}, errors.Join(core.ErrGoToolchainOutput, keyErr)
		}
		if key.Kind() == jsontext.KindEndObject {
			return wire, nil
		}
		if key.Kind() != jsontext.KindString {
			return moduleSelectionWire{}, core.ErrGoToolchainOutput
		}
		var destination *string
		switch key.String() {
		case "Path":
			destination = &wire.path
		case "Version":
			destination = &wire.version
		case "Dir":
			destination = &wire.directory
		case "GoMod":
			destination = &wire.goMod
		case "Sum":
			destination = &wire.sum
		case "GoModSum":
			destination = &wire.goModSum
		case "Main":
			value, valueErr := c.token()
			if valueErr != nil || value.Kind() != jsontext.KindTrue && value.Kind() != jsontext.KindFalse {
				return moduleSelectionWire{}, errors.Join(core.ErrGoToolchainOutput, valueErr)
			}
			wire.main = value.Kind() == jsontext.KindTrue
		case "Replace", "Error":
			// A non-null replacement/error cannot lend immutable selected
			// cache authority, even when native Go exited successfully.
			if c.decoder.PeekKind() != jsontext.KindNull {
				wire.refusal = true
			}
			if err := c.skip(); err != nil {
				return moduleSelectionWire{}, errors.Join(core.ErrGoToolchainOutput, err)
			}
		default:
			if err := c.skip(); err != nil {
				return moduleSelectionWire{}, errors.Join(core.ErrGoToolchainOutput, err)
			}
		}
		if destination != nil {
			value, valueErr := c.token()
			if valueErr != nil || value.Kind() != jsontext.KindString {
				return moduleSelectionWire{}, errors.Join(core.ErrGoToolchainOutput, valueErr)
			}
			*destination = value.String()
		}
	}
}

type moduleSelectionVisit struct {
	source io.Reader
	cache  core.AbsolutePath
	sums   moduleSumIndex
	visit  func(context.Context, SelectedModuleObservation) error
}

func (moduleSelectionVisit) goToolchainInternalFlow() {}

func visitSelectedModules(ctx context.Context, input moduleSelectionVisit) (uint64, error) {
	cursor := moduleJSONCursor{ctx: ctx, decoder: jsontext.NewDecoder(input.source)}
	var count uint64
	seen := make(map[gomodule.Path]bool)
	mainSeen := false
	for {
		wire, err := readModuleSelection(cursor)
		if err == io.EOF {
			if !mainSeen {
				return 0, core.ErrGoToolchainOutput
			}
			return count, contextstate.Validate(ctx)
		}
		if err != nil || wire.refusal {
			return 0, errors.Join(core.ErrGoToolchainOutput, err)
		}
		if wire.main {
			if mainSeen || wire.version != "" || wire.path == "" {
				return 0, core.ErrGoToolchainOutput
			}
			mainSeen = true
			continue
		}
		observed, err := projectSelectedModule(ctx, wire, input)
		if err != nil {
			return 0, errors.Join(core.ErrGoToolchainOutput, err)
		}
		if seen[observed.Module] {
			return 0, core.ErrGoToolchainOutput
		}
		seen[observed.Module] = true
		if err := input.visit(ctx, observed); err != nil {
			return 0, err
		}
		count++
	}
}

func projectSelectedModule(ctx context.Context, wire moduleSelectionWire, input moduleSelectionVisit) (SelectedModuleObservation, error) {
	path, err := gomodule.ParsePath(wire.path)
	if err != nil {
		return SelectedModuleObservation{}, err
	}
	version, err := ParseModuleVersion(wire.version)
	if err != nil {
		return SelectedModuleObservation{}, err
	}
	if err := module.Check(path.String(), version.String()); err != nil {
		return SelectedModuleObservation{}, err
	}
	observed, err := moduleCacheLocations(input.cache, path, version)
	if err != nil {
		return SelectedModuleObservation{}, err
	}
	observed.Sum, err = ParseModuleChecksum(wire.sum)
	if err == nil {
		observed.GoModSum, err = ParseModuleChecksum(wire.goModSum)
	}
	if err != nil || wire.directory != observed.Directory.String() || wire.goMod != observed.GoMod.String() {
		return SelectedModuleObservation{}, errors.Join(core.ErrGoToolchainOutput, err)
	}
	key := moduleSumKey{module: path, version: version}
	if input.sums.expected[key] != observed.Sum {
		return SelectedModuleObservation{}, core.ErrGoToolchainOutput
	}
	key.goMod = true
	if input.sums.expected[key] != observed.GoModSum {
		return SelectedModuleObservation{}, core.ErrGoToolchainOutput
	}
	if err := verifySelectedModule(ctx, observed); err != nil {
		return SelectedModuleObservation{}, err
	}
	return observed, observed.Validate()
}

func verifySelectedModule(ctx context.Context, observed SelectedModuleObservation) error {
	kind, err := moduleCachePathKind(ctx, observed.Directory)
	if err != nil {
		return err
	}
	if kind != filestore.PathKindDirectory {
		return core.ErrGoToolchainOutput
	}
	partial, err := core.ParseAbsolutePath(strings.TrimSuffix(observed.GoMod.String(), ".mod") + ".partial")
	if err != nil {
		return err
	}
	partialKind, err := moduleCachePathKind(ctx, partial)
	if err != nil || partialKind != filestore.PathKindAbsent && partialKind != filestore.PathKindUnreachable {
		return errors.Join(core.ErrGoToolchainOutput, err)
	}
	hash, err := readCacheChecksum(ctx, observed.ZipHash)
	if err != nil || hash != observed.Sum {
		return errors.Join(core.ErrGoToolchainOutput, err)
	}
	var identity struct{ Version string }
	if err := withModuleCacheFile(ctx, observed.Info, func(source io.Reader) error { return json.UnmarshalRead(source, &identity) }); err != nil || identity.Version != observed.Version.String() {
		return errors.Join(core.ErrGoToolchainOutput, err)
	}
	if err := verifyGoModSum(ctx, observed.GoMod, observed.GoModSum); err != nil {
		return err
	}
	actual, err := dirhash.HashDir(observed.Directory.String(), observed.Module.String()+"@"+observed.Version.String(), dirhash.Hash1)
	if err != nil || actual != observed.Sum.String() {
		return errors.Join(core.ErrGoToolchainOutput, err)
	}
	return contextstate.Validate(ctx)
}
