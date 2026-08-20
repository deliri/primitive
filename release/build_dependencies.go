package release

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// BuildDependencyMaximumCount bounds the distinct non-main modules in one
	// exact four-target build closure. The ceiling is chosen so the largest
	// admissible closure still projects into a decodable dependency document:
	// every module path, version, and sum admits only characters the JSON
	// encoder emits verbatim, so one entry costs at most
	// mainPackageMaximumBytes + goModuleVersionMaximumBytes +
	// goModuleSumMaximumBytes + dependencyEntryPunctuationBytes bytes and the
	// whole document stays under dependencyDocumentExtentMaximum.
	BuildDependencyMaximumCount = 1024
	goModuleVersionMaximumBytes = 256
	goModuleSumPrefix           = "h1:"
	goModuleSumMaximumBytes     = 47
	// dependencyDocumentExtentMaximum is Core's supported document ceiling. The
	// projection must never produce a document the decoder would refuse, so the
	// module ceiling above is derived from this bound rather than declared
	// independently of it.
	dependencyDocumentExtentMaximum = 1 << 20
	// dependencyEntryPunctuationBytes is the exact canonical JSON framing for
	// one module plus its worst-case array separator. The final module has no
	// comma, so charging every module for one is a safe exact upper bound.
	dependencyEntryPunctuationBytes = len(`{"path":"","version":"","sum":""},`)
	// dependencyDocumentHeaderBytes is the exact fixed document framing plus
	// the largest admitted main-module path and the pinned toolchain token.
	dependencyDocumentHeaderBytes = len(`{"main_module":"","go_toolchain":"","modules":[]}`) +
		mainPackageMaximumBytes + len(goToolchainVersionPrimitive2026)
)

// Each dependency fact is rejected from more than one admission site, so its
// operator-facing diagnostic is named once here rather than respelled at every
// site. One spelling per fact keeps the rejection reason attributable to the
// contract instead of to whichever branch happened to catch it.
const (
	goModulePathInvalidDiagnostic    = "go module path is invalid"
	goModuleVersionInvalidDiagnostic = "go module version is invalid"
	goModuleSumInvalidDiagnostic     = "go module sum is invalid"
	buildDependencyCountDiagnostic   = "build dependency count exceeds its bound"
)

// dependencyDocumentHeadroom is the compiler's proof that the largest
// admissible module closure still projects into a document the decoder accepts.
// A module ceiling raised past its document ceiling makes this constant
// negative, and an unsigned conversion of a negative constant does not compile.
const dependencyDocumentHeadroom = uint(dependencyDocumentExtentMaximum -
	dependencyDocumentHeaderBytes - BuildDependencyMaximumCount*(mainPackageMaximumBytes+
	goModuleVersionMaximumBytes+goModuleSumMaximumBytes+dependencyEntryPunctuationBytes))

// GoModulePath is one bounded canonical module path observed from cmd/go.
type GoModulePath struct {
	value string
}

func parseGoModulePath(value string) (GoModulePath, error) {
	if err := validateGoPackagePath(value, mainPackageMaximumBytes); err != nil {
		return GoModulePath{}, contractError(errors.New(goModulePathInvalidDiagnostic), err)
	}
	return GoModulePath{value: value}, nil
}

func (p GoModulePath) Validate() error {
	parsed, err := parseGoModulePath(p.value)
	if err != nil || parsed != p {
		return contractError(errors.New(goModulePathInvalidDiagnostic), err)
	}
	return nil
}

func (p GoModulePath) String() string {
	if p.Validate() != nil {
		return ""
	}
	return p.value
}

// GoModuleVersion is one bounded cmd/go module-version token.
type GoModuleVersion struct {
	value string
}

// parseGoModuleVersion admits the exact character set cmd/go can report for a
// module version: a leading v, then the semantic-version alphabet used by
// releases, pre-releases, build metadata, pseudo-versions, and +incompatible.
// The set deliberately excludes every character the JSON encoder would escape,
// which is what makes the dependency document's byte ceiling provable rather
// than hopeful, and every character that is hostile in argv.
func parseGoModuleVersion(value string) (GoModuleVersion, error) {
	if len(value) < 2 || len(value) > goModuleVersionMaximumBytes || value[0] != 'v' || !utf8.ValidString(value) {
		return GoModuleVersion{}, contractError(errors.New(goModuleVersionInvalidDiagnostic))
	}
	for _, character := range value {
		if !goModuleVersionRune(character) {
			return GoModuleVersion{}, contractError(errors.New(goModuleVersionInvalidDiagnostic))
		}
	}
	return GoModuleVersion{value: value}, nil
}

func goModuleVersionRune(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' ||
		strings.ContainsRune(".+-", character)
}

func (v GoModuleVersion) Validate() error {
	parsed, err := parseGoModuleVersion(v.value)
	if err != nil || parsed != v {
		return contractError(errors.New(goModuleVersionInvalidDiagnostic), err)
	}
	return nil
}

func (v GoModuleVersion) String() string {
	if v.Validate() != nil {
		return ""
	}
	return v.value
}

// GoModuleSum is one exact h1 module-content checksum.
type GoModuleSum struct {
	digest [core.SHA256DigestBytes]byte
	valid  bool
}

func parseGoModuleSum(value string) (GoModuleSum, error) {
	encoded, found := strings.CutPrefix(value, goModuleSumPrefix)
	if !found {
		return GoModuleSum{}, contractError(errors.New(goModuleSumInvalidDiagnostic))
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) != core.SHA256DigestBytes {
		return GoModuleSum{}, contractError(errors.New(goModuleSumInvalidDiagnostic), err)
	}
	var digest [core.SHA256DigestBytes]byte
	copy(digest[:], decoded)
	return GoModuleSum{digest: digest, valid: true}, nil
}

func (s GoModuleSum) Validate() error {
	if !s.valid {
		return contractError(errors.New(goModuleSumInvalidDiagnostic))
	}
	return nil
}

func (s GoModuleSum) String() string {
	if s.Validate() != nil {
		return ""
	}
	return "h1:" + base64.StdEncoding.EncodeToString(s.digest[:])
}

// BuildDependency is one non-main module in the exact package closure.
type BuildDependency struct {
	path    GoModulePath
	version GoModuleVersion
	sum     GoModuleSum
}

func newBuildDependency(path, version, sum string) (BuildDependency, error) {
	parsedPath, err := parseGoModulePath(path)
	if err != nil {
		return BuildDependency{}, err
	}
	parsedVersion, err := parseGoModuleVersion(version)
	if err != nil {
		return BuildDependency{}, err
	}
	parsedSum, err := parseGoModuleSum(sum)
	if err != nil {
		return BuildDependency{}, err
	}
	value := BuildDependency{path: parsedPath, version: parsedVersion, sum: parsedSum}
	return value, value.Validate()
}

func (d BuildDependency) Validate() error {
	for _, err := range [...]error{d.path.Validate(), d.version.Validate(), d.sum.Validate()} {
		if err != nil {
			return contractError(errors.New("build dependency is invalid"), err)
		}
	}
	return nil
}

func (d BuildDependency) Path() GoModulePath       { return d.path }
func (d BuildDependency) Version() GoModuleVersion { return d.version }
func (d BuildDependency) Sum() GoModuleSum         { return d.sum }

// BuildDependencies is the fixed, path-sorted union of all four target package
// closures observed from the verified Go tool before Garble erases build info.
type BuildDependencies struct {
	storage     *buildDependencyStorage
	main        GoModulePath
	goToolchain GoToolchainIdentity
	valid       bool
}

type buildDependencyStorage struct {
	modules [BuildDependencyMaximumCount]BuildDependency
	count   int
}

func newBuildDependencies(
	main GoModulePath,
	toolchain GoToolchainIdentity,
	modules []BuildDependency,
) (BuildDependencies, error) {
	if len(modules) > BuildDependencyMaximumCount {
		return BuildDependencies{}, contractError(errors.New(buildDependencyCountDiagnostic))
	}
	ordered := append([]BuildDependency(nil), modules...)
	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left].path.value < ordered[right].path.value
	})
	storage := &buildDependencyStorage{count: len(ordered)}
	copy(storage.modules[:], ordered)
	value := BuildDependencies{storage: storage, main: main, goToolchain: toolchain, valid: true}
	if err := value.Validate(); err != nil {
		return BuildDependencies{}, err
	}
	return value, nil
}

func (d BuildDependencies) Validate() error {
	if !d.valid || d.storage == nil ||
		d.storage.count < 0 || d.storage.count > len(d.storage.modules) {
		return contractError(errors.New("build dependencies are unset or outside storage bounds"))
	}
	for _, err := range [...]error{d.main.Validate(), d.goToolchain.Validate()} {
		if err != nil {
			return contractError(errors.New("build dependencies are invalid"), err)
		}
	}
	return d.validateModules()
}

func (d BuildDependencies) validateModules() error {
	for index, module := range d.storage.modules[:d.storage.count] {
		if err := module.Validate(); err != nil {
			return err
		}
		if module.path == d.main || index > 0 && d.storage.modules[index-1].path.value >= module.path.value {
			return contractError(errors.New("build dependencies are not distinct and path-sorted"))
		}
	}
	for _, padding := range d.storage.modules[d.storage.count:] {
		if padding != (BuildDependency{}) {
			return contractError(errors.New("build dependency padding is nonzero"))
		}
	}
	return nil
}

type buildDependencyWire struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Sum     string `json:"sum"`
}

type buildDependenciesWire struct {
	MainModule  string                `json:"main_module"`
	GoToolchain string                `json:"go_toolchain"`
	Modules     []buildDependencyWire `json:"modules"`
}

// MarshalJSON projects the observed closure into the exact customer-visible
// dependency document. Release owns this projection so every consumer publishes
// one dependency document shape instead of inventing its own.
func (d BuildDependencies) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	toolchain, err := d.goToolchain.Version()
	if err != nil {
		return nil, err
	}
	modules := make([]buildDependencyWire, d.storage.count)
	for index, module := range d.storage.modules[:d.storage.count] {
		modules[index] = buildDependencyWire{
			Path: module.path.value, Version: module.version.value, Sum: module.sum.String(),
		}
	}
	return json.Marshal(buildDependenciesWire{
		MainModule: d.main.value, GoToolchain: toolchain, Modules: modules,
	})
}

// UnmarshalJSON reconstructs a published dependency document and re-proves its
// module bound, distinctness, and canonical order.
func (d *BuildDependencies) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("build dependencies receiver is nil"))
	}
	wire, err := decodeDependencyStructure[buildDependenciesWire](data)
	if err != nil {
		return err
	}
	candidate, err := buildDependenciesFromWire(wire)
	if err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

func buildDependenciesFromWire(w buildDependenciesWire) (BuildDependencies, error) {
	main, err := parseGoModulePath(w.MainModule)
	if err != nil {
		return BuildDependencies{}, err
	}
	toolchain, err := parseGoToolchainVersion(w.GoToolchain)
	if err != nil {
		return BuildDependencies{}, err
	}
	if len(w.Modules) > BuildDependencyMaximumCount {
		return BuildDependencies{}, contractError(errors.New(buildDependencyCountDiagnostic))
	}
	modules := make([]BuildDependency, len(w.Modules))
	for index, module := range w.Modules {
		modules[index], err = newBuildDependency(module.Path, module.Version, module.Sum)
		if err != nil {
			return BuildDependencies{}, err
		}
		// A published dependency document is already canonical. Construction
		// sorts, so rejecting a reordered document here keeps decode and
		// re-encode byte-stable instead of silently accepting two documents
		// that project to one value.
		if index > 0 && modules[index-1].path.value >= modules[index].path.value {
			return BuildDependencies{}, contractError(errors.New("dependency document is not distinct and path-sorted"))
		}
	}
	return newBuildDependencies(main, toolchain, modules)
}

func (d BuildDependencies) MainModule() GoModulePath         { return d.main }
func (d BuildDependencies) GoToolchain() GoToolchainIdentity { return d.goToolchain }
func (d BuildDependencies) Count() int {
	if d.Validate() != nil {
		return 0
	}
	return d.storage.count
}
func (d BuildDependencies) At(index int) (BuildDependency, bool) {
	if d.Validate() != nil || index < 0 || index >= d.storage.count {
		return BuildDependency{}, false
	}
	return d.storage.modules[index], true
}
