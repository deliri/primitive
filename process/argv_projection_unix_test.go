//go:build !windows

package process_test

// Unix exec argv is a NUL-delimited byte projection.
func nativeArgumentProjection(value string) (string, error) { return value, nil }
