package objectstore

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	// AmazonS3PutObjectMaximumBytes is the single PutObject extent ceiling.
	AmazonS3PutObjectMaximumBytes uint64 = 5 * 1024 * 1024 * 1024
	// AmazonS3ObjectMaximumBytes is the complete S3 object extent ceiling.
	AmazonS3ObjectMaximumBytes uint64 = 5 * 1024 * 1024 * 1024 * 1024
	// AmazonS3VersionIDMaximumBytes is the published UTF-8 version-ID ceiling.
	AmazonS3VersionIDMaximumBytes = 1024
	// GoogleCloudStorageObjectMaximumBytes is the Cloud Storage object ceiling.
	GoogleCloudStorageObjectMaximumBytes uint64 = 5 * 1024 * 1024 * 1024 * 1024
	// CloudflareImagesUploadMaximumBytes is the hosted-image upload ceiling.
	CloudflareImagesUploadMaximumBytes uint64 = 10_000_000
)

// Provider is the closed vendor destination domain.
type Provider uint8

const (
	// ProviderUnknown is the invalid zero provider.
	ProviderUnknown Provider = iota
	// ProviderAmazonS3 identifies Amazon S3.
	ProviderAmazonS3
	// ProviderGoogleCloudStorage identifies Google Cloud Storage.
	ProviderGoogleCloudStorage
	// ProviderCloudflareImages identifies Cloudflare Images.
	ProviderCloudflareImages
	providerLimit
)

// VendorAPI identifies the vendor-published operation family.
type VendorAPI uint8

const (
	// VendorAPIUnknown is the invalid zero API.
	VendorAPIUnknown VendorAPI = iota
	// VendorAPIAmazonS3Object identifies S3 PutObject and GetObject.
	VendorAPIAmazonS3Object
	// VendorAPIGoogleCloudStorageXML identifies Cloud Storage XML PUT and GET.
	VendorAPIGoogleCloudStorageXML
	// VendorAPICloudflareImagesDirect identifies Images direct creator upload.
	VendorAPICloudflareImagesDirect
	vendorAPILimit
)

// DirectionCapability is the closed operation support declared by a vendor.
type DirectionCapability uint8

const (
	// DirectionCapabilityUnknown is the invalid zero capability.
	DirectionCapabilityUnknown DirectionCapability = iota
	// DirectionCapabilityUploadOnly admits upload but not download.
	DirectionCapabilityUploadOnly
	// DirectionCapabilityUploadDownload admits both whole-object directions.
	DirectionCapabilityUploadDownload
	directionCapabilityLimit
)

// UploadEncoding is the vendor-required request-body encoding.
type UploadEncoding uint8

const (
	// UploadEncodingUnknown is the invalid zero encoding.
	UploadEncodingUnknown UploadEncoding = iota
	// UploadEncodingRawObject sends the object bytes as the request body.
	UploadEncodingRawObject
	// UploadEncodingMultipartFile sends one multipart field named file.
	UploadEncodingMultipartFile
	uploadEncodingLimit
)

// ProviderIntegrity is the checksum behavior promised by the vendor API.
type ProviderIntegrity uint8

const (
	// ProviderIntegrityUnknown is the invalid zero integrity behavior.
	ProviderIntegrityUnknown ProviderIntegrity = iota
	// ProviderIntegrityCRC32C requires provider-side CRC32C verification.
	ProviderIntegrityCRC32C
	// ProviderIntegrityLocalOnly records that the provider exposes no upload
	// checksum contract and Objectstore can prove only its local stream.
	ProviderIntegrityLocalOnly
	providerIntegrityLimit
)

// WritePreference is the package-selected vendor write behavior.
type WritePreference uint8

const (
	// WritePreferenceUnknown is the invalid zero preference.
	WritePreferenceUnknown WritePreference = iota
	// WritePreferenceCreateOnly uses a vendor create-only precondition.
	WritePreferenceCreateOnly
	// WritePreferenceOneTimeCapability uses a vendor-issued one-time target.
	WritePreferenceOneTimeCapability
	writePreferenceLimit
)

// VendorSpec is the compiler-owned projection of one vendor-published API and
// the narrow behavior Objectstore selects from it.
type VendorSpec struct {
	Provider          Provider
	API               VendorAPI
	Directions        DirectionCapability
	UploadMethod      exchange.Method
	DownloadMethod    exchange.Method
	UploadEncoding    UploadEncoding
	ProviderIntegrity ProviderIntegrity
	WritePreference   WritePreference
	UploadMaximum     core.ByteLength
	DownloadMaximum   core.ByteLength
}

// Spec returns the complete immutable contract for provider.
func Spec(provider Provider) (VendorSpec, error) {
	if err := provider.Validate(); err != nil {
		return VendorSpec{}, err
	}
	specs, err := vendorSpecs()
	if err != nil {
		return VendorSpec{}, err
	}
	spec := specs[provider]
	if err := spec.Validate(); err != nil {
		return VendorSpec{}, err
	}
	return spec, nil
}

// Validate closes the vendor API, direction, encoding, integrity, write, and
// extent lattice.
func (s VendorSpec) Validate() error {
	if err := validateVendorSpecEnums(s); err != nil {
		return core.ErrObjectStoreContract
	}
	if err := errors.Join(s.UploadMethod.Validate(), s.UploadMaximum.Validate()); err != nil ||
		s.UploadMaximum.Uint64() == 0 {
		return core.ErrObjectStoreContract
	}
	if s.Directions == DirectionCapabilityUploadOnly {
		return validateUploadOnlySpec(s)
	}
	if err := errors.Join(s.DownloadMethod.Validate(), s.DownloadMaximum.Validate()); err != nil ||
		s.DownloadMaximum.Uint64() == 0 {
		return core.ErrObjectStoreContract
	}
	return validateVendorSpecIdentity(s)
}

func validateVendorSpecEnums(spec VendorSpec) error {
	for _, err := range [...]error{
		spec.Provider.Validate(),
		spec.API.Validate(),
		spec.Directions.Validate(),
		spec.UploadEncoding.Validate(),
		spec.ProviderIntegrity.Validate(),
		spec.WritePreference.Validate(),
	} {
		if err != nil {
			return core.ErrObjectStoreContract
		}
	}
	return nil
}

func validateUploadOnlySpec(spec VendorSpec) error {
	if spec.DownloadMethod != exchange.MethodUnknown ||
		spec.DownloadMaximum.Uint64() != 0 {
		return core.ErrObjectStoreContract
	}
	return validateVendorSpecIdentity(spec)
}

func validateVendorSpecIdentity(spec VendorSpec) error {
	specs, err := vendorSpecs()
	if err != nil {
		return core.ErrObjectStoreContract
	}
	expected := specs[spec.Provider]
	if spec != expected {
		return core.ErrObjectStoreContract
	}
	return nil
}

// Validate rejects values outside the closed provider domain.
func (p Provider) Validate() error {
	if p <= ProviderUnknown || p >= providerLimit {
		return core.ErrObjectStoreContract
	}
	return nil
}

// IsValid reports whether p belongs to the closed provider domain.
func (p Provider) IsValid() bool { return p.Validate() == nil }

// String returns a diagnostic provider name.
func (p Provider) String() string {
	if !p.IsValid() {
		return ""
	}
	return providerDiagnostics()[p]
}

func providerDiagnostics() [providerLimit]string {
	return [providerLimit]string{
		ProviderUnknown:            "",
		ProviderAmazonS3:           "amazon_s3",
		ProviderGoogleCloudStorage: "google_cloud_storage",
		ProviderCloudflareImages:   "cloudflare_images",
	}
}

// OffWireEnum declares Provider as an execution enum rather than wire syntax.
func (Provider) OffWireEnum() {}

// Validate rejects values outside the closed vendor API domain.
func (a VendorAPI) Validate() error {
	if a <= VendorAPIUnknown || a >= vendorAPILimit {
		return core.ErrObjectStoreContract
	}
	return nil
}

// IsValid reports whether a belongs to the closed API domain.
func (a VendorAPI) IsValid() bool { return a.Validate() == nil }

// String returns a diagnostic API name.
func (a VendorAPI) String() string {
	if !a.IsValid() {
		return ""
	}
	return vendorAPIDiagnostics()[a]
}

func vendorAPIDiagnostics() [vendorAPILimit]string {
	return [vendorAPILimit]string{
		VendorAPIUnknown:                "",
		VendorAPIAmazonS3Object:         "amazon_s3_object",
		VendorAPIGoogleCloudStorageXML:  "google_cloud_storage_xml",
		VendorAPICloudflareImagesDirect: "cloudflare_images_direct",
	}
}

// OffWireEnum declares VendorAPI as an execution enum.
func (VendorAPI) OffWireEnum() {}

// Validate rejects values outside the closed direction-capability domain.
func (c DirectionCapability) Validate() error {
	if c <= DirectionCapabilityUnknown || c >= directionCapabilityLimit {
		return core.ErrObjectStoreContract
	}
	return nil
}

// IsValid reports whether c belongs to the closed capability domain.
func (c DirectionCapability) IsValid() bool { return c.Validate() == nil }

// String returns a diagnostic capability name.
func (c DirectionCapability) String() string {
	if !c.IsValid() {
		return ""
	}
	return directionCapabilityDiagnostics()[c]
}

func directionCapabilityDiagnostics() [directionCapabilityLimit]string {
	return [directionCapabilityLimit]string{
		DirectionCapabilityUnknown:        "",
		DirectionCapabilityUploadOnly:     "upload_only",
		DirectionCapabilityUploadDownload: "upload_download",
	}
}

// OffWireEnum declares DirectionCapability as an execution enum.
func (DirectionCapability) OffWireEnum() {}

// Validate rejects values outside the closed upload-encoding domain.
func (e UploadEncoding) Validate() error {
	if e <= UploadEncodingUnknown || e >= uploadEncodingLimit {
		return core.ErrObjectStoreContract
	}
	return nil
}

// IsValid reports whether e belongs to the closed encoding domain.
func (e UploadEncoding) IsValid() bool { return e.Validate() == nil }

// String returns a diagnostic encoding name.
func (e UploadEncoding) String() string {
	if !e.IsValid() {
		return ""
	}
	return uploadEncodingDiagnostics()[e]
}

func uploadEncodingDiagnostics() [uploadEncodingLimit]string {
	return [uploadEncodingLimit]string{
		UploadEncodingUnknown:       "",
		UploadEncodingRawObject:     "raw_object",
		UploadEncodingMultipartFile: "multipart_file",
	}
}

// OffWireEnum declares UploadEncoding as an execution enum.
func (UploadEncoding) OffWireEnum() {}

// Validate rejects values outside the closed provider-integrity domain.
func (i ProviderIntegrity) Validate() error {
	if i <= ProviderIntegrityUnknown || i >= providerIntegrityLimit {
		return core.ErrObjectStoreContract
	}
	return nil
}

// IsValid reports whether i belongs to the closed integrity domain.
func (i ProviderIntegrity) IsValid() bool { return i.Validate() == nil }

// String returns a diagnostic integrity name.
func (i ProviderIntegrity) String() string {
	if !i.IsValid() {
		return ""
	}
	return providerIntegrityDiagnostics()[i]
}

func providerIntegrityDiagnostics() [providerIntegrityLimit]string {
	return [providerIntegrityLimit]string{
		ProviderIntegrityUnknown:   "",
		ProviderIntegrityCRC32C:    "crc32c",
		ProviderIntegrityLocalOnly: "local_only",
	}
}

// OffWireEnum declares ProviderIntegrity as an execution enum.
func (ProviderIntegrity) OffWireEnum() {}

// Validate rejects values outside the closed write-preference domain.
func (p WritePreference) Validate() error {
	if p <= WritePreferenceUnknown || p >= writePreferenceLimit {
		return core.ErrObjectStoreContract
	}
	return nil
}

// IsValid reports whether p belongs to the closed preference domain.
func (p WritePreference) IsValid() bool { return p.Validate() == nil }

// String returns a diagnostic write-preference name.
func (p WritePreference) String() string {
	if !p.IsValid() {
		return ""
	}
	return writePreferenceDiagnostics()[p]
}

func writePreferenceDiagnostics() [writePreferenceLimit]string {
	return [writePreferenceLimit]string{
		WritePreferenceUnknown:           "",
		WritePreferenceCreateOnly:        "create_only",
		WritePreferenceOneTimeCapability: "one_time_capability",
	}
}

// OffWireEnum declares WritePreference as an execution enum.
func (WritePreference) OffWireEnum() {}

func vendorSpecs() ([providerLimit]VendorSpec, error) {
	amazonUpload, err := core.NewByteLength(AmazonS3PutObjectMaximumBytes)
	if err != nil {
		return [providerLimit]VendorSpec{}, err
	}
	amazonDownload, err := core.NewByteLength(AmazonS3ObjectMaximumBytes)
	if err != nil {
		return [providerLimit]VendorSpec{}, err
	}
	googleMaximum, err := core.NewByteLength(GoogleCloudStorageObjectMaximumBytes)
	if err != nil {
		return [providerLimit]VendorSpec{}, err
	}
	cloudflareUpload, err := core.NewByteLength(CloudflareImagesUploadMaximumBytes)
	if err != nil {
		return [providerLimit]VendorSpec{}, err
	}
	return [...]VendorSpec{
		ProviderUnknown: {},
		ProviderAmazonS3: {
			Provider: ProviderAmazonS3, API: VendorAPIAmazonS3Object,
			Directions:   DirectionCapabilityUploadDownload,
			UploadMethod: exchange.MethodPut, DownloadMethod: exchange.MethodGet,
			UploadEncoding:    UploadEncodingRawObject,
			ProviderIntegrity: ProviderIntegrityCRC32C,
			WritePreference:   WritePreferenceCreateOnly,
			UploadMaximum:     amazonUpload,
			DownloadMaximum:   amazonDownload,
		},
		ProviderGoogleCloudStorage: {
			Provider:     ProviderGoogleCloudStorage,
			API:          VendorAPIGoogleCloudStorageXML,
			Directions:   DirectionCapabilityUploadDownload,
			UploadMethod: exchange.MethodPut, DownloadMethod: exchange.MethodGet,
			UploadEncoding:    UploadEncodingRawObject,
			ProviderIntegrity: ProviderIntegrityCRC32C,
			WritePreference:   WritePreferenceCreateOnly,
			UploadMaximum:     googleMaximum,
			DownloadMaximum:   googleMaximum,
		},
		ProviderCloudflareImages: {
			Provider:          ProviderCloudflareImages,
			API:               VendorAPICloudflareImagesDirect,
			Directions:        DirectionCapabilityUploadOnly,
			UploadMethod:      exchange.MethodPost,
			UploadEncoding:    UploadEncodingMultipartFile,
			ProviderIntegrity: ProviderIntegrityLocalOnly,
			WritePreference:   WritePreferenceOneTimeCapability,
			UploadMaximum:     cloudflareUpload,
		},
	}, nil
}

var (
	_ core.Validatable = ProviderUnknown
	_ core.OffWireEnum = ProviderUnknown
	_ core.Validatable = VendorAPIUnknown
	_ core.OffWireEnum = VendorAPIUnknown
	_ core.Validatable = DirectionCapabilityUnknown
	_ core.OffWireEnum = DirectionCapabilityUnknown
	_ core.Validatable = UploadEncodingUnknown
	_ core.OffWireEnum = UploadEncodingUnknown
	_ core.Validatable = ProviderIntegrityUnknown
	_ core.OffWireEnum = ProviderIntegrityUnknown
	_ core.Validatable = WritePreferenceUnknown
	_ core.OffWireEnum = WritePreferenceUnknown
	_ core.Validatable = VendorSpec{}
)
