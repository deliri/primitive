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
	PrimitivePackageCount = 23
	// PrimitiveDirectImportCount is the number of admitted direct import edges.
	PrimitiveDirectImportCount = 55
	// PrimitiveDirectTestImportCount is the number of admitted test-only edges.
	PrimitiveDirectTestImportCount = 6
	// PrimitiveMaximumDirectImports caps direct sibling imports per package.
	PrimitiveMaximumDirectImports = 6
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
	// PackageGarble identifies the garbling package.
	PackageGarble
	// PackageKeygen identifies the key-generation package.
	PackageKeygen
	// PackageTestSerial identifies the serial test-support package.
	PackageTestSerial
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
			{Identity: PackageGarble, Kind: PackageKindProduction},
			{Identity: PackageKeygen, Kind: PackageKindProduction},
			{Identity: PackageTestSerial, Kind: PackageKindTestSupport},
			{Identity: PackageFilestore, Kind: PackageKindProduction},
			{Identity: PackageHostFacts, Kind: PackageKindProduction},
			{Identity: PackageTemporal, Kind: PackageKindProduction},
			{Identity: PackageExchange, Kind: PackageKindProduction},
			{Identity: PackageFuzzFinder, Kind: PackageKindProduction},
			{Identity: PackageLease, Kind: PackageKindProduction},
			{Identity: PackageGate, Kind: PackageKindProduction},
			{Identity: PackageReceipt, Kind: PackageKindProduction},
			{Identity: PackageProcess, Kind: PackageKindProduction},
			{Identity: PackageRelease, Kind: PackageKindProduction},
			{Identity: PackageShutdown, Kind: PackageKindProduction},
			{Identity: PackageObjectStore, Kind: PackageKindProduction},
			{Identity: PackageTimeProof, Kind: PackageKindProduction},
			{Identity: PackageCloudIdentity, Kind: PackageKindProduction},
			{Identity: PackageDeploy, Kind: PackageKindProduction},
			{Identity: PackageUpgrade, Kind: PackageKindProduction},
		},
		imports: [PrimitiveDirectImportCount]DirectImportContract{
			{Importer: PackageAttest, Imported: PackageCore},
			{Importer: PackageContextState, Imported: PackageCore},
			{Importer: PackageCurrency, Imported: PackageCore},
			{Importer: PackageGarble, Imported: PackageCore},
			{Importer: PackageKeygen, Imported: PackageCore},
			{Importer: PackageTestSerial, Imported: PackageCore},

			{Importer: PackageFilestore, Imported: PackageCore},
			{Importer: PackageFilestore, Imported: PackageContextState},
			{Importer: PackageHostFacts, Imported: PackageCore},
			{Importer: PackageHostFacts, Imported: PackageContextState},
			{Importer: PackageTemporal, Imported: PackageCore},
			{Importer: PackageTemporal, Imported: PackageContextState},

			{Importer: PackageExchange, Imported: PackageCore},
			{Importer: PackageExchange, Imported: PackageContextState},
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
			{Importer: PackageProcess, Imported: PackageCore},
			{Importer: PackageProcess, Imported: PackageContextState},
			{Importer: PackageProcess, Imported: PackageTemporal},
			{Importer: PackageRelease, Imported: PackageCore},
			{Importer: PackageRelease, Imported: PackageTemporal},
			{Importer: PackageRelease, Imported: PackageAttest},
			{Importer: PackageRelease, Imported: PackageGarble},
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
		},
		testImports: [PrimitiveDirectTestImportCount]DirectTestImportContract{
			{Importer: PackageGate, Imported: PackageAttest},
			{Importer: PackageGate, Imported: PackageTemporal},
			{Importer: PackageProcess, Imported: PackageTestSerial},
			{Importer: PackageDeploy, Imported: PackageAttest},
			{Importer: PackageDeploy, Imported: PackageExchange},
			{Importer: PackageDeploy, Imported: PackageTemporal},
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
	if c.Identity == PackageTestSerial {
		if c.Kind != PackageKindTestSupport {
			return architectureContractError("testserial must be classified as test support")
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
		PackageCore:          "Shared nominal values, errors, paths, protocol facts, numeric and encoding contracts",
		PackageAttest:        "Canonical Ed25519 envelopes and proof-carrying verification",
		PackageContextState:  "Nil-safe context ingress and terminal observation",
		PackageCurrency:      "Exact minor-unit values, arithmetic, ordering, and decimal projection",
		PackageGarble:        "Tool identity, seed custody and derivation, and typed build intent",
		PackageKeygen:        "Exact secret and Ed25519 key generation",
		PackageTestSerial:    "Test-only isolation declaration and analyzer contract",
		PackageFilestore:     "Rooted OS handles, confinement, durability, activation, append rotation, and recovery",
		PackageHostFacts:     "Host disk, memory, cgroup, tree, and OOM observations",
		PackageTemporal:      "Time, duration, arithmetic, persistence, waits, and tickers",
		PackageExchange:      "Bounded client and server boundary policy over net/http",
		PackageFuzzFinder:    "Bounded classification and observation of Go-generated fuzz artifacts",
		PackageLease:         "Signed lease timeline, assessment, renewal, and monotonic advance",
		PackageGate:          "Pure CLI-side new-work authorization over one authentic Lease assessment",
		PackageReceipt:       "Authenticated accepted-evidence facts and fixed-size monotonic watermarks",
		PackageProcess:       "Argv, environment, containment, bounded output, exit, and reaping over os/exec",
		PackageRelease:       "Verified build tools, deterministic Garble build and process plans, executable inspection, signed tool and metadata provenance, immutable artifacts, manifests, Latest, and selection",
		PackageShutdown:      "Signal observation and phased bounded cleanup",
		PackageObjectStore:   "One bounded vendor-specified S3, GCS, or Cloudflare Images transfer with integrity and commitment",
		PackageTimeProof:     "RFC 3161 request construction, response verification, and replay",
		PackageCloudIdentity: "Bounded Google Cloud or AWS outbound identity-token acquisition and redacted disclosure",
		PackageDeploy:        "Exact create-only GCS publication of one authenticated release and its metadata",
		PackageUpgrade:       "Crash-recoverable installation, activation, startup truth, rollback, and recovery",
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
		"garble",
		"keygen",
		"testserial",
		"filestore",
		"hostfacts",
		"temporal",
		"exchange",
		"fuzzfinder",
		"lease",
		"gate",
		"receipt",
		"process",
		"release",
		"shutdown",
		"objectstore",
		"timeproof",
		"cloudidentity",
		"deploy",
		"upgrade",
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
