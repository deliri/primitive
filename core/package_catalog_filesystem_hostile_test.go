package core

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestPrimitiveArchitectureCatalogMatchesEveryLandedPackage makes the package
// role catalog an exact ratchet against the repository. A new package cannot
// land without a compiler-owned identity, purpose, kind, and primary role; a
// deleted package cannot survive as stale catalog prose.
func TestPrimitiveArchitectureCatalogMatchesEveryLandedPackage(t *testing.T) {
	t.Parallel()

	got, err := landedPackageNames("..")
	if err != nil {
		t.Fatalf("landedPackageNames() error = %v, want nil", err)
	}
	want := make([]string, 0, PrimitivePackageCount)
	for contract := range PrimitiveArchitecture().Packages() {
		want = append(want, contract.Identity.String())
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("landed package names = %v, want exact architecture catalog %v", got, want)
	}
}

func landedPackageNames(root string) ([]string, error) {
	packages := make([]string, 0, PrimitivePackageCount)
	err := filepath.WalkDir(root, func(directory string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.Join(ErrPrimitiveContract, walkErr)
		}
		if !entry.IsDir() || directory == root {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") || entry.Name() == "vendor" || entry.Name() == "testdata" {
			return filepath.SkipDir
		}
		landed, err := directoryHasGoPackage(directory)
		if err != nil {
			return err
		}
		if !landed {
			return nil
		}
		name, err := filepath.Rel(root, directory)
		if err != nil {
			return err
		}
		identity, err := ParsePackageIdentity(filepath.ToSlash(name))
		if err != nil {
			return errors.Join(ErrPrimitiveContract, err)
		}
		packages = append(packages, identity.String())
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(packages)
	return packages, nil
}

func directoryHasGoPackage(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, errors.Join(ErrPrimitiveContract, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true, nil
		}
	}
	return false, nil
}
