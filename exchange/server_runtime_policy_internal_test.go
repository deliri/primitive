package exchange

import (
	"errors"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Each duration field has the same closed scalar domain; the outer table
// exercises every compiler-visible projection, not four invented policies.
func TestServerRuntimePolicyProjectionBoundaryTable(t *testing.T) {
	t.Parallel()
	fields := []struct {
		name  string
		set   func(*ServerRuntimePolicy, temporal.Duration)
		index int
	}{
		{name: "header read", index: 0, set: func(p *ServerRuntimePolicy, d temporal.Duration) { p.ReadHeaderTimeout = d }},
		{name: "whole request read", index: 1, set: func(p *ServerRuntimePolicy, d temporal.Duration) { p.ReadTimeout = d }},
		{name: "response write", index: 2, set: func(p *ServerRuntimePolicy, d temporal.Duration) { p.WriteTimeout = d }},
		{name: "keepalive idle", index: 3, set: func(p *ServerRuntimePolicy, d temporal.Duration) { p.IdleTimeout = d }},
	}
	cases := []struct {
		name        string
		nanoseconds int64
		wantTimeout time.Duration
		wantErr     error
	}{
		{name: "zero must not select Go unbounded timeout", wantErr: core.ErrExchangeContract},
		{name: "smallest positive timeout cannot round to unbounded", nanoseconds: 1, wantTimeout: 1},
		{name: "one above minimum cannot round to its neighbor", nanoseconds: 2, wantTimeout: 2},
		{name: "below maximum retains its final nanosecond", nanoseconds: math.MaxInt64 - 1, wantTimeout: math.MaxInt64 - 1},
		{name: "maximum cannot overflow during Go projection", nanoseconds: math.MaxInt64, wantTimeout: math.MaxInt64},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					policy := runtimeAgreementPolicy(t)
					// Distinct neighboring fields make a swapped Go field visible.
					policy.ReadHeaderTimeout, _ = temporal.DurationFromNanoseconds(3)
					policy.ReadTimeout, _ = temporal.DurationFromNanoseconds(5)
					policy.WriteTimeout, _ = temporal.DurationFromNanoseconds(7)
					policy.IdleTimeout, _ = temporal.DurationFromNanoseconds(11)
					want := [4]time.Duration{3, 5, 7, 11}
					duration, err := temporal.DurationFromNanoseconds(tc.nanoseconds)
					if err != nil {
						t.Fatal(err)
					}
					field.set(&policy, duration)
					want[field.index] = tc.wantTimeout
					if err := policy.Validate(); !errors.Is(err, tc.wantErr) {
						t.Fatalf("policy validation = %v, want %v", err, tc.wantErr)
					}
					address, err := ParseListenAddress("127.0.0.1:0")
					if err != nil {
						t.Fatal(err)
					}
					handler := http.NewServeMux()
					got, err := NewServerRuntime(ServerRuntimeConfiguration{Address: address, Policy: policy}, handler)
					if !errors.Is(err, tc.wantErr) {
						t.Fatalf("constructor error = %v, want %v", err, tc.wantErr)
					}
					if tc.wantErr != nil {
						if got != nil {
							t.Fatalf("refused timeout capability=%v, want nil", got)
						}
						return
					}
					t.Cleanup(func() {
						if err := got.Close(); err != nil {
							t.Errorf("close = %v", err)
						}
					})
					observed := [4]time.Duration{got.server.ReadHeaderTimeout, got.server.ReadTimeout, got.server.WriteTimeout, got.server.IdleTimeout}
					if observed != want || got.server.Handler != handler {
						t.Fatalf("Go projection = %v, want %v and exact handler", observed, want)
					}
					if err := got.Validate(); err != nil {
						t.Fatalf("returned capability validation = %v", err)
					}
				})
			}
		})
	}
}

func TestServerRuntimeHeaderExtentBoundaryTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		inputs     []uint64
		wantHeader int
		wantErr    error
	}{
		{name: "absent extent cannot select Go default", inputs: []uint64{0}, wantErr: core.ErrExchangeContract},
		{name: "smallest positive extent remains exact", inputs: []uint64{1}, wantHeader: 1},
		{name: "one above minimum cannot round down", inputs: []uint64{2}, wantHeader: 2},
		{name: "one below portable ceiling remains exact", inputs: []uint64{core.HTTPServerHeaderMaximumBytes - 1}, wantHeader: core.HTTPServerHeaderMaximumBytes - 1},
		{name: "portable ceiling remains exact", inputs: []uint64{core.HTTPServerHeaderMaximumBytes}, wantHeader: core.HTTPServerHeaderMaximumBytes},
		{name: "overflow class refuses before Go conversion or read allowance addition", inputs: []uint64{core.HTTPServerHeaderMaximumBytes + 1, math.MaxInt64, math.MaxUint64}, wantErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, maximum := range tc.inputs {
				policy := runtimeAgreementPolicy(t)
				policy.MaximumHeaderBytes, _ = core.NewByteCount(maximum)
				if err := policy.Validate(); !errors.Is(err, tc.wantErr) {
					t.Fatalf("owning policy validation = %v, want %v", err, tc.wantErr)
				}
				address, err := ParseListenAddress("127.0.0.1:0")
				if err != nil {
					t.Fatalf("address fixture = %v, want nil", err)
				}
				configuration := ServerRuntimeConfiguration{Address: address, Policy: policy}
				if err := configuration.Validate(); !errors.Is(err, tc.wantErr) {
					t.Fatalf("configuration validation for %d = %v, want %v", maximum, err, tc.wantErr)
				}
				got, err := NewServerRuntime(configuration, http.NewServeMux())
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("public constructor for %d = (%v,%v), want %v", maximum, got, err, tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != nil {
						t.Fatalf("refused capability = %v, want nil", got)
					}
					continue
				}
				t.Cleanup(func() {
					if err := got.Close(); err != nil {
						t.Errorf("dormant close = %v, want nil", err)
					}
				})
				if err := got.Validate(); err != nil {
					t.Fatalf("admitted capability validation = %v, want nil", err)
				}
				if got.server.MaxHeaderBytes != tc.wantHeader {
					t.Fatalf("Go header limit = %d, want %d", got.server.MaxHeaderBytes, tc.wantHeader)
				}
				if acquired, err := got.Address(); acquired != (ListenAddress{}) || !errors.Is(err, core.ErrExchangeContract) {
					t.Fatalf("dormant acquisition = (%v,%v), want zero and contract refusal", acquired, err)
				}
				select {
				case event := <-got.Ready():
					t.Fatalf("constructor readiness = %v, want no acquisition event", event)
				default:
				}
			}
		})
	}
}
