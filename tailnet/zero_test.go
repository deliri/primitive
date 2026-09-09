package tailnet

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestZeroCapabilityCannotBlockOrPerformEffects(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		run  func(*testing.T) error
	}{
		{"nil close refuses without panic", func(*testing.T) error { return (*Client)(nil).Close() }},
		{"zero close refuses without waiting on nil gate", func(*testing.T) error { return new(Client).Close() }},
		{"zero dial refuses before gate or provider", func(t *testing.T) error {
			connection, err := new(Client).dial(t.Context(), "tcp", "100.64.0.1:1")
			if connection != nil {
				t.Errorf("zero client connection = %v, want nil", connection)
			}
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.run(t); !errors.Is(got, core.ErrTailnetContract) {
				t.Fatalf("zero operation = %v, want Tailnet contract", got)
			}
		})
	}
}
