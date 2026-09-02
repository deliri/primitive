package keygen_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func TestEntropyReaderCapabilityBoundary(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		wantErr   error
		name      string
		extent    int
		wantCount int
		issued    bool
	}{
		{name: "issued empty read is a neutral no-op", issued: true},
		{name: "issued one-byte read is filled", extent: 1, issued: true, wantCount: 1},
		{name: "issued exact maximum read is filled", extent: core.SecretMaterialMaximumBytes, issued: true, wantCount: core.SecretMaterialMaximumBytes},
		{name: "issued one-above-maximum read is refused before mutation", extent: core.SecretMaterialMaximumBytes + 1, issued: true, wantErr: core.ErrKeygenContract},
		{name: "zero capability refuses a read", extent: 1, wantErr: core.ErrKeygenContract},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			reader := keygen.EntropyReader{}
			if testCase.issued {
				reader = keygen.NewEntropyReader()
			}
			destination := bytes.Repeat([]byte{0xa5}, testCase.extent)
			before := bytes.Clone(destination)
			gotCount, gotErr := reader.Read(destination)
			if !errors.Is(gotErr, testCase.wantErr) || gotCount != testCase.wantCount {
				t.Fatalf("EntropyReader.Read(%d bytes) = (%d, %v), want (%d, %v)", testCase.extent, gotCount, gotErr, testCase.wantCount, testCase.wantErr)
			}
			if testCase.wantErr != nil && !bytes.Equal(destination, before) {
				t.Fatalf("EntropyReader.Read(rejected) destination = %x, want preserved %x", destination, before)
			}
		})
	}
}
