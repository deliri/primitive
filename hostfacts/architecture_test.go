package hostfacts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
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
		"AmbientEnvironment",
		"AssessDisk",
		"AssessGoMemory",
		"ClassifyGoOOMBanner",
		"CurrentPlatform",
		"Executable",
		"LookupAmbientEnvironment",
		"NewPercent",
		"ObserveDiskRotation",
		"ObserveEffectiveWorkloadMemoryLimit",
		"ObserveHostname",
		"ObserveLogicalCPUCount",
		"ObservePhysicalMemory",
		"ObserveTerminalGeometry",
		"ResolveWorkingPath",
		"TemporaryDirectory",
		"UserCacheDirectory",
		"UserConfigDirectory",
		"UserHomeDirectory",
		"WorkingDirectory",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("exported Hostfacts operations = %q, want exactly %q", got, want)
	}
}

type hostfactsIngress[T any] struct{ Value T }
type hostfactsObservation[T any] struct{ Value T }
type hostfactsKernelFlow[T any] struct{ Value T }
type hostfactsPersistence[T any] struct{ Value T }
type hostfactsCapability[T any] struct{ Value T }
type hostfactsError[T any] struct{ Value T }

type hostfactsStructInventory struct {
	Failure                   hostfactsError[Failure]
	DiskPressurePolicy        hostfactsIngress[DiskPressurePolicy]
	DiskAssessmentRequest     hostfactsIngress[DiskAssessmentRequest]
	DiskRotationRequest       hostfactsIngress[DiskRotationRequest]
	GoMemoryPressurePolicy    hostfactsIngress[GoMemoryPressurePolicy]
	GoMemoryAssessmentRequest hostfactsIngress[GoMemoryAssessmentRequest]
	GoOOMBannerRequest        hostfactsIngress[GoOOMBannerRequest]
	TerminalGeometryRequest   hostfactsIngress[TerminalGeometryRequest]
	DiskCapacity              hostfactsObservation[DiskCapacity]
	DiskAssessment            hostfactsObservation[DiskAssessment]
	GoMemorySnapshot          hostfactsObservation[GoMemorySnapshot]
	GoMemoryAssessment        hostfactsObservation[GoMemoryAssessment]
	Hostname                  hostfactsObservation[Hostname]
	LogicalCPUCount           hostfactsObservation[LogicalCPUCount]
	PhysicalMemory            hostfactsObservation[PhysicalMemory]
	WorkloadMemoryLimit       hostfactsObservation[WorkloadMemoryLimit]
	GoOOMBannerEvidence       hostfactsPersistence[GoOOMBannerEvidence]
	Percent                   hostfactsIngress[Percent]
	TerminalGeometry          hostfactsObservation[TerminalGeometry]
	OOMWire                   hostfactsPersistence[goOOMBannerWire]
	OOMScan                   hostfactsKernelFlow[oomScanner]
	Membership                hostfactsKernelFlow[cgroupMembership]
	Mount                     hostfactsKernelFlow[cgroupMount]
	MountSelection            hostfactsKernelFlow[cgroupMountSelection]
	LimitFold                 hostfactsKernelFlow[cgroupLimitFold]
	LevelLimit                hostfactsKernelFlow[cgroupLevelLimit]
	LevelRequest              hostfactsKernelFlow[cgroupLevelRequest]
	VirtualFileRequest        hostfactsKernelFlow[virtualFileRequest]
	LineScan                  hostfactsKernelFlow[boundedLineScan]
	Root                      hostfactsCapability[platformRoot]
}

func TestProductionStructDataFlowInventory(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("source inventory = %v, want nil", err)
	}
	var got []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s = %v, want nil", name, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := spec.Type.(*ast.StructType); ok {
				got = append(got, spec.Name.Name)
			}
			return false
		})
	}
	slices.Sort(got)
	got = slices.Compact(got) // platformRoot has one platform-specific definition per build.
	inventory := reflect.TypeFor[hostfactsStructInventory]()
	var want []string
	for entry := range inventory.Fields() {
		value := entry.Type.Field(0).Type
		if value.Kind() != reflect.Struct || value.Name() == "" {
			t.Fatalf("inventory %s = %v, want named production struct", entry.Name, value)
		}
		want = append(want, value.Name())
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("production structs = %q, want exactly typed inventory %q", got, want)
	}
}
