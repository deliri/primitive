package gotoolchain

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
)

type moduleSumKey struct {
	module  gomodule.Path
	version ModuleVersion
	goMod   bool
}

func (moduleSumKey) goToolchainInternalFlow() {}

// This is a checksum index of the caller's one frozen go.sum, not a module
// graph. Order preserves first occurrence; duplicates cannot select a hash.
type moduleSumIndex struct {
	order    []moduleSumKey
	expected map[moduleSumKey]ModuleChecksum
}

func (moduleSumIndex) goToolchainInternalFlow() {}

func readModuleSums(ctx context.Context, source io.Reader) (moduleSumIndex, error) {
	index := moduleSumIndex{expected: make(map[moduleSumKey]ModuleChecksum)}
	reader := bufio.NewReader(source)
	var fields [3]strings.Builder
	used, active := 0, false
	finish := func() error {
		if used == 0 {
			return nil
		}
		if used != len(fields) {
			return core.ErrGoToolchainOutput
		}
		path, err := gomodule.ParsePath(fields[0].String())
		if err != nil {
			return err
		}
		text := fields[1].String()
		versionText, goMod := strings.CutSuffix(text, "/go.mod")
		version, err := ParseModuleVersion(versionText)
		if err != nil {
			return err
		}
		if err := module.Check(path.String(), version.String()); err != nil {
			return err
		}
		sum, err := ParseModuleChecksum(fields[2].String())
		if err != nil {
			return err
		}
		key := moduleSumKey{module: path, version: version, goMod: goMod}
		if previous, found := index.expected[key]; found {
			if previous != sum {
				return core.ErrGoToolchainOutput
			}
		} else {
			index.order = append(index.order, key)
			index.expected[key] = sum
		}
		for i := range fields {
			fields[i].Reset()
		}
		used, active = 0, false
		return nil
	}
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return moduleSumIndex{}, err
		}
		value, width, err := reader.ReadRune()
		if err != nil && err != io.EOF {
			return moduleSumIndex{}, errors.Join(core.ErrGoToolchainOutput, err)
		}
		if err == io.EOF || value == '\n' {
			if parseErr := finish(); parseErr != nil {
				return moduleSumIndex{}, errors.Join(core.ErrGoToolchainOutput, parseErr)
			}
			if err == io.EOF {
				return index, nil
			}
			continue
		}
		if value == utf8.RuneError && width == 1 {
			return moduleSumIndex{}, core.ErrGoToolchainOutput
		}
		if unicode.IsSpace(value) {
			active = false
			continue
		}
		if !active {
			used++
			active = true
		}
		if used > len(fields) {
			return moduleSumIndex{}, core.ErrGoToolchainOutput
		}
		fields[used-1].WriteRune(value)
	}
}

func moduleCacheLocations(cache core.AbsolutePath, path gomodule.Path, version ModuleVersion) (SelectedModuleObservation, error) {
	escapedPath, err := module.EscapePath(path.String())
	if err != nil {
		return SelectedModuleObservation{}, err
	}
	escapedVersion, err := module.EscapeVersion(version.String())
	if err != nil {
		return SelectedModuleObservation{}, err
	}
	prefix := filepath.Join(cache.String(), "cache", "download", escapedPath, "@v", escapedVersion)
	var observed SelectedModuleObservation
	observed.Module, observed.Version = path, version
	observed.Directory, err = core.ParseAbsolutePath(filepath.Join(cache.String(), escapedPath+"@"+escapedVersion))
	if err == nil {
		observed.GoMod, err = core.ParseAbsolutePath(prefix + ".mod")
	}
	if err == nil {
		observed.Info, err = core.ParseAbsolutePath(prefix + ".info")
	}
	if err == nil {
		observed.ZipHash, err = core.ParseAbsolutePath(prefix + ".ziphash")
	}
	return observed, err
}

type moduleContextReader struct {
	ctx    context.Context
	source io.Reader
}

func (moduleContextReader) goToolchainInternalFlow() {}

func (r moduleContextReader) Read(data []byte) (int, error) {
	if err := contextstate.Validate(r.ctx); err != nil {
		return 0, err
	}
	n, err := r.source.Read(data)
	return n, errors.Join(err, contextstate.Validate(r.ctx))
}

func verifyGoModSum(ctx context.Context, path core.AbsolutePath, expected ModuleChecksum) (err error) {
	info, err := os.Lstat(path.String())
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return core.ErrGoToolchainOutput
	}
	file, err := os.Open(path.String())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	actual, err := dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) {
		return io.NopCloser(moduleContextReader{ctx: ctx, source: file}), nil
	})
	if err != nil || actual != expected.String() {
		return errors.Join(core.ErrGoToolchainOutput, err)
	}
	return contextstate.Validate(ctx)
}

type goModMetadataVisit struct {
	cache core.AbsolutePath
	sums  moduleSumIndex
	visit func(context.Context, GoModMetadataObservation) error
}

func (goModMetadataVisit) goToolchainInternalFlow() {}

func visitGoModMetadata(ctx context.Context, input goModMetadataVisit) (uint64, error) {
	var count uint64
	for _, key := range input.sums.order {
		if !key.goMod {
			continue
		}
		location, err := moduleCacheLocations(input.cache, key.module, key.version)
		if err != nil {
			return 0, errors.Join(core.ErrGoToolchainOutput, err)
		}
		sum := input.sums.expected[key]
		if _, err := os.Lstat(location.GoMod.String()); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return 0, errors.Join(core.ErrGoToolchainOutput, err)
		}
		if err := verifyGoModSum(ctx, location.GoMod, sum); err != nil {
			return 0, errors.Join(core.ErrGoToolchainOutput, err)
		}
		observed := GoModMetadataObservation{Module: key.module, Version: key.version, File: location.GoMod, Sum: sum}
		if err := input.visit(ctx, observed); err != nil {
			return 0, err
		}
		count++
	}
	return count, contextstate.Validate(ctx)
}

func readCacheChecksum(ctx context.Context, path core.AbsolutePath) (value ModuleChecksum, err error) {
	file, err := os.Open(path.String())
	if err != nil {
		return value, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	reader := bufio.NewReader(moduleContextReader{ctx: ctx, source: file})
	var hash [47]byte
	used, ended := 0, false
	for {
		r, width, err := reader.ReadRune()
		if err == io.EOF {
			return ParseModuleChecksum(string(hash[:used]))
		}
		if err != nil {
			return value, err
		}
		if unicode.IsSpace(r) {
			ended = used != 0
			continue
		}
		if ended || width != 1 || used == len(hash) {
			return value, core.ErrGoToolchainOutput
		}
		hash[used] = byte(r)
		used++
	}
}
