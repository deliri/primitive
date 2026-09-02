package compileonly

import (
	"os"
	"testing"
)

// TestMain exits unsuccessfully if the compiled test binary is ever started.
// CompilePackage must link this function without executing it.
func TestMain(*testing.M) {
	os.Exit(93)
}
