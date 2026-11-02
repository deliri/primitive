package webmcp_test

import (
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/webmcp"
)

func TestBrowserModulePublicationHasNoExternalGoInput(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeOf(webmcp.BrowserModule)
	if typ.NumIn() != 0 || typ.NumOut() != 1 || typ.Out(0).Kind() != reflect.String {
		t.Fatalf("module inputs/outputs = %d/%d, want zero inputs and one immutable string", typ.NumIn(), typ.NumOut())
	}
}
