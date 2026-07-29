package core

import (
	"errors"
	"os"
	"strings"
	"testing"
)

type architectureProjectionKind uint8

const (
	architectureProjectionUnknown architectureProjectionKind = iota
	architectureProjectionReadme
)

const (
	architectureProjectionImportMaximum    = PrimitiveDirectImportCount + 1
	architectureProjectionViolationMaximum = (PrimitivePackageCount + PrimitiveDirectImportCount) * 2
)

type architectureProjection struct {
	packages     [PrimitivePackageCount]PackageIdentity
	imports      [architectureProjectionImportMaximum]DirectImportContract
	packageCount uint8
	importCount  uint8
}

type architectureProjectionViolationKind uint8

const (
	architectureProjectionViolationUnknown architectureProjectionViolationKind = iota
	architectureProjectionViolationPackageMissing
	architectureProjectionViolationPackageExtra
	architectureProjectionViolationImportMissing
	architectureProjectionViolationImportExtra
)

type architectureProjectionViolation struct {
	importContract  DirectImportContract
	packageIdentity PackageIdentity
	kind            architectureProjectionViolationKind
}

type architectureProjectionViolations struct {
	values [architectureProjectionViolationMaximum]architectureProjectionViolation
	count  uint8
}

func TestAttestCompilerEdgeActivatesSentinel(t *testing.T) {
	t.Parallel()

	gotExists, gotViolations, gotErr := auditPackageImports("..", PackageAttest, PrimitiveArchitecture())
	if gotErr != nil {
		t.Fatalf("auditPackageImports(attest) error = %v, want nil", gotErr)
	}
	if !gotExists {
		t.Fatal("auditPackageImports(attest) exists = false, want true")
	}
	if gotViolations.count != 0 {
		t.Fatalf("auditPackageImports(attest) violations = %+v, want none", gotViolations.Values())
	}
}

func TestArchitectureReadmeProjectionMatchesCompilerCatalog(t *testing.T) {
	t.Parallel()

	const path = "../README.md"
	source, gotReadErr := os.ReadFile(path)
	if gotReadErr != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, gotReadErr)
	}
	gotViolations, gotAuditErr := auditArchitectureProjection(
		architectureProjectionReadme,
		string(source),
		PrimitiveArchitecture(),
	)
	if gotAuditErr != nil {
		t.Fatalf("auditArchitectureProjection() error = %v, want nil", gotAuditErr)
	}
	if gotViolations.count != 0 {
		t.Fatalf("architecture projection violations = %+v, want none", gotViolations.Values())
	}
}

func TestArchitectureProjectionMatcherSyntheticRedGreenRatchet(t *testing.T) {
	t.Parallel()

	readme := mustReadProjectionFixture(t, "../README.md")
	cases := []struct {
		wantErr       error
		name          string
		source        string
		kind          architectureProjectionKind
		wantViolation architectureProjectionViolationKind
	}{
		{name: "unchanged readme is green", source: readme, kind: architectureProjectionReadme},
		{
			name:          "readme missing attest core edge is red",
			source:        strings.Replace(readme, "    attest[attest] --> core\n", "    attest[attest]\n", 1),
			kind:          architectureProjectionReadme,
			wantViolation: architectureProjectionViolationImportMissing,
		},
		{
			name: "readme extra attest contextstate edge is red",
			source: strings.Replace(
				readme,
				"    attest[attest] --> core\n",
				"    attest[attest] --> core\n    attest --> contextstate\n",
				1,
			),
			kind:          architectureProjectionReadme,
			wantViolation: architectureProjectionViolationImportExtra,
		},
		{
			name:    "readme duplicate attest edge is typed red",
			source:  strings.Replace(readme, "    contextstate[contextstate] --> core", "    attest --> core", 1),
			kind:    architectureProjectionReadme,
			wantErr: ErrPrimitiveContract,
		},
		{
			name:    "readme unknown package is typed red",
			source:  strings.Replace(readme, "    attest[attest] --> core", "    futurepackage[futurepackage] --> core", 1),
			kind:    architectureProjectionReadme,
			wantErr: ErrPrimitiveContract,
		},
		{
			name:    "unknown projection kind is typed red",
			source:  readme,
			kind:    architectureProjectionUnknown,
			wantErr: ErrPrimitiveContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotViolations, gotErr := auditArchitectureProjection(tc.kind, tc.source, PrimitiveArchitecture())
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("auditArchitectureProjection() error = %v, want %v", gotErr, tc.wantErr)
			}
			if gotErr != nil {
				return
			}
			gotCount := gotViolations.CountKind(tc.wantViolation)
			wantCount := 0
			if tc.wantViolation != architectureProjectionViolationUnknown {
				wantCount = 1
			}
			if gotCount != wantCount {
				t.Fatalf(
					"architecture projection violation kind %d count = %d, want %d",
					tc.wantViolation,
					gotCount,
					wantCount,
				)
			}
			if gotViolations.count != uint8(wantCount) {
				t.Fatalf("architecture projection total violations = %d, want %d", gotViolations.count, wantCount)
			}
		})
	}
}

func auditArchitectureProjection(
	kind architectureProjectionKind,
	source string,
	catalog ArchitectureCatalog,
) (architectureProjectionViolations, error) {
	projection, err := parseArchitectureProjection(kind, source)
	if err != nil {
		return architectureProjectionViolations{}, err
	}
	var violations architectureProjectionViolations
	for packageContract := range catalog.Packages() {
		if !projection.ContainsPackage(packageContract.Identity) {
			if addErr := violations.Add(architectureProjectionViolation{
				packageIdentity: packageContract.Identity,
				kind:            architectureProjectionViolationPackageMissing,
			}); addErr != nil {
				return architectureProjectionViolations{}, addErr
			}
		}
	}
	for _, identity := range projection.Packages() {
		if _, exists := catalog.Lookup(identity); !exists {
			if addErr := violations.Add(architectureProjectionViolation{
				packageIdentity: identity,
				kind:            architectureProjectionViolationPackageExtra,
			}); addErr != nil {
				return architectureProjectionViolations{}, addErr
			}
		}
	}
	for contract := range catalog.DirectImports() {
		if !projection.ContainsImport(contract) {
			if addErr := violations.Add(architectureProjectionViolation{
				importContract: contract,
				kind:           architectureProjectionViolationImportMissing,
			}); addErr != nil {
				return architectureProjectionViolations{}, addErr
			}
		}
	}
	for _, contract := range projection.Imports() {
		if !catalogContainsDirectImport(catalog, contract) {
			if addErr := violations.Add(architectureProjectionViolation{
				importContract: contract,
				kind:           architectureProjectionViolationImportExtra,
			}); addErr != nil {
				return architectureProjectionViolations{}, addErr
			}
		}
	}
	return violations, nil
}

func parseArchitectureProjection(
	kind architectureProjectionKind,
	source string,
) (architectureProjection, error) {
	switch kind {
	case architectureProjectionReadme:
		return parseReadmeArchitectureProjection(source)
	default:
		return architectureProjection{}, architectureContractError("architecture projection kind is unset")
	}
}

func parseReadmeArchitectureProjection(source string) (architectureProjection, error) {
	var projection architectureProjection
	inDiagram := false
	for _, rawLine := range strings.Split(source, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "```mermaid" {
			inDiagram = true
			continue
		}
		if inDiagram && line == "```" {
			break
		}
		if !inDiagram || line == "" || strings.HasPrefix(line, "flowchart ") {
			continue
		}
		parts := strings.Fields(line)
		importer, err := ParsePackageIdentity(readmeNodeName(parts[0]))
		if err != nil {
			return architectureProjection{}, err
		}
		if err := projection.AddPackage(importer); err != nil && !projection.ContainsPackage(importer) {
			return architectureProjection{}, err
		}
		if len(parts) == 1 {
			continue
		}
		if len(parts) != 3 || parts[1] != "-->" {
			return architectureProjection{}, architectureContractError("README architecture edge is malformed")
		}
		imported, err := ParsePackageIdentity(readmeNodeName(parts[2]))
		if err != nil {
			return architectureProjection{}, err
		}
		if !projection.ContainsPackage(imported) {
			if addErr := projection.AddPackage(imported); addErr != nil {
				return architectureProjection{}, addErr
			}
		}
		if addErr := projection.AddImport(DirectImportContract{
			Importer: importer,
			Imported: imported,
		}); addErr != nil {
			return architectureProjection{}, addErr
		}
	}
	return projection, nil
}

func readmeNodeName(value string) string {
	if bracket := strings.IndexByte(value, '['); bracket >= 0 {
		return value[:bracket]
	}
	return value
}

func mustReadProjectionFixture(t testing.TB, path string) string {
	t.Helper()
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	return string(value)
}

func (p *architectureProjection) AddPackage(identity PackageIdentity) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	if p.ContainsPackage(identity) {
		return architectureContractError("architecture projection contains a duplicate package")
	}
	if int(p.packageCount) >= len(p.packages) {
		return architectureContractError("architecture projection exceeds package capacity")
	}
	p.packages[p.packageCount] = identity
	p.packageCount++
	return nil
}

func (p *architectureProjection) AddImport(contract DirectImportContract) error {
	if err := contract.Validate(); err != nil {
		return err
	}
	if p.ContainsImport(contract) {
		return architectureContractError("architecture projection contains a duplicate import")
	}
	if int(p.importCount) >= len(p.imports) {
		return architectureContractError("architecture projection exceeds import capacity")
	}
	p.imports[p.importCount] = contract
	p.importCount++
	return nil
}

func (p architectureProjection) ContainsPackage(identity PackageIdentity) bool {
	for _, candidate := range p.Packages() {
		if candidate == identity {
			return true
		}
	}
	return false
}

func (p architectureProjection) ContainsImport(contract DirectImportContract) bool {
	for _, candidate := range p.Imports() {
		if candidate == contract {
			return true
		}
	}
	return false
}

func (p architectureProjection) Packages() []PackageIdentity {
	return p.packages[:p.packageCount]
}

func (p architectureProjection) Imports() []DirectImportContract {
	return p.imports[:p.importCount]
}

func (v *architectureProjectionViolations) Add(violation architectureProjectionViolation) error {
	if violation.kind == architectureProjectionViolationUnknown {
		return architectureContractError("architecture projection violation kind is unset")
	}
	if int(v.count) >= len(v.values) {
		return architectureContractError("architecture projection violation capacity exceeded")
	}
	v.values[v.count] = violation
	v.count++
	return nil
}

func (v architectureProjectionViolations) CountKind(kind architectureProjectionViolationKind) int {
	count := 0
	for _, violation := range v.Values() {
		if violation.kind == kind {
			count++
		}
	}
	return count
}

func (v architectureProjectionViolations) Values() []architectureProjectionViolation {
	return v.values[:v.count]
}
