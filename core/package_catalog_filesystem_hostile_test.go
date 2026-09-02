package core

import (
	"errors"
	"os"
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
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, errors.Join(ErrPrimitiveContract, err)
	}
	packages := make([]string, 0, PrimitivePackageCount)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "_docs" {
			continue
		}
		landed, err := directoryHasGoPackage(root + "/" + entry.Name())
		if err != nil {
			return nil, err
		}
		if !landed {
			continue
		}
		identity, err := ParsePackageIdentity(entry.Name())
		if err != nil {
			return nil, errors.Join(ErrPrimitiveContract, err)
		}
		packages = append(packages, identity.String())
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
