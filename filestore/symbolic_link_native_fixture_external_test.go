package filestore_test

import (
	"os"
	"path/filepath"
)

// Fixture construction only. Native observations and comparisons stay in each
// test; the outside directory belongs to that same test's temporary container.
func createSymbolicLinkNativeFixture(container string) error {
	for _, name := range []string{"root", "outside", filepath.Join("root", "directory")} {
		if err := os.Mkdir(filepath.Join(container, name), 0o700); err != nil {
			return err
		}
	}
	for _, name := range []string{filepath.Join("root", "file"), filepath.Join("root", "directory", "entry"), filepath.Join("outside", "entry")} {
		if err := os.WriteFile(filepath.Join(container, name), []byte{0, 255, 1, 127}, 0o600); err != nil {
			return err
		}
	}
	for _, link := range []struct{ name, target string }{
		{name: "link", target: "file"}, {name: "chain", target: "link"}, {name: "parent", target: "directory"},
		{name: "outside", target: filepath.Join(container, "outside")},
		{name: "absolute", target: filepath.Join(container, "outside", "entry")},
		{name: "dangling", target: "missing"}, {name: "self", target: "self"}, {name: "first", target: "second"}, {name: "second", target: "first"},
		{name: "opaque", target: "socket:[123]"}, {name: "lexical", target: "./directory/../file"}, {name: "directory-link", target: "directory"},
		{name: "directory/child-link", target: "entry"}, {name: "outside/child-link", target: "entry"},
	} {
		if err := os.Symlink(link.target, filepath.Join(container, "root", link.name)); err != nil {
			return err
		}
	}
	return nil
}
