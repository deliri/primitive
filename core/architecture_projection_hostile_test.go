package core

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

type architectureProjectionKind uint8

const (
	architectureProjectionUnknown architectureProjectionKind = iota
	architectureProjectionReadme
	architectureProjectionPlan
)

const (
	architectureProjectionImportMaximum     = PrimitiveDirectImportCount + 1
	architectureProjectionTestImportMaximum = PrimitiveDirectTestImportCount + 1
	architectureProjectionViolationMaximum  = (PrimitivePackageCount + PrimitiveDirectImportCount +
		PrimitiveDirectTestImportCount) * 2
)

type architectureProjection struct {
	purposes            [packageIdentityLimit]string
	imports             [architectureProjectionImportMaximum]DirectImportContract
	packages            [PrimitivePackageCount]PackageIdentity
	testImports         [architectureProjectionTestImportMaximum]DirectTestImportContract
	packageCount        uint8
	importCount         uint8
	testImportCount     uint8
	declaresTestImports bool
	declaresPurposes    bool
}

type architectureProjectionViolationKind uint8

const (
	architectureProjectionViolationUnknown architectureProjectionViolationKind = iota
	architectureProjectionViolationPackageMissing
	architectureProjectionViolationPackageExtra
	architectureProjectionViolationImportMissing
	architectureProjectionViolationImportExtra
	architectureProjectionViolationTestImportMissing
	architectureProjectionViolationTestImportExtra
	architectureProjectionViolationPurposeDrift
)

type architectureProjectionViolation struct {
	importContract     DirectImportContract
	testImportContract DirectTestImportContract
	packageIdentity    PackageIdentity
	kind               architectureProjectionViolationKind
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

func TestArchitecturePlanProjectionMatchesCompilerCatalog(t *testing.T) {
	t.Parallel()

	const path = "../_docs/primitive_policy.md"
	source, gotReadErr := os.ReadFile(path)
	if gotReadErr != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, gotReadErr)
	}
	gotViolations, gotAuditErr := auditArchitectureProjection(
		architectureProjectionPlan,
		string(source),
		PrimitiveArchitecture(),
	)
	if gotAuditErr != nil {
		t.Fatalf("auditArchitectureProjection(plan) error = %v, want nil", gotAuditErr)
	}
	if gotViolations.count != 0 {
		t.Fatalf("PLAN architecture projection violations = %+v, want none", gotViolations.Values())
	}
}

func TestArchitectureProjectionMatcherSyntheticRedGreenRatchet(t *testing.T) {
	t.Parallel()

	readme := mustReadProjectionFixture(t, "../README.md")
	plan := mustReadProjectionFixture(t, "../_docs/primitive_policy.md")
	const attestPlanRow = "| 2 | `attest` | Canonical Ed25519 envelopes and proof-carrying verification | `core` | none |"
	const gatePlanRow = "| 5 | `gate` | Pure CLI-side new-work authorization over one authentic Lease assessment | `core`, `lease` | `attest`, `temporal` |"
	cases := []struct {
		wantErr       error
		name          string
		source        string
		kind          architectureProjectionKind
		wantViolation architectureProjectionViolationKind
	}{
		{name: "unchanged readme is green", source: readme, kind: architectureProjectionReadme},
		{name: "unchanged plan is green", source: plan, kind: architectureProjectionPlan},
		{
			name: "plan exchange purpose drift is red",
			source: strings.Replace(
				plan,
				"| 4 | `exchange` | Bounded client and server boundary policy over `net/http` | `core`, `contextstate`, `keygen`, `temporal` | none |",
				"| 4 | `exchange` | Unratcheted HTTP behavior | `core`, `contextstate`, `keygen`, `temporal` | none |",
				1,
			),
			kind:          architectureProjectionPlan,
			wantViolation: architectureProjectionViolationPurposeDrift,
		},
		{
			name: "plan missing attest core edge is red",
			source: strings.Replace(
				plan,
				attestPlanRow,
				"| 2 | `attest` | Canonical Ed25519 envelopes and proof-carrying verification | none | none |",
				1,
			),
			kind:          architectureProjectionPlan,
			wantViolation: architectureProjectionViolationImportMissing,
		},
		{
			name: "plan extra attest contextstate edge is red",
			source: strings.Replace(
				plan,
				attestPlanRow,
				"| 2 | `attest` | Canonical Ed25519 envelopes and proof-carrying verification | `core`, `contextstate` | none |",
				1,
			),
			kind:          architectureProjectionPlan,
			wantViolation: architectureProjectionViolationImportExtra,
		},
		{
			name: "plan missing gate temporal test edge is red",
			source: strings.Replace(
				plan,
				gatePlanRow,
				"| 5 | `gate` | Pure CLI-side new-work authorization over one authentic Lease assessment | `core`, `lease` | `attest` |",
				1,
			),
			kind:          architectureProjectionPlan,
			wantViolation: architectureProjectionViolationTestImportMissing,
		},
		{
			name: "plan extra attest testserial test edge is red",
			source: strings.Replace(
				plan,
				attestPlanRow,
				"| 2 | `attest` | Canonical Ed25519 envelopes and proof-carrying verification | `core` | `testserial` |",
				1,
			),
			kind:          architectureProjectionPlan,
			wantViolation: architectureProjectionViolationTestImportExtra,
		},
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
	if projection.declaresPurposes {
		for packageContract := range catalog.Packages() {
			if projection.Purpose(packageContract.Identity) == packagePurposeText(packageContract.Identity) {
				continue
			}
			if addErr := violations.Add(architectureProjectionViolation{
				packageIdentity: packageContract.Identity,
				kind:            architectureProjectionViolationPurposeDrift,
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
		if !catalog.ContainsDirectImport(contract) {
			if addErr := violations.Add(architectureProjectionViolation{
				importContract: contract,
				kind:           architectureProjectionViolationImportExtra,
			}); addErr != nil {
				return architectureProjectionViolations{}, addErr
			}
		}
	}
	if projection.declaresTestImports {
		for contract := range catalog.DirectTestImports() {
			if !projection.ContainsTestImport(contract) {
				if addErr := violations.Add(architectureProjectionViolation{
					testImportContract: contract,
					kind:               architectureProjectionViolationTestImportMissing,
				}); addErr != nil {
					return architectureProjectionViolations{}, addErr
				}
			}
		}
		for _, contract := range projection.TestImports() {
			if !catalog.ContainsDirectTestImport(contract) {
				if addErr := violations.Add(architectureProjectionViolation{
					testImportContract: contract,
					kind:               architectureProjectionViolationTestImportExtra,
				}); addErr != nil {
					return architectureProjectionViolations{}, addErr
				}
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
	case architectureProjectionPlan:
		return parsePlanArchitectureProjection(source)
	default:
		return architectureProjection{}, architectureContractError("architecture projection kind is unset")
	}
}

func parsePlanArchitectureProjection(source string) (architectureProjection, error) {
	var projection architectureProjection
	projection.declaresTestImports = true
	projection.declaresPurposes = true
	inGraphSection := false
	inGraphTable := false
	for rawLine := range strings.SplitSeq(source, "\n") {
		line := strings.TrimSpace(rawLine)
		// The heading the policy actually uses. Pinning the literal here is
		// deliberate: renaming the section in _docs/primitive_policy.md must
		// break this parse rather than silently yield an empty projection that
		// audits clean against a catalog it never read.
		if line == "## 15. Exact package graph" {
			inGraphSection = true
			continue
		}
		if inGraphSection && strings.HasPrefix(line, "## ") {
			break
		}
		if !inGraphSection {
			continue
		}
		if !inGraphTable {
			if !strings.HasPrefix(line, "| Order |") {
				continue
			}
			inGraphTable = true
		}
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells := markdownTableCells(line)
		if len(cells) != 5 {
			return architectureProjection{}, architectureContractError("PLAN architecture row has the wrong column count")
		}
		if cells[0] == "Order" || strings.HasPrefix(cells[0], "---") {
			continue
		}
		identity, err := ParsePackageIdentity(strings.Trim(cells[1], "`"))
		if err != nil {
			return architectureProjection{}, err
		}
		if err := projection.AddPackage(identity); err != nil {
			return architectureProjection{}, err
		}
		if err := projection.AddPurpose(identity, strings.ReplaceAll(cells[2], "`", "")); err != nil {
			return architectureProjection{}, err
		}
		if err := addPlanImportCell(&projection, identity, cells[3], false); err != nil {
			return architectureProjection{}, err
		}
		if err := addPlanImportCell(&projection, identity, cells[4], true); err != nil {
			return architectureProjection{}, err
		}
	}
	if !inGraphSection || !inGraphTable || projection.packageCount == 0 {
		return architectureProjection{}, architectureContractError("PLAN exact graph table is missing")
	}
	return projection, nil
}

func markdownTableCells(line string) []string {
	trimmed := strings.Trim(line, "|")
	parts := strings.Split(trimmed, "|")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func addPlanImportCell(
	projection *architectureProjection,
	importer PackageIdentity,
	cell string,
	testOnly bool,
) error {
	if cell == "none" {
		return nil
	}
	for rawImported := range strings.SplitSeq(cell, ",") {
		imported, err := ParsePackageIdentity(strings.Trim(strings.TrimSpace(rawImported), "`"))
		if err != nil {
			return err
		}
		if testOnly {
			if err := projection.AddTestImport(DirectTestImportContract{Importer: importer, Imported: imported}); err != nil {
				return err
			}
			continue
		}
		if err := projection.AddImport(DirectImportContract{Importer: importer, Imported: imported}); err != nil {
			return err
		}
	}
	return nil
}

func parseReadmeArchitectureProjection(source string) (architectureProjection, error) {
	var projection architectureProjection
	inDiagram := false
	for rawLine := range strings.SplitSeq(source, "\n") {
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
	if before, _, ok := strings.Cut(value, "["); ok {
		return before
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

func (p *architectureProjection) AddPurpose(identity PackageIdentity, purpose string) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	if !p.ContainsPackage(identity) || purpose == "" || p.purposes[identity] != "" {
		return architectureContractError("architecture projection contains an invalid package purpose")
	}
	p.purposes[identity] = purpose
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

func (p *architectureProjection) AddTestImport(contract DirectTestImportContract) error {
	if err := contract.Validate(); err != nil {
		return err
	}
	if p.ContainsTestImport(contract) {
		return architectureContractError("architecture projection contains a duplicate test import")
	}
	if int(p.testImportCount) >= len(p.testImports) {
		return architectureContractError("architecture projection exceeds test import capacity")
	}
	p.testImports[p.testImportCount] = contract
	p.testImportCount++
	return nil
}

func (p architectureProjection) ContainsPackage(identity PackageIdentity) bool {
	return slices.Contains(p.Packages(), identity)
}

func (p architectureProjection) ContainsImport(contract DirectImportContract) bool {
	return slices.Contains(p.Imports(), contract)
}

func (p architectureProjection) ContainsTestImport(contract DirectTestImportContract) bool {
	return slices.Contains(p.TestImports(), contract)
}

func (p architectureProjection) Packages() []PackageIdentity {
	return p.packages[:p.packageCount]
}

func (p architectureProjection) Imports() []DirectImportContract {
	return p.imports[:p.importCount]
}

func (p architectureProjection) TestImports() []DirectTestImportContract {
	return p.testImports[:p.testImportCount]
}

func (p architectureProjection) Purpose(identity PackageIdentity) string {
	if identity >= packageIdentityLimit {
		return ""
	}
	return p.purposes[identity]
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
