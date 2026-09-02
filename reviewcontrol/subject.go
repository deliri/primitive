package reviewcontrol

import (
	"errors"
	"path"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/projectstandards"
)

type Subject struct {
	Project projectstandards.SubjectIdentity  `json:"project"`
	Source  projectstandards.SourceCoordinate `json:"source"`
	Module  gomodule.Path                     `json:"module"`
	Package gomodule.ImportPath               `json:"package"`
	File    projectstandards.SourcePath       `json:"file"`
	SHA256  core.SHA256Digest                 `json:"sha256"`
	Bytes   core.ByteCount                    `json:"bytes"`
}

func (s Subject) Validate() error {
	if err := errors.Join(s.Project.Validate(), s.Source.Validate(), s.Module.Validate(),
		s.Package.Validate(), s.File.Validate(), s.SHA256.Validate(), s.Bytes.Validate()); err != nil {
		return contractError(err)
	}
	if s.Project.Repository != s.Source.Repository || !packageWithinModule(s.Module.String(), s.Package.String()) ||
		!fileWithinPackage(s.Module.String(), s.Package.String(), s.File.String()) {
		return errors.Join(core.ErrReviewControlSubjectMismatch, contractError())
	}
	return nil
}

func packageWithinModule(module, packagePath string) bool {
	return packagePath == module || strings.HasPrefix(packagePath, module+"/")
}

func fileWithinPackage(module, packagePath, file string) bool {
	directory := strings.TrimPrefix(packagePath, module)
	directory = strings.TrimPrefix(directory, "/")
	if directory == "" {
		return path.Dir(file) == "."
	}
	return path.Dir(file) == directory
}

func SameSubject(left, right Subject) bool { return left == right }

// VerifySubject distinguishes a different source identity from a changed
// revision or byte projection of the same identity.
func VerifySubject(reviewed, current Subject) error {
	if err := errors.Join(reviewed.Validate(), current.Validate()); err != nil {
		return err
	}
	if !sameSubjectIdentity(reviewed, current) {
		return errors.Join(core.ErrReviewControlSubjectMismatch, contractError())
	}
	if !sameSourceProjection(reviewed, current) {
		return errors.Join(core.ErrReviewControlStaleSource, contractError())
	}
	return nil
}

func sameSubjectIdentity(left, right Subject) bool {
	return left.Project == right.Project && left.Source.Repository == right.Source.Repository &&
		left.Module == right.Module && left.Package == right.Package && left.File == right.File
}

func sameSourceProjection(left, right Subject) bool {
	return left.Source.Commit == right.Source.Commit && left.Source.Tree == right.Source.Tree &&
		left.SHA256 == right.SHA256 && left.Bytes == right.Bytes
}
