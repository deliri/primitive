package runworkspace

import (
	"context"
	"errors"
	"io"
	"io/fs"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/standard"
)

// ArtifactEvidence is a filestore-observed experiment file. The expectation
// remains the authority for kind, path, media type, and byte ceiling; these
// fields are repeated here only as the signed observation of the exact file.
type ArtifactEvidence struct {
	Path       core.RelativePath
	MediaType  core.HTTPMediaType
	Bytes      core.ByteLength
	SHA256     core.SHA256Digest
	Experiment standard.ExperimentID
	Kind       runnercontrol.ArtifactKind
}

func (e ArtifactEvidence) Validate() error {
	return errors.Join(e.Experiment.Validate(), e.Kind.Validate(), e.Path.Validate(), e.MediaType.Validate(), e.SHA256.Validate(), e.Bytes.Validate())
}

// ObserveArtifact streams and hashes a signed expected output through the
// rooted filestore. Optional absence is returned as present=false; every other
// refusal remains an error.
func (m Manager) ObserveArtifact(ctx context.Context, workspace Experiment, expectation runnercontrol.ArtifactExpectation) (ArtifactEvidence, bool, error) {
	if err := errors.Join(m.Validate(), workspace.Validate(), expectation.Validate()); err != nil {
		return ArtifactEvidence{}, false, err
	}
	path, err := artifactNativePath(expectation.Path)
	if err != nil {
		return ArtifactEvidence{}, false, err
	}
	if err := validateExperimentArtifactPath(workspace, path); err != nil {
		return ArtifactEvidence{}, false, err
	}
	digest := core.NewDigestWriter()
	extent, err := filestore.Read(ctx, filestore.ReadRequest{
		Destination: digest, Location: filestore.Location{Root: m.root, Path: path}, MaximumBytes: expectation.MaximumBytes,
	})
	if errors.Is(err, fs.ErrNotExist) && !expectation.Required {
		return ArtifactEvidence{}, false, nil
	}
	if err != nil {
		return ArtifactEvidence{}, false, err
	}
	sha256, digested, err := digest.Seal()
	if err != nil || digested != extent {
		return ArtifactEvidence{}, false, errors.Join(core.ErrPrimitiveContract, err)
	}
	evidence := ArtifactEvidence{Experiment: workspace.Identity, Kind: expectation.Kind, Path: path, MediaType: expectation.MediaType, SHA256: sha256, Bytes: extent}
	return evidence, true, evidence.Validate()
}

func (m Manager) ValidateArtifactExpectations(workspace Experiment, expectations []runnercontrol.ArtifactExpectation) error {
	if err := errors.Join(m.Validate(), workspace.Validate()); err != nil {
		return err
	}
	for index := range expectations {
		if err := expectations[index].Validate(); err != nil {
			return err
		}
		path, err := artifactNativePath(expectations[index].Path)
		if err != nil {
			return err
		}
		if err := validateExperimentArtifactPath(workspace, path); err != nil {
			return err
		}
	}
	return nil
}

func artifactNativePath(path standard.SourcePath) (core.RelativePath, error) {
	if err := path.Validate(); err != nil {
		return core.RelativePath{}, err
	}
	return core.ParseRelativePath(path.String())
}

// StreamArtifactEvidence proves the retained bytes still match the observation
// while streaming them to the next evidence authority.
func (m Manager) StreamArtifactEvidence(ctx context.Context, workspace Experiment, evidence ArtifactEvidence, destination io.Writer) error {
	if destination == nil {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(m.Validate(), workspace.Validate(), evidence.Validate(), validateExperimentArtifactPath(workspace, evidence.Path)); err != nil {
		return err
	}
	if workspace.Identity != evidence.Experiment {
		return core.ErrPrimitiveContract
	}
	maximum := evidence.Bytes.Uint64()
	if maximum == 0 {
		maximum = 1
	}
	limit, err := core.NewByteCount(maximum)
	if err != nil {
		return err
	}
	read, digest, err := m.streamArtifact(ctx, evidence.Path, limit, destination)
	if err != nil {
		return err
	}
	return verifyStreamedArtifact(read, digest, evidence)
}

func verifyStreamedArtifact(read core.ByteLength, digest *core.DigestWriter, evidence ArtifactEvidence) error {
	sha256, digested, err := digest.Seal()
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if read != evidence.Bytes || digested != evidence.Bytes || sha256 != evidence.SHA256 {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (m Manager) streamArtifact(ctx context.Context, path core.RelativePath, maximum core.ByteCount, destination io.Writer) (core.ByteLength, *core.DigestWriter, error) {
	digest := core.NewDigestWriter()
	read, err := filestore.Read(ctx, filestore.ReadRequest{
		Destination: io.MultiWriter(destination, digest), Location: filestore.Location{Root: m.root, Path: path}, MaximumBytes: maximum,
	})
	return read, digest, err
}

func validateExperimentArtifactPath(workspace Experiment, path core.RelativePath) error {
	base, err := pathBase(path)
	if err != nil {
		return err
	}
	want, err := workspace.Output.Join(base)
	if err != nil || want != path {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

func pathBase(path core.RelativePath) (core.PathComponent, error) {
	if err := path.Validate(); err != nil {
		return core.PathComponent{}, err
	}
	absolute, err := core.ParseAbsolutePath("/" + path.String())
	if err != nil {
		return core.PathComponent{}, err
	}
	return absolute.Base()
}

var _ core.Validatable = ArtifactEvidence{}
