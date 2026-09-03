package claims

import (
	"slices"
	"sort"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceclaim"
)

func TestPrimitiveClaimStreamCoversEveryCompiledPackageWithoutAClaimQuota(t *testing.T) {
	t.Parallel()

	var gotPackageSubjects []string
	var gotProjectClaims []string
	var previousPackage string
	gotSummary, gotErr := sourceclaim.Consume(Stream, func(claim sourceclaim.Claim) error {
		switch claim.Subject.Kind {
		case core.SourceSubjectProject:
			gotProjectClaims = append(gotProjectClaims, claim.ID.String())
		case core.SourceSubjectPackage:
			path := claim.Subject.Path.String()
			if path != previousPackage {
				gotPackageSubjects = append(gotPackageSubjects, path)
				previousPackage = path
			}
		}
		return nil
	})
	if gotErr != nil {
		t.Fatalf("sourceclaim.Consume(Primitive claims) error = %v, want nil", gotErr)
	}

	wantPackageSubjects := compiledPackagePaths(t)
	if !slices.Equal(gotPackageSubjects, wantPackageSubjects) {
		t.Fatalf("Primitive package claim subjects = %q, want exact compiled packages %q", gotPackageSubjects, wantPackageSubjects)
	}
	if gotSummary.Projects != 1 || gotSummary.Packages != uint64(len(wantPackageSubjects)) || gotSummary.Files != 0 {
		t.Fatalf("Primitive claim subjects = %+v, want one project, %d packages, and zero file subjects", gotSummary, len(wantPackageSubjects))
	}
	if gotSummary.ProjectClaims != uint64(len(gotProjectClaims)) || gotSummary.PackageClaims < uint64(len(wantPackageSubjects)) || gotSummary.Claims != gotSummary.ProjectClaims+gotSummary.PackageClaims+gotSummary.FileClaims {
		t.Fatalf("Primitive claim cardinality = %+v, want exact accounting and at least one authored claim per package", gotSummary)
	}
	if len(gotProjectClaims) == 0 {
		t.Fatal("Primitive project claim count = 0, want independently authored project reasons")
	}
}

func compiledPackagePaths(t testing.TB) []string {
	t.Helper()
	architecture := core.PrimitiveArchitecture()
	if gotErr := architecture.Validate(); gotErr != nil {
		t.Fatalf("core.PrimitiveArchitecture().Validate() error = %v, want nil", gotErr)
	}
	paths := make([]string, 0, core.PrimitivePackageCount)
	for contract := range architecture.Packages() {
		paths = append(paths, contract.Identity.String())
	}
	sort.Strings(paths)
	return paths
}
