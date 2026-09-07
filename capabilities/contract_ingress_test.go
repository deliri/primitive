package capabilities

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

func TestSymbolContractsPreserveCrossPackageErrorIdentity(t *testing.T) {
	t.Parallel()
	valid := handoffSymbol(t)
	invalid := valid
	invalid.ImportPath = gomodule.ImportPath{}
	wantCause := invalid.ImportPath.Validate()
	cases := []struct {
		name     string
		validate func() error
	}{
		{"symbol ingress", invalid.Validate},
		{"fact ingress", (StandardSymbolFact{Symbol: invalid, Classification: Classification{Disposition: StandardSymbolUnresolved}}).Validate},
		{"operation function ingress", (OperationContract{Function: invalid, Result: valid.Selector, ResultPackage: core.PackageCore}).Validate},
	}
	// The shared nominal import-path sentinel remains visible across the package wall.
	if !errors.Is(wantCause, core.ErrGoModuleContract) {
		t.Fatalf("invalid import fixture = %v, want GoModule identity", wantCause)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.validate()
			if !errors.Is(err, core.ErrCapabilitiesContract) || !errors.Is(err, core.ErrGoModuleContract) {
				t.Fatalf("cross-package refusal = %v, want both identities", err)
			}
		})
	}
}

func TestOperationContractBoundaryMutations(t *testing.T) {
	t.Parallel()
	for _, operation := range operationDomain() {
		if operation == OperationUnavailable {
			continue
		}
		t.Run(operation.String(), func(t *testing.T) {
			t.Parallel()
			original, found, err := operation.Contract()
			if err != nil || !found {
				t.Fatalf("fixture = (%+v,%t,%v)", original, found, err)
			}
			cases := []struct {
				name    string
				mutate  func(*OperationContract)
				wantErr error
			}{
				{"unchanged callable", func(*OperationContract) {}, nil},
				{"missing result package", func(c *OperationContract) { c.ResultPackage = core.PackageUnknown }, core.ErrCapabilitiesContract},
				{"future result package", func(c *OperationContract) { c.ResultPackage = core.PackageIdentity(255) }, core.ErrCapabilitiesContract},
				{"missing result type", func(c *OperationContract) { c.Result = SymbolName{} }, core.ErrCapabilitiesContract},
				{"missing function selector", func(c *OperationContract) { c.Function.Selector = SymbolName{} }, core.ErrCapabilitiesContract},
				{"missing function import", func(c *OperationContract) { c.Function.ImportPath = gomodule.ImportPath{} }, core.ErrCapabilitiesContract},
				{"method cannot substitute package function", func(c *OperationContract) { receiver := original.Result; c.Function.Receiver = &receiver }, core.ErrCapabilitiesContract},
				{"present empty receiver is not absent", func(c *OperationContract) { receiver := SymbolName{}; c.Function.Receiver = &receiver }, core.ErrCapabilitiesContract},
				{"request presence flip", func(c *OperationContract) { c.HasRequest = !c.HasRequest }, core.ErrCapabilitiesContract},
			}
			if original.HasRequest {
				cases = append(cases, struct {
					name    string
					mutate  func(*OperationContract)
					wantErr error
				}{"missing declared request", func(c *OperationContract) { c.Request = SymbolName{} }, core.ErrCapabilitiesContract})
			} else {
				cases = append(cases, struct {
					name    string
					mutate  func(*OperationContract)
					wantErr error
				}{"unsolicited request", func(c *OperationContract) { c.Request = original.Result }, core.ErrCapabilitiesContract})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					value := original
					tc.mutate(&value)
					if tc.wantErr != nil && value == original {
						t.Fatal("mutation did not change input")
					}
					if err := value.Validate(); !errors.Is(err, tc.wantErr) {
						t.Fatalf("contract %+v = %v, want %v", value, err, tc.wantErr)
					}
				})
			}
		})
	}
}

func TestCatalogIteratorExactPrefixAndZero(t *testing.T) {
	t.Parallel()
	var zero Catalog
	if err := zero.Validate(); !errors.Is(err, core.ErrCapabilitiesContract) {
		t.Fatalf("zero catalog = %v", err)
	}
	for value := range zero.Capabilities() {
		t.Fatalf("zero catalog yielded %+v", value)
	}
	if got, err := zero.Resolve(ForEffect(ScopeProduction, EffectFilesystem)); !errors.Is(err, core.ErrCapabilitiesContract) || got != (Match{}) {
		t.Fatalf("zero catalog resolve = (%+v,%v)", got, err)
	}
	catalog, err := All()
	if err != nil {
		t.Fatal(err)
	}
	var want []Capability
	for contract := range core.PrimitiveArchitecture().Packages() {
		want = append(want, Capability{Package: contract.Identity, Kind: contract.Kind, Role: contract.Role})
	}
	for stop := 1; stop <= len(want); stop++ {
		count := 0
		for got := range catalog.Capabilities() {
			if got != want[count] {
				t.Fatalf("prefix position %d = %+v, want %+v", count, got, want[count])
			}
			count++
			if count == stop {
				break
			}
		}
		if count != stop {
			t.Fatalf("iterator stop %d yielded %d", stop, count)
		}
	}
}
