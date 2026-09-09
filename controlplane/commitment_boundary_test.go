package controlplane_test

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

func commitmentFixture(t testing.TB) controlplane.ResponseCommitment {
	t.Helper()
	fixture := issueTestRegistration(t)
	encoded, err := fixture.document.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON(fixture) error = %v, want nil", err)
	}
	length, err := core.NewByteLength(uint64(len(encoded)))
	if err != nil {
		t.Fatalf("NewByteLength(fixture) error = %v, want nil", err)
	}
	return controlplane.ResponseCommitment{Header: fixture.document.Payload.Header, BodyLength: length, BodySHA256: core.SHA256Of(encoded)}
}

func TestResponseCommitmentSchemaLayerTriadClosesBodyExtent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		length      uint64
		bodyless    bool
		emptyDigest bool
		wantErr     error
	}{
		{name: "one byte is the smallest present body", length: 1},
		{name: "one below response ceiling is representable", length: core.JSONDocumentMaximumBytes - 1},
		{name: "response ceiling remains an admitted extent", length: core.JSONDocumentMaximumBytes},
		{name: "one above response ceiling cannot be committed", length: core.JSONDocumentMaximumBytes + 1, wantErr: core.ErrControlPlaneResponseDocument},
		{name: "extreme extent cannot bypass body ceiling", length: math.MaxInt64, wantErr: core.ErrControlPlaneResponseDocument},
		{name: "absent ordinary body is not a response", wantErr: core.ErrControlPlaneResponseDocument},
		{name: "bodyless refusal commits exactly the empty digest", bodyless: true, emptyDigest: true},
		{name: "bodyless refusal cannot commit a different digest", bodyless: true, wantErr: core.ErrControlPlaneResponseDocument},
		{name: "refusal cannot conceal one body byte", length: 1, bodyless: true, emptyDigest: true, wantErr: core.ErrControlPlaneResponseDocument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			candidate := commitmentFixture(t)
			length, err := core.NewByteLength(tc.length)
			if err != nil {
				t.Fatalf("NewByteLength(%d) error = %v, want nil", tc.length, err)
			}
			candidate.BodyLength = length
			if tc.bodyless {
				candidate.Header.Status = controlplane.ProductStatusUpgradeRequired
			}
			if tc.emptyDigest {
				candidate.BodySHA256 = core.SHA256Of(nil)
			}
			gotErr := candidate.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Errorf("Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
			encoded, err := candidate.MarshalJSON()
			if tc.wantErr != nil {
				if len(encoded) != 0 || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, tc.wantErr) {
					t.Fatalf("MarshalJSON() = (%d bytes, %v), want zero and typed refusal", len(encoded), err)
				}
				return
			}
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v, want nil", err)
			}
			var got controlplane.ResponseCommitment
			if err := got.UnmarshalJSON(encoded); err != nil || got != candidate {
				t.Fatalf("commitment round trip = (%v, %v), want %v", got, err, candidate)
			}
		})
	}
}

func FuzzResponseCommitmentExternalDecoder(f *testing.F) {
	seed := commitmentFixture(f)
	for _, length := range []uint64{1, seed.BodyLength.Uint64(), core.JSONDocumentMaximumBytes} {
		candidate := seed
		value, err := core.NewByteLength(length)
		if err != nil {
			f.Fatalf("NewByteLength(seed) error = %v, want nil", err)
		}
		candidate.BodyLength = value
		encoded, err := candidate.MarshalJSON()
		if err != nil {
			f.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(encoded)
	}
	absent := seed
	absent.Header.Status = controlplane.ProductStatusUpgradeRequired
	absent.BodyLength = core.ByteLength{}
	absent.BodySHA256 = core.SHA256Of(nil)
	encoded, err := absent.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON(absent seed) error = %v, want nil", err)
	}
	f.Add(encoded)
	for _, data := range [][]byte{nil, []byte("null"), []byte("{}"), []byte("[]"), bytes.Repeat([]byte{' '}, controlplane.ResponseCommitmentJSONMaximumBytes+1)} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		if err := got.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrControlPlaneContract) || got != seed {
				t.Fatalf("UnmarshalJSON(rejected) = (%v, %v), want unchanged and typed refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted Validate() error = %v, want nil", err)
		}
		if got.BodyLength.Uint64() > core.JSONDocumentMaximumBytes {
			t.Fatalf("accepted body extent = %d, want at most %d", got.BodyLength.Uint64(), core.JSONDocumentMaximumBytes)
		}
		if got.Header.Status == controlplane.ProductStatusUpgradeRequired && (got.BodyLength.Uint64() != 0 || got.BodySHA256 != core.SHA256Of(nil)) {
			t.Fatalf("bodyless commitment = %v, want exact empty body facts", got)
		}
		canonical, err := got.MarshalJSON()
		if err != nil || len(canonical) > controlplane.ResponseCommitmentJSONMaximumBytes {
			t.Fatalf("MarshalJSON() = (%d bytes, %v), want bounded and nil", len(canonical), err)
		}
		var again controlplane.ResponseCommitment
		if err := again.UnmarshalJSON(canonical); err != nil || again != got {
			t.Fatalf("canonical round trip = (%v, %v), want %v", again, err, got)
		}
		second, err := again.MarshalJSON()
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("second canonical = (%d bytes, %v), want exact %d bytes", len(second), err, len(canonical))
		}
	})
}
