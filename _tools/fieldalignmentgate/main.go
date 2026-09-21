// fieldalignmentgate checks layout suggestions without changing the canonical
// bytes of published JSON agreements or positional wire test fixtures.
// Every admitted declaration is pinned
// exactly; new suggestions and changes to a pinned declaration fail closed.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const publishedLayoutMaximum = 31

var (
	errWireLayoutChanged = errors.New("pinned wire declaration changed")
	errUnpinnedLayout    = errors.New("unadmitted layout finding")
	errAnalyzerFailure   = errors.New("fieldalignment analysis failed")
)

type wireLayout struct {
	File        string `json:"file"`
	Type        string `json:"type"`
	Declaration string `json:"declaration"`
	Reason      string `json:"reason"`
}

type diagnostic struct {
	Position string `json:"posn"`
	Message  string `json:"message"`
}

type packageResult struct {
	Findings []diagnostic `json:"fieldalignment"`
	Error    string       `json:"error"`
}

func main() {
	root := flag.String("root", ".", "repository root")
	manifest := flag.String("layouts", "scripts/fieldalignment_wire_layouts.json", "exact wire declarations")
	flag.Parse()
	if err := run(*root, *manifest, os.Stdout, os.Stderr); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}
}

func run(root, manifest string, output, diagnostics io.Writer) (err error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	sourceRoot, err := os.OpenRoot(absRoot)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, sourceRoot.Close()) }()
	pins, err := readLayouts(sourceRoot, manifest)
	if err != nil {
		return err
	}
	positions, err := verifyLayouts(sourceRoot, pins)
	if err != nil {
		return err
	}
	command := exec.Command("fieldalignment", "-json", "./...")
	command.Dir, command.Stderr = absRoot, diagnostics
	raw, err := command.Output()
	// Preserve the native tool output, including every admitted diagnostic.
	if _, writeErr := output.Write(raw); writeErr != nil {
		return errors.Join(err, writeErr)
	}
	if err != nil {
		return err
	}
	return checkFindings(raw, positions)
}

func readLayouts(root *os.Root, path string) (pins []wireLayout, err error) {
	file, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&pins); err != nil {
		return nil, err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.Join(errors.New("layout manifest has trailing input"), err)
	}
	if len(pins) == 0 || len(pins) > publishedLayoutMaximum {
		return nil, fmt.Errorf("published layout count %d is outside 1..%d", len(pins), publishedLayoutMaximum)
	}
	return pins, nil
}

func verifyLayouts(root *os.Root, pins []wireLayout) (map[string]wireLayout, error) {
	positions := make(map[string]wireLayout, len(pins))
	for _, pin := range pins {
		if !filepath.IsLocal(pin.File) || pin.Type == "" || pin.Reason == "" {
			return nil, fmt.Errorf("invalid layout pin for %q", pin.File)
		}
		position, err := verifyLayout(root, pin)
		if err != nil {
			return nil, err
		}
		if _, duplicate := positions[position]; duplicate {
			return nil, fmt.Errorf("duplicate published layout at %s", position)
		}
		positions[position] = pin
	}
	return positions, nil
}

func verifyLayout(root *os.Root, pin wireLayout) (string, error) {
	data, err := root.ReadFile(pin.File)
	if err != nil {
		return "", err
	}
	path := filepath.Join(root.Name(), pin.File)
	set := token.NewFileSet()
	source, err := parser.ParseFile(set, path, data, 0)
	if err != nil {
		return "", err
	}
	declaration := namedStruct(source, pin.Type)
	if declaration == nil {
		return "", fmt.Errorf("published layout %s:%s is missing", pin.File, pin.Type)
	}
	if !hasWireField(declaration) {
		return "", fmt.Errorf("layout %s:%s has no JSON or ASN.1 agreement", pin.File, pin.Type)
	}
	var canonical bytes.Buffer
	if err := format.Node(&canonical, token.NewFileSet(), declaration); err != nil {
		return "", err
	}
	if canonical.String() != pin.Declaration {
		return "", fmt.Errorf("%w: %s:%s", errWireLayoutChanged, pin.File, pin.Type)
	}
	return set.Position(declaration.Pos()).String(), nil
}

func namedStruct(source *ast.File, name string) *ast.StructType {
	for _, declaration := range source.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.TYPE {
			continue
		}
		for _, item := range group.Specs {
			spec, ok := item.(*ast.TypeSpec)
			if ok && spec.Name.Name == name {
				result, _ := spec.Type.(*ast.StructType)
				return result
			}
		}
	}
	return nil
}

func hasWireField(declaration *ast.StructType) bool {
	for _, field := range declaration.Fields.List {
		if field.Tag == nil {
			continue
		}
		tag, err := strconv.Unquote(field.Tag.Value)
		if err == nil && (strings.Contains(tag, `json:"`) || strings.Contains(tag, `asn1:"`)) {
			return true
		}
	}
	return false
}

func checkFindings(raw []byte, positions map[string]wireLayout) error {
	var packages map[string]packageResult
	if err := json.Unmarshal(raw, &packages); err != nil {
		return errors.Join(errAnalyzerFailure, err)
	}
	if packages == nil {
		return errAnalyzerFailure
	}
	var failures []error
	for _, result := range packages {
		if result.Findings == nil && result.Error == "" {
			failures = append(failures, errAnalyzerFailure)
		}
		if result.Error != "" {
			failures = append(failures, fmt.Errorf("%w: %s", errAnalyzerFailure, result.Error))
		}
		for _, finding := range result.Findings {
			if _, pinned := positions[finding.Position]; !pinned {
				failures = append(failures, fmt.Errorf("%w: %s: %s", errUnpinnedLayout, finding.Position, finding.Message))
			}
		}
	}
	return errors.Join(failures...)
}
