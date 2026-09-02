package core

import "encoding"

var (
	_ Validatable              = ArchitectureCatalog{}
	_ Validatable              = PackageIdentity(0)
	_ Validatable              = PackageKind(0)
	_ Validatable              = PackageRole(0)
	_ Validatable              = PackageContract{}
	_ Validatable              = StrictJSONLimits{}
	_ Validatable              = ErrorIdentity(0)
	_ Validatable              = Comparison(0)
	_ Validatable              = ByteCount{}
	_ Validatable              = ByteLength{}
	_ Validatable              = SHA256Digest{}
	_ encoding.TextUnmarshaler = (*SHA256Digest)(nil)
	_ Validatable              = CRC32C{}
	_ encoding.TextUnmarshaler = (*CRC32C)(nil)
	_ Validatable              = Ed25519PublicKey{}
	_ encoding.TextUnmarshaler = (*Ed25519PublicKey)(nil)
	_ Validatable              = SecretMaterial{}
	_ Validatable              = PathComponent{}
	_ Validatable              = AbsolutePath{}
	_ Validatable              = HTTPEndpoint{}
	_ Validatable              = HTTPStatusCode{}
	_ Validatable              = HTTPHeaderName{}
	_ Validatable              = HTTPMediaType{}
	_ Validatable              = Platform{}
	_ Validatable              = OperatingSystem(0)
	_ Validatable              = CPUArchitecture(0)
	_ Validatable              = Offering{}
	_ Validatable              = ReleaseVersion{}
	_ Validatable              = BuildCommit{}
	_ Validatable              = BuildIdentity{}
	_ encoding.TextUnmarshaler = (*Platform)(nil)
	_ encoding.TextUnmarshaler = (*Offering)(nil)
	_ encoding.TextUnmarshaler = (*ReleaseVersion)(nil)
	_ Validatable              = TestIsolationHazardUnknown
	_ Validatable              = TestIsolationScopeUnknown
)

var (
	_ ValidatedJSONMarshaler = PackageIdentity(0)
	_ ValidatedJSONMarshaler = PackageKind(0)
	_ ValidatedJSONMarshaler = PackageRole(0)
	_ ValidatedJSONMarshaler = ErrorIdentity(0)
	_ ValidatedJSONMarshaler = ByteCount{}
	_ ValidatedJSONMarshaler = ByteLength{}
	_ ValidatedJSONMarshaler = SHA256Digest{}
	_ ValidatedJSONMarshaler = CRC32C{}
	_ ValidatedJSONMarshaler = Ed25519PublicKey{}
	_ ValidatedJSONMarshaler = SecretMaterial{}
	_ ValidatedJSONMarshaler = PathComponent{}
	_ ValidatedJSONMarshaler = AbsolutePath{}
	_ ValidatedJSONMarshaler = HTTPEndpoint{}
	_ ValidatedJSONMarshaler = HTTPStatusCode{}
	_ ValidatedJSONMarshaler = HTTPHeaderName{}
	_ ValidatedJSONMarshaler = HTTPMediaType{}
	_ ValidatedJSONMarshaler = Platform{}
	_ ValidatedJSONMarshaler = OperatingSystem(0)
	_ ValidatedJSONMarshaler = CPUArchitecture(0)
	_ ValidatedJSONMarshaler = Offering{}
	_ ValidatedJSONMarshaler = ReleaseVersion{}
	_ ValidatedJSONMarshaler = BuildCommit{}
	_ ValidatedJSONMarshaler = BuildIdentity{}
	_ ValidatedJSONMarshaler = CatalogPageLimit{}
	_ ValidatedJSONMarshaler = CatalogSelectionKind(0)
	_ ValidatedJSONMarshaler = CatalogPositionKind(0)
	_ ValidatedJSONMarshaler = CatalogContinuationState(0)
)

// Off-wire enums are bound to the shared interface here so that the marker is a
// compiler-checked contract rather than a repeated method name. A declaring
// enum that later grows a marshaler still compiles, so the paired absence proof
// stays in each owner's tests.
var (
	_ OffWireEnum = Comparison(0)
	_ OffWireEnum = TestIsolationHazardUnknown
	_ OffWireEnum = TestIsolationScopeUnknown
)
