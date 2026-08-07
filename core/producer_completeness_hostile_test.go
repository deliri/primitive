package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Producer completeness: Primitive must produce every type it demands.
//
// A door that requires a value no Primitive function returns cannot be opened
// by a caller obeying the layering law, because that caller is forbidden from
// constructing the value itself. The failure looks like a consumer stopping
// mid-task to add something to Primitive, and it happened four times in one
// session: filelock.Request demanded an *os.File nothing produced,
// Inspection.ModifiedAt was readable but unwritable, keygen.SigningKey had no
// adopt door, and Inspection carried no owner or permission facts.
//
// This is a graph property of Primitive's own source, so it is decided here
// rather than discovered by a consumer later.
//
// Not every unproduced type is a gap. The exclusions below are each a real way
// a caller legitimately obtains a value, and every one of them was added
// because it produced a false positive against the live tree.

// producerForbiddenStdlib names standard-library types a consumer may not
// construct, because CAPABILITIES.md gives the operation to a Primitive door.
// A Primitive door demanding one of these must hand one over.
var producerForbiddenStdlib = map[string]string{
	"os.File":            "filestore owns opening; never os.Open or os.OpenFile from a product",
	"os.Root":            "filestore owns confinement; never os.OpenRoot from a product",
	"ed25519.PrivateKey": "keygen is the entropy boundary; never crypto/rand directly",
	"ed25519.PublicKey":  "keygen and attest own key material",
	"hash.Hash":          "core.DigestWriter owns streaming digests",
	"exec.Cmd":           "process owns subprocess execution",
}

var producerPredeclared = map[string]bool{
	"bool": true, "string": true, "int": true, "int8": true, "int16": true,
	"int32": true, "int64": true, "uint": true, "uint8": true, "uint16": true,
	"uint32": true, "uint64": true, "uintptr": true, "byte": true, "rune": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
	"error": true, "any": true,
}

type producerSite struct {
	pkg   string
	where string
}

type producerGap struct {
	name  string
	why   string
	sites []producerSite
}

// producerShape records how a caller could obtain a value of one type.
type producerShape struct {
	isInterface  bool // satisfied, never constructed
	isFunc       bool // the caller writes a closure
	isStruct     bool
	hasUnexonted bool // an unexported field defeats a composite literal
	hasDecoder   bool // UnmarshalJSON and friends: the wire produces it
	hasNamedVals bool // exported constants or vars name a legal value
}

type producerScan struct {
	demanded   map[string][]producerSite
	produced   map[string]bool
	shapes     map[string]*producerShape
	typeParams map[string]bool
	pkgs       map[string]bool
}

func newProducerScan() *producerScan {
	return &producerScan{
		demanded:   map[string][]producerSite{},
		produced:   map[string]bool{},
		shapes:     map[string]*producerShape{},
		typeParams: map[string]bool{},
		pkgs:       map[string]bool{},
	}
}

func (s *producerScan) shapeOf(key string) *producerShape {
	if s.shapes[key] == nil {
		s.shapes[key] = &producerShape{}
	}
	return s.shapes[key]
}

func producerTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return producerTypeName(t.X)
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

func producerQualify(pkg, name string) string {
	if strings.Contains(name, ".") {
		return name
	}
	return pkg + "." + name
}

// auditProducerCompleteness walks every package directory under root and
// reports each type Primitive demands that no caller could obtain.
func auditProducerCompleteness(root string) ([]producerGap, error) {
	scan := newProducerScan()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		scan.pkgs[entry.Name()] = true
	}
	fset := token.NewFileSet()
	for pkg := range scan.pkgs {
		files, globErr := filepath.Glob(filepath.Join(root, pkg, "*.go"))
		if globErr != nil {
			return nil, globErr
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			parsed, parseErr := parser.ParseFile(fset, file, nil, 0)
			if parseErr != nil {
				return nil, parseErr
			}
			scan.collectNamedValues(pkg, parsed)
			scan.collectDeclarations(pkg, parsed)
		}
	}
	return scan.gaps(), nil
}

// collectNamedValues credits exported constants and vars. An iota block writes
// its type once, on the first specification, and every later constant inherits
// it, so the block's type is carried forward rather than read per line.
func (s *producerScan) collectNamedValues(pkg string, file *ast.File) {
	for _, decl := range file.Decls {
		general, ok := decl.(*ast.GenDecl)
		if !ok || (general.Tok != token.CONST && general.Tok != token.VAR) {
			continue
		}
		blockType := ""
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if value.Type != nil {
				blockType = producerTypeName(value.Type)
			}
			if blockType == "" || !anyExported(value.Names) {
				continue
			}
			s.shapeOf(producerQualify(pkg, blockType)).hasNamedVals = true
		}
	}
}

func anyExported(names []*ast.Ident) bool {
	for _, name := range names {
		if name.IsExported() {
			return true
		}
	}
	return false
}

func (s *producerScan) collectDeclarations(pkg string, file *ast.File) {
	ast.Inspect(file, func(node ast.Node) bool {
		switch decl := node.(type) {
		case *ast.FuncDecl:
			s.collectFunc(pkg, decl)
		case *ast.TypeSpec:
			s.collectType(pkg, decl)
		}
		return true
	})
}

func (s *producerScan) collectFunc(pkg string, decl *ast.FuncDecl) {
	if decl.Recv != nil {
		s.creditDecoder(pkg, decl)
	}
	// A method is a producer too. keygen.SigningKey.PrivateKey() is how a
	// caller obtains an ed25519.PrivateKey, and skipping methods reported that
	// as a missing door.
	if !decl.Name.IsExported() {
		return
	}
	if decl.Type.TypeParams != nil {
		s.creditTypeParams(pkg, decl.Type.TypeParams)
	}
	if decl.Type.Results != nil {
		for _, result := range decl.Type.Results.List {
			if name := producerTypeName(result.Type); name != "" {
				key := producerQualify(pkg, name)
				s.produced[key] = true
			}
		}
	}
	if decl.Type.Params == nil {
		return
	}
	for _, param := range decl.Type.Params.List {
		if name := producerTypeName(param.Type); name != "" {
			key := producerQualify(pkg, name)
			s.demanded[key] = append(s.demanded[key], producerSite{pkg, decl.Name.Name + "()"})
		}
	}
}

// creditDecoder treats a decode method as a producer. A wire type arrives by
// being decoded, not by being constructed, so demanding one is not a gap.
func (s *producerScan) creditDecoder(pkg string, decl *ast.FuncDecl) {
	switch decl.Name.Name {
	case "UnmarshalJSON", "UnmarshalText", "UnmarshalBinary", "Decode":
	default:
		return
	}
	if len(decl.Recv.List) != 1 {
		return
	}
	if name := producerTypeName(decl.Recv.List[0].Type); name != "" {
		s.shapeOf(producerQualify(pkg, name)).hasDecoder = true
	}
}

func (s *producerScan) creditTypeParams(pkg string, params *ast.FieldList) {
	for _, param := range params.List {
		for _, name := range param.Names {
			s.typeParams[pkg+"."+name.Name] = true
		}
	}
}

func (s *producerScan) collectType(pkg string, decl *ast.TypeSpec) {
	if !decl.Name.IsExported() {
		return
	}
	if decl.TypeParams != nil {
		s.creditTypeParams(pkg, decl.TypeParams)
	}
	key := pkg + "." + decl.Name.Name
	shape := s.shapeOf(key)
	switch typed := decl.Type.(type) {
	case *ast.InterfaceType:
		shape.isInterface = true
		return
	case *ast.FuncType:
		shape.isFunc = true
		return
	case *ast.StructType:
		shape.isStruct = true
		s.collectStructFields(pkg, decl.Name.Name, typed, shape)
	}
}

func (s *producerScan) collectStructFields(pkg, owner string, structure *ast.StructType, shape *producerShape) {
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			if !name.IsExported() {
				shape.hasUnexonted = true
				continue
			}
			if typeName := producerTypeName(field.Type); typeName != "" {
				key := producerQualify(pkg, typeName)
				s.demanded[key] = append(s.demanded[key], producerSite{pkg, owner + "." + name.Name})
			}
		}
	}
}

// obtainable reports whether a caller has any legitimate route to a value.
func (s *producerScan) obtainable(name string) bool {
	if s.produced[name] || s.typeParams[name] {
		return true
	}
	bare := name
	if index := strings.Index(name, "."); index >= 0 {
		bare = name[index+1:]
	}
	if producerPredeclared[bare] {
		return true
	}
	shape := s.shapes[name]
	if shape == nil {
		return false
	}
	if shape.isInterface || shape.isFunc || shape.hasDecoder || shape.hasNamedVals {
		return true
	}
	// A struct whose fields are all exported is built with a composite literal.
	// That is the ordinary request-struct pattern, not a missing door.
	return shape.isStruct && !shape.hasUnexonted
}

func (s *producerScan) gaps() []producerGap {
	var found []producerGap
	for name, sites := range s.demanded {
		if s.obtainable(name) {
			continue
		}
		qualifier := ""
		if index := strings.Index(name, "."); index >= 0 {
			qualifier = name[:index]
		}
		switch {
		case s.pkgs[qualifier]:
			found = append(found, producerGap{name: name, sites: sites})
		case producerForbiddenStdlib[name] != "":
			found = append(found, producerGap{name: name, why: producerForbiddenStdlib[name], sites: sites})
		default:
			// A standard-library type the caller legitimately holds already:
			// an io.Reader over their own bytes, an http.ResponseWriter handed
			// over by the server, an fmt.State handed over by fmt.
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].name < found[j].name })
	return found
}

// TestEveryDemandedTypeHasAProducer is the live scan. It fails when Primitive
// requires a value a caller cannot obtain, which is the defect that stops
// consumer work and sends someone back here mid-task.
func TestEveryDemandedTypeHasAProducer(t *testing.T) {
	t.Parallel()

	gaps, err := auditProducerCompleteness("..")
	if err != nil {
		t.Fatalf("auditProducerCompleteness() error = %v, want nil", err)
	}
	if len(gaps) == 0 {
		return
	}
	report := []string{"Primitive demands types it does not produce:"}
	for _, gap := range gaps {
		line := "  " + gap.name
		if gap.why != "" {
			line += " (" + gap.why + ")"
		}
		report = append(report, line)
		for _, site := range gap.sites {
			report = append(report, "      demanded by "+site.pkg+"."+site.where)
		}
	}
	report = append(report,
		"",
		"Add the door that produces the value, or the caller cannot open this one",
		"without reaching past Primitive, which the layering law forbids.")
	t.Fatal(strings.Join(report, "\n"))
}

// TestProducerCompletenessMatcherSyntheticRedGreenRatchet proves the matcher
// itself. Every exclusion below exists because it produced a false positive
// against the live tree, so each one is pinned with source that must be
// accepted and, where it applies, a sibling that must still be rejected. A
// matcher that silently stopped detecting anything would pass the live scan.
func TestProducerCompletenessMatcherSyntheticRedGreenRatchet(t *testing.T) {
	t.Parallel()

	cases := [...]struct {
		name     string
		source   string
		wantGaps int
	}{
		{
			name:     "opaque scalar with no constructor is a gap",
			wantGaps: 1,
			source: `package sample
type Activation uint64
func Compare(a Activation) bool { return a > 0 }`,
		},
		{
			name:     "opaque scalar with a constructor is obtainable",
			wantGaps: 0,
			source: `package sample
type Activation uint64
func NewActivation(v uint64) (Activation, error) { return Activation(v), nil }
func Compare(a Activation) bool { return a > 0 }`,
		},
		{
			name:     "opaque scalar with exported constants is nameable",
			wantGaps: 0,
			source: `package sample
type Kind uint8
const (
	kindUnknown Kind = iota
	KindFirst
	KindSecond
)
func Use(k Kind) bool { return k == KindFirst }`,
		},
		{
			name:     "all exported struct is built with a literal",
			wantGaps: 0,
			source: `package sample
type Request struct { Path string; Bytes int64 }
func Do(r Request) error { return nil }`,
		},
		{
			name:     "struct with an unexported field is not literal constructible",
			wantGaps: 1,
			source: `package sample
type Handle struct { Path string; fd int }
func Do(h Handle) error { return nil }`,
		},
		{
			name:     "wire type reached by decoding is obtainable",
			wantGaps: 0,
			source: `package sample
type Signature struct { Path string; raw []byte }
func (s *Signature) UnmarshalJSON(b []byte) error { return nil }
func Do(s Signature) error { return nil }`,
		},
		{
			name:     "interface is satisfied rather than constructed",
			wantGaps: 0,
			source: `package sample
type Target interface { String() string }
func Do(t Target) error { return nil }`,
		},
		{
			name:     "func type is written by the caller as a closure",
			wantGaps: 0,
			source: `package sample
type Action func(int) error
func Do(a Action) error { return nil }`,
		},
		{
			name:     "type parameter is not a type",
			wantGaps: 0,
			source: `package sample
func Marshal[Document any](d Document) ([]byte, error) { return nil, nil }`,
		},
		{
			name:     "predeclared types are always held",
			wantGaps: 0,
			source: `package sample
func Do(count uint64, name string, ok bool) error { return nil }`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			packageDir := filepath.Join(root, "sample")
			if err := os.MkdirAll(packageDir, 0o700); err != nil {
				t.Fatalf("os.MkdirAll(%q) error = %v, want nil", packageDir, err)
			}
			path := filepath.Join(packageDir, "sample.go")
			if err := os.WriteFile(path, []byte(testCase.source), 0o600); err != nil {
				t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
			}
			gaps, err := auditProducerCompleteness(root)
			if err != nil {
				t.Fatalf("auditProducerCompleteness() error = %v, want nil", err)
			}
			if len(gaps) != testCase.wantGaps {
				t.Fatalf("auditProducerCompleteness() gaps = %d %v, want %d", len(gaps), gapNames(gaps), testCase.wantGaps)
			}
		})
	}
}

func gapNames(gaps []producerGap) []string {
	names := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		names = append(names, gap.name)
	}
	return names
}
