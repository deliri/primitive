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
	// PackageGoModule identifies the canonical Go module identity package.
	PackageGoModule
	// PackageGoToolchain identifies bounded typed cmd/go observations.
	PackageGoToolchain
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
	// PackageSecretStore identifies bounded secret-provider access.
	PackageSecretStore
	// PackageProjectStandards identifies reusable project/package knowledge and evidence projection.
	PackageProjectStandards
	// PackageMachineProbe identifies bounded execution of one admitted machine probe.
	PackageMachineProbe
	// PackageRunnerControl identifies typed domain-blind runner control contracts.
	PackageRunnerControl
	// PackageRunWorkspace identifies owned per-run workspace and evidence effects.
	PackageRunWorkspace
	// PackageProviderWire identifies provider-authenticated domain-blind HTTP plugs.
	PackageProviderWire
	// PackageCapabilities identifies the compiler-owned Primitive capability catalog.
	PackageCapabilities
	// PackagePrimitiveProject identifies Primitive's authored project policy.
	PackagePrimitiveProject
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

// ArchitectureCatalog is the complete validated Primitive package-ownership catalog.
type ArchitectureCatalog struct {
	packages [PrimitivePackageCount]PackageContract
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
			{Identity: PackageGoModule, Kind: PackageKindProduction},
			{Identity: PackageGoToolchain, Kind: PackageKindProduction},
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
			{Identity: PackageSecretStore, Kind: PackageKindProduction},
			{Identity: PackageProjectStandards, Kind: PackageKindProduction},
			{Identity: PackageMachineProbe, Kind: PackageKindProduction},
			{Identity: PackageRunnerControl, Kind: PackageKindProduction},
			{Identity: PackageRunWorkspace, Kind: PackageKindProduction},
			{Identity: PackageProviderWire, Kind: PackageKindProduction},
			{Identity: PackageCapabilities, Kind: PackageKindProduction},
			{Identity: PackagePrimitiveProject, Kind: PackageKindTestSupport},
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

// Purpose returns the compiler-owned statement of what the package provides.
func (p PackageIdentity) Purpose() (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	return packagePurposeText(p), nil
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
	if c.Identity == PackageTestSerial || c.Identity == PackageControlPlaneTest || c.Identity == PackagePrimitiveProject {
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

// #nosec G101 -- this catalog contains package-purpose prose, not credentials.
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
		PackageGoModule:         "Canonical bounded Go module identity parsing, validation, and JSON",
		PackageGoToolchain:      "Bounded typed module, build-context, package-list, and compilation observations through cmd/go",
		PackageRelease:          "Clean repository binding, verified Go builds and process plans, bounded maintainer material exchange, executable inspection, signed tool and metadata provenance, immutable artifacts, manifests, Latest, and selection",
		PackageShutdown:         "Signal observation and phased bounded cleanup",
		PackageObjectStore:      "Bounded vendor-specified S3, GCS, or Cloudflare Images transfers through issued HTTPS capabilities, with integrity and provider evidence",
		PackageTimeProof:        "RFC 3161 request construction, response verification, and replay",
		PackageCloudIdentity:    "Bounded Google Cloud identity-token and OAuth access-token or AWS identity-token acquisition with redacted disclosure",
		PackageDeploy:           "Exact create-only GCS publication of one authenticated release and its metadata",
		PackageUpgrade:          "Crash-recoverable installation, activation, startup truth, rollback, and recovery",
		PackageGCSObjects:       "Authenticated Google Cloud Storage bucket provisioning and public-read IAM, typed logical namespace composition, create-only writes, IAM-signed short-lived upload and whole-object retrieval capabilities, exact-generation observation, digest-bound reads, and generation-matched permanent deletion through official SDKs over Exchange",
		PackageID:               "Standard-library-backed UUIDv7 and canonical ULID time-ordered identifiers from one observed instant and caller-supplied entropy",
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
		PackageProjectStandards: "Validated project, package, and file knowledge with exact Primitive-effect posture, evidence references, deterministic reports, and bounded exchange",
		PackageMachineProbe:     "Bounded execution and typed evidence capture for one admitted machine-observation script",
		PackageRunnerControl:    "Typed domain-blind runner admission, execution, evidence, completion, and delivery contracts",
		PackageRunWorkspace:     "Owned per-run writable workspace, source acquisition, evidence retention, and cleanup effects",
		PackageSecretStore:      "Bounded exact-version secret access through official provider SDKs",
		PackageProviderWire:     "Provider-authenticated domain-blind streamed HTTP plugs for Stripe, Twilio, Plunk, and PayPal",
		PackageCapabilities:     "Compiler-owned discovery and exact resolution of Primitive package and real-world effect capabilities",
		PackagePrimitiveProject: "Authored Primitive project policy expressed through the product-neutral Project Standards contract",
	}
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
		"gomodule",
		"gotoolchain",
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
		"secretstore",
		"projectstandards",
		"machineprobe",
		"runnercontrol",
		"runworkspace",
		"providerwire",
		"capabilities",
		"primitiveproject",
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
