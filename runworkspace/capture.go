package runworkspace

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/projectstandards"
)

type CaptureKind uint8

const (
	CaptureKindUnknown CaptureKind = iota
	CaptureStdout
	CaptureStderr
	captureKindLimit
)

func (k CaptureKind) Validate() error {
	if k <= CaptureKindUnknown || k >= captureKindLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (k CaptureKind) String() string {
	switch k {
	case CaptureStdout:
		return "stdout"
	case CaptureStderr:
		return "stderr"
	default:
		return ""
	}
}

type CaptureEvidence struct {
	Experiment projectstandards.ExperimentID
	Kind       CaptureKind
	Path       core.RelativePath
	SHA256     core.SHA256Digest
	Bytes      core.ByteLength
}

func (e CaptureEvidence) Validate() error {
	return errors.Join(e.Experiment.Validate(), e.Kind.Validate(), e.Path.Validate(), e.SHA256.Validate(), e.Bytes.Validate())
}

type Capture struct {
	root       *os.Root
	file       *os.File
	experiment projectstandards.ExperimentID
	kind       CaptureKind
	path       core.RelativePath
	digest     *core.DigestWriter
	sealed     bool
}

func (m Manager) OpenCapture(ctx context.Context, workspace Experiment, kind CaptureKind) (*Capture, error) {
	if err := errors.Join(m.Validate(), workspace.Validate(), kind.Validate()); err != nil {
		return nil, err
	}
	name, err := captureName(workspace.Identity, kind)
	if err != nil {
		return nil, err
	}
	path, err := workspace.Output.Join(name)
	if err != nil {
		return nil, err
	}
	file, err := filestore.OpenAppend(ctx, filestore.AppendRequest{
		Location: filestore.Location{Root: m.root, Path: path}, Mode: fs.FileMode(0o600), Append: filestore.AppendCreate,
	})
	if err != nil {
		return nil, err
	}
	capture := &Capture{root: m.root, file: file, experiment: workspace.Identity, kind: kind, path: path, digest: core.NewDigestWriter()}
	if err := capture.Validate(); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return capture, nil
}

// StreamCaptureEvidence reads one sealed capture through the rooted filestore
// boundary and proves the on-disk bytes still match the typed reference.
func (m Manager) StreamCaptureEvidence(ctx context.Context, workspace Experiment, evidence CaptureEvidence, destination io.Writer) error {
	maximum, err := m.validateCaptureEvidenceStream(workspace, evidence, destination)
	if err != nil {
		return err
	}
	digest := core.NewDigestWriter()
	read, err := filestore.Read(ctx, filestore.ReadRequest{
		Destination:  io.MultiWriter(destination, digest),
		Location:     filestore.Location{Root: m.root, Path: evidence.Path},
		MaximumBytes: maximum,
	})
	if err != nil {
		return err
	}
	gotDigest, gotBytes, err := digest.Seal()
	if err != nil || read != evidence.Bytes || gotBytes != evidence.Bytes || gotDigest != evidence.SHA256 {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

func (m Manager) validateCaptureEvidenceStream(workspace Experiment, evidence CaptureEvidence, destination io.Writer) (core.ByteCount, error) {
	if destination == nil {
		return core.ByteCount{}, core.ErrPrimitiveContract
	}
	if err := errors.Join(m.Validate(), workspace.Validate(), evidence.Validate()); err != nil || workspace.Identity != evidence.Experiment {
		return core.ByteCount{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	name, err := captureName(evidence.Experiment, evidence.Kind)
	if err != nil {
		return core.ByteCount{}, err
	}
	wantPath, err := workspace.Output.Join(name)
	if err != nil || wantPath != evidence.Path {
		return core.ByteCount{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	maximum := evidence.Bytes.Uint64()
	if maximum == 0 {
		maximum = 1
	}
	return core.NewByteCount(maximum)
}

func captureName(experiment projectstandards.ExperimentID, kind CaptureKind) (core.PathComponent, error) {
	encoded, err := experiment.MarshalJSON()
	if err != nil {
		return core.PathComponent{}, err
	}
	identity, err := core.DecodeJSONStringToken(encoded)
	if err != nil {
		return core.PathComponent{}, err
	}
	return core.ParsePathComponent(identity + "." + kind.String())
}

func (c *Capture) Validate() error {
	if c == nil || c.root == nil || c.file == nil || c.digest == nil || c.sealed {
		return core.ErrPrimitiveContract
	}
	return errors.Join(c.experiment.Validate(), c.kind.Validate(), c.path.Validate())
}

func (c *Capture) Write(data []byte) (int, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	written, writeErr := c.file.Write(data)
	if written > 0 {
		digested, digestErr := c.digest.Write(data[:written])
		if digestErr != nil || digested != written {
			return written, errors.Join(core.ErrPrimitiveContract, writeErr, digestErr)
		}
	}
	if writeErr != nil || written != len(data) {
		return written, errors.Join(core.ErrPrimitiveContract, writeErr)
	}
	return written, nil
}

func (c *Capture) Seal() (CaptureEvidence, error) {
	if err := c.Validate(); err != nil {
		return CaptureEvidence{}, err
	}
	syncErr := c.file.Sync()
	closeErr := c.file.Close()
	c.sealed = true
	if err := errors.Join(syncErr, closeErr); err != nil {
		return CaptureEvidence{}, err
	}
	digest, extent, err := c.digest.Seal()
	if err != nil {
		return CaptureEvidence{}, err
	}
	evidence := CaptureEvidence{Experiment: c.experiment, Kind: c.kind, Path: c.path, SHA256: digest, Bytes: extent}
	return evidence, evidence.Validate()
}

func (c *Capture) Abort(ctx context.Context) error {
	if c == nil || c.root == nil || c.file == nil || c.sealed {
		return core.ErrPrimitiveContract
	}
	closeErr := c.file.Close()
	c.sealed = true
	removeErr := filestore.Remove(ctx, filestore.RemovalRequest{Location: filestore.Location{Root: c.root, Path: c.path}})
	return errors.Join(closeErr, removeErr)
}

var (
	_ core.Validatable = CaptureKindUnknown
	_ core.Validatable = CaptureEvidence{}
)
