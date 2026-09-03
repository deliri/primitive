package runworkspace

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
)

const GoDiscoverySourceMaximumBytes = 4 * 1024 * 1024

const (
	GoPackageTestFileMaximum       = 256
	GoPackageDirectoryEntryMaximum = 4096
)

type GoDeclaration struct {
	Symbol runprotocol.Name
	Kind   runprotocol.ProbeKind
}

func (d GoDeclaration) Validate() error {
	if err := errors.Join(d.Kind.Validate(), d.Symbol.Validate()); err != nil {
		return err
	}
	if d.Kind != runprotocol.ProbeKindGoTest && d.Kind != runprotocol.ProbeKindGoRace && d.Kind != runprotocol.ProbeKindGoBenchmark && d.Kind != runprotocol.ProbeKindGoFuzz && d.Kind != runprotocol.ProbeKindGoDiagnosticProfile {
		return core.ErrPrimitiveContract
	}
	return nil
}

type GoDiscovery struct {
	File         runprotocol.SourcePath
	Declarations []GoDiscoveredDeclaration
}

type GoDiscoveredDeclaration struct {
	Declaration        GoDeclaration
	BuildContextDigest core.SHA256Digest
}

func (d GoDiscoveredDeclaration) Validate() error {
	return errors.Join(d.Declaration.Validate(), d.BuildContextDigest.Validate())
}

type GoFileDiscoveryRequest struct {
	Target   runprotocol.GoFileTarget
	Profile  runprotocol.ProfileIdentity
	Contexts runnercontrol.GoBuildContextSet
	Source   VerifiedSource
}

func (r GoFileDiscoveryRequest) Validate() error {
	if err := errors.Join(r.Source.Validate(), r.Target.Validate(), r.Profile.Validate(), r.Contexts.Validate()); err != nil {
		return err
	}
	if len(r.Contexts.Entries) != len(r.Target.ChildKinds) {
		return core.ErrPrimitiveContract
	}
	for _, kind := range r.Target.ChildKinds {
		if _, ok := r.Contexts.Find(kind, r.Profile); !ok {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

type GoPackageDiscoveredDeclaration struct {
	File               runprotocol.SourcePath
	Declaration        GoDeclaration
	BuildContextDigest core.SHA256Digest
}

func (d GoPackageDiscoveredDeclaration) Validate() error {
	return errors.Join(d.File.Validate(), d.Declaration.Validate(), d.BuildContextDigest.Validate())
}

type GoPackageDiscovery struct {
	Package      runprotocol.SourcePath
	Declarations []GoPackageDiscoveredDeclaration
}

func (d GoPackageDiscovery) Validate() error {
	if err := d.Package.Validate(); err != nil || len(d.Declarations) > runnercontrol.ExpansionChildMaximum {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	for index := range d.Declarations {
		if err := d.Declarations[index].Validate(); err != nil {
			return err
		}
		if index > 0 && !goPackageDeclarationLess(d.Declarations[index-1], d.Declarations[index]) {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

type GoPackageDiscoveryRequest struct {
	Target   runprotocol.GoPackageTarget
	Profile  runprotocol.ProfileIdentity
	Contexts runnercontrol.GoBuildContextSet
	Source   VerifiedSource
}

func (r GoPackageDiscoveryRequest) Validate() error {
	if err := errors.Join(r.Source.Validate(), r.Target.Validate(), r.Profile.Validate(), r.Contexts.Validate()); err != nil {
		return err
	}
	if len(r.Contexts.Entries) != len(r.Target.ChildKinds) {
		return core.ErrPrimitiveContract
	}
	for _, kind := range r.Target.ChildKinds {
		if _, ok := r.Contexts.Find(kind, r.Profile); !ok {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func (d GoDiscovery) Validate() error {
	if err := d.File.Validate(); err != nil || len(d.Declarations) > runnercontrol.ExpansionChildMaximum {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	for index := range d.Declarations {
		if err := d.Declarations[index].Validate(); err != nil {
			return err
		}
		if index > 0 && !goDiscoveredDeclarationLess(d.Declarations[index-1], d.Declarations[index]) {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func (m Manager) DiscoverGoFile(ctx context.Context, request GoFileDiscoveryRequest) (GoDiscovery, error) {
	if err := errors.Join(m.Validate(), request.Validate()); err != nil {
		return GoDiscovery{}, err
	}
	relative, err := core.ParseRelativePath(filepath.Join(request.Source.Checkout.String(), request.Target.File.String()))
	if err != nil {
		return GoDiscovery{}, err
	}
	content, err := m.readGoSource(ctx, relative)
	if err != nil {
		return GoDiscovery{}, err
	}
	declarations, err := discoverGoFileContexts(content, request)
	if err != nil {
		return GoDiscovery{}, err
	}
	discovery := GoDiscovery{File: request.Target.File, Declarations: declarations}
	return discovery, discovery.Validate()
}

func (m Manager) DiscoverGoPackage(ctx context.Context, request GoPackageDiscoveryRequest) (GoPackageDiscovery, error) {
	if err := errors.Join(m.Validate(), request.Validate()); err != nil {
		return GoPackageDiscovery{}, err
	}
	directory, err := core.ParseRelativePath(filepath.Join(request.Source.Checkout.String(), request.Target.Package.String()))
	maximum, maximumErr := filestore.NewDirectoryEntryMaximum(GoPackageDirectoryEntryMaximum)
	if err != nil || maximumErr != nil {
		return GoPackageDiscovery{}, errors.Join(err, maximumErr)
	}
	declarations := make([]GoPackageDiscoveredDeclaration, 0)
	files := 0
	walkErr := filestore.Walk(ctx, filestore.WalkRequest{
		Location: filestore.Location{Root: m.root, Path: directory}, Order: filestore.WalkOrderLexical, DirectoryEntryMaximum: maximum,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if entry.Entry.IsDir() {
				return filestore.WalkSkipDirectory, nil
			}
			if !strings.HasSuffix(entry.Entry.Name(), runprotocol.GoTestFileSuffix) {
				return filestore.WalkContinue, nil
			}
			files++
			if files > GoPackageTestFileMaximum {
				return filestore.WalkDirectiveUnknown, core.ErrPrimitiveContract
			}
			found, discoverErr := m.discoverGoPackageFile(ctx, request, entry)
			declarations = append(declarations, found...)
			return filestore.WalkContinue, discoverErr
		},
	})
	if walkErr != nil {
		return GoPackageDiscovery{}, walkErr
	}
	sort.Slice(declarations, func(left, right int) bool { return goPackageDeclarationLess(declarations[left], declarations[right]) })
	discovery := GoPackageDiscovery{Package: request.Target.Package, Declarations: declarations}
	return discovery, discovery.Validate()
}

func (m Manager) discoverGoPackageFile(ctx context.Context, request GoPackageDiscoveryRequest, entry filestore.WalkEntry) ([]GoPackageDiscoveredDeclaration, error) {
	file, err := runprotocol.ParseSourcePath(filepath.Join(request.Target.Package.String(), entry.Entry.Name()))
	if err != nil {
		return nil, err
	}
	content, err := m.readGoSource(ctx, entry.Path)
	if err != nil {
		return nil, err
	}
	fileRequest := GoFileDiscoveryRequest{
		Source:  request.Source,
		Target:  runprotocol.GoFileTarget{Module: request.Target.Module, Package: request.Target.Package, File: file, ChildKinds: append([]runprotocol.ProbeKind(nil), request.Target.ChildKinds...)},
		Profile: request.Profile, Contexts: request.Contexts,
	}
	found, err := discoverGoFileContexts(content, fileRequest)
	if err != nil {
		return nil, err
	}
	declarations := make([]GoPackageDiscoveredDeclaration, len(found))
	for index := range found {
		declarations[index] = GoPackageDiscoveredDeclaration{File: file, Declaration: found[index].Declaration, BuildContextDigest: found[index].BuildContextDigest}
	}
	return declarations, nil
}

func (m Manager) readGoSource(ctx context.Context, relative core.RelativePath) ([]byte, error) {
	maximum, err := core.NewByteCount(GoDiscoverySourceMaximumBytes)
	if err != nil {
		return nil, err
	}
	var content bytes.Buffer
	if _, err := filestore.Read(ctx, filestore.ReadRequest{Destination: &content, Location: filestore.Location{Root: m.root, Path: relative}, MaximumBytes: maximum}); err != nil {
		return nil, err
	}
	return content.Bytes(), nil
}

func discoverGoFileContexts(source []byte, request GoFileDiscoveryRequest) ([]GoDiscoveredDeclaration, error) {
	declarations := make([]GoDiscoveredDeclaration, 0)
	for _, kind := range request.Target.ChildKinds {
		entry, ok := request.Contexts.Find(kind, request.Profile)
		if !ok {
			return nil, core.ErrPrimitiveContract
		}
		matches, err := matchGoFileContext(source, request.Target.File, entry.Context)
		if err != nil {
			return nil, err
		}
		if !matches {
			continue
		}
		found, err := ParseGoDeclarations(source, []runprotocol.ProbeKind{kind})
		if err != nil {
			return nil, err
		}
		for _, declaration := range found {
			declarations = append(declarations, GoDiscoveredDeclaration{Declaration: declaration, BuildContextDigest: entry.Digest})
		}
	}
	if len(declarations) > runnercontrol.ExpansionChildMaximum {
		return nil, core.ErrPrimitiveContract
	}
	sort.Slice(declarations, func(left, right int) bool {
		return goDiscoveredDeclarationLess(declarations[left], declarations[right])
	})
	return declarations, nil
}

func goDiscoveredDeclarationLess(left, right GoDiscoveredDeclaration) bool {
	return goDeclarationLess(left.Declaration, right.Declaration)
}

func goPackageDeclarationLess(left, right GoPackageDiscoveredDeclaration) bool {
	if left.File != right.File {
		return left.File.String() < right.File.String()
	}
	return goDeclarationLess(left.Declaration, right.Declaration)
}

func matchGoFileContext(source []byte, file runprotocol.SourcePath, context runnercontrol.GoBuildContext) (bool, error) {
	if len(source) == 0 || len(source) > GoDiscoverySourceMaximumBytes {
		return false, core.ErrPrimitiveContract
	}
	if err := errors.Join(file.Validate(), context.Validate()); err != nil {
		return false, err
	}
	buildContext := build.Context{
		GOOS: context.GOOS.String(), GOARCH: context.GOARCH.String(), CgoEnabled: context.CGOEnabled, Compiler: "gc",
		BuildTags: goBuildTagStrings(context.BuildTags), ReleaseTags: goBuildTagStrings(context.ReleaseTags),
		ToolTags: goToolTags(context), JoinPath: path.Join,
		OpenFile: func(string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(source)), nil },
	}
	matched, err := buildContext.MatchFile(".", path.Base(file.String()))
	if err != nil {
		return false, errors.Join(core.ErrPrimitiveContract, err)
	}
	return matched, nil
}

func goBuildTagStrings(values []runnercontrol.GoBuildTag) []string {
	result := make([]string, len(values))
	for index := range values {
		result[index] = values[index].String()
	}
	return result
}

func goToolTags(context runnercontrol.GoBuildContext) []string {
	experiments := goBuildTagStrings(context.GOExperiment)
	values := make([]string, len(experiments))
	for index := range experiments {
		values[index] = "goexperiment." + experiments[index]
	}
	if context.Instrumentation == runnercontrol.GoInstrumentationRace {
		values = append(values, runprotocol.GoRaceText)
	}
	return values
}

// ParseGoDeclarations admits one bounded external Go source file and returns
// only declarations the Go test harness can execute under the requested kinds.
func ParseGoDeclarations(source []byte, requested []runprotocol.ProbeKind) ([]GoDeclaration, error) {
	if len(source) == 0 || len(source) > GoDiscoverySourceMaximumBytes {
		return nil, core.ErrPrimitiveContract
	}
	if err := validateDiscoveryKinds(requested); err != nil {
		return nil, err
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), "selection_test.go", source, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, errors.Join(core.ErrPrimitiveContract, err)
	}
	return discoverGoDeclarations(parsed, requested)
}

func discoverGoDeclarations(parsed *ast.File, requested []runprotocol.ProbeKind) ([]GoDeclaration, error) {
	declarations := make([]GoDeclaration, 0)
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		base, ok := executableGoDeclaration(function, parsed.Comments)
		if !ok {
			continue
		}
		for _, kind := range requested {
			if !discoveryKindSupports(base, kind) {
				continue
			}
			name, nameErr := runprotocol.NewName(function.Name.Name)
			if nameErr != nil {
				return nil, nameErr
			}
			declarations = append(declarations, GoDeclaration{Kind: kind, Symbol: name})
		}
	}
	if len(declarations) > runnercontrol.ExpansionChildMaximum {
		return nil, core.ErrPrimitiveContract
	}
	sort.Slice(declarations, func(left, right int) bool { return goDeclarationLess(declarations[left], declarations[right]) })
	return declarations, nil
}

func validateDiscoveryKinds(kinds []runprotocol.ProbeKind) error {
	if len(kinds) == 0 || len(kinds) > runprotocol.ProbeKindMaximum {
		return core.ErrPrimitiveContract
	}
	for index, kind := range kinds {
		if !validDiscoveryKind(kind) {
			return core.ErrPrimitiveContract
		}
		if index > 0 && kinds[index-1] >= kind {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func validDiscoveryKind(kind runprotocol.ProbeKind) bool {
	return kind == runprotocol.ProbeKindGoTest || kind == runprotocol.ProbeKindGoRace ||
		kind == runprotocol.ProbeKindGoBenchmark || kind == runprotocol.ProbeKindGoFuzz ||
		kind == runprotocol.ProbeKindGoDiagnosticProfile
}

type goDeclarationCandidate struct {
	prefix    string
	parameter string
	kind      runprotocol.ProbeKind
	example   bool
}

func executableGoDeclaration(function *ast.FuncDecl, comments []*ast.CommentGroup) (runprotocol.ProbeKind, bool) {
	if function == nil || function.Recv != nil || function.Type.TypeParams != nil || function.Name == nil {
		return runprotocol.ProbeKindUnknown, false
	}
	candidates := [...]goDeclarationCandidate{
		{prefix: "Test", parameter: "T", kind: runprotocol.ProbeKindGoTest},
		{prefix: "Benchmark", parameter: "B", kind: runprotocol.ProbeKindGoBenchmark},
		{prefix: "Fuzz", parameter: "F", kind: runprotocol.ProbeKindGoFuzz},
		{prefix: "Example", kind: runprotocol.ProbeKindGoTest, example: true},
	}
	for _, candidate := range candidates {
		if declarationMatches(function, comments, candidate) {
			return candidate.kind, true
		}
	}
	return runprotocol.ProbeKindUnknown, false
}

func declarationMatches(function *ast.FuncDecl, comments []*ast.CommentGroup, candidate goDeclarationCandidate) bool {
	if !eligibleGoTestName(function.Name.Name, candidate.prefix) {
		return false
	}
	if candidate.example {
		return exampleSignature(function) && exampleHasOutput(function, comments)
	}
	return testingParameter(function, candidate.parameter)
}

func eligibleGoTestName(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) {
		return true
	}
	next, _ := utf8.DecodeRuneInString(name[len(prefix):])
	return !unicode.IsLower(next)
}

func testingParameter(function *ast.FuncDecl, parameter string) bool {
	if function.Type.Results != nil || function.Type.Params == nil || len(function.Type.Params.List) != 1 {
		return false
	}
	field := function.Type.Params.List[0]
	pointer, ok := field.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	packageName, packageOK := selector.X.(*ast.Ident)
	return packageOK && packageName.Name == "testing" && selector.Sel.Name == parameter
}

func exampleSignature(function *ast.FuncDecl) bool {
	return (function.Type.Params == nil || len(function.Type.Params.List) == 0) && function.Type.Results == nil
}

func exampleHasOutput(function *ast.FuncDecl, comments []*ast.CommentGroup) bool {
	if function.Body == nil {
		return false
	}
	for _, group := range comments {
		if group.Pos() <= function.Body.Lbrace || group.End() >= function.Body.Rbrace {
			continue
		}
		for _, comment := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			if strings.HasPrefix(text, "Output:") || strings.HasPrefix(text, "Unordered output:") {
				return true
			}
		}
	}
	return false
}

func discoveryKindSupports(base, requested runprotocol.ProbeKind) bool {
	if base == requested {
		return true
	}
	return base == runprotocol.ProbeKindGoTest && (requested == runprotocol.ProbeKindGoRace || requested == runprotocol.ProbeKindGoDiagnosticProfile)
}

func goDeclarationLess(left, right GoDeclaration) bool {
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	return left.Symbol.String() < right.Symbol.String()
}

var (
	_ core.Validatable = GoDeclaration{}
	_ core.Validatable = GoDiscovery{}
	_ core.Validatable = GoPackageDiscoveredDeclaration{}
	_ core.Validatable = GoPackageDiscovery{}
	_ core.Validatable = GoPackageDiscoveryRequest{}
)
