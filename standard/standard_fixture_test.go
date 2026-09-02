package standard

import (
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func fixtureCatalog(t testing.TB) Catalog {
	t.Helper()

	commit := fixtureCommit(t, strings.Repeat("a", 40))
	path := fixturePath(t, "standard")
	featureID := fixtureIdentifier(t, "typed-project-standard")
	surfaceID := fixtureIdentifier(t, "package-proof")
	groupID := fixtureIdentifier(t, "foundation")
	subject := SubjectIdentity{
		Project:    core.Offering{Token: "primitive"},
		Repository: fixtureRepository(t, "github.com/deliri/primitive"),
	}
	changed := GitOrigin{Commit: commit, At: temporal.InstantFromNanoseconds(1_000_000)}
	feature := Feature{
		ID: featureID, Title: fixtureName(t, "Typed About"),
		Technical:        fixtureText(t, "One validated contract describes project and package engineering knowledge."),
		Benefit:          fixtureText(t, "Every subject is measured with the same vocabulary."),
		ProofRequirement: fixtureText(t, "Independent evidence remains bound to the exact source revision."),
		Delivery:         DeliveryDelivered,
	}
	usage := Usage{
		ID: fixtureIdentifier(t, "read-project-standard"), Title: fixtureName(t, "Read engineering context"),
		Audience: fixtureText(t, "Maintainers and engineering systems."), Goal: fixtureText(t, "Understand one package without reconstructing its contracts."),
		Steps:   []UsageStep{{Title: fixtureName(t, "Request"), Detail: fixtureText(t, "Request the exact committed revision through the typed client.")}},
		Outcome: fixtureText(t, "A validated deterministic project or package projection."),
	}
	reason := Reason{Title: fixtureName(t, "One vocabulary"), Detail: fixtureText(t, "Shared facts must not drift across products.")}
	owns := Boundary{Title: fixtureName(t, "Engineering knowledge"), Detail: fixtureText(t, "Defines validated project, package, code, and evidence projections.")}
	excludes := Boundary{Title: fixtureName(t, "Product policy"), Detail: fixtureText(t, "Does not decide what a product claims or accepts.")}
	reference := CodeReference{Path: fixturePath(t, "standard/catalog.go")}
	assurance := Assurance{
		Policy:     fixtureAssurance(AssuranceStagePolicy, AssuranceAuthorityProduct, surfaceID, reference, fixtureText(t, "The product authors its own purpose and boundaries.")),
		Validation: fixtureAssurance(AssuranceStageValidation, AssuranceAuthorityCore, surfaceID, reference, fixtureText(t, "Compiler-owned types reject contradictory records.")),
		Effects:    fixtureAssurance(AssuranceStageEffects, AssuranceAuthorityPrimitive, surfaceID, reference, fixtureText(t, "Primitive owns transport and deterministic report output.")),
		Proof:      fixtureAssurance(AssuranceStageProof, AssuranceAuthorityIndependent, surfaceID, reference, fixtureText(t, "Independent observations remain distinct from product claims.")),
	}
	knowledge := PackageKnowledge{
		Path: path, AuthorRole: core.PackageRoleValueContract, AuthorTitle: fixtureName(t, "Standard"), AuthorProblem: fixtureText(t, "Engineering knowledge otherwise fragments across projects."),
		AuthorPurpose: fixtureText(t, "Provide one reusable engineering knowledge contract."), AuthorAudience: fixtureText(t, "Go products and engineering tools."),
		AuthorValue: fixtureText(t, "Exact context and evidence can be inspected over time."), AuthorSteward: fixtureName(t, "Primitive"),
		AuthorSubstrate: fixtureName(t, "Go standard library"), AuthorRuntime: fixtureName(t, "Go 1.27"), Changed: changed,
		AuthorReasons: []Reason{reason}, AuthorOwns: []Boundary{owns}, AuthorDoesNotOwn: []Boundary{excludes},
		AuthorRemoval: fixtureText(t, "Projects would return to competing and unverifiable Standard models."),
		AuthorUsage:   []Usage{usage}, AuthorFeatures: []Feature{feature}, AuthorAssurance: assurance,
	}
	target := ProbeTarget{Kind: ProbeTargetGoPackage, GoPackage: &GoPackageTarget{Module: fixtureIdentifier(t, "primitive"), Package: path, ChildKinds: []ProbeKind{ProbeKindGoTest}}}
	surface := EvidenceSurface{
		ID: surfaceID, Subject: subject, Target: target, EligibleKinds: []ProbeKind{ProbeKindGoTest},
		Profiles: []ProfileIdentity{fixtureProfile(t, "acceptance")}, Placement: ReportPlacementPackage,
	}
	inventory := Inventory{GoPackages: 1, Files: 2, TestFiles: 1, TestDeclarations: 1}
	packageSnapshot := PackageSnapshot{
		SchemaVersion: SchemaVersion,
		Package:       Package{Key: fixtureIdentifier(t, "project-standard-package"), Subject: subject, Revision: commit, GroupID: groupID, Language: fixtureName(t, "Go"), Knowledge: knowledge},
		Code:          Code{Package: path, Inventory: inventory},
		Evidence:      Evidence{Package: path, Surfaces: []EvidenceSurface{surface}},
	}
	summary, err := packageSnapshot.Summary()
	if err != nil {
		t.Fatalf("PackageSnapshot.Summary() setup error = %v, want nil", err)
	}
	project := Project{
		SchemaVersion: SchemaVersion, Subject: subject, Revision: commit,
		Knowledge: ProjectKnowledge{
			AuthorTitle: fixtureName(t, "Primitive"), AuthorProblem: fixtureText(t, "Products need one owned real-world substrate."),
			AuthorPurpose: fixtureText(t, "Own reusable effects and low-level contracts."), AuthorAudience: fixtureText(t, "Cooperating Go products."),
			AuthorPromise: fixtureText(t, "Products do not bypass the standard library or duplicate real-world effects."),
			SourcePath:    fixturePath(t, "README.md"), Changed: changed, AuthorReasons: []Reason{reason}, AuthorOwns: []Boundary{owns},
			AuthorNonGoals: []Boundary{excludes}, AuthorFeatures: []Feature{feature},
		},
		Code: ProjectCode{Inventory: inventory}, Usage: []Usage{usage},
		Groups:       []PackageGroup{{ID: groupID, Title: fixtureName(t, "Foundation"), Purpose: fixtureText(t, "Reusable compiler-owned contracts.")}},
		Capabilities: []ProjectCapability{{FeatureID: featureID, Contributions: []PackageContribution{{Package: path, FeatureID: featureID, Role: fixtureText(t, "Defines and validates the shared Standard contract."), SurfaceIDs: []Identifier{surfaceID}}}}},
		Packages:     []PackageSummary{summary},
	}
	catalog := Catalog{Project: project, Packages: []PackageSnapshot{packageSnapshot}}
	if err := catalog.Validate(); err != nil {
		t.Fatalf("Catalog.Validate() setup error = %v, want nil", err)
	}
	return catalog
}

func fixtureAssurance(stage AssuranceStage, authority AssuranceAuthority, surface Identifier, reference CodeReference, statement Text) AssuranceControl {
	return AssuranceControl{Stage: stage, Authority: authority, Statement: statement, References: []CodeReference{reference}, SurfaceIDs: []Identifier{surface}}
}

func fixtureIdentifier(t testing.TB, value string) Identifier {
	t.Helper()
	got, gotErr := NewIdentifier(value)
	if gotErr != nil {
		t.Fatalf("NewIdentifier(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixtureName(t testing.TB, value string) Name {
	t.Helper()
	got, gotErr := NewName(value)
	if gotErr != nil {
		t.Fatalf("NewName(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixtureText(t testing.TB, value string) Text {
	t.Helper()
	got, gotErr := NewText(value)
	if gotErr != nil {
		t.Fatalf("NewText(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixturePath(t testing.TB, value string) SourcePath {
	t.Helper()
	got, gotErr := ParseSourcePath(value)
	if gotErr != nil {
		t.Fatalf("ParseSourcePath(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixtureRepository(t testing.TB, value string) RepositoryIdentity {
	t.Helper()
	got, gotErr := NewRepositoryIdentity(value)
	if gotErr != nil {
		t.Fatalf("NewRepositoryIdentity(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixtureProfile(t testing.TB, value string) ProfileIdentity {
	t.Helper()
	got, gotErr := NewProfileIdentity(fixtureIdentifier(t, value), 1)
	if gotErr != nil {
		t.Fatalf("NewProfileIdentity(%q, 1) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func fixtureCommit(t testing.TB, value string) core.BuildCommit {
	t.Helper()
	got, gotErr := core.ParseBuildCommit(value)
	if gotErr != nil {
		t.Fatalf("ParseBuildCommit(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}
