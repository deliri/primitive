package hostfacts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"testing"
)

func TestPublicOperationsAreExactIntentEntryPoints(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("filepath.Glob() error = %v, want nil", err)
	}
	set := token.NewFileSet()
	var got []string
	for _, filePath := range files {
		if len(filePath) >= len("_test.go") &&
			filePath[len(filePath)-len("_test.go"):] == "_test.go" {
			continue
		}
		file, parseErr := parser.ParseFile(set, filePath, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", filePath, parseErr)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !ast.IsExported(function.Name.Name) {
				continue
			}
			got = append(got, function.Name.Name)
		}
	}
	slices.Sort(got)
	want := []string{
		"AssessDisk",
		"AssessGoMemory",
		"ClassifyGoOOMBanner",
		"CurrentPlatform",
		"MeasureTree",
		"NewPercent",
		"ObserveEffectiveWorkloadMemoryLimit",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("exported Hostfacts operations = %q, want exactly %q", got, want)
	}
}

func TestProductionStructDataFlowInventory(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("filepath.Glob() error = %v, want nil", err)
	}
	set := token.NewFileSet()
	found := 0
	for _, filePath := range files {
		if filepath.Ext(filePath) != ".go" ||
			len(filePath) >= len("_test.go") &&
				filePath[len(filePath)-len("_test.go"):] == "_test.go" {
			continue
		}
		file, parseErr := parser.ParseFile(set, filePath, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", filePath, parseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := spec.Type.(*ast.StructType); !ok {
				return true
			}
			role, classified := productionStructRole(spec.Name.Name)
			if !classified || role == "" {
				t.Errorf("production struct %s has role %q classified %t, want precise data-flow role", spec.Name.Name, role, classified)
			}
			found++
			return false
		})
	}
	if found < 20 {
		t.Fatalf("production struct inventory found %d structs, want at least 20", found)
	}
}

func productionStructRole(name string) (string, bool) {
	switch name {
	case "Failure":
		return "typed error context", true
	case "DiskPressurePolicy", "DiskAssessmentRequest", "GoMemoryPressurePolicy",
		"GoMemoryAssessmentRequest", "TreeUsageRequest", "GoOOMBannerRequest":
		return "public execution ingress", true
	case "DiskCapacity", "DiskAssessment", "GoMemorySnapshot",
		"GoMemoryAssessment", "WorkloadMemoryLimit", "TreeUsage",
		"GoOOMBannerEvidence", "Percent", "RegularFileCount":
		return "validated immutable observation or policy fact", true
	case "goOOMBannerWire":
		return "bounded persistence projection", true
	case "bannerMatcher", "bannerCursor", "oomScanner", "treeEntry", "treeFrame",
		"treeAccumulator", "treeWalk":
		return "internal bounded streaming flow", true
	case "cgroupMembership", "cgroupMount", "cgroupMountSelection", "cgroupLimitFold",
		"cgroupLevelLimit", "cgroupLevelRequest", "virtualFileRequest", "boundedLineScan":
		return "internal typed kernel-interface fact", true
	case "platformRoot":
		return "held operating-system capability", true
	default:
		return "", false
	}
}
