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
		validate func() error
		name     string
	}{
		{name: "symbol ingress", validate: invalid.Validate},
		{name: "fact ingress", validate: (StandardSymbolFact{Symbol: invalid, Disposition: StandardSymbolUnresolved}).Validate},
		{name: "operation function ingress", validate: (OperationContract{Function: invalid, Result: valid.Selector, ResultPackage: core.PackageCore}).Validate},
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
				wantErr error
				mutate  func(*OperationContract)
				name    string
			}{
				{name: "unchanged callable", mutate: func(*OperationContract) {}, wantErr: nil},
				{name: "missing result package", mutate: func(c *OperationContract) { c.ResultPackage = core.PackageUnknown }, wantErr: core.ErrCapabilitiesContract},
				{name: "future result package", mutate: func(c *OperationContract) { c.ResultPackage = core.PackageIdentity(255) }, wantErr: core.ErrCapabilitiesContract},
				{name: "missing result type", mutate: func(c *OperationContract) { c.Result = SymbolName{} }, wantErr: core.ErrCapabilitiesContract},
				{name: "missing function selector", mutate: func(c *OperationContract) { c.Function.Selector = SymbolName{} }, wantErr: core.ErrCapabilitiesContract},
				{name: "missing function import", mutate: func(c *OperationContract) { c.Function.ImportPath = gomodule.ImportPath{} }, wantErr: core.ErrCapabilitiesContract},
				{name: "method cannot substitute package function", mutate: func(c *OperationContract) { receiver := original.Result; c.Function.Receiver = &receiver }, wantErr: core.ErrCapabilitiesContract},
				{name: "present empty receiver is not absent", mutate: func(c *OperationContract) { receiver := SymbolName{}; c.Function.Receiver = &receiver }, wantErr: core.ErrCapabilitiesContract},
				{name: "request presence flip", mutate: func(c *OperationContract) { c.HasRequest = !c.HasRequest }, wantErr: core.ErrCapabilitiesContract},
			}
			if original.HasRequest {
				cases = append(cases, struct {
					wantErr error
					mutate  func(*OperationContract)
					name    string
				}{name: "missing declared request", mutate: func(c *OperationContract) { c.Request = SymbolName{} }, wantErr: core.ErrCapabilitiesContract})
			} else {
				cases = append(cases, struct {
					wantErr error
					mutate  func(*OperationContract)
					name    string
				}{name: "unsolicited request", mutate: func(c *OperationContract) { c.Request = original.Result }, wantErr: core.ErrCapabilitiesContract})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					value := original
					tc.mutate(&value)
					if tc.wantErr != nil && value == original {
						t.Fatalf("mutation=%+v, want different from %+v", value, original)
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
