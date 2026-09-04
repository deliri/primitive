package sourceobservation

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Resolver supplies separately retained observations and membership streams.
// Storage and source-inspection policy remain outside this agreement.
type Resolver interface {
	ResolveFile(context.Context, FileReference) (File, error)
	ResolvePackage(context.Context, PackageReference) (Package, error)
	StreamProjectFiles(context.Context, Project, EmitFileReference) error
	StreamProjectPackages(context.Context, Project, EmitPackageReference) error
	StreamPackageFiles(context.Context, Package, EmitFileReference) error
}

// Summary reports exact mechanically verified cardinality. It is derived
// accounting, never an acceptance result or repository-size ceiling.
type Summary struct {
	Packages        uint64 `json:"packages"`
	Files           uint64 `json:"files"`
	PackagedFiles   uint64 `json:"packaged_files"`
	UnpackagedFiles uint64 `json:"unpackaged_files"`
	Bytes           uint64 `json:"bytes"`
	Declarations    uint64 `json:"declarations"`
	Tests           uint64 `json:"tests"`
	Benchmarks      uint64 `json:"benchmarks"`
	FuzzTargets     uint64 `json:"fuzz_targets"`
}

func (s Summary) Validate() error {
	if !observationSumEquals(s.Files, s.PackagedFiles, s.UnpackagedFiles) {
		return conflictError(errors.New("source observation file accounting does not close"))
	}
	executable, ok := observationSum(s.Tests, s.Benchmarks, s.FuzzTargets)
	if !ok || executable > s.Declarations {
		return conflictError(errors.New("source observation executable declaration accounting exceeds all declarations"))
	}
	return nil
}

func observationSumEquals(want uint64, values ...uint64) bool {
	got, ok := observationSum(values...)
	return ok && got == want
}

func observationSum(values ...uint64) (uint64, bool) {
	var total uint64
	for _, value := range values {
		if total > ^uint64(0)-value {
			return 0, false
		}
		total += value
	}
	return total, true
}

// VerifyProject checks every stream commitment and child observation while
// retaining only counters, digests, and the current record.
func VerifyProject(ctx context.Context, project Project, resolver Resolver) (Summary, error) {
	if ctx == nil || resolver == nil {
		return Summary{}, contractError(errors.New("source observation verification context or resolver is nil"))
	}
	if err := project.Validate(); err != nil {
		return Summary{}, err
	}
	packagedMembership, err := verifyPackages(ctx, project, resolver)
	if err != nil {
		return Summary{}, err
	}
	summary, projectPackaged, err := verifyFiles(ctx, project, resolver)
	if err != nil {
		return Summary{}, err
	}
	if packagedMembership != projectPackaged {
		return Summary{}, conflictError(errors.New("source observation package membership does not close against project files"))
	}
	summary.Packages = project.Packages.Count
	if err := summary.Validate(); err != nil {
		return Summary{}, err
	}
	return summary, nil
}

func verifyPackages(ctx context.Context, project Project, resolver Resolver) (FileMembership, error) {
	allFiles := fileMembershipConsumer{destination: discardFileReference, digest: core.NewDigestWriter()}
	verifier := packageVerifier{ctx: ctx, project: project, resolver: resolver, allFiles: &allFiles}
	actual, err := ConsumePackageReferences(
		func(emit EmitPackageReference) error {
			return resolver.StreamProjectPackages(ctx, project, emit)
		},
		verifier.verify,
	)
	if err != nil {
		return FileMembership{}, err
	}
	if actual != project.Packages {
		return FileMembership{}, conflictError(errors.New("source observation project package stream differs from its index"))
	}
	return allFiles.seal()
}

type packageVerifier struct {
	ctx      context.Context
	project  Project
	resolver Resolver
	allFiles *fileMembershipConsumer
}

func (v packageVerifier) verify(reference PackageReference) error {
	if err := v.ctx.Err(); err != nil {
		return err
	}
	observed, err := v.resolver.ResolvePackage(v.ctx, reference)
	if err != nil {
		return contractError(err)
	}
	if err := observed.Validate(); err != nil {
		return err
	}
	if err := verifyResolvedPackage(v.project, reference, observed); err != nil {
		return err
	}
	return v.verifyPackageFiles(observed)
}

func verifyResolvedPackage(project Project, reference PackageReference, observed Package) error {
	digest, err := observed.ObservationDigest()
	if err != nil {
		return err
	}
	if observed.Repository != project.Repository || observed.Path != reference.Path || observed.Snapshot != project.Snapshot || digest != reference.ObservationDigest {
		return conflictError(errors.New("source observation resolved package differs from its project reference"))
	}
	return nil
}

func (v packageVerifier) verifyPackageFiles(observed Package) error {
	fileVerifier := packageFileVerifier{
		ctx: v.ctx, project: v.project, packagePath: observed.Path,
		resolver: v.resolver, allFiles: v.allFiles,
	}
	actual, err := ConsumeFileReferences(
		func(emit EmitFileReference) error {
			return v.resolver.StreamPackageFiles(v.ctx, observed, emit)
		},
		fileVerifier.verify,
	)
	if err != nil {
		return err
	}
	if actual != observed.Files {
		return conflictError(errors.New("source observation package file stream differs from its index"))
	}
	return nil
}

type packageFileVerifier struct {
	ctx         context.Context
	project     Project
	packagePath core.SourcePath
	resolver    Resolver
	allFiles    *fileMembershipConsumer
}

func (v packageFileVerifier) verify(reference FileReference) error {
	if err := v.ctx.Err(); err != nil {
		return err
	}
	if reference.Package == nil || *reference.Package != v.packagePath {
		return conflictError(errors.New("source observation package contains a foreign file"))
	}
	if err := v.allFiles.accept(reference); err != nil {
		return err
	}
	file, err := v.resolver.ResolveFile(v.ctx, reference)
	if err != nil {
		return contractError(err)
	}
	if err := verifyResolvedFile(v.project, reference, file); err != nil {
		return err
	}
	if file.Package == nil || *file.Package != v.packagePath {
		return conflictError(errors.New("source observation resolved file package differs from its membership owner"))
	}
	return nil
}

func verifyFiles(ctx context.Context, project Project, resolver Resolver) (Summary, FileMembership, error) {
	packaged := fileMembershipConsumer{destination: discardFileReference, digest: core.NewDigestWriter()}
	var summary Summary
	verifier := projectFileVerifier{ctx: ctx, project: project, resolver: resolver, summary: &summary, packaged: &packaged}
	actual, err := ConsumeFileReferences(
		func(emit EmitFileReference) error {
			return resolver.StreamProjectFiles(ctx, project, emit)
		},
		verifier.verify,
	)
	if err != nil {
		return Summary{}, FileMembership{}, err
	}
	if actual != project.Files {
		return Summary{}, FileMembership{}, conflictError(errors.New("source observation project file stream differs from its index"))
	}
	packagedMembership, err := packaged.seal()
	if err != nil {
		return Summary{}, FileMembership{}, err
	}
	return summary, packagedMembership, nil
}

type projectFileVerifier struct {
	ctx      context.Context
	project  Project
	resolver Resolver
	summary  *Summary
	packaged *fileMembershipConsumer
}

func (v projectFileVerifier) verify(reference FileReference) error {
	if err := v.ctx.Err(); err != nil {
		return err
	}
	file, err := v.resolver.ResolveFile(v.ctx, reference)
	if err != nil {
		return contractError(err)
	}
	if err := verifyResolvedFile(v.project, reference, file); err != nil {
		return err
	}
	if file.Package == nil {
		if v.summary.UnpackagedFiles == ^uint64(0) {
			return contractError(errors.New("source observation unpackaged file count overflows uint64"))
		}
		v.summary.UnpackagedFiles++
	} else {
		if v.summary.PackagedFiles == ^uint64(0) {
			return contractError(errors.New("source observation packaged file count overflows uint64"))
		}
		if err := v.packaged.accept(reference); err != nil {
			return err
		}
		v.summary.PackagedFiles++
	}
	return addFileSummary(v.summary, file)
}

func verifyResolvedFile(project Project, reference FileReference, file File) error {
	if err := file.Validate(); err != nil {
		return err
	}
	if err := verifyResolvedFileIdentity(project, reference, file); err != nil {
		return err
	}
	return verifyResolvedFileContexts(project, file)
}

func verifyResolvedFileIdentity(project Project, reference FileReference, file File) error {
	digest, err := file.ObservationDigest()
	if err != nil {
		return err
	}
	if file.Repository != project.Repository || file.Path != reference.Path || file.Snapshot != project.Snapshot || digest != reference.ObservationDigest {
		return conflictError(errors.New("source observation resolved file differs from its project reference"))
	}
	if !samePackageCoordinate(file.Package, reference.Package) {
		return conflictError(errors.New("source observation resolved file package differs from its file reference"))
	}
	return nil
}

func verifyResolvedFileContexts(project Project, file File) error {
	if len(file.Selections) != len(project.Contexts) {
		return conflictError(errors.New("source observation file does not account for every project build context"))
	}
	for index := range project.Contexts {
		if file.Selections[index].Context != project.Contexts[index].ID {
			return conflictError(errors.New("source observation file build context differs from the project context index"))
		}
	}
	return nil
}

func samePackageCoordinate(left, right *core.SourcePath) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func addFileSummary(summary *Summary, file File) error {
	if summary.Files == ^uint64(0) || summary.Bytes > ^uint64(0)-file.Bytes.Uint64() {
		return contractError(errors.New("source observation project accounting overflows uint64"))
	}
	summary.Files++
	summary.Bytes += file.Bytes.Uint64()
	for _, declaration := range file.Declarations {
		if summary.Declarations == ^uint64(0) {
			return contractError(errors.New("source observation declaration accounting overflows uint64"))
		}
		summary.Declarations++
		switch declaration.Kind {
		case DeclarationTest:
			summary.Tests++
		case DeclarationBenchmark:
			summary.Benchmarks++
		case DeclarationFuzzTarget:
			summary.FuzzTargets++
		}
	}
	return nil
}

func discardFileReference(FileReference) error { return nil }
