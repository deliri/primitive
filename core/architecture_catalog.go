package core

import (
	"errors"
	"iter"
	"strings"
)

const (
	// PrimitiveModulePath is the canonical Go module path.
	PrimitiveModulePath = "github.com/deliri/primitive/v2026"
	// PrimitivePackagePathPrefix prefixes every Primitive package import path.
	PrimitivePackagePathPrefix = PrimitiveModulePath + "/"
	// PrimitivePackageCount is the number of packages in the complete catalog.
	PrimitivePackageCount = 41
	// PrimitiveDirectImportCount is the number of admitted direct import edges.
	PrimitiveDirectImportCount = 150
	// PrimitiveDirectTestImportCount is the number of admitted test-only edges.
	PrimitiveDirectTestImportCount = 31
	// PrimitiveMaximumDirectImports caps direct sibling imports per package.
	PrimitiveMaximumDirectImports = 10
)

// PackageIdentity is a closed identity for a package in Primitive's catalog.
type PackageIdentity uint8

const (
	// PackageUnknown is the invalid zero package identity.
	PackageUnknown PackageIdentity = iota
	// PackageCore identifies the shared core package.
	PackageCore
	// PackageAttest identifies the attestation package.
	PackageAttest
	// PackageContextState identifies the context-state package.
	PackageContextState
	// PackageCurrency identifies the currency package.
	PackageCurrency
	// PackageKeygen identifies the key-generation package.
	PackageKeygen
	// PackageTestSerial identifies the serial test-support package.
	PackageTestSerial
	// PackageFileLock identifies the advisory file-lock package.
	PackageFileLock
	// PackageFilestore identifies the file-store package.
	PackageFilestore
	// PackageHostFacts identifies the host-facts package.
	PackageHostFacts
	// PackageTemporal identifies the temporal package.
	PackageTemporal
	// PackageExchange identifies the HTTP exchange package.
	PackageExchange
	// PackageFuzzFinder identifies the fuzz-artifact finder package.
	PackageFuzzFinder
	// PackageLease identifies the lease package.
	PackageLease
	// PackageGate identifies the new-work authorization package.
	PackageGate
	// PackageReceipt identifies authenticated accepted-evidence facts and watermarks.
	PackageReceipt
	// PackageControlWire identifies the shared control-wire scalar package.
	PackageControlWire
	// PackageControlPlane identifies the signed control-plane document package.
	PackageControlPlane
	// PackageSubmission identifies evidence-submission authorization documents.
	PackageSubmission
	// PackageSubmissionAuth identifies installation-credential binding for one
	// evidence-submission request.
	PackageSubmissionAuth
	// PackageControlPlaneTest identifies real control-plane test fixtures.
	PackageControlPlaneTest
	// PackageProcess identifies the process package.
	PackageProcess
	// PackageRelease identifies the release package.
	PackageRelease
	// PackageShutdown identifies the shutdown package.
	PackageShutdown
	// PackageObjectStore identifies the object-store package.
	PackageObjectStore
	// PackageTimeProof identifies the time-proof package.
	PackageTimeProof
	// PackageCloudIdentity identifies the cloud-identity package.
	PackageCloudIdentity
	// PackageDeploy identifies exact release publication to GCS.
	PackageDeploy
	// PackageUpgrade identifies the upgrade package.
	PackageUpgrade
	// PackageGCSObjects identifies the authenticated Cloud Storage package.
	PackageGCSObjects
	// PackageID identifies the time-ordered identifier package.
	PackageID
	// PackageChit identifies immutable custody tickets and bounded catalogs.
	PackageChit
	// PackageChitAuth identifies installation binding for chit catalog queries.
	PackageChitAuth
	// PackageRetrieval identifies authenticated exact-object retrieval grants.
	PackageRetrieval
	// PackageRetrievalAuth identifies installation binding for retrieval requests.
	PackageRetrievalAuth
	// PackagePayment identifies signed payment receipts and bounded catalogs.
	PackagePayment
	// PackagePaymentAuth identifies installation binding for payment catalog queries.
	PackagePaymentAuth
	// PackageDistribution identifies signed software publication, update, and
	// upgrade agreements shared by release authorities and installed tools.
	PackageDistribution
	// PackageDistributionAuth identifies installation binding for update and upgrade requests.
	PackageDistributionAuth
	// PackageWiring identifies bounded runtime component-graph proof.
	PackageWiring
	// PackageLineIO identifies bounded line scanning over one reader.
	PackageLineIO
	// PackageManual identifies bounded human and machine manual projection.
	PackageManual
	packageIdentityLimit
)

// PackageKind classifies a catalog package as production or test support.
type PackageKind uint8

const (
	// PackageKindUnknown is the invalid zero package kind.
	PackageKindUnknown PackageKind = iota
	// PackageKindProduction identifies a runtime library package.
	PackageKindProduction
	// PackageKindTestSupport identifies a package used only by tests.
	PackageKindTestSupport
	packageKindLimit
)

const (
	packageKindProductionText  = "production"
	packageKindTestSupportText = "test_support"
)

// PackageContract binds a package identity to its admitted kind.
type PackageContract struct {
	// Identity is the catalog package.
	Identity PackageIdentity
	// Kind is the package's production or test-support classification.
	Kind PackageKind
}

// DirectImportContract admits one direct importer-to-imported package edge.
type DirectImportContract struct {
	// Importer is the package that owns the import declaration.
	Importer PackageIdentity
	// Imported is the directly imported Primitive package.
	Imported PackageIdentity
}

// DirectTestImportContract admits one test-only importer-to-imported edge.
//
// A test-only edge exists when a package's tests require either the real
// substrate that produces an ingress value or the typed Testserial declaration
// for a process-wide isolation fact. It grants no production dependency:
// production sources that import the edge remain an undeclared production
// edge, and a declared test edge that no test file uses is a ceremonial import
// and equally rejected.
type DirectTestImportContract struct {
	// Importer is the package whose test sources own the import declaration.
	Importer PackageIdentity
	// Imported is the directly imported Primitive package.
	Imported PackageIdentity
}

// ArchitectureCatalog is the complete, validated Primitive package graph.
type ArchitectureCatalog struct {
	packages    [PrimitivePackageCount]PackageContract
	imports     [PrimitiveDirectImportCount]DirectImportContract
	testImports [PrimitiveDirectTestImportCount]DirectTestImportContract
}

// PrimitiveArchitecture returns the complete compiler-owned package catalog.
func PrimitiveArchitecture() ArchitectureCatalog {
	return ArchitectureCatalog{
		packages: [PrimitivePackageCount]PackageContract{
			{Identity: PackageCore, Kind: PackageKindProduction},
			{Identity: PackageAttest, Kind: PackageKindProduction},
			{Identity: PackageContextState, Kind: PackageKindProduction},
			{Identity: PackageCurrency, Kind: PackageKindProduction},
			{Identity: PackageKeygen, Kind: PackageKindProduction},
			{Identity: PackageTestSerial, Kind: PackageKindTestSupport},
			{Identity: PackageFileLock, Kind: PackageKindProduction},
			{Identity: PackageFilestore, Kind: PackageKindProduction},
			{Identity: PackageHostFacts, Kind: PackageKindProduction},
			{Identity: PackageTemporal, Kind: PackageKindProduction},
			{Identity: PackageExchange, Kind: PackageKindProduction},
			{Identity: PackageFuzzFinder, Kind: PackageKindProduction},
			{Identity: PackageLease, Kind: PackageKindProduction},
			{Identity: PackageGate, Kind: PackageKindProduction},
			{Identity: PackageReceipt, Kind: PackageKindProduction},
			{Identity: PackageControlWire, Kind: PackageKindProduction},
			{Identity: PackageControlPlane, Kind: PackageKindProduction},
			{Identity: PackageSubmission, Kind: PackageKindProduction},
			{Identity: PackageSubmissionAuth, Kind: PackageKindProduction},
			{Identity: PackageControlPlaneTest, Kind: PackageKindTestSupport},
			{Identity: PackageProcess, Kind: PackageKindProduction},
			{Identity: PackageRelease, Kind: PackageKindProduction},
			{Identity: PackageShutdown, Kind: PackageKindProduction},
			{Identity: PackageObjectStore, Kind: PackageKindProduction},
			{Identity: PackageTimeProof, Kind: PackageKindProduction},
			{Identity: PackageCloudIdentity, Kind: PackageKindProduction},
			{Identity: PackageDeploy, Kind: PackageKindProduction},
			{Identity: PackageUpgrade, Kind: PackageKindProduction},
			{Identity: PackageGCSObjects, Kind: PackageKindProduction},
			{Identity: PackageID, Kind: PackageKindProduction},
			{Identity: PackageChit, Kind: PackageKindProduction},
			{Identity: PackageChitAuth, Kind: PackageKindProduction},
			{Identity: PackageRetrieval, Kind: PackageKindProduction},
			{Identity: PackageRetrievalAuth, Kind: PackageKindProduction},
			{Identity: PackagePayment, Kind: PackageKindProduction},
			{Identity: PackagePaymentAuth, Kind: PackageKindProduction},
			{Identity: PackageDistribution, Kind: PackageKindProduction},
			{Identity: PackageDistributionAuth, Kind: PackageKindProduction},
			{Identity: PackageWiring, Kind: PackageKindProduction},
			{Identity: PackageLineIO, Kind: PackageKindProduction},
			{Identity: PackageManual, Kind: PackageKindProduction},
		},
		imports: [PrimitiveDirectImportCount]DirectImportContract{
			{Importer: PackageAttest, Imported: PackageCore},
			{Importer: PackageContextState, Imported: PackageCore},
			{Importer: PackageCurrency, Imported: PackageCore},
			{Importer: PackageKeygen, Imported: PackageCore},
			{Importer: PackageTestSerial, Imported: PackageCore},

			{Importer: PackageFileLock, Imported: PackageCore},
			{Importer: PackageFileLock, Imported: PackageContextState},
			{Importer: PackageFilestore, Imported: PackageCore},
			{Importer: PackageFilestore, Imported: PackageContextState},
			{Importer: PackageFilestore, Imported: PackageTemporal},
			{Importer: PackageHostFacts, Imported: PackageCore},
			{Importer: PackageHostFacts, Imported: PackageContextState},
			{Importer: PackageTemporal, Imported: PackageCore},
			{Importer: PackageTemporal, Imported: PackageContextState},

			{Importer: PackageExchange, Imported: PackageCore},
			{Importer: PackageExchange, Imported: PackageContextState},
			{Importer: PackageExchange, Imported: PackageKeygen},
			{Importer: PackageExchange, Imported: PackageTemporal},
			{Importer: PackageFuzzFinder, Imported: PackageCore},
			{Importer: PackageFuzzFinder, Imported: PackageFilestore},
			{Importer: PackageLease, Imported: PackageCore},
			{Importer: PackageLease, Imported: PackageTemporal},
			{Importer: PackageLease, Imported: PackageAttest},
			{Importer: PackageGate, Imported: PackageCore},
			{Importer: PackageGate, Imported: PackageLease},
			{Importer: PackageReceipt, Imported: PackageCore},
			{Importer: PackageReceipt, Imported: PackageAttest},
			{Importer: PackageReceipt, Imported: PackageTemporal},
			{Importer: PackageControlWire, Imported: PackageCore},
			{Importer: PackageControlWire, Imported: PackageKeygen},
			{Importer: PackageControlWire, Imported: PackageExchange},
			{Importer: PackageControlWire, Imported: PackageTemporal},
			{Importer: PackageControlPlane, Imported: PackageCore},
			{Importer: PackageControlPlane, Imported: PackageControlWire},
			{Importer: PackageControlPlane, Imported: PackageAttest},
			{Importer: PackageControlPlane, Imported: PackageLease},
			{Importer: PackageControlPlane, Imported: PackageTemporal},
			{Importer: PackageControlPlane, Imported: PackageReceipt},
			{Importer: PackageSubmission, Imported: PackageCore},
			{Importer: PackageSubmission, Imported: PackageAttest},
			{Importer: PackageSubmission, Imported: PackageChit},
			{Importer: PackageSubmission, Imported: PackageControlWire},
			{Importer: PackageSubmission, Imported: PackageID},
			{Importer: PackageSubmission, Imported: PackageObjectStore},
			{Importer: PackageSubmission, Imported: PackageTemporal},
			{Importer: PackageSubmission, Imported: PackageReceipt},
			{Importer: PackageSubmissionAuth, Imported: PackageCore},
			{Importer: PackageSubmissionAuth, Imported: PackageAttest},
			{Importer: PackageSubmissionAuth, Imported: PackageControlPlane},
			{Importer: PackageSubmissionAuth, Imported: PackageControlWire},
			{Importer: PackageSubmissionAuth, Imported: PackageSubmission},
			{Importer: PackageSubmissionAuth, Imported: PackageChit},
			{Importer: PackageSubmissionAuth, Imported: PackageObjectStore},
			{Importer: PackageSubmissionAuth, Imported: PackageReceipt},
			{Importer: PackageControlPlaneTest, Imported: PackageCore},
			{Importer: PackageControlPlaneTest, Imported: PackageControlPlane},
			{Importer: PackageControlPlaneTest, Imported: PackageControlWire},
			{Importer: PackageControlPlaneTest, Imported: PackageLease},
			{Importer: PackageControlPlaneTest, Imported: PackageReceipt},
			{Importer: PackageControlPlaneTest, Imported: PackageTemporal},
			{Importer: PackageProcess, Imported: PackageCore},
			{Importer: PackageProcess, Imported: PackageContextState},
			{Importer: PackageProcess, Imported: PackageTemporal},
			{Importer: PackageRelease, Imported: PackageCore},
			{Importer: PackageRelease, Imported: PackageTemporal},
			{Importer: PackageRelease, Imported: PackageAttest},
			{Importer: PackageRelease, Imported: PackageFilestore},
			{Importer: PackageRelease, Imported: PackageControlWire},
			{Importer: PackageRelease, Imported: PackageKeygen},
			{Importer: PackageRelease, Imported: PackageProcess},
			{Importer: PackageShutdown, Imported: PackageCore},
			{Importer: PackageShutdown, Imported: PackageContextState},
			{Importer: PackageShutdown, Imported: PackageTemporal},

			{Importer: PackageObjectStore, Imported: PackageCore},
			{Importer: PackageObjectStore, Imported: PackageContextState},
			{Importer: PackageObjectStore, Imported: PackageTemporal},
			{Importer: PackageObjectStore, Imported: PackageExchange},
			{Importer: PackageTimeProof, Imported: PackageCore},
			{Importer: PackageTimeProof, Imported: PackageTemporal},
			{Importer: PackageTimeProof, Imported: PackageKeygen},
			{Importer: PackageCloudIdentity, Imported: PackageCore},
			{Importer: PackageCloudIdentity, Imported: PackageTemporal},
			{Importer: PackageCloudIdentity, Imported: PackageExchange},
			{Importer: PackageDeploy, Imported: PackageCore},
			{Importer: PackageDeploy, Imported: PackageObjectStore},
			{Importer: PackageDeploy, Imported: PackageRelease},

			{Importer: PackageUpgrade, Imported: PackageCore},
			{Importer: PackageUpgrade, Imported: PackageFilestore},
			{Importer: PackageUpgrade, Imported: PackageHostFacts},
			{Importer: PackageUpgrade, Imported: PackageObjectStore},
			{Importer: PackageUpgrade, Imported: PackageRelease},
			{Importer: PackageUpgrade, Imported: PackageTemporal},

			{Importer: PackageGCSObjects, Imported: PackageCore},
			{Importer: PackageGCSObjects, Imported: PackageContextState},
			{Importer: PackageGCSObjects, Imported: PackageTemporal},
			{Importer: PackageGCSObjects, Imported: PackageObjectStore},

			{Importer: PackageID, Imported: PackageCore},
			{Importer: PackageID, Imported: PackageTemporal},

			{Importer: PackageChit, Imported: PackageAttest},
			{Importer: PackageChit, Imported: PackageCore},
			{Importer: PackageChit, Imported: PackageControlWire},
			{Importer: PackageChit, Imported: PackageID},
			{Importer: PackageChit, Imported: PackageReceipt},
			{Importer: PackageChit, Imported: PackageTemporal},
			{Importer: PackageChitAuth, Imported: PackageAttest},
			{Importer: PackageChitAuth, Imported: PackageChit},
			{Importer: PackageChitAuth, Imported: PackageControlPlane},
			{Importer: PackageChitAuth, Imported: PackageControlWire},
			{Importer: PackageChitAuth, Imported: PackageCore},

			{Importer: PackageRetrieval, Imported: PackageAttest},
			{Importer: PackageRetrieval, Imported: PackageChit},
			{Importer: PackageRetrieval, Imported: PackageControlWire},
			{Importer: PackageRetrieval, Imported: PackageCore},
			{Importer: PackageRetrieval, Imported: PackageFilestore},
			{Importer: PackageRetrieval, Imported: PackageObjectStore},
			{Importer: PackageRetrieval, Imported: PackageTemporal},

			{Importer: PackageRetrievalAuth, Imported: PackageAttest},
			{Importer: PackageRetrievalAuth, Imported: PackageControlPlane},
			{Importer: PackageRetrievalAuth, Imported: PackageControlWire},
			{Importer: PackageRetrievalAuth, Imported: PackageCore},
			{Importer: PackageRetrievalAuth, Imported: PackageRetrieval},

			{Importer: PackagePayment, Imported: PackageAttest},
			{Importer: PackagePayment, Imported: PackageCore},
			{Importer: PackagePayment, Imported: PackageControlWire},
			{Importer: PackagePayment, Imported: PackageCurrency},
			{Importer: PackagePayment, Imported: PackageID},
			{Importer: PackagePayment, Imported: PackageReceipt},
			{Importer: PackagePayment, Imported: PackageTemporal},
			{Importer: PackagePaymentAuth, Imported: PackageAttest},
			{Importer: PackagePaymentAuth, Imported: PackageControlPlane},
			{Importer: PackagePaymentAuth, Imported: PackageControlWire},
			{Importer: PackagePaymentAuth, Imported: PackageCore},
			{Importer: PackagePaymentAuth, Imported: PackagePayment},

			{Importer: PackageDistribution, Imported: PackageAttest},
			{Importer: PackageDistribution, Imported: PackageControlWire},
			{Importer: PackageDistribution, Imported: PackageCore},
			{Importer: PackageDistribution, Imported: PackageDeploy},
			{Importer: PackageDistribution, Imported: PackageObjectStore},
			{Importer: PackageDistribution, Imported: PackageRelease},
			{Importer: PackageDistribution, Imported: PackageTemporal},
			{Importer: PackageDistribution, Imported: PackageUpgrade},
			{Importer: PackageDistributionAuth, Imported: PackageAttest},
			{Importer: PackageDistributionAuth, Imported: PackageControlPlane},
			{Importer: PackageDistributionAuth, Imported: PackageControlWire},
			{Importer: PackageDistributionAuth, Imported: PackageCore},
			{Importer: PackageDistributionAuth, Imported: PackageDistribution},
			{Importer: PackageDistributionAuth, Imported: PackageRelease},
			{Importer: PackageWiring, Imported: PackageCore},
			{Importer: PackageLineIO, Imported: PackageCore},
			{Importer: PackageManual, Imported: PackageCore},
		},
		testImports: [PrimitiveDirectTestImportCount]DirectTestImportContract{
			{Importer: PackageGate, Imported: PackageAttest},
			{Importer: PackageGate, Imported: PackageTemporal},
			{Importer: PackageFilestore, Imported: PackageFileLock},
			{Importer: PackageProcess, Imported: PackageTestSerial},
			{Importer: PackageDeploy, Imported: PackageAttest},
			{Importer: PackageDeploy, Imported: PackageExchange},
			{Importer: PackageDeploy, Imported: PackageTemporal},
			{Importer: PackageUpgrade, Imported: PackageExchange},
			{Importer: PackageGCSObjects, Imported: PackageExchange},
			{Importer: PackageGCSObjects, Imported: PackageTestSerial},
			{Importer: PackageSubmissionAuth, Imported: PackageControlPlaneTest},
			{Importer: PackageSubmissionAuth, Imported: PackageExchange},
			{Importer: PackageSubmission, Imported: PackageExchange},
			{Importer: PackageControlWire, Imported: PackageControlPlane},
			{Importer: PackageControlWire, Imported: PackageControlPlaneTest},
			{Importer: PackageRetrievalAuth, Imported: PackageControlPlaneTest},
			{Importer: PackageRetrieval, Imported: PackageExchange},
			{Importer: PackageRetrieval, Imported: PackageReceipt},
			{Importer: PackageChitAuth, Imported: PackageControlPlaneTest},
			{Importer: PackageChitAuth, Imported: PackageReceipt},
			{Importer: PackagePaymentAuth, Imported: PackageControlPlaneTest},
			{Importer: PackagePaymentAuth, Imported: PackageCurrency},
			{Importer: PackagePaymentAuth, Imported: PackageReceipt},
			{Importer: PackagePaymentAuth, Imported: PackageTemporal},
			{Importer: PackageDistributionAuth, Imported: PackageControlPlaneTest},
			{Importer: PackageDistributionAuth, Imported: PackageDeploy},
			{Importer: PackageDistributionAuth, Imported: PackageExchange},
			{Importer: PackageDistributionAuth, Imported: PackageObjectStore},
			{Importer: PackageDistribution, Imported: PackageExchange},
			{Importer: PackageRelease, Imported: PackageTestSerial},
			{Importer: PackageLineIO, Imported: PackageFilestore},
		},
	}
}

// Validate rejects incomplete, duplicate, cyclic, or over-coupled catalogs.
func (c ArchitectureCatalog) Validate() error {
	if err := c.validatePackages(); err != nil {
		return err
	}
	if err := c.validateDirectImports(); err != nil {
		return err
	}
	if err := c.validateDirectTestImports(); err != nil {
		return err
	}
	if c.hasCycle() {
		return architectureContractError("architecture catalog contains an import cycle")
	}
	return c.validateImportCardinality()
}

// Packages yields every package contract in catalog order.
func (c ArchitectureCatalog) Packages() iter.Seq[PackageContract] {
	return func(yield func(PackageContract) bool) {
		for _, contract := range c.packages {
			if !yield(contract) {
				return
			}
		}
	}
}

// DirectImports yields every admitted direct import edge in catalog order.
func (c ArchitectureCatalog) DirectImports() iter.Seq[DirectImportContract] {
	return func(yield func(DirectImportContract) bool) {
		for _, contract := range c.imports {
			if !yield(contract) {
				return
			}
		}
	}
}

// DirectTestImports yields every admitted test-only edge in catalog order.
func (c ArchitectureCatalog) DirectTestImports() iter.Seq[DirectTestImportContract] {
	return func(yield func(DirectTestImportContract) bool) {
		for _, contract := range c.testImports {
			if !yield(contract) {
				return
			}
		}
	}
}

// ContainsDirectImport reports membership in the admitted production graph.
func (c ArchitectureCatalog) ContainsDirectImport(target DirectImportContract) bool {
	for _, contract := range c.imports {
		if contract == target {
			return true
		}
	}
	return false
}

// ContainsDirectTestImport reports membership in the admitted test-only graph.
func (c ArchitectureCatalog) ContainsDirectTestImport(target DirectTestImportContract) bool {
	for _, contract := range c.testImports {
		if contract == target {
			return true
		}
	}
	return false
}

// Lookup returns the contract for identity.
func (c ArchitectureCatalog) Lookup(identity PackageIdentity) (PackageContract, bool) {
	for _, contract := range c.packages {
		if contract.Identity == identity {
			return contract, true
		}
	}
	return PackageContract{}, false
}

// Validate rejects identities outside the closed package domain.
func (p PackageIdentity) Validate() error {
	if p <= PackageUnknown || p >= packageIdentityLimit ||
		packageIdentityTexts()[p] == "" {
		return architectureContractError("package identity is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether p belongs to the closed package domain.
func (p PackageIdentity) IsValid() bool { return p.Validate() == nil }

// String returns the canonical package name, or an empty string when invalid.
func (p PackageIdentity) String() string {
	return packageIdentityText(p)
}

// Name returns the canonical package name after validation.
func (p PackageIdentity) Name() (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	return packageIdentityText(p), nil
}

// MarshalJSON emits the canonical package name as a JSON string.
func (p PackageIdentity) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(p.String())
}

// UnmarshalJSON accepts only a canonical admitted package name.
func (p *PackageIdentity) UnmarshalJSON(data []byte) error {
	if p == nil {
		return errors.Join(ErrJSONContract, architectureContractError("nil package identity receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := ParsePackageIdentity(value)
	if err != nil {
		return errors.Join(ErrJSONContract, err)
	}
	*p = parsed
	return nil
}

// ImportPath returns the canonical full Go import path.
func (p PackageIdentity) ImportPath() (string, error) {
	name, err := p.Name()
	if err != nil {
		return "", err
	}
	return PrimitivePackagePathPrefix + name, nil
}

// Validate rejects kinds outside the closed package-kind domain.
func (k PackageKind) Validate() error {
	if k <= PackageKindUnknown || k >= packageKindLimit ||
		packageKindTexts()[k] == "" {
		return architectureContractError("package kind is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether k belongs to the closed package-kind domain.
func (k PackageKind) IsValid() bool { return k.Validate() == nil }

// String returns the canonical kind text, or an empty string when invalid.
func (k PackageKind) String() string {
	if k >= packageKindLimit {
		return ""
	}
	return packageKindTexts()[k]
}

func packageKindTexts() [packageKindLimit]string {
	return [...]string{
		"",
		packageKindProductionText,
		packageKindTestSupportText,
	}
}

// MarshalJSON emits the canonical package-kind string.
func (k PackageKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(k.String())
}

// UnmarshalJSON accepts only a canonical package-kind string.
func (k *PackageKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.Join(ErrJSONContract, architectureContractError("nil package kind receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	var parsed PackageKind
	switch value {
	case packageKindProductionText:
		parsed = PackageKindProduction
	case packageKindTestSupportText:
		parsed = PackageKindTestSupport
	default:
		return errors.Join(ErrJSONContract, architectureContractError("package kind text is not admitted"))
	}
	*k = parsed
	return nil
}

// Validate enforces the identity-to-kind classification.
func (c PackageContract) Validate() error {
	if err := c.Identity.Validate(); err != nil {
		return err
	}
	if packagePurposeText(c.Identity) == "" {
		return architectureContractError("package purpose is missing from the compiler catalog")
	}
	if err := c.Kind.Validate(); err != nil {
		return err
	}
	if c.Identity == PackageTestSerial || c.Identity == PackageControlPlaneTest {
		if c.Kind != PackageKindTestSupport {
			return architectureContractError("test-support package must be classified as test support")
		}
		return nil
	}
	if c.Kind != PackageKindProduction {
		return architectureContractError("non-test package must be classified as production")
	}
	return nil
}

func packagePurposeText(identity PackageIdentity) string {
	if identity >= packageIdentityLimit {
		return ""
	}
	return packagePurposeTexts()[identity]
}

func packagePurposeTexts() [packageIdentityLimit]string {
	return [...]string{
		PackageCore:             "Shared nominal values, errors, paths, protocol facts, numeric and encoding contracts",
		PackageAttest:           "Canonical Ed25519 envelopes and proof-carrying verification",
		PackageContextState:     "Nil-safe context ingress and terminal observation",
		PackageCurrency:         "Exact minor-unit values, arithmetic, ordering, and decimal projection",
		PackageKeygen:           "Exact secret and Ed25519 key generation",
		PackageTestSerial:       "Test-only isolation declaration and analyzer contract",
		PackageFileLock:         "One advisory whole-file lock on one already-open file",
		PackageFilestore:        "Rooted OS handles, confinement, inspection, durability, activation, append rotation, rename, and recovery",
		PackageHostFacts:        "Host disk, memory, cgroup, tree, and OOM observations",
		PackageTemporal:         "Time, duration, arithmetic, persistence, waits, and tickers",
		PackageExchange:         "Bounded client and server boundary policy over net/http",
		PackageFuzzFinder:       "Bounded classification and observation of Go-generated fuzz artifacts",
		PackageLease:            "Signed lease timeline, assessment, renewal, and monotonic advance",
		PackageGate:             "Pure CLI-side new-work authorization over one authentic Lease assessment",
		PackageReceipt:          "Authenticated accepted-evidence facts and fixed-size monotonic watermarks",
		PackageControlWire:      "Shared control-wire facts and paired authenticated socket with request-owner body limits",
		PackageControlPlane:     "Signed control-plane request and response documents, their binding to one exact request, product status, and usage watermark",
		PackageSubmission:       "Authenticated evidence declarations, authority upload grants, and device-signed provider completion evidence bound to one exact request",
		PackageSubmissionAuth:   "Installation-certificate binding, device authentication, and authority reconciliation for evidence submissions",
		PackageControlPlaneTest: "Real authority-signed installation certificate fixtures for hostile control-plane tests",
		PackageProcess:          "Argv, environment, containment, bounded output, exit, and reaping over os/exec",
		PackageRelease:          "Clean repository binding, verified Go builds and process plans, bounded maintainer material exchange, executable inspection, signed tool and metadata provenance, immutable artifacts, manifests, Latest, and selection",
		PackageShutdown:         "Signal observation and phased bounded cleanup",
		PackageObjectStore:      "Bounded vendor-specified S3, GCS, or Cloudflare Images transfers through issued HTTPS capabilities, with integrity and provider evidence",
		PackageTimeProof:        "RFC 3161 request construction, response verification, and replay",
		PackageCloudIdentity:    "Bounded Google Cloud identity-token and OAuth access-token or AWS identity-token acquisition with redacted disclosure",
		PackageDeploy:           "Exact create-only GCS publication of one authenticated release and its metadata",
		PackageUpgrade:          "Crash-recoverable installation, activation, startup truth, rollback, and recovery",
		PackageGCSObjects:       "Authenticated Google Cloud Storage bucket provisioning, typed logical namespace composition, create-only writes, IAM-signed short-lived upload capabilities, exact-generation observation, digest-bound reads, and generation-matched permanent deletion through official SDKs",
		PackageID:               "Canonical UUIDv7 and ULID time-ordered identifiers from one observed instant and caller-supplied entropy",
		PackageChit:             "Authority-signed immutable custody tickets, streaming manifest closure, bounded catalogs, and device-signed catalog queries",
		PackageChitAuth:         "Installation-certificate binding and device authentication for one chit catalog query",
		PackageRetrieval:        "Device-signed exact-object requests, authority-signed expiring download capabilities bound to authenticated chit manifests, and atomic exact-file retrieval execution",
		PackageRetrievalAuth:    "Installation-certificate binding and device authentication for one evidence-retrieval request",
		PackagePayment:          "Authority-signed exact payment receipts, bounded catalogs, and device-signed catalog queries",
		PackagePaymentAuth:      "Installation-certificate binding and device authentication for one payment catalog query",
		PackageDistribution:     "Signed product-neutral release publication, update discovery, and exact upgrade-download agreements",
		PackageDistributionAuth: "Authenticated release-material responses plus installation-certificate binding and device authentication for publication, update, and upgrade requests",
		PackageWiring:           "Bounded immutable runtime component graphs with exact Primitive-door declarations",
		PackageLineIO:           "Bounded line scanning over one io.Reader through Go bufio.Scanner and bufio.ScanLines",
		PackageManual:           "Bounded validated human text and stable machine JSON manuals from one product-owned typed book",
	}
}

// Validate enforces a legal direct package relationship.
func (c DirectImportContract) Validate() error {
	if err := c.Importer.Validate(); err != nil {
		return err
	}
	if err := c.Imported.Validate(); err != nil {
		return err
	}
	if c.Importer == c.Imported || c.Importer == PackageCore {
		return architectureContractError("direct import has an invalid importer relationship")
	}
	return nil
}

// Validate enforces a legal test-only package relationship.
//
// A test edge obeys the identical relationship legality as a production edge:
// both endpoints are admitted packages and a package never imports itself or
// is imported by Core. Unlike a production edge, a test edge may target the
// test-support package, because declaring test isolation is exactly what test
// sources do.
func (c DirectTestImportContract) Validate() error {
	return DirectImportContract(c).Validate()
}

func packageIdentityText(identity PackageIdentity) string {
	if identity >= packageIdentityLimit {
		return ""
	}
	return packageIdentityTexts()[identity]
}

func packageIdentityTexts() [packageIdentityLimit]string {
	return [...]string{
		"",
		"core",
		"attest",
		"contextstate",
		"currency",
		"keygen",
		"testserial",
		"filelock",
		"filestore",
		"hostfacts",
		"temporal",
		"exchange",
		"fuzzfinder",
		"lease",
		"gate",
		"receipt",
		"controlwire",
		"controlplane",
		"submission",
		"submissionauth",
		"controlplanetest",
		"process",
		"release",
		"shutdown",
		"objectstore",
		"timeproof",
		"cloudidentity",
		"deploy",
		"upgrade",
		"gcsobjects",
		"id",
		"chit",
		"chitauth",
		"retrieval",
		"retrievalauth",
		"payment",
		"paymentauth",
		"distribution",
		"distributionauth",
		"wiring",
		"lineio",
		"manual",
	}
}

func (c ArchitectureCatalog) validatePackages() error {
	var found [packageIdentityLimit]bool
	for _, contract := range c.packages {
		if err := contract.Validate(); err != nil || found[contract.Identity] {
			return architectureContractError("architecture catalog contains an invalid or duplicate package")
		}
		found[contract.Identity] = true
	}
	for identity := PackageCore; identity < packageIdentityLimit; identity++ {
		if !found[identity] {
			return architectureContractError("architecture catalog omits an admitted package")
		}
	}
	return nil
}

func (c ArchitectureCatalog) validateDirectImports() error {
	for index, contract := range c.imports {
		if err := contract.Validate(); err != nil {
			return err
		}
		imported, found := c.Lookup(contract.Imported)
		if !found || imported.Kind == PackageKindTestSupport {
			return architectureContractError("direct import targets an absent or test-support package")
		}
		for prior := range index {
			if c.imports[prior] == contract {
				return architectureContractError("architecture catalog contains a duplicate direct import")
			}
		}
	}
	return nil
}

func (c ArchitectureCatalog) validateDirectTestImports() error {
	for index, contract := range c.testImports {
		if err := contract.Validate(); err != nil {
			return err
		}
		if _, found := c.Lookup(contract.Imported); !found {
			return architectureContractError("direct test import targets an absent package")
		}
		if c.ContainsDirectImport(DirectImportContract(contract)) {
			return architectureContractError("direct test import duplicates a production edge")
		}
		for prior := range index {
			if c.testImports[prior] == contract {
				return architectureContractError("architecture catalog contains a duplicate direct test import")
			}
		}
	}
	return nil
}

// validateImportCardinality bounds each package's total compiler-visible
// sibling coupling. Test-only edges count against the same ceiling as
// production edges; a package does not buy extra coupling by spending it in
// its test sources.
func (c ArchitectureCatalog) validateImportCardinality() error {
	var counts [packageIdentityLimit]uint8
	for _, contract := range c.imports {
		counts[contract.Importer]++
	}
	for _, contract := range c.testImports {
		counts[contract.Importer]++
	}
	for identity := PackageAttest; identity < packageIdentityLimit; identity++ {
		if counts[identity] == 0 {
			return architectureContractError("non-core package has no direct imports")
		}
		if counts[identity] > PrimitiveMaximumDirectImports {
			return architectureContractError("package exceeds the direct import limit")
		}
	}
	return nil
}

func (c ArchitectureCatalog) hasCycle() bool {
	var visiting [packageIdentityLimit]bool
	var visited [packageIdentityLimit]bool
	for identity := PackageCore; identity < packageIdentityLimit; identity++ {
		if c.packageHasCycle(identity, &visiting, &visited) {
			return true
		}
	}
	return false
}

func (c ArchitectureCatalog) packageHasCycle(
	identity PackageIdentity,
	visiting *[packageIdentityLimit]bool,
	visited *[packageIdentityLimit]bool,
) bool {
	if visiting[identity] {
		return true
	}
	if visited[identity] {
		return false
	}
	visiting[identity] = true
	for _, contract := range c.imports {
		if contract.Importer == identity &&
			c.packageHasCycle(contract.Imported, visiting, visited) {
			return true
		}
	}
	visiting[identity] = false
	visited[identity] = true
	return false
}

// ParsePackageIdentity parses one canonical package name.
func ParsePackageIdentity(value string) (PackageIdentity, error) {
	if value == "" || strings.Contains(value, "/") {
		return PackageUnknown, architectureContractError("package name is empty or contains a path separator")
	}
	for identity := PackageCore; identity < packageIdentityLimit; identity++ {
		if packageIdentityText(identity) == value {
			return identity, nil
		}
	}
	return PackageUnknown, architectureContractError("package name is not admitted")
}

func architectureContractError(message string) error {
	return errors.Join(ErrPrimitiveContract, errors.New(message))
}
