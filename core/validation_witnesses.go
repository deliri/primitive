package core

var (
	_ Validatable = ArchitectureCatalog{}
	_ Validatable = PackageIdentity(0)
	_ Validatable = PackageKind(0)
	_ Validatable = PackageContract{}
	_ Validatable = DirectImportContract{}
	_ Validatable = StrictJSONLimits{}
	_ Validatable = ErrorIdentity(0)
	_ Validatable = Comparison(0)
	_ Validatable = ByteCount{}
	_ Validatable = SHA256Digest{}
	_ Validatable = CRC32C{}
	_ Validatable = Ed25519PublicKey{}
	_ Validatable = SecretMaterial{}
	_ Validatable = PathComponent{}
	_ Validatable = AbsolutePath{}
	_ Validatable = HTTPMethod(0)
	_ Validatable = HTTPStatusCode{}
	_ Validatable = HTTPHeaderName{}
	_ Validatable = HTTPMediaType{}
	_ Validatable = HTTPContentCoding{}
	_ Validatable = Platform{}
	_ Validatable = OperatingSystem(0)
	_ Validatable = CPUArchitecture(0)
)

var (
	_ ValidatedJSONMarshaler = PackageIdentity(0)
	_ ValidatedJSONMarshaler = PackageKind(0)
	_ ValidatedJSONMarshaler = ErrorIdentity(0)
	_ ValidatedJSONMarshaler = ByteCount{}
	_ ValidatedJSONMarshaler = SHA256Digest{}
	_ ValidatedJSONMarshaler = CRC32C{}
	_ ValidatedJSONMarshaler = Ed25519PublicKey{}
	_ ValidatedJSONMarshaler = SecretMaterial{}
	_ ValidatedJSONMarshaler = PathComponent{}
	_ ValidatedJSONMarshaler = AbsolutePath{}
	_ ValidatedJSONMarshaler = HTTPMethod(0)
	_ ValidatedJSONMarshaler = HTTPStatusCode{}
	_ ValidatedJSONMarshaler = HTTPHeaderName{}
	_ ValidatedJSONMarshaler = HTTPMediaType{}
	_ ValidatedJSONMarshaler = HTTPContentCoding{}
	_ ValidatedJSONMarshaler = Platform{}
	_ ValidatedJSONMarshaler = OperatingSystem(0)
	_ ValidatedJSONMarshaler = CPUArchitecture(0)
)

// Off-wire enums are bound to the shared interface here so that the marker is a
// compiler-checked contract rather than a repeated method name. A declaring
// enum that later grows a marshaler still compiles, so the paired absence proof
// stays in each owner's tests.
var (
	_ OffWireEnum = Comparison(0)
)
