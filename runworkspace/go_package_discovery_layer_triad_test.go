package runworkspace

import (
	"bytes"
	"errors"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestGoPackageDiscoveryLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive package walk discovers executable declarations across lexical test files", func(t *testing.T) {
		t.Parallel()
		manager, request := packageDiscoveryFixture(t)
		defer closePackageDiscoveryManager(t, manager)
		writePackageSource(t, manager, request.Source, "alpha_test.go", "package subject\nimport \"testing\"\nfunc TestAlpha(t *testing.T) {}\n")
		writePackageSource(t, manager, request.Source, "benchmark_test.go", "package subject\nimport \"testing\"\nfunc BenchmarkEncode(b *testing.B) {}\n")

		got, gotErr := manager.DiscoverGoPackage(t.Context(), request)
		if gotErr != nil {
			t.Fatalf("Manager.DiscoverGoPackage(two test files) error = %v, want nil", gotErr)
		}
		if len(got.Declarations) != 2 || got.Declarations[0].File.String() != "subject/alpha_test.go" || got.Declarations[0].Declaration.Symbol.String() != "TestAlpha" || got.Declarations[1].File.String() != "subject/benchmark_test.go" || got.Declarations[1].Declaration.Symbol.String() != "BenchmarkEncode" {
			t.Fatalf("GoPackageDiscovery declarations = %+v, want TestAlpha then BenchmarkEncode with exact source files", got.Declarations)
		}
		file, fileErr := runprotocol.ParseSourcePath("subject/alpha_test.go")
		fileRequest := GoFileDiscoveryRequest{
			Source: request.Source,
			Target: runprotocol.GoFileTarget{
				Module: request.Target.Module, Package: request.Target.Package, File: file,
				ChildKinds: append([]runprotocol.ProbeKind(nil), request.Target.ChildKinds...),
			},
			Profile: request.Profile, Contexts: request.Contexts,
		}
		fileDiscovery, discoverFileErr := manager.DiscoverGoFile(t.Context(), fileRequest)
		if err := errors.Join(fileErr, discoverFileErr); err != nil || fileDiscovery.Validate() != nil || len(fileDiscovery.Declarations) != 1 || fileDiscovery.Declarations[0].Declaration.Symbol.String() != "TestAlpha" {
			t.Fatalf("Manager.DiscoverGoFile(alpha_test.go) = (%+v, %v), want one validated TestAlpha declaration and nil", fileDiscovery, err)
		}
	})

	t.Run("negative malformed test file refuses the package without returning partial declarations", func(t *testing.T) {
		t.Parallel()
		manager, request := packageDiscoveryFixture(t)
		defer closePackageDiscoveryManager(t, manager)
		writePackageSource(t, manager, request.Source, "alpha_test.go", "package subject\nimport \"testing\"\nfunc TestAlpha(t *testing.T) {}\n")
		writePackageSource(t, manager, request.Source, "broken_test.go", "package subject\nfunc TestBroken(")

		got, gotErr := manager.DiscoverGoPackage(t.Context(), request)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || got.Package != (runprotocol.SourcePath{}) || len(got.Declarations) != 0 {
			t.Fatalf("Manager.DiscoverGoPackage(malformed sibling) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("neutral package without test files returns a valid empty discovery", func(t *testing.T) {
		t.Parallel()
		manager, request := packageDiscoveryFixture(t)
		defer closePackageDiscoveryManager(t, manager)
		writePackageSource(t, manager, request.Source, "subject.go", "package subject\n")

		got, gotErr := manager.DiscoverGoPackage(t.Context(), request)
		if gotErr != nil || got.Validate() != nil || len(got.Declarations) != 0 || got.Package != request.Target.Package {
			t.Fatalf("Manager.DiscoverGoPackage(no test files) = (%+v, %v), want exact package with zero declarations and nil", got, gotErr)
		}
	})
}

func packageDiscoveryFixture(t *testing.T) (Manager, GoPackageDiscoveryRequest) {
	t.Helper()
	root, rootErr := core.ParseAbsolutePath(t.TempDir())
	manager, managerErr := Open(t.Context(), Configuration{RunParent: root})
	unit := packageDiscoveryUnitFixture(t, manager)
	checkout, checkoutErr := joinLiteral(unit.Root, "checkout")
	packageRoot, packageRootErr := joinLiteral(checkout, "subject")
	if err := errors.Join(rootErr, managerErr, checkoutErr, packageRootErr); err != nil {
		t.Fatalf("Go package discovery workspace setup error = %v, want nil", err)
	}
	if err := filestore.EnsureDirectory(t.Context(), filestore.DirectoryRequest{Location: filestore.Location{Root: manager.root, Path: packageRoot}, Mode: fs.FileMode(0o700)}); err != nil {
		t.Fatalf("filestore.EnsureDirectory(package checkout) setup error = %v, want nil", err)
	}
	repository, repositoryErr := runprotocol.NewRepositoryIdentity("github.com/example/project")
	commit, commitErr := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	module, moduleErr := runprotocol.NewIdentifier("project")
	packagePath, packageErr := runprotocol.ParseSourcePath("subject")
	profileName, profileNameErr := runprotocol.NewIdentifier("focused")
	profile, profileErr := runprotocol.NewProfileIdentity(profileName, 1)
	context := goBuildContextFixture(t)
	contextDigest, contextErr := context.Digest()
	if err := errors.Join(repositoryErr, commitErr, moduleErr, packageErr, profileNameErr, profileErr, contextErr); err != nil {
		t.Fatalf("Go package discovery contract setup error = %v, want nil", err)
	}
	target := runprotocol.GoPackageTarget{Module: module, Package: packagePath, ChildKinds: []runprotocol.ProbeKind{runprotocol.ProbeKindGoTest, runprotocol.ProbeKindGoBenchmark}}
	benchmarkContext := context
	benchmarkDigest, benchmarkErr := benchmarkContext.Digest()
	contexts := runnercontrol.GoBuildContextSet{Entries: []runnercontrol.GoBuildContextEntry{
		{Kind: runprotocol.ProbeKindGoBenchmark, Profile: profile, Context: benchmarkContext, Digest: benchmarkDigest},
		{Kind: runprotocol.ProbeKindGoTest, Profile: profile, Context: context, Digest: contextDigest},
	}}
	request := GoPackageDiscoveryRequest{
		Source: VerifiedSource{Coordinate: runprotocol.SourceCoordinate{Repository: repository, Commit: commit, Tree: core.SHA256Of([]byte("package-tree"))}, Checkout: checkout, Files: 1, Directories: 1},
		Target: target, Profile: profile, Contexts: contexts,
	}
	if err := errors.Join(benchmarkErr, request.Validate()); err != nil {
		t.Fatalf("GoPackageDiscoveryRequest.Validate() setup error = %v, want nil", err)
	}
	return manager, request
}

func packageDiscoveryUnitFixture(t *testing.T, manager Manager) Unit {
	t.Helper()
	uuid, uuidErr := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000990")
	if uuidErr != nil {
		t.Fatalf("id.ParseUUIDv7(package discovery unit) setup error = %v, want nil", uuidErr)
	}
	identity := runnercontrol.SchedulingUnitIdentity{Kind: runnercontrol.SchedulingUnitRunPlan, Identity: uuid}
	unit, err := manager.CreateUnit(t.Context(), identity)
	if err != nil {
		t.Fatalf("Manager.CreateUnit(package discovery) setup error = %v, want nil", err)
	}
	return unit
}

func writePackageSource(t *testing.T, manager Manager, source VerifiedSource, name, content string) {
	t.Helper()
	component, componentErr := core.ParsePathComponent(name)
	packageRoot, packageRootErr := core.ParseRelativePath(source.Checkout.String() + "/subject")
	target, targetErr := packageRoot.Join(component)
	temporaryComponent, temporaryErr := core.ParsePathComponent("." + name + ".stage")
	temporary, temporaryPathErr := packageRoot.Join(temporaryComponent)
	maximum, maximumErr := core.NewByteCount(uint64(len(content)))
	if err := errors.Join(componentErr, packageRootErr, targetErr, temporaryErr, temporaryPathErr, maximumErr); err != nil {
		t.Fatalf("package source %q path setup error = %v, want nil", name, err)
	}
	_, err := filestore.Write(t.Context(), filestore.WriteRequest{
		Source: bytes.NewReader([]byte(content)), Location: filestore.Location{Root: manager.root, Path: target}, Temporary: temporary,
		Mode: fs.FileMode(0o600), Install: filestore.InstallCreate, MaximumBytes: maximum,
	})
	if err != nil {
		t.Fatalf("filestore.Write(%q) setup error = %v, want nil", name, err)
	}
}

func closePackageDiscoveryManager(t *testing.T, manager Manager) {
	t.Helper()
	if err := manager.Close(); err != nil {
		t.Fatalf("Manager.Close(package discovery) cleanup error = %v, want nil", err)
	}
}
