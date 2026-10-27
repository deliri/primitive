package firestoredocuments

import (
	"fmt"
	"os"
	"testing"
)

// Anvil owns this local service endpoint. It stays immutable while tests run.
var nativeEmulatorEndpoint = os.Getenv("FIRESTORE_EMULATOR_HOST")

func TestMain(m *testing.M) {
	// The pinned official constructor eagerly dials the emulator environment
	// before applying WithGRPCConn. Hide it while our counted, explicitly owned
	// native connections are supplied, so no hidden second connection is leaked.
	previous, present := os.LookupEnv("FIRESTORE_EMULATOR_HOST")
	if err := os.Unsetenv("FIRESTORE_EMULATOR_HOST"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	if present {
		if err := os.Setenv("FIRESTORE_EMULATOR_HOST", previous); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	os.Exit(code)
}
