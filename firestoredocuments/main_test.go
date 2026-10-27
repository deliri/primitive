package firestoredocuments

import (
	"fmt"
	"os"
	"testing"
)

// Anvil owns this local service endpoint. It stays immutable while tests run.
var nativeEmulatorEndpoint = os.Getenv("PRIMITIVE_FIRESTORE_EMULATOR_HOST")

func TestMain(m *testing.M) {
	// The pinned official constructor eagerly dials the emulator environment
	// before applying WithGRPCConn. Hide it while our counted, explicitly owned
	// native connections are supplied, so no hidden second connection is leaked.
	previous, present := os.LookupEnv("FIRESTORE_EMULATOR_HOST")
	// Fuzz workers are new processes. Carry the same owned service endpoint in
	// the fixture's private environment while leaving the SDK's discovery unset.
	privatePrevious, privatePresent := os.LookupEnv("PRIMITIVE_FIRESTORE_EMULATOR_HOST")
	if previous != "" {
		if privatePresent && privatePrevious != previous {
			fmt.Fprintln(os.Stderr, "native emulator fixture endpoints disagree")
			os.Exit(1)
		}
		nativeEmulatorEndpoint = previous
		if err := os.Setenv("PRIMITIVE_FIRESTORE_EMULATOR_HOST", previous); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
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
	var restoreErr error
	if privatePresent {
		restoreErr = os.Setenv("PRIMITIVE_FIRESTORE_EMULATOR_HOST", privatePrevious)
	} else {
		restoreErr = os.Unsetenv("PRIMITIVE_FIRESTORE_EMULATOR_HOST")
	}
	if restoreErr != nil {
		fmt.Fprintln(os.Stderr, restoreErr)
		code = 1
	}
	os.Exit(code)
}
