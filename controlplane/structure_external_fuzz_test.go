package controlplane_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

type structureExternalDoor[T any] struct {
	Seed         T
	Marshal      func(T) ([]byte, error)
	Unmarshal    func(*T, []byte) error
	Validate     func(T) error
	Authenticate func(T, bool) error
	Mutations    []T
	WantError    core.ErrorIdentity
}

func TestRegistrationRequestDecoderLayerTriad(t *testing.T) {
	t.Parallel()

	var seed controlplane.RegistrationRequest
	canonical := readGolden(t, "registration_request.json")
	if err := seed.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("registration request golden UnmarshalJSON() error = %v, want nil", err)
	}

	t.Run("positive canonical request closes exact typed facts", func(t *testing.T) {
		t.Parallel()

		var got controlplane.RegistrationRequest
		gotErr := got.UnmarshalJSON(canonical)
		if gotErr != nil {
			t.Fatalf("RegistrationRequest.UnmarshalJSON(canonical) error = %v, want nil", gotErr)
		}
		gotValidateErr := got.Validate()
		gotProjection := mustRegistrationRequestProjection(t, got)
		if gotValidateErr != nil || !bytes.Equal(gotProjection, canonical) {
			t.Fatalf("RegistrationRequest canonical closure = (projection match %t, validation %v), want exact projection and nil",
				bytes.Equal(gotProjection, canonical), gotValidateErr)
		}
	})

	t.Run("negative foreign installation preserves receiver and every owner identity", func(t *testing.T) {
		t.Parallel()

		_, otherInstallation := testDeviceKey(t, checkInOtherDeviceSeed)
		mutated := bytes.Replace(
			canonical,
			[]byte(seed.Installation.String()),
			[]byte(otherInstallation.String()),
			1,
		)
		if bytes.Equal(mutated, canonical) {
			t.Fatalf("typed installation mutation changed no wire bytes")
		}
		got := seed
		gotErr := got.UnmarshalJSON(mutated)
		if !errors.Is(gotErr, core.ErrJSONContract) ||
			!errors.Is(gotErr, core.ErrControlPlaneContract) ||
			!errors.Is(gotErr, core.ErrControlPlaneRegistration) ||
			!errors.Is(gotErr, core.ErrControlPlaneInstallationBinding) ||
			!bytes.Equal(mustRegistrationRequestProjection(t, got), canonical) {
			t.Fatalf("RegistrationRequest.UnmarshalJSON(foreign installation) error = %v, want preserved canonical receiver and JSON/control-plane/registration/installation identities", gotErr)
		}
	})

	t.Run("neutral absent request creates no registration facts", func(t *testing.T) {
		t.Parallel()

		got := seed
		gotErr := got.UnmarshalJSON(nil)
		if !errors.Is(gotErr, core.ErrJSONContract) ||
			!errors.Is(gotErr, core.ErrControlPlaneContract) ||
			!errors.Is(gotErr, core.ErrControlPlaneRegistration) ||
			!bytes.Equal(mustRegistrationRequestProjection(t, got), canonical) {
			t.Fatalf("RegistrationRequest.UnmarshalJSON(absent) error = %v, want preserved canonical receiver and typed refusal", gotErr)
		}
	})
}

func mustRegistrationRequestProjection(t testing.TB, value controlplane.RegistrationRequest) []byte {
	t.Helper()
	projection, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("RegistrationRequest.MarshalJSON() error = %v, want nil", err)
	}
	return projection
}

func FuzzRegistrationRequestExternalDecoder(f *testing.F) {
	var seed controlplane.RegistrationRequest
	if err := seed.UnmarshalJSON(readGolden(f, "registration_request.json")); err != nil {
		f.Fatalf("registration request golden UnmarshalJSON() error = %v, want nil", err)
	}
	verifier, err := seed.Token.Verifier()
	if err != nil {
		f.Fatalf("RegistrationToken.Verifier(seed) error = %v, want nil", err)
	}
	prior, err := controlwire.CommitReplayIdentity(seed)
	if err != nil {
		f.Fatalf("CommitReplayIdentity(seed) error = %v, want nil", err)
	}
	mutation := seed
	mutation.RequestNonce = otherRequestNonce(f)
	fuzzStructureExternalDoor(f, structureExternalDoor[controlplane.RegistrationRequest]{
		Seed: seed, Mutations: []controlplane.RegistrationRequest{mutation},
		Marshal: func(value controlplane.RegistrationRequest) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.RegistrationRequest, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		Validate: func(value controlplane.RegistrationRequest) error { return value.Validate() },
		Authenticate: func(value controlplane.RegistrationRequest, authentic bool) error {
			proof, verifyErr := controlplane.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{
				Request: value, ExpectedVerifier: verifier, PriorReplay: &prior,
			})
			return registrationAuthorityAuthenticationOracle(proof, verifyErr, authentic)
		},
		WantError: core.ErrControlPlaneRegistration,
	})
}

func FuzzRegistrationPayloadExternalDecoder(f *testing.F) {
	issued := issueTestRegistration(f)
	seed := issued.document.Payload
	mutation := seed
	mutation.Header.RequestNonce = otherRequestNonce(f)
	fuzzStructureExternalDoor(f, structureExternalDoor[controlplane.RegistrationPayload]{
		Seed: seed, Mutations: []controlplane.RegistrationPayload{mutation},
		Marshal: func(value controlplane.RegistrationPayload) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.RegistrationPayload, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		Validate: func(value controlplane.RegistrationPayload) error { return value.Validate() },
		Authenticate: func(value controlplane.RegistrationPayload, authentic bool) error {
			request := issued.verification()
			request.Document.Payload = value
			proof, err := controlplane.VerifyRegistration(request)
			return registrationAuthenticationOracle(proof, err, authentic)
		},
		WantError: core.ErrControlPlaneRegistration,
	})
}

func FuzzCheckInPayloadExternalDecoder(f *testing.F) {
	issued := issueTestCheckIn(f, controlplaneOffering(f, 2), testCheckInWindow())
	seed := issued.request.Payload
	mutation := seed
	mutation.RequestNonce = otherRequestNonce(f)
	fuzzStructureExternalDoor(f, structureExternalDoor[controlplane.CheckInPayload]{
		Seed: seed, Mutations: []controlplane.CheckInPayload{mutation},
		Marshal: func(value controlplane.CheckInPayload) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.CheckInPayload, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		Validate: func(value controlplane.CheckInPayload) error { return value.Validate() },
		Authenticate: func(value controlplane.CheckInPayload, authentic bool) error {
			request := issued.request
			request.Payload = value
			proof, err := controlplane.VerifyCheckIn(controlplane.CheckInVerification{
				Request: request, TrustedKeys: issued.trusted,
			})
			return checkInAuthenticationOracle(proof, err, authentic)
		},
		WantError: core.ErrControlPlaneCheckIn,
	})
}

func FuzzCheckInResponsePayloadExternalDecoder(f *testing.F) {
	issued := issueTestCheckInResponse(f)
	seed := issued.document.Payload
	mutation := seed
	mutation.Header.RequestNonce = otherRequestNonce(f)
	fuzzStructureExternalDoor(f, structureExternalDoor[controlplane.CheckInResponsePayload]{
		Seed: seed, Mutations: []controlplane.CheckInResponsePayload{mutation},
		Marshal: func(value controlplane.CheckInResponsePayload) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.CheckInResponsePayload, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		Validate: func(value controlplane.CheckInResponsePayload) error { return value.Validate() },
		Authenticate: func(value controlplane.CheckInResponsePayload, authentic bool) error {
			request := issued.verification()
			request.Document.Payload = value
			proof, err := controlplane.VerifyCheckInResponse(request)
			return checkInResponseAuthenticationOracle(proof, err, authentic)
		},
		WantError: core.ErrControlPlaneCheckInResponse,
	})
}

func FuzzResponseHeaderExternalDecoder(f *testing.F) {
	issued := issueTestRegistration(f)
	seed := issued.document.Payload.Header
	mutation := seed
	mutation.RequestNonce = otherRequestNonce(f)
	fuzzStructureExternalDoor(f, structureExternalDoor[controlplane.ResponseHeader]{
		Seed: seed, Mutations: []controlplane.ResponseHeader{mutation},
		Marshal: func(value controlplane.ResponseHeader) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.ResponseHeader, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		Validate: func(value controlplane.ResponseHeader) error { return value.Validate() },
		Authenticate: func(value controlplane.ResponseHeader, authentic bool) error {
			request := issued.verification()
			request.Document.Payload.Header = value
			proof, err := controlplane.VerifyRegistration(request)
			return registrationAuthenticationOracle(proof, err, authentic)
		},
		WantError: core.ErrControlPlaneResponseHeader,
	})
}

func FuzzUsageWatermarkExternalDecoder(f *testing.F) {
	issued := issueTestCheckInResponse(f)
	seed := issued.document.Payload.Watermark
	mutation, err := controlplane.AdvanceUsageWatermark(seed, testCheckInWindow())
	if err != nil {
		f.Fatalf("AdvanceUsageWatermark(fuzz seed) error = %v, want nil", err)
	}
	fuzzStructureExternalDoor(f, structureExternalDoor[controlplane.UsageWatermark]{
		Seed: seed, Mutations: []controlplane.UsageWatermark{mutation},
		Marshal: func(value controlplane.UsageWatermark) ([]byte, error) { return value.MarshalJSON() },
		Unmarshal: func(value *controlplane.UsageWatermark, data []byte) error {
			return value.UnmarshalJSON(data)
		},
		Validate: func(value controlplane.UsageWatermark) error { return value.Validate() },
		Authenticate: func(value controlplane.UsageWatermark, authentic bool) error {
			request := issued.verification()
			request.Document.Payload.Watermark = value
			proof, verifyErr := controlplane.VerifyCheckInResponse(request)
			return checkInResponseAuthenticationOracle(proof, verifyErr, authentic)
		},
		WantError: core.ErrControlPlaneUsageWatermark,
	})
}

func fuzzStructureExternalDoor[T any](f *testing.F, door structureExternalDoor[T]) {
	f.Helper()
	canonical := mustStructureProjection(f, door, door.Seed)
	f.Add(canonical)
	for _, mutation := range door.Mutations {
		f.Add(mustStructureProjection(f, door, mutation))
	}
	for _, data := range [][]byte{
		nil, {}, []byte("null"), []byte("{}"), []byte("[]"),
		[]byte(`{"unknown":true}`), []byte(`{"payload":null}`),
		bytes.Repeat([]byte{' '}, controlplane.CheckInRequestJSONMaximumBytes+1),
	} {
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := door.Seed
		decodeErr := door.Unmarshal(&candidate, data)
		if decodeErr != nil {
			requireStructureDecodeRefusal(t, door, candidate, canonical, decodeErr)
			return
		}
		encoded := mustStructureProjection(t, door, candidate)
		requireStructureCanonicalClosure(t, door, candidate, encoded)
		if door.Authenticate != nil {
			if err := door.Authenticate(candidate, bytes.Equal(encoded, canonical)); err != nil {
				t.Fatalf("accepted structure authentication oracle error = %v, want nil", err)
			}
		}
	})
}

func mustStructureProjection[T any](t testing.TB, door structureExternalDoor[T], value T) []byte {
	t.Helper()
	encoded, err := door.Marshal(value)
	if err != nil {
		t.Fatalf("external structure MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

func requireStructureDecodeRefusal[T any](t *testing.T, door structureExternalDoor[T], candidate T, before []byte, err error) {
	t.Helper()
	if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrControlPlaneContract) || !errors.Is(err, door.WantError) {
		t.Fatalf("external structure UnmarshalJSON() error = %v, want %v/%v/%v", err, core.ErrJSONContract, core.ErrControlPlaneContract, door.WantError)
	}
	if after := mustStructureProjection(t, door, candidate); !bytes.Equal(after, before) {
		t.Fatalf("rejected external structure receiver projection = %x, want preserved %x", after, before)
	}
}

func requireStructureCanonicalClosure[T any](t *testing.T, door structureExternalDoor[T], candidate T, encoded []byte) {
	t.Helper()
	if err := door.Validate(candidate); err != nil {
		t.Fatalf("accepted external structure Validate() error = %v, want nil", err)
	}
	var roundTrip T
	decodeErr := door.Unmarshal(&roundTrip, encoded)
	second := mustStructureProjection(t, door, roundTrip)
	if decodeErr != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("accepted external structure canonical closure = (%x, %v), want (%x, nil)", second, decodeErr, encoded)
	}
}

func registrationAuthenticationOracle(proof controlplane.VerifiedRegistration, err error, authentic bool) error {
	if authentic {
		return errors.Join(err, proof.Validate())
	}
	if !errors.Is(err, core.ErrControlPlaneContract) || proof != (controlplane.VerifiedRegistration{}) {
		return errors.Join(core.ErrControlPlaneContract, err)
	}
	return nil
}

func registrationAuthorityAuthenticationOracle(proof controlplane.VerifiedRegistrationAuthority, err error, authentic bool) error {
	if authentic {
		if err != nil {
			return err
		}
		_, disposition, replayErr := proof.Replay()
		if replayErr != nil || disposition != controlwire.ReplayDispositionExact {
			return errors.Join(core.ErrControlPlaneContract, replayErr)
		}
		return nil
	}
	if !errors.Is(err, core.ErrControlPlaneRegistration) || proof.Validate() == nil {
		return errors.Join(core.ErrControlPlaneContract, err)
	}
	return nil
}

func checkInAuthenticationOracle(proof controlplane.VerifiedCheckIn, err error, authentic bool) error {
	if authentic {
		return errors.Join(err, proof.Validate())
	}
	if !errors.Is(err, core.ErrControlPlaneContract) {
		return errors.Join(core.ErrControlPlaneContract, err)
	}
	request, requestErr := proof.Request()
	if !errors.Is(requestErr, core.ErrControlPlaneContract) || !isZeroCheckInRequest(request) {
		return core.ErrControlPlaneContract
	}
	return nil
}

func checkInResponseAuthenticationOracle(proof controlplane.VerifiedCheckInResponse, err error, authentic bool) error {
	if authentic {
		return errors.Join(err, proof.Validate())
	}
	if !errors.Is(err, core.ErrControlPlaneContract) || proof != (controlplane.VerifiedCheckInResponse{}) {
		return errors.Join(core.ErrControlPlaneContract, err)
	}
	return nil
}
