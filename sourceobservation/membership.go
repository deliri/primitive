package sourceobservation

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// EmitFileReference receives one validated canonical membership record.
type EmitFileReference func(FileReference) error

// FileReferenceStream emits file references in ascending package-then-path
// order. Unpackaged files precede packaged files. Cardinality is reported by
// the resulting FileMembership, not capped here.
type FileReferenceStream func(EmitFileReference) error

// EmitPackageReference receives one validated canonical membership record.
type EmitPackageReference func(PackageReference) error

// PackageReferenceStream emits package references in ascending source-path
// order. Cardinality is reported rather than constrained by a JSON array.
type PackageReferenceStream func(EmitPackageReference) error

// ConsumeFileReferences validates, forwards, counts, and hashes one complete
// JSON-lines membership stream while retaining only its preceding path.
func ConsumeFileReferences(stream FileReferenceStream, destination EmitFileReference) (FileMembership, error) {
	if stream == nil || destination == nil {
		return FileMembership{}, contractError(errors.New("source observation file membership dependency is nil"))
	}
	consumer := fileMembershipConsumer{destination: destination, digest: core.NewDigestWriter()}
	streamErr := stream(consumer.accept)
	if err := errors.Join(streamErr, consumer.err); err != nil {
		return FileMembership{}, err
	}
	return consumer.seal()
}

type fileMembershipConsumer struct {
	destination EmitFileReference
	digest      *core.DigestWriter
	previous    FileReference
	count       uint64
	err         error
	seen        bool
}

func (c *fileMembershipConsumer) accept(reference FileReference) error {
	if c.err != nil {
		return c.err
	}
	c.err = c.acceptOne(reference)
	return c.err
}

func (c *fileMembershipConsumer) acceptOne(reference FileReference) error {
	if err := reference.Validate(); err != nil {
		return err
	}
	if c.seen && compareFileReferences(c.previous, reference) >= 0 {
		return conflictError(errors.New("source observation file membership is duplicated or not canonical"))
	}
	if c.count == ^uint64(0) {
		return contractError(errors.New("source observation file membership count overflows uint64"))
	}
	if err := writeFileReference(c.digest, reference); err != nil {
		return err
	}
	c.previous = reference
	c.seen = true
	c.count++
	return c.destination(reference)
}

func compareFileReferences(left, right FileReference) int {
	leftPackage := fileReferencePackageCoordinate(left.Package)
	rightPackage := fileReferencePackageCoordinate(right.Package)
	if leftPackage < rightPackage {
		return -1
	}
	if leftPackage > rightPackage {
		return 1
	}
	if left.Path.String() < right.Path.String() {
		return -1
	}
	if left.Path.String() > right.Path.String() {
		return 1
	}
	return 0
}

func fileReferencePackageCoordinate(value *core.SourcePath) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func (c *fileMembershipConsumer) seal() (FileMembership, error) {
	digest, length, err := c.digest.Seal()
	if err != nil {
		return FileMembership{}, contractError(err)
	}
	return FileMembership{Digest: digest, Bytes: length, Count: c.count}, nil
}

// ConsumePackageReferences validates, forwards, counts, and hashes one
// complete JSON-lines membership stream with O(1) retained state.
func ConsumePackageReferences(stream PackageReferenceStream, destination EmitPackageReference) (PackageMembership, error) {
	if stream == nil || destination == nil {
		return PackageMembership{}, contractError(errors.New("source observation package membership dependency is nil"))
	}
	consumer := packageMembershipConsumer{destination: destination, digest: core.NewDigestWriter()}
	streamErr := stream(consumer.accept)
	if err := errors.Join(streamErr, consumer.err); err != nil {
		return PackageMembership{}, err
	}
	digest, length, err := consumer.digest.Seal()
	if err != nil {
		return PackageMembership{}, contractError(err)
	}
	return PackageMembership{Digest: digest, Bytes: length, Count: consumer.count}, nil
}

type packageMembershipConsumer struct {
	destination EmitPackageReference
	digest      *core.DigestWriter
	previous    core.SourcePath
	count       uint64
	err         error
	seen        bool
}

func (c *packageMembershipConsumer) accept(reference PackageReference) error {
	if c.err != nil {
		return c.err
	}
	c.err = c.acceptOne(reference)
	return c.err
}

func (c *packageMembershipConsumer) acceptOne(reference PackageReference) error {
	if err := reference.Validate(); err != nil {
		return err
	}
	if c.seen && c.previous.String() >= reference.Path.String() {
		return conflictError(errors.New("source observation package membership is duplicated or not canonical"))
	}
	if c.count == ^uint64(0) {
		return contractError(errors.New("source observation package membership count overflows uint64"))
	}
	if err := writePackageReference(c.digest, reference); err != nil {
		return err
	}
	c.previous = reference.Path
	c.seen = true
	c.count++
	return c.destination(reference)
}

func writeFileReference(destination *core.DigestWriter, reference FileReference) error {
	encoded, err := core.MarshalCanonicalJSONDocument(fileReferenceWire(reference))
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceObservationContract, err)
	}
	return writeMembershipRecord(destination, encoded)
}

func writePackageReference(destination *core.DigestWriter, reference PackageReference) error {
	encoded, err := core.MarshalCanonicalJSONDocument(packageReferenceWire(reference))
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrSourceObservationContract, err)
	}
	return writeMembershipRecord(destination, encoded)
}

func writeMembershipRecord(destination *core.DigestWriter, encoded []byte) error {
	if _, err := destination.Write(encoded); err != nil {
		return contractError(err)
	}
	if _, err := destination.Write([]byte{'\n'}); err != nil {
		return contractError(err)
	}
	return nil
}

type fileReferenceWire FileReference
type packageReferenceWire PackageReference
