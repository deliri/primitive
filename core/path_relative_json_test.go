package core_test

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

type relativePathJSONFile struct {
	Path core.RelativePath `json:"path"`
}

func (f relativePathJSONFile) Validate() error { return f.Path.Validate() }

func TestRelativePathJSONPreservesNativeNames(t *testing.T) {
	t.Parallel()
	for _, name := range [...]string{".", "a/file", " leading space", "trailing space ", "line\nbreak", "雪/file", strings.Repeat("a", 1<<20+1)} {
		path, err := core.ParseRelativePath(name)
		if err != nil {
			t.Fatal(err)
		}
		var source bytes.Buffer
		writer, err := jsonio.NewObjectEncoder[relativePathJSONFile](jsonio.ObjectDestinationRequest{Destination: &source})
		if err != nil {
			t.Fatal(err)
		}
		if err := writer.Encode(t.Context(), relativePathJSONFile{Path: path}); err != nil {
			t.Fatal(err)
		}
		reader, err := jsonio.NewObjectDecoder[relativePathJSONFile](jsonio.ObjectSourceRequest{Source: &source})
		if err != nil {
			t.Fatal(err)
		}
		got, err := reader.Decode(t.Context())
		if err != nil || got.Path != path {
			t.Fatalf("native name bytes=%d roundtrip=%v err=%v", len(name), got.Path == path, err)
		}
	}
}

func TestRelativePathJSONRefusalPreservesReceiver(t *testing.T) {
	t.Parallel()
	for _, data := range [...]string{`""`, `"/absolute"`, `"../escape"`, `"a/../b"`, `"a\u0000b"`, `null`, `{}`, `3`, `"a" "b"`} {
		path, err := core.ParseRelativePath("retained")
		if err != nil {
			t.Fatal(err)
		}
		want := path
		err = path.UnmarshalJSON([]byte(data))
		if !errors.Is(err, core.ErrJSONContract) || path != want {
			t.Fatalf("refusal data=%q retained=%v err=%v", data, path == want, err)
		}
	}
	var empty core.RelativePath
	if _, err := empty.MarshalJSON(); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("empty path marshal=%v", err)
	}
	var nilPath *core.RelativePath
	if err := nilPath.UnmarshalJSON([]byte(`"a"`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil receiver=%v", err)
	}
}

func FuzzRelativePathJSONMatchesNativeAdmission(f *testing.F) {
	f.Add("a/file")
	f.Add("line\nbreak")
	f.Add("../refused")
	f.Fuzz(func(t *testing.T, name string) {
		want, err := core.ParseRelativePath(name)
		encoded, marshalErr := json.Marshal(name)
		if marshalErr != nil {
			if err == nil {
				t.Fatal(marshalErr)
			}
			return
		}
		var got core.RelativePath
		decodeErr := got.UnmarshalJSON(encoded)
		if err != nil {
			if decodeErr == nil {
				t.Fatalf("invalid native path admitted bytes=%d", len(name))
			}
			return
		}
		if decodeErr != nil || got != want {
			t.Fatalf("native admission bytes=%d identity=%v err=%v", len(name), got == want, decodeErr)
		}
	})
}
