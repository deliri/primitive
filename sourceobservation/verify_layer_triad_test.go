package sourceobservation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/sourceobservation"
)

type observationResolver struct {
	files           map[core.SourcePath]sourceobservation.File
	packages        map[core.SourcePath]sourceobservation.Package
	projectFiles    []sourceobservation.FileReference
	projectPackages []sourceobservation.PackageReference
	packageFiles    map[core.SourcePath][]sourceobservation.FileReference
}

func (r observationResolver) StreamProjectFiles(_ context.Context, _ sourceobservation.Project, emit sourceobservation.EmitFileReference) error {
	for _, reference := range r.projectFiles {
		if err := emit(reference); err != nil {
			return err
		}
	}
	return nil
}

func (r observationResolver) StreamProjectPackages(_ context.Context, _ sourceobservation.Project, emit sourceobservation.EmitPackageReference) error {
	for _, reference := range r.projectPackages {
		if err := emit(reference); err != nil {
			return err
		}
	}
	return nil
}

func (r observationResolver) StreamPackageFiles(_ context.Context, observed sourceobservation.Package, emit sourceobservation.EmitFileReference) error {
	for _, reference := range r.packageFiles[observed.Path] {
		if err := emit(reference); err != nil {
			return err
		}
	}
	return nil
}

func (r observationResolver) ResolveFile(_ context.Context, reference sourceobservation.FileReference) (sourceobservation.File, error) {
	value, ok := r.files[reference.Path]
	if !ok {
		return sourceobservation.File{}, errors.New("file observation unavailable")
	}
	return value, nil
}

func (r observationResolver) ResolvePackage(_ context.Context, reference sourceobservation.PackageReference) (sourceobservation.Package, error) {
	value, ok := r.packages[reference.Path]
	if !ok {
		return sourceobservation.Package{}, errors.New("package observation unavailable")
	}
	return value, nil
}

func TestProjectVerificationLayerTriadClosesSeparatelyRetainedObservations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		setup       func(testing.TB) (sourceobservation.Project, observationResolver)
		wantErr     error
		wantSummary sourceobservation.Summary
		name        string
	}{
		{
			name: "positive exact package membership resolves every child digest",
			setup: func(t testing.TB) (sourceobservation.Project, observationResolver) {
				return packagedObservationFixture(t, true)
			},
			wantSummary: sourceobservation.Summary{Packages: 1, Files: 2, PackagedFiles: 2, Bytes: 44, Declarations: 3, Tests: 1, Benchmarks: 1, FuzzTargets: 1},
		},
		{
			name: "negative omitted package membership cannot hide one owned file",
			setup: func(t testing.TB) (sourceobservation.Project, observationResolver) {
				return packagedObservationFixture(t, false)
			},
			wantErr: core.ErrSourceObservationConflict,
		},
		{
			name: "negative package observation from another revision is stale",
			setup: func(t testing.TB) (sourceobservation.Project, observationResolver) {
				project, resolver := packagedObservationFixture(t, true)
				path := observedPath(t, "exchange")
				packageObservation := resolver.packages[path]
				packageObservation.Revision = observedOtherCommit(t)
				resolver.packages[path] = packageObservation
				return project, resolver
			},
			wantErr: core.ErrSourceObservationConflict,
		},
		{
			name: "negative file observation whose bytes changed cannot satisfy its digest reference",
			setup: func(t testing.TB) (sourceobservation.Project, observationResolver) {
				project, resolver := rootObservationFixture(t)
				path := observedPath(t, "README.md")
				file := resolver.files[path]
				file.SourceDigest = core.SHA256Of([]byte("changed source bytes"))
				resolver.files[path] = file
				return project, resolver
			},
			wantErr: core.ErrSourceObservationConflict,
		},
		{
			name: "negative file observation from another revision is stale",
			setup: func(t testing.TB) (sourceobservation.Project, observationResolver) {
				project, resolver := rootObservationFixture(t)
				path := observedPath(t, "README.md")
				file := resolver.files[path]
				file.Revision = observedOtherCommit(t)
				resolver.files[path] = file
				return project, resolver
			},
			wantErr: core.ErrSourceObservationConflict,
		},
		{
			name: "negative project file index count cannot exceed its streamed membership",
			setup: func(t testing.TB) (sourceobservation.Project, observationResolver) {
				project, resolver := rootObservationFixture(t)
				project.Files.Count++
				return project, resolver
			},
			wantErr: core.ErrSourceObservationConflict,
		},
		{
			name: "neutral root file needs no invented package observation",
			setup: func(t testing.TB) (sourceobservation.Project, observationResolver) {
				return rootObservationFixture(t)
			},
			wantSummary: sourceobservation.Summary{Files: 1, UnpackagedFiles: 1, Bytes: 9},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			project, resolver := tc.setup(t)
			gotSummary, gotErr := sourceobservation.VerifyProject(context.Background(), project, resolver)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || gotSummary != (sourceobservation.Summary{}) {
					t.Fatalf("VerifyProject() = (%+v, %v), want (zero, %v)", gotSummary, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || gotSummary != tc.wantSummary {
				t.Fatalf("VerifyProject() = (%+v, %v), want (%+v, nil)", gotSummary, gotErr, tc.wantSummary)
			}
		})
	}
}

func TestProjectVerificationAcceptsParentAndChildPackagesWithoutGlobalPathMerge(t *testing.T) {
	t.Parallel()

	buildContext := observationContext(t)
	parentPath := observedPath(t, "foo")
	childPath := observedPath(t, "foo/bar")
	parentFile := observedFile(t, "foo/util.go", &parentPath, buildContext.ID, nil)
	childFile := observedFile(t, "foo/bar/x.go", &childPath, buildContext.ID, nil)
	parentFileReference := observedFileReference(t, parentFile)
	childFileReference := observedFileReference(t, childFile)
	commit := observedCommit(t)
	parent := sourceobservation.Package{
		Repository: observedRepository(t), Path: parentPath, Revision: commit,
		Files: observedFileMembership(t, []sourceobservation.FileReference{parentFileReference}),
	}
	child := sourceobservation.Package{
		Repository: observedRepository(t), Path: childPath, Revision: commit,
		Files: observedFileMembership(t, []sourceobservation.FileReference{childFileReference}),
	}
	parentDigest, parentDigestErr := parent.ObservationDigest()
	childDigest, childDigestErr := child.ObservationDigest()
	if err := errors.Join(parentDigestErr, childDigestErr); err != nil {
		t.Fatalf("Package.ObservationDigest(parent and child) error = %v, want nil", err)
	}
	projectFiles := []sourceobservation.FileReference{parentFileReference, childFileReference}
	projectPackages := []sourceobservation.PackageReference{
		{Path: parentPath, ObservationDigest: parentDigest},
		{Path: childPath, ObservationDigest: childDigest},
	}
	project := observedProject(t, commit, buildContext, projectFiles, projectPackages)
	resolver := observationResolver{
		files: map[core.SourcePath]sourceobservation.File{
			parentFile.Path: parentFile,
			childFile.Path:  childFile,
		},
		packages: map[core.SourcePath]sourceobservation.Package{
			parentPath: parent,
			childPath:  child,
		},
		projectFiles:    projectFiles,
		projectPackages: projectPackages,
		packageFiles: map[core.SourcePath][]sourceobservation.FileReference{
			parentPath: {parentFileReference},
			childPath:  {childFileReference},
		},
	}

	gotSummary, gotErr := sourceobservation.VerifyProject(context.Background(), project, resolver)
	wantSummary := sourceobservation.Summary{
		Packages: 2, Files: 2, PackagedFiles: 2,
		Bytes: parentFile.Bytes.Uint64() + childFile.Bytes.Uint64(),
	}
	if gotErr != nil || gotSummary != wantSummary {
		t.Fatalf("VerifyProject(parent and child packages) = (%+v, %v), want (%+v, nil)", gotSummary, gotErr, wantSummary)
	}
}

func packagedObservationFixture(t testing.TB, includeSecond bool) (sourceobservation.Project, observationResolver) {
	t.Helper()

	context := observationContext(t)
	packagePath := observedPath(t, "exchange")
	first := observedFile(t, "exchange/client_test.go", &packagePath, context.ID, []sourceobservation.Declaration{
		observedDeclaration(t, "TestClient", sourceobservation.DeclarationTest, 10),
		observedDeclaration(t, "BenchmarkClient", sourceobservation.DeclarationBenchmark, 20),
	})
	second := observedFile(t, "exchange/fuzz_test.go", &packagePath, context.ID, []sourceobservation.Declaration{
		observedDeclaration(t, "FuzzClient", sourceobservation.DeclarationFuzzTarget, 10),
	})
	firstReference := observedFileReference(t, first)
	secondReference := observedFileReference(t, second)
	packageFiles := []sourceobservation.FileReference{firstReference}
	if includeSecond {
		packageFiles = append(packageFiles, secondReference)
	}
	commit := observedCommit(t)
	packageMembership := observedFileMembership(t, packageFiles)
	packageObservation := sourceobservation.Package{Repository: observedRepository(t), Path: packagePath, Revision: commit, Files: packageMembership}
	packageDigest, packageDigestErr := packageObservation.ObservationDigest()
	if packageDigestErr != nil {
		t.Fatalf("Package.ObservationDigest() error = %v, want nil", packageDigestErr)
	}
	projectFiles := []sourceobservation.FileReference{firstReference, secondReference}
	projectPackages := []sourceobservation.PackageReference{{Path: packagePath, ObservationDigest: packageDigest}}
	project := observedProject(t, commit, context, projectFiles, projectPackages)
	resolver := observationResolver{
		files:           map[core.SourcePath]sourceobservation.File{first.Path: first, second.Path: second},
		packages:        map[core.SourcePath]sourceobservation.Package{packagePath: packageObservation},
		projectFiles:    projectFiles,
		projectPackages: projectPackages,
		packageFiles:    map[core.SourcePath][]sourceobservation.FileReference{packagePath: packageFiles},
	}
	return project, resolver
}

func rootObservationFixture(t testing.TB) (sourceobservation.Project, observationResolver) {
	t.Helper()

	context := observationContext(t)
	file := observedFile(t, "README.md", nil, context.ID, nil)
	reference := observedFileReference(t, file)
	project := observedProject(t, observedCommit(t), context, []sourceobservation.FileReference{reference}, nil)
	return project, observationResolver{
		files: map[core.SourcePath]sourceobservation.File{file.Path: file}, packages: map[core.SourcePath]sourceobservation.Package{},
		projectFiles: []sourceobservation.FileReference{reference}, packageFiles: map[core.SourcePath][]sourceobservation.FileReference{},
	}
}

func observedProject(t testing.TB, commit core.BuildCommit, buildContext sourceobservation.BuildContext, files []sourceobservation.FileReference, packages []sourceobservation.PackageReference) sourceobservation.Project {
	t.Helper()

	repository := observedRepository(t)
	toolchain, toolchainErr := sourceobservation.NewToolchain("go1.27.1")
	if toolchainErr != nil {
		t.Fatalf("project observation identity setup error = %v, want nil", toolchainErr)
	}
	return sourceobservation.Project{
		Repository: repository, Revision: commit, Toolchain: toolchain,
		Contexts: []sourceobservation.BuildContext{buildContext},
		Files:    observedFileMembership(t, files), Packages: observedPackageMembership(t, packages),
	}
}

func observedFile(t testing.TB, path string, packagePath *core.SourcePath, contextID sourceobservation.ContextID, declarations []sourceobservation.Declaration) sourceobservation.File {
	t.Helper()

	parsedPath := observedPath(t, path)
	language, languageErr := sourceobservation.NewLanguage("go")
	if packagePath == nil {
		language, languageErr = sourceobservation.NewLanguage("markdown")
	}
	extent, extentErr := core.NewByteLength(uint64(len(path)))
	if err := errors.Join(languageErr, extentErr); err != nil {
		t.Fatalf("file observation fixture error = %v, want nil", err)
	}
	return sourceobservation.File{
		Repository: observedRepository(t), Path: parsedPath, Package: packagePath, Revision: observedCommit(t), SourceDigest: core.SHA256Of([]byte(path)), Bytes: extent,
		Language: language, Generated: sourceobservation.GeneratedAuthored,
		Selections:   []sourceobservation.BuildSelection{{Context: contextID, State: sourceobservation.SelectionIncluded}},
		Declarations: declarations, Imports: []sourceobservation.Import{}, Effects: []sourceobservation.Effect{}, References: []sourceobservation.Reference{},
	}
}

func observedRepository(t testing.TB) core.RepositoryIdentity {
	t.Helper()

	got, err := core.NewRepositoryIdentity("github.com/deliri/primitive")
	if err != nil {
		t.Fatalf("core.NewRepositoryIdentity() error = %v, want nil", err)
	}
	return got
}

func observedFileMembership(t testing.TB, references []sourceobservation.FileReference) sourceobservation.FileMembership {
	t.Helper()

	got, err := sourceobservation.ConsumeFileReferences(func(emit sourceobservation.EmitFileReference) error {
		for _, reference := range references {
			if emitErr := emit(reference); emitErr != nil {
				return emitErr
			}
		}
		return nil
	}, func(sourceobservation.FileReference) error { return nil })
	if err != nil {
		t.Fatalf("sourceobservation.ConsumeFileReferences() error = %v, want nil", err)
	}
	return got
}

func observedPackageMembership(t testing.TB, references []sourceobservation.PackageReference) sourceobservation.PackageMembership {
	t.Helper()

	got, err := sourceobservation.ConsumePackageReferences(func(emit sourceobservation.EmitPackageReference) error {
		for _, reference := range references {
			if emitErr := emit(reference); emitErr != nil {
				return emitErr
			}
		}
		return nil
	}, func(sourceobservation.PackageReference) error { return nil })
	if err != nil {
		t.Fatalf("sourceobservation.ConsumePackageReferences() error = %v, want nil", err)
	}
	return got
}

func observedFileReference(t testing.TB, file sourceobservation.File) sourceobservation.FileReference {
	t.Helper()

	digest, err := file.ObservationDigest()
	if err != nil {
		t.Fatalf("File.ObservationDigest() error = %v, want nil", err)
	}
	return sourceobservation.FileReference{Path: file.Path, Package: file.Package, ObservationDigest: digest}
}

func observedDeclaration(t testing.TB, name string, kind sourceobservation.DeclarationKind, line uint32) sourceobservation.Declaration {
	t.Helper()

	symbol, err := sourceobservation.NewSymbol(name)
	if err != nil {
		t.Fatalf("sourceobservation.NewSymbol(%q) error = %v, want nil", name, err)
	}
	return sourceobservation.Declaration{Name: symbol, Kind: kind, Line: line, Column: 1, Exported: true}
}

func observationContext(t testing.TB) sourceobservation.BuildContext {
	t.Helper()

	id, idErr := sourceobservation.NewContextID("darwin-arm64")
	goos, goosErr := sourceobservation.NewSymbol("darwin")
	goarch, goarchErr := sourceobservation.NewSymbol("arm64")
	if err := errors.Join(idErr, goosErr, goarchErr); err != nil {
		t.Fatalf("build context fixture error = %v, want nil", err)
	}
	return sourceobservation.BuildContext{ID: id, GOOS: goos, GOARCH: goarch, Tags: []sourceobservation.Symbol{}}
}

func observedCommit(t testing.TB) core.BuildCommit {
	t.Helper()

	got, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	return got
}

func observedOtherCommit(t testing.TB) core.BuildCommit {
	t.Helper()

	got, err := core.ParseBuildCommit("fedcba9876543210fedcba9876543210fedcba98")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit(other) error = %v, want nil", err)
	}
	return got
}
