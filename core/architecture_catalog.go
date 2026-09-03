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
	// PrimitivePackageCount is derived from the closed package-identity domain.
	// Adding or removing an identity changes the catalog shape at compile time;
	// no copied package count can drift from the enum.
	PrimitivePackageCount = int(packageIdentityLimit - PackageCore)
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
	// PackageGoModule identifies the canonical Go module identity package.
	PackageGoModule
	// PackageGoToolchain identifies bounded typed cmd/go observations.
	PackageGoToolchain
	// PackageCompass identifies reusable project-configuration declarations and decoding.
	PackageCompass
	// PackageVersion identifies project release coordinates and canonical Git tags.
	PackageVersion
	// PackageRelease identifies the release package.
	PackageRelease
	// PackageShutdown identifies the shutdown package.
	PackageShutdown
	// PackageObjectStore identifies the object-store package.
	PackageObjectStore
	// PackageTimeProof identifies the time-proof package.
	PackageTimeProof
	// PackageGoogleIdentity identifies the Google identity package.
	PackageGoogleIdentity
	// PackageAWSIdentity identifies the AWS identity package.
	PackageAWSIdentity
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
	// PackageSecretStore identifies bounded secret-provider access.
	PackageSecretStore
	// PackageRunProtocol identifies the shared independent-run agreement.
	PackageRunProtocol
	// PackageSourceClaim identifies human-authored offline source claims.
	PackageSourceClaim
	// PackageSourceObservation identifies compiler-derived source observations.
	PackageSourceObservation
	// PackageSourceProof identifies claim-to-observation proof results.
	PackageSourceProof
	// PackageMachineProbe identifies bounded execution of one admitted machine probe.
	PackageMachineProbe
	// PackageRunnerControl identifies typed domain-blind runner control contracts.
	PackageRunnerControl
	// PackageRunWorkspace identifies owned per-run workspace and evidence effects.
	PackageRunWorkspace
	// PackageStripe identifies Stripe-authenticated HTTP and webhook contracts.
	PackageStripe
	// PackagePayPal identifies PayPal OAuth, HTTP, and webhook contracts.
	PackagePayPal
	// PackageTwilio identifies Twilio-authenticated HTTP and webhook contracts.
	PackageTwilio
	// PackagePlunk identifies Plunk-authenticated HTTP and webhook contracts.
	PackagePlunk
	// PackageCapabilities identifies the compiler-owned Primitive capability catalog.
	PackageCapabilities
	// PackageProofLedger identifies the blind append-only proof-chain agreement.
	PackageProofLedger
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
	// Role is the package's one primary architectural responsibility.
	Role PackageRole
}

// ArchitectureCatalog is the complete validated Primitive package-ownership catalog.
type ArchitectureCatalog struct {
	packages [PrimitivePackageCount]PackageContract
}

// PrimitiveArchitecture returns the complete compiler-owned package catalog.
func PrimitiveArchitecture() ArchitectureCatalog {
	return ArchitectureCatalog{
		packages: [PrimitivePackageCount]PackageContract{
			{Identity: PackageCore, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageAttest, Kind: PackageKindProduction, Role: PackageRoleAuthenticationBinding},
			{Identity: PackageContextState, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageCurrency, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageKeygen, Kind: PackageKindProduction, Role: PackageRoleEffectCapability},
			{Identity: PackageTestSerial, Kind: PackageKindTestSupport, Role: PackageRoleValueContract},
			{Identity: PackageFileLock, Kind: PackageKindProduction, Role: PackageRoleEffectCapability},
			{Identity: PackageFilestore, Kind: PackageKindProduction, Role: PackageRoleEffectCapability},
			{Identity: PackageHostFacts, Kind: PackageKindProduction, Role: PackageRoleEffectCapability},
			{Identity: PackageTemporal, Kind: PackageKindProduction, Role: PackageRoleEffectCapability},
			{Identity: PackageExchange, Kind: PackageKindProduction, Role: PackageRoleEffectCapability},
			{Identity: PackageFuzzFinder, Kind: PackageKindProduction, Role: PackageRoleOrchestration},
			{Identity: PackageLease, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackageReceipt, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackageControlWire, Kind: PackageKindProduction, Role: PackageRoleWireProtocol},
			{Identity: PackageControlPlane, Kind: PackageKindProduction, Role: PackageRoleAuthenticationBinding},
			{Identity: PackageSubmission, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackageSubmissionAuth, Kind: PackageKindProduction, Role: PackageRoleAuthenticationBinding},
			{Identity: PackageControlPlaneTest, Kind: PackageKindTestSupport, Role: PackageRoleAuthenticationBinding},
			{Identity: PackageProcess, Kind: PackageKindProduction, Role: PackageRoleEffectCapability},
			{Identity: PackageGoModule, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageGoToolchain, Kind: PackageKindProduction, Role: PackageRoleOrchestration},
			{Identity: PackageCompass, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageVersion, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageRelease, Kind: PackageKindProduction, Role: PackageRoleOrchestration},
			{Identity: PackageShutdown, Kind: PackageKindProduction, Role: PackageRoleEffectCapability},
			{Identity: PackageObjectStore, Kind: PackageKindProduction, Role: PackageRoleEffectCapability},
			{Identity: PackageTimeProof, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackageGoogleIdentity, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackageAWSIdentity, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackageDeploy, Kind: PackageKindProduction, Role: PackageRoleOrchestration},
			{Identity: PackageUpgrade, Kind: PackageKindProduction, Role: PackageRoleOrchestration},
			{Identity: PackageGCSObjects, Kind: PackageKindProduction, Role: PackageRoleOrchestration},
			{Identity: PackageID, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageChit, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackageChitAuth, Kind: PackageKindProduction, Role: PackageRoleAuthenticationBinding},
			{Identity: PackageRetrieval, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackageRetrievalAuth, Kind: PackageKindProduction, Role: PackageRoleAuthenticationBinding},
			{Identity: PackagePayment, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackagePaymentAuth, Kind: PackageKindProduction, Role: PackageRoleAuthenticationBinding},
			{Identity: PackageDistribution, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackageDistributionAuth, Kind: PackageKindProduction, Role: PackageRoleAuthenticationBinding},
			{Identity: PackageWiring, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageLineIO, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageManual, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageSecretStore, Kind: PackageKindProduction, Role: PackageRoleEffectCapability},
			{Identity: PackageRunProtocol, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackageSourceClaim, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageSourceObservation, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageSourceProof, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageMachineProbe, Kind: PackageKindProduction, Role: PackageRoleOrchestration},
			{Identity: PackageRunnerControl, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
			{Identity: PackageRunWorkspace, Kind: PackageKindProduction, Role: PackageRoleOrchestration},
			{Identity: PackageStripe, Kind: PackageKindProduction, Role: PackageRoleWireProtocol},
			{Identity: PackagePayPal, Kind: PackageKindProduction, Role: PackageRoleWireProtocol},
			{Identity: PackageTwilio, Kind: PackageKindProduction, Role: PackageRoleWireProtocol},
			{Identity: PackagePlunk, Kind: PackageKindProduction, Role: PackageRoleWireProtocol},
			{Identity: PackageCapabilities, Kind: PackageKindProduction, Role: PackageRoleValueContract},
			{Identity: PackageProofLedger, Kind: PackageKindProduction, Role: PackageRoleDomainAgreement},
		},
	}
}

// Validate rejects incomplete or duplicate package ownership.
func (c ArchitectureCatalog) Validate() error {
	return c.validatePackages()
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
	if err := c.Kind.Validate(); err != nil {
		return err
	}
	if err := c.Role.Validate(); err != nil {
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
		"receipt",
		"controlwire",
		"controlplane",
		"submission",
		"submissionauth",
		"controlplanetest",
		"process",
		"gomodule",
		"gotoolchain",
		"compass",
		"version",
		"release",
		"shutdown",
		"objectstore",
		"timeproof",
		"googleidentity",
		"awsidentity",
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
		"secretstore",
		"runprotocol",
		"sourceclaim",
		"sourceobservation",
		"sourceproof",
		"machineprobe",
		"runnercontrol",
		"runworkspace",
		"stripe",
		"paypal",
		"twilio",
		"plunk",
		"capabilities",
		"proofledger",
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
