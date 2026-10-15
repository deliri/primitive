package testserial

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const forbiddenSuppressionDirective = "witness:waiver"

func TestPrimitiveSourcesHaveNoWitnessSuppressions(t *testing.T) {
	t.Parallel()
	root := ".."
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor", ".hammer", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, comment := range suppressionComments(file) {
			t.Errorf("forbidden suppression at %s; fix its owning code", positions.Position(comment.Pos()))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir(%q) error = %v, want nil", root, err)
	}
}

func suppressionComments(file *ast.File) []*ast.Comment {
	var found []*ast.Comment
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.Contains(comment.Text, forbiddenSuppressionDirective) {
				found = append(found, comment)
			}
		}
	}
	return found
}

func TestSuppressionCommentsKeepNegativeFixturesAsData(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		source string
		want   int
	}{
		{name: "ordinary ownership comment", source: "package sample\n// Close joins the native failure.\n"},
		{name: "negative fixture string", source: "package sample\nconst fixture = \"// " + forbiddenSuppressionDirective + " doctrine/error\"\n"},
		{name: "line suppression", source: "package sample\n// " + forbiddenSuppressionDirective + " doctrine/error\n", want: 1},
		{name: "inline suppression", source: "package sample\nvar v int // " + forbiddenSuppressionDirective + " doctrine/error\n", want: 1},
		{name: "block suppression", source: "package sample\n/* " + forbiddenSuppressionDirective + " doctrine/error */\n", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "sample.go", tc.source, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			if got := suppressionComments(file); len(got) != tc.want {
				t.Fatalf("suppression comments = %v, want %d", got, tc.want)
			}
		})
	}
}
