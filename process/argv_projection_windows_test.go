package process_test

import "syscall"

// Use Go's actual WTF-16 conversion, including unpaired surrogates, instead
// of inventing a second Unicode or command-line interpretation in the oracle.
func nativeArgumentProjection(value string) (string, error) {
	encoded, err := syscall.UTF16FromString(value)
	if err != nil {
		return "", err
	}
	return syscall.UTF16ToString(encoded), nil
}
