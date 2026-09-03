package sourceobservation

import (
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

type FileReference struct {
	Path              core.SourcePath   `json:"path"`
	Package           *core.SourcePath  `json:"package,omitempty"`
	ObservationDigest core.SHA256Digest `json:"observation_digest"`
}

func (r FileReference) Validate() error {
	if err := contractJoin(r.Path.Validate(), r.ObservationDigest.Validate()); err != nil {
		return err
	}
	if r.Package == nil {
		return nil
	}
	if err := r.Package.Validate(); err != nil {
		return err
	}
	if !fileOwnedByPackage(*r.Package, r.Path) {
		return conflictError(errors.New("source observation file reference is outside its package"))
	}
	return nil
}

// FileMembership commits to one complete canonical package-then-path stream
// of file references. Count reports cardinality; Digest binds every record
// and its order without placing the records in one JSON document.
type FileMembership struct {
	Digest core.SHA256Digest `json:"digest"`
	Bytes  core.ByteLength   `json:"bytes"`
	Count  uint64            `json:"count"`
}

func (m FileMembership) Validate() error {
	if err := contractJoin(m.Digest.Validate(), m.Bytes.Validate()); err != nil {
		return err
	}
	if (m.Count == 0) != (m.Bytes.Uint64() == 0) {
		return conflictError(errors.New("source observation file membership byte accounting does not close"))
	}
	return nil
}

type PackageReference struct {
	Path              core.SourcePath   `json:"path"`
	ObservationDigest core.SHA256Digest `json:"observation_digest"`
}

func (r PackageReference) Validate() error {
	return contractJoin(r.Path.Validate(), r.ObservationDigest.Validate())
}

// PackageMembership commits to one complete canonical stream of package
// references. An empty stream is valid for a project with no Go packages.
type PackageMembership struct {
	Digest core.SHA256Digest `json:"digest"`
	Bytes  core.ByteLength   `json:"bytes"`
	Count  uint64            `json:"count"`
}

func (m PackageMembership) Validate() error {
	if err := contractJoin(m.Digest.Validate(), m.Bytes.Validate()); err != nil {
		return err
	}
	if (m.Count == 0) != (m.Bytes.Uint64() == 0) {
		return conflictError(errors.New("source observation package membership byte accounting does not close"))
	}
	return nil
}

// Package is a lossless index over separately retained file observations.
// Files is a stream commitment rather than a repository-sized slice.
type Package struct {
	Repository core.RepositoryIdentity `json:"repository"`
	Path       core.SourcePath         `json:"path"`
	Revision   core.BuildCommit        `json:"revision"`
	Files      FileMembership          `json:"files"`
}

func (p Package) Validate() error {
	if err := contractJoin(p.Repository.Validate(), p.Path.Validate(), p.Revision.Validate(), p.Files.Validate()); err != nil {
		return err
	}
	if p.Files.Count == 0 {
		return contractError(errors.New("source observation package has no file membership"))
	}
	return nil
}

// Project is the exact-revision index over separately retained observations.
// Dirty reports whether inspected bytes differed from Revision. Files and
// Packages are stream commitments, not materialized project inventories.
type Project struct {
	Repository core.RepositoryIdentity `json:"repository"`
	Revision   core.BuildCommit        `json:"revision"`
	Toolchain  Toolchain               `json:"toolchain"`
	Contexts   []BuildContext          `json:"build_contexts"`
	Files      FileMembership          `json:"files"`
	Packages   PackageMembership       `json:"packages"`
	Dirty      bool                    `json:"dirty"`
}

func (p Project) Validate() error {
	if err := contractJoin(p.Repository.Validate(), p.Revision.Validate(), p.Toolchain.Validate(), p.Files.Validate(), p.Packages.Validate()); err != nil {
		return err
	}
	if len(p.Contexts) == 0 || p.Files.Count == 0 {
		return contractError(errors.New("source observation project omits build contexts or files"))
	}
	if p.Packages.Count > p.Files.Count {
		return conflictError(errors.New("source observation project has more packages than files"))
	}
	return validateBuildContexts(p.Contexts)
}

func fileOwnedByPackage(packagePath, filePath core.SourcePath) bool {
	packageText := packagePath.String()
	fileText := filePath.String()
	separator := strings.LastIndexByte(fileText, '/')
	if separator < 0 {
		return packageText == "."
	}
	return packageText == fileText[:separator]
}
