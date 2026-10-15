package testserial

import (
	"bufio"
	"context"
	"errors"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	primitivecore "github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

const forbiddenSuppressionDirective = "witness:waiver"

func suppressionPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?m)(?://|/\*|^[ \t]*\*)[ \t]*(?:` + regexp.QuoteMeta(forbiddenSuppressionDirective) + `)`)
}

func TestPrimitiveSourcesHaveNoWitnessSuppressions(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	absolute, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	path, err := primitivecore.ParseAbsolutePath(absolute)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filestore.OpenRoot(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Errorf("source root Close() error = %v, want nil", err)
		}
	}()
	start, err := primitivecore.ParseRelativePath(".")
	if err != nil {
		t.Fatal(err)
	}
	pattern := suppressionPattern()
	err = filestore.Walk(ctx, filestore.WalkRequest{
		Location: filestore.Location{Root: root, Path: start},
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if entry.Entry.IsDir() {
				if name := entry.Entry.Name(); name == ".git" || name == "vendor" || name == ".hammer" || name == "node_modules" {
					return filestore.WalkSkipDirectory, nil
				}
				return filestore.WalkContinue, nil
			}
			switch filepath.Ext(entry.Path.String()) {
			case ".go", ".js", ".mjs":
			default:
				return filestore.WalkContinue, nil
			}
			file, err := filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: filestore.Location{Root: root, Path: entry.Path}})
			if err != nil {
				return filestore.WalkContinue, err
			}
			line, scanErr := firstSuppressionLine(ctx, file, pattern)
			if err := errors.Join(scanErr, file.Close()); err != nil {
				return filestore.WalkContinue, err
			}
			if line != 0 {
				t.Errorf("forbidden suppression text at %s:%d; fix its owning code", entry.Path, line)
			}
			return filestore.WalkContinue, nil
		},
	})
	if err != nil {
		t.Fatalf("source scan error = %v, want nil", err)
	}
}

// Grep-like forward scan: Primitive owns a fixed framing window and drains each
// record. Go regexp reads runes from a second fixed window. Neither a complete
// line nor a file nor a list of matches is ever retained. A source failure
// withholds the provisional match instead of pretending the scan completed.
func firstSuppressionLine(ctx context.Context, source io.Reader, pattern *regexp.Regexp) (uint64, error) {
	window, err := primitivecore.NewByteCount(4096)
	if err != nil {
		return 0, err
	}
	reader, err := lineio.New(lineio.Request{Source: source, BufferBytes: window})
	if err != nil {
		return 0, err
	}
	var first, line uint64
	matched := false
	runes := bufio.NewReaderSize(source, 4096)
	request := lineio.RecordReadRequest{Use: func(_ context.Context, record io.Reader) error {
		if first == 0 {
			runes.Reset(record)
			matched = pattern.MatchReader(runes)
		}
		return nil
	}}
	for {
		matched = false
		_, err := reader.WithRecord(ctx, request)
		if err == io.EOF {
			return first, nil
		}
		if err != nil {
			return 0, err
		}
		if line == math.MaxUint64 {
			return 0, primitivecore.ErrNumericOverflow
		}
		line++
		if first == 0 && matched {
			first = line
		}
	}
}

func TestSuppressionScanStreamsSourceText(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		source string
		want   uint64
	}{
		{name: "empty source"},
		{name: "ordinary ownership comment", source: "// Resource lifetime remains owned.\n"},
		{name: "directive name without comment prefix", source: forbiddenSuppressionDirective},
		{name: "leading directive", source: "// " + forbiddenSuppressionDirective + " doctrine/error\n", want: 1},
		{name: "inline directive", source: "var value int //" + forbiddenSuppressionDirective + " doctrine/error\n", want: 1},
		{name: "block directive", source: "/*\t" + forbiddenSuppressionDirective + " doctrine/error */\n", want: 1},
		{name: "block continuation", source: "/*\n * " + forbiddenSuppressionDirective + " doctrine/error\n */", want: 2},
		{name: "quoted suppression remains forbidden source text", source: `const fixture = "// ` + forbiddenSuppressionDirective + ` doctrine/error"`, want: 1},
		{name: "directive crosses native window", source: strings.Repeat(" ", 4094) + "// " + forbiddenSuppressionDirective, want: 1},
		{name: "long line never becomes an extent quota", source: strings.Repeat("x", 128<<10) + "\n// " + forbiddenSuppressionDirective, want: 2},
		{name: "only first coordinate is retained", source: "// " + forbiddenSuppressionDirective + "\n// " + forbiddenSuppressionDirective + "\n", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := firstSuppressionLine(t.Context(), strings.NewReader(tc.source), suppressionPattern())
			if err != nil || got != tc.want {
				t.Fatalf("first suppression line = %d/%v, want %d/nil", got, err, tc.want)
			}
		})
	}
}
