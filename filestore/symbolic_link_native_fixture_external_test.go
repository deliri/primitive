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
		{"link", "file"}, {"chain", "link"}, {"parent", "directory"},
		{"outside", filepath.Join(container, "outside")},
		{"absolute", filepath.Join(container, "outside", "entry")},
		{"dangling", "missing"}, {"self", "self"}, {"first", "second"}, {"second", "first"},
		{"opaque", "socket:[123]"}, {"lexical", "./directory/../file"}, {"directory-link", "directory"},
		{"directory/child-link", "entry"}, {"outside/child-link", "entry"},
	} {
		if err := os.Symlink(link.target, filepath.Join(container, "root", link.name)); err != nil {
			return err
		}
	}
	return nil
}
