package core

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
)

func TestValidatedJSONUsesRealPlatformContract(t *testing.T) {
	t.Parallel()

	platform, err := NewPlatform(OperatingSystemLinux, CPUArchitectureARM64)
	if err != nil {
		t.Fatalf("NewPlatform() error = %v, want nil", err)
	}
	limits := DefaultStrictJSONLimits()
	gotWire, gotEncodeErr := EncodeValidatedJSON(platform, limits)
	if gotEncodeErr != nil {
		t.Fatalf("EncodeValidatedJSON(Platform) error = %v, want nil", gotEncodeErr)
	}
	gotDecoded, gotDecodeErr := DecodeStrictJSON[Platform](gotWire, limits)
	if gotDecodeErr != nil || gotDecoded != platform {
		t.Fatalf("DecodeStrictJSON(Platform) = (%v, %v), want (%v, nil)", gotDecoded, gotDecodeErr, platform)
	}

	_, gotZeroEncodeErr := EncodeValidatedJSON(Platform{}, limits)
	if !errors.Is(gotZeroEncodeErr, ErrJSONContract) ||
		!errors.Is(gotZeroEncodeErr, ErrPrimitiveContract) {
		t.Fatalf("EncodeValidatedJSON(zero Platform) error = %v, want %v and %v", gotZeroEncodeErr, ErrJSONContract, ErrPrimitiveContract)
	}
	_, gotNullDecodeErr := DecodeStrictJSON[Platform]([]byte("null"), limits)
	if !errors.Is(gotNullDecodeErr, ErrJSONContract) ||
		!errors.Is(gotNullDecodeErr, ErrPrimitiveContract) {
		t.Fatalf("DecodeStrictJSON(Platform null) error = %v, want %v and %v", gotNullDecodeErr, ErrJSONContract, ErrPrimitiveContract)
	}
}

func TestJSONUnmarshalNilReceiversReturnTypedIdentity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		run     func() error
		name    string
	}{
		{name: "nil ByteCount receiver", run: func() error { return (*ByteCount)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil ByteLength receiver", run: func() error { return (*ByteLength)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil SHA256Digest receiver", run: func() error { return (*SHA256Digest)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil CRC32C receiver", run: func() error { return (*CRC32C)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil Ed25519PublicKey receiver", run: func() error { return (*Ed25519PublicKey)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil PathComponent receiver", run: func() error { return (*PathComponent)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil AbsolutePath receiver", run: func() error { return (*AbsolutePath)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil HTTPMethod receiver", run: func() error { return (*HTTPMethod)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil HTTPStatusCode receiver", run: func() error { return (*HTTPStatusCode)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil HTTPHeaderName receiver", run: func() error { return (*HTTPHeaderName)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil HTTPMediaType receiver", run: func() error { return (*HTTPMediaType)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil Platform receiver", run: func() error { return (*Platform)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil OperatingSystem receiver", run: func() error { return (*OperatingSystem)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil CPUArchitecture receiver", run: func() error { return (*CPUArchitecture)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil PackageIdentity receiver", run: func() error { return (*PackageIdentity)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil PackageKind receiver", run: func() error { return (*PackageKind)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
		{name: "nil ErrorIdentity receiver", run: func() error { return (*ErrorIdentity)(nil).UnmarshalJSON(nil) }, wantErr: ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.run()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("nil receiver UnmarshalJSON() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestNumericConversionAndSaturationBoundaries(t *testing.T) {
	t.Parallel()

	uint32Cases := []struct {
		wantErr error
		name    string
		value   int
		want    uint32
	}{
		{name: "negative one is rejected", value: -1, wantErr: ErrNumericOverflow},
		{name: "minimum integer is rejected", value: math.MinInt, wantErr: ErrNumericOverflow},
		{name: "zero is accepted", value: 0},
		{name: "one is accepted", value: 1, want: 1},
		{name: "uint32 maximum minus one is accepted", value: math.MaxUint32 - 1, want: math.MaxUint32 - 1},
		{name: "uint32 maximum is accepted", value: math.MaxUint32, want: math.MaxUint32},
		{name: "uint32 maximum plus one is rejected", value: math.MaxUint32 + 1, wantErr: ErrNumericOverflow},
		{name: "maximum integer is rejected", value: math.MaxInt, wantErr: ErrNumericOverflow},
	}
	for _, tc := range uint32Cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := CheckedUint32FromInt(tc.value)
			if !errors.Is(gotErr, tc.wantErr) || gotErr == nil && got != tc.want {
				t.Fatalf("CheckedUint32FromInt(%d) = (%d, %v), want (%d, %v)", tc.value, got, gotErr, tc.want, tc.wantErr)
			}
		})
	}

}

func TestRejectedJSONPreservesTypedReceivers(t *testing.T) {
	t.Parallel()

	status, err := NewHTTPStatusCode(200)
	if err != nil {
		t.Fatalf("NewHTTPStatusCode(200) error = %v, want nil", err)
	}
	beforeStatus := status
	if gotErr := json.Unmarshal([]byte("99"), &status); !errors.Is(gotErr, ErrJSONContract) {
		t.Fatalf("json.Unmarshal(HTTPStatusCode 99) error = %v, want %v", gotErr, ErrJSONContract)
	}
	if status != beforeStatus {
		t.Fatalf("rejected HTTP status JSON mutated receiver: got %v, want %v", status, beforeStatus)
	}

	beforePlatform, err := NewPlatform(OperatingSystemLinux, CPUArchitectureAMD64)
	if err != nil {
		t.Fatalf("NewPlatform() error = %v, want nil", err)
	}
	gotPlatform := beforePlatform
	if gotErr := json.Unmarshal([]byte(`"linux-386"`), &gotPlatform); !errors.Is(gotErr, ErrJSONContract) {
		t.Fatalf("json.Unmarshal(Platform future architecture) error = %v, want %v", gotErr, ErrJSONContract)
	}
	if gotPlatform != beforePlatform {
		t.Fatalf("rejected Platform JSON mutated receiver: got %v, want %v", gotPlatform, beforePlatform)
	}
}

func TestInvalidZeroValueAccessorsReturnTypedErrors(t *testing.T) {
	t.Parallel()

	if got, gotErr := (ByteCount{}).Uint64(); got != 0 || !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("ByteCount{}.Uint64() = (%d, %v), want (0, %v)", got, gotErr, ErrPrimitiveContract)
	}
	if got, gotErr := (ByteCount{}).Int64(); got != 0 || !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("ByteCount{}.Int64() = (%d, %v), want (0, %v)", got, gotErr, ErrPrimitiveContract)
	}
	if got, gotErr := (SecretMaterial{}).CopyBytes(); got != nil || !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("SecretMaterial{}.CopyBytes() = (%v, %v), want (nil, %v)", got, gotErr, ErrPrimitiveContract)
	}
	if gotErr := (SecretMaterial{}).Destroy(); !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("SecretMaterial{}.Destroy() error = %v, want %v", gotErr, ErrPrimitiveContract)
	}
}
