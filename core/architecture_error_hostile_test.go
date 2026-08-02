package core

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestPrimitiveArchitectureExactCatalogRatchet(t *testing.T) {
	t.Parallel()

	catalog := PrimitiveArchitecture()
	if gotErr := catalog.Validate(); gotErr != nil {
		t.Fatalf("PrimitiveArchitecture().Validate() error = %v, want nil", gotErr)
	}
	gotPackages := 0
	for contract := range catalog.Packages() {
		gotPackages++
		if gotErr := contract.Validate(); gotErr != nil {
			t.Errorf("PackageContract(%v).Validate() error = %v, want nil", contract.Identity, gotErr)
		}
		gotName, gotNameErr := contract.Identity.Name()
		if gotNameErr != nil || gotName == "" {
			t.Errorf("PackageIdentity(%d).Name() = (%q, %v), want nonempty/nil", contract.Identity, gotName, gotNameErr)
		}
		gotPath, gotPathErr := contract.Identity.ImportPath()
		wantPath := PrimitiveModulePath + "/" + gotName
		if gotPathErr != nil || gotPath != wantPath {
			t.Errorf("PackageIdentity(%d).ImportPath() = (%q, %v), want (%q, nil)", contract.Identity, gotPath, gotPathErr, wantPath)
		}
	}
	if gotPackages != PrimitivePackageCount {
		t.Fatalf("catalog package count = %d, want %d", gotPackages, PrimitivePackageCount)
	}
	gotImports := 0
	for contract := range catalog.DirectImports() {
		gotImports++
		if gotErr := contract.Validate(); gotErr != nil {
			t.Errorf("DirectImportContract(%v -> %v).Validate() error = %v, want nil", contract.Importer, contract.Imported, gotErr)
		}
	}
	if gotImports != PrimitiveDirectImportCount {
		t.Fatalf("catalog direct import count = %d, want %d", gotImports, PrimitiveDirectImportCount)
	}
	gotTestImports := 0
	for contract := range catalog.DirectTestImports() {
		gotTestImports++
		if gotErr := contract.Validate(); gotErr != nil {
			t.Errorf(
				"DirectTestImportContract(%v -> %v).Validate() error = %v, want nil",
				contract.Importer, contract.Imported, gotErr,
			)
		}
		if catalog.ContainsDirectImport(DirectImportContract(contract)) {
			t.Errorf(
				"DirectTestImportContract(%v -> %v) duplicates a production edge",
				contract.Importer, contract.Imported,
			)
		}
		if !catalog.ContainsDirectTestImport(contract) {
			t.Errorf(
				"ContainsDirectTestImport(%v -> %v) = false, want true",
				contract.Importer, contract.Imported,
			)
		}
	}
	if gotTestImports != PrimitiveDirectTestImportCount {
		t.Fatalf(
			"catalog direct test import count = %d, want %d",
			gotTestImports, PrimitiveDirectTestImportCount,
		)
	}
}

func TestArchitectureCatalogRejectsEveryStructuralFailureMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		mutate  func(*ArchitectureCatalog)
		name    string
	}{
		{name: "zero catalog has missing package identities", mutate: func(c *ArchitectureCatalog) { *c = ArchitectureCatalog{} }, wantErr: ErrPrimitiveContract},
		{name: "unknown package identity replaces core", mutate: func(c *ArchitectureCatalog) { c.packages[0].Identity = PackageUnknown }, wantErr: ErrPrimitiveContract},
		{name: "future package identity exceeds closed domain", mutate: func(c *ArchitectureCatalog) { c.packages[0].Identity = packageIdentityLimit }, wantErr: ErrPrimitiveContract},
		{name: "duplicate package identity removes one owner", mutate: func(c *ArchitectureCatalog) { c.packages[1].Identity = PackageCore }, wantErr: ErrPrimitiveContract},
		{name: "production package marked test support", mutate: func(c *ArchitectureCatalog) { c.packages[1].Kind = PackageKindTestSupport }, wantErr: ErrPrimitiveContract},
		{name: "testserial marked production", mutate: func(c *ArchitectureCatalog) { c.packages[6].Kind = PackageKindProduction }, wantErr: ErrPrimitiveContract},
		{name: "core imports a sibling", mutate: func(c *ArchitectureCatalog) { c.imports[0].Importer = PackageCore }, wantErr: ErrPrimitiveContract},
		{name: "package imports itself", mutate: func(c *ArchitectureCatalog) { c.imports[0].Imported = PackageAttest }, wantErr: ErrPrimitiveContract},
		{name: "production imports test support", mutate: func(c *ArchitectureCatalog) { c.imports[0].Imported = PackageTestSerial }, wantErr: ErrPrimitiveContract},
		{name: "duplicate direct edge hides required edge", mutate: func(c *ArchitectureCatalog) { c.imports[1] = c.imports[0] }, wantErr: ErrPrimitiveContract},
		{name: "unknown importer crosses catalog boundary", mutate: func(c *ArchitectureCatalog) { c.imports[0].Importer = PackageUnknown }, wantErr: ErrPrimitiveContract},
		{name: "unknown imported package crosses catalog boundary", mutate: func(c *ArchitectureCatalog) { c.imports[0].Imported = PackageUnknown }, wantErr: ErrPrimitiveContract},
		{name: "future importer crosses closed domain", mutate: func(c *ArchitectureCatalog) { c.imports[0].Importer = packageIdentityLimit }, wantErr: ErrPrimitiveContract},
		{name: "future imported package crosses closed domain", mutate: func(c *ArchitectureCatalog) { c.imports[0].Imported = packageIdentityLimit }, wantErr: ErrPrimitiveContract},
		{name: "unset test edge is rejected", mutate: func(c *ArchitectureCatalog) { c.testImports[0] = DirectTestImportContract{} }, wantErr: ErrPrimitiveContract},
		{name: "core may not own a test edge", mutate: func(c *ArchitectureCatalog) { c.testImports[0].Importer = PackageCore }, wantErr: ErrPrimitiveContract},
		{name: "test edge may not import itself", mutate: func(c *ArchitectureCatalog) { c.testImports[0].Imported = PackageGate }, wantErr: ErrPrimitiveContract},
		{name: "unknown test importer crosses catalog boundary", mutate: func(c *ArchitectureCatalog) { c.testImports[0].Importer = PackageUnknown }, wantErr: ErrPrimitiveContract},
		{name: "unknown test target crosses catalog boundary", mutate: func(c *ArchitectureCatalog) { c.testImports[0].Imported = PackageUnknown }, wantErr: ErrPrimitiveContract},
		{name: "future test importer crosses closed domain", mutate: func(c *ArchitectureCatalog) { c.testImports[0].Importer = packageIdentityLimit }, wantErr: ErrPrimitiveContract},
		{name: "future test target crosses closed domain", mutate: func(c *ArchitectureCatalog) { c.testImports[0].Imported = packageIdentityLimit }, wantErr: ErrPrimitiveContract},
		{name: "duplicate test edge hides required proof substrate", mutate: func(c *ArchitectureCatalog) { c.testImports[1] = c.testImports[0] }, wantErr: ErrPrimitiveContract},
		{name: "test edge may not duplicate production dependency", mutate: func(c *ArchitectureCatalog) {
			c.testImports[0] = DirectTestImportContract(c.imports[0])
		}, wantErr: ErrPrimitiveContract},
		{name: "cycle from core to upgrade is rejected", mutate: func(c *ArchitectureCatalog) {
			c.imports[0] = DirectImportContract{Importer: PackageCore, Imported: PackageUpgrade}
		}, wantErr: ErrPrimitiveContract},
		{name: "seventh direct import exceeds graph maximum", mutate: func(c *ArchitectureCatalog) {
			c.imports[0] = DirectImportContract{Importer: PackageUpgrade, Imported: PackageContextState}
		}, wantErr: ErrPrimitiveContract},
		{name: "seventh combined production and test import exceeds graph maximum", mutate: func(c *ArchitectureCatalog) {
			c.testImports[0] = DirectTestImportContract{
				Importer: PackageUpgrade,
				Imported: PackageAttest,
			}
		}, wantErr: ErrPrimitiveContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := PrimitiveArchitecture()
			tc.mutate(&got)
			gotErr := got.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("mutated ArchitectureCatalog.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestArchitectureCatalogAllowsTestsToImportTestSupport(t *testing.T) {
	t.Parallel()

	catalog := PrimitiveArchitecture()
	catalog.testImports[0].Imported = PackageTestSerial
	if gotErr := catalog.Validate(); gotErr != nil {
		t.Fatalf(
			"ArchitectureCatalog with test-only Testserial edge Validate() error = %v, want nil",
			gotErr,
		)
	}
}

func TestPackageIdentityExhaustsClosedDomain(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		identity := PackageIdentity(raw)
		gotErr := identity.Validate()
		wantValid := identity > PackageUnknown && identity < packageIdentityLimit
		if gotValid := identity.IsValid(); gotValid != wantValid {
			t.Fatalf("PackageIdentity(%d).IsValid() = %t, want %t", raw, gotValid, wantValid)
		}
		if (gotErr == nil) != wantValid {
			t.Fatalf("PackageIdentity(%d).Validate() error = %v, want valid %t", raw, gotErr, wantValid)
		}
		if !wantValid && !errors.Is(gotErr, ErrPrimitiveContract) {
			t.Fatalf("PackageIdentity(%d).Validate() error = %v, want %v", raw, gotErr, ErrPrimitiveContract)
		}
		if !wantValid {
			continue
		}
		gotWire, gotMarshalErr := json.Marshal(identity)
		if gotMarshalErr != nil {
			t.Fatalf("json.Marshal(PackageIdentity(%d)) error = %v, want nil", raw, gotMarshalErr)
		}
		var gotRoundTrip PackageIdentity
		gotUnmarshalErr := json.Unmarshal(gotWire, &gotRoundTrip)
		if gotUnmarshalErr != nil || gotRoundTrip != identity {
			t.Fatalf(
				"PackageIdentity(%d) JSON round trip = (%v, %v), want (%v, nil)",
				raw,
				gotRoundTrip,
				gotUnmarshalErr,
				identity,
			)
		}
	}
}

func TestPackageKindExhaustsClosedDomainAndJSON(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		kind := PackageKind(raw)
		gotValid := kind.IsValid()
		wantValid := kind == PackageKindProduction || kind == PackageKindTestSupport
		if gotValid != wantValid {
			t.Fatalf("PackageKind(%d).IsValid() = %t, want %t", raw, gotValid, wantValid)
		}
		if !wantValid {
			continue
		}
		gotWire, gotMarshalErr := json.Marshal(kind)
		if gotMarshalErr != nil {
			t.Fatalf("json.Marshal(PackageKind(%d)) error = %v, want nil", raw, gotMarshalErr)
		}
		var gotRoundTrip PackageKind
		gotUnmarshalErr := json.Unmarshal(gotWire, &gotRoundTrip)
		if gotUnmarshalErr != nil || gotRoundTrip != kind {
			t.Fatalf("PackageKind(%d) JSON round trip = (%v, %v), want (%v, nil)", raw, gotRoundTrip, gotUnmarshalErr, kind)
		}
	}
}

func TestErrorIdentityExhaustsClosedDomainAndParentDecisions(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint16; raw++ {
		identity := ErrorIdentity(raw)
		gotErr := identity.Validate()
		wantValid := identity > ErrUnknown && identity < errorIdentityLimit
		if gotValid := identity.IsValid(); gotValid != wantValid {
			t.Fatalf("ErrorIdentity(%d).IsValid() = %t, want %t", raw, gotValid, wantValid)
		}
		if (gotErr == nil) != wantValid {
			t.Fatalf("ErrorIdentity(%d).Validate() error = %v, want valid %t", raw, gotErr, wantValid)
		}
		if wantValid {
			parents := errorIdentityParents(identity)
			if parents.countValues() > errorIdentityMaximumParents {
				t.Fatalf("errorIdentityParents(%d).countValues() = %d, want <= %d", raw, parents.countValues(), errorIdentityMaximumParents)
			}
			for index := 0; index < parents.countValues(); index++ {
				parent, ok := parents.at(index)
				if !ok {
					t.Fatalf("errorIdentityParents(%d).at(%d) failed inside reported count", raw, index)
				}
				if gotParentErr := parent.Validate(); gotParentErr != nil {
					t.Fatalf("errorIdentityParents(%d).at(%d).Validate() error = %v, want nil", raw, index, gotParentErr)
				}
				for prior := 0; prior < index; prior++ {
					priorParent, priorOK := parents.at(prior)
					if !priorOK {
						t.Fatalf("errorIdentityParents(%d).at(%d) failed inside reported count", raw, prior)
					}
					if parent == priorParent {
						t.Fatalf("errorIdentityParents(%d) duplicates parent %d", raw, parent)
					}
				}
			}
			if !errors.Is(identity, identity) {
				t.Fatalf("errors.Is(ErrorIdentity(%d), itself) = false, want true", raw)
			}
			wantPrimitiveContract := true
			switch identity {
			case ErrHostFacts, ErrHostFactsObservation, ErrHostFactsUnsupported,
				ErrHostFactsPressure, ErrHostFactsEvidence,
				ErrDiskCapacityUnsupported, ErrTreeMeasurementUnsupported,
				ErrDiskFloorReached, ErrMemoryLimitReached:
				wantPrimitiveContract = false
			}
			if gotPrimitiveContract := errors.Is(identity, ErrPrimitiveContract); gotPrimitiveContract != wantPrimitiveContract {
				t.Fatalf(
					"errors.Is(ErrorIdentity(%d), ErrPrimitiveContract) = %t, want %t",
					raw,
					gotPrimitiveContract,
					wantPrimitiveContract,
				)
			}
		}
	}

	decisions := []struct {
		name       string
		produced   ErrorIdentity
		wantMatch  ErrorIdentity
		wantReject ErrorIdentity
	}{
		{name: "currency overflow is numeric overflow", produced: ErrCurrencyOverflow, wantMatch: ErrNumericOverflow, wantReject: ErrCurrencyMismatch},
		{name: "currency mismatch remains currency family", produced: ErrCurrencyMismatch, wantMatch: ErrCurrencyContract, wantReject: ErrNumericOverflow},
		{name: "indeterminate activation is activation failure", produced: ErrFilestoreActivationIndeterminate, wantMatch: ErrFilestoreActivation, wantReject: ErrFilestoreCleanup},
		{name: "disk pressure is host facts pressure not contract", produced: ErrDiskFloorReached, wantMatch: ErrHostFactsPressure, wantReject: ErrHostFactsContract},
		{name: "tree unsupported is host facts unsupported not observation", produced: ErrTreeMeasurementUnsupported, wantMatch: ErrHostFactsUnsupported, wantReject: ErrHostFactsObservation},
		{name: "exchange transport is exchange family", produced: ErrExchangeTransport, wantMatch: ErrExchangeContract, wantReject: ErrExchangeResponse},
		{name: "object absence is objectstore family", produced: ErrObjectStoreAbsent, wantMatch: ErrObjectStoreContract, wantReject: ErrObjectStoreIntegrity},
	}
	for _, tc := range decisions {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !errors.Is(tc.produced, tc.wantMatch) {
				t.Fatalf("errors.Is(%v, %v) = false, want true", tc.produced, tc.wantMatch)
			}
			if errors.Is(tc.produced, tc.wantReject) {
				t.Fatalf("errors.Is(%v, %v) = true, want false", tc.produced, tc.wantReject)
			}
		})
	}
}

// TestErrorIdentityMatchesEveryClosedDomainPair makes traversal depth and every
// caller decision observable. The reference walk is recursive over the closed
// compiler-sized domain, independent of Matches' iterative stack mechanics.
func TestErrorIdentityMatchesEveryClosedDomainPair(t *testing.T) {
	t.Parallel()

	for produced := ErrPrimitiveContract; produced < errorIdentityLimit; produced++ {
		for target := ErrPrimitiveContract; target < errorIdentityLimit; target++ {
			var visited [errorIdentityLimit]bool
			want := referenceErrorIdentityMatch(produced, target, &visited)
			if got := produced.Matches(target); got != want {
				t.Fatalf("ErrorIdentity(%d).Matches(ErrorIdentity(%d)) = %t, want %t", produced, target, got, want)
			}
		}
	}
}

func referenceErrorIdentityMatch(
	produced ErrorIdentity,
	target ErrorIdentity,
	visited *[errorIdentityLimit]bool,
) bool {
	if produced == target {
		return true
	}
	if visited[produced] {
		return false
	}
	visited[produced] = true
	parents := errorIdentityParents(produced)
	for index := 0; index < parents.countValues(); index++ {
		parent, ok := parents.at(index)
		if ok && referenceErrorIdentityMatch(parent, target, visited) {
			return true
		}
	}
	return false
}

func TestErrorIdentityStableTextAndJSONExhaustClosedDomain(t *testing.T) {
	t.Parallel()

	var texts [errorIdentityLimit]string
	for identity := ErrPrimitiveContract; identity < errorIdentityLimit; identity++ {
		text := identity.String()
		if text == "" || text == unknownErrorIdentityText {
			t.Fatalf("ErrorIdentity(%d).String() = %q, want admitted stable text", identity, text)
		}
		for prior := ErrPrimitiveContract; prior < identity; prior++ {
			if text == texts[prior] {
				t.Fatalf(
					"ErrorIdentity(%d).String() duplicates ErrorIdentity(%d) text %q",
					identity,
					prior,
					text,
				)
			}
		}
		texts[identity] = text

		wire, gotMarshalErr := json.Marshal(identity)
		if gotMarshalErr != nil {
			t.Fatalf("json.Marshal(ErrorIdentity(%d)) error = %v, want nil", identity, gotMarshalErr)
		}
		var roundTrip ErrorIdentity
		gotUnmarshalErr := json.Unmarshal(wire, &roundTrip)
		if gotUnmarshalErr != nil || roundTrip != identity {
			t.Fatalf(
				"ErrorIdentity(%d) JSON round trip = (%d, %v), want (%d, nil)",
				identity,
				roundTrip,
				gotUnmarshalErr,
				identity,
			)
		}
	}
}
