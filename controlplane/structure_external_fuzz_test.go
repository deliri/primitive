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
	issued := issueTestCheckIn(f, controlplaneOffering(f, 3), testCheckInWindow())
	defer clear(issued.device)
	defer clear(issued.authority)
	server := issued.server(f)
	token, err := controlwire.NewRegistrationToken([controlwire.RegistrationTokenBytes]byte{83})
	if err != nil {
		f.Fatalf("NewRegistrationToken(seed) error = %v, want nil", err)
	}
	defer token.Destroy()
	seed := controlplane.RegistrationRequest{
		Token: token, Build: issued.certificate.Body.Build, DeviceKey: issued.certificate.Body.DeviceKey,
		Installation: issued.subject.DeviceID, RequestNonce: issued.request.Payload.RequestNonce,
		Revision: issued.certificate.Body.Revision,
	}
	wantIdentity, err := seed.Identity()
	if err != nil {
		f.Fatalf("seed.Identity() error = %v, want nil", err)
	}
	verifier, err := seed.Token.Verifier()
	if err != nil {
		f.Fatalf("RegistrationToken.Verifier(seed) error = %v, want nil", err)
	}
	canonical := mustRegistrationRequestProjection(f, seed)
	defer clear(canonical)
	f.Add(canonical)
	mutation := seed
	mutation.RequestNonce = otherRequestNonce(f)
	if mutation.RequestNonce == seed.RequestNonce {
		f.Fatalf("mutated nonce = %v, want distinct from %v", mutation.RequestNonce, seed.RequestNonce)
	}
	f.Add(mustRegistrationRequestProjection(f, mutation))
	mutation = seed
	mutation.DeviceKey, mutation.Installation = testDeviceKey(f, checkInOtherDeviceSeed)
	if mutation.Installation == seed.Installation {
		f.Fatalf("mutated installation = %v, want distinct from %v", mutation.Installation, seed.Installation)
	}
	f.Add(mustRegistrationRequestProjection(f, mutation))
	for _, size := range []int{controlplane.RegistrationRequestJSONMaximumBytes - 1, controlplane.RegistrationRequestJSONMaximumBytes, controlplane.RegistrationRequestJSONMaximumBytes + 1} {
		f.Add(append(append([]byte(nil), canonical...), bytes.Repeat([]byte{' '}, size-len(canonical))...))
	}
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add([]byte("{}"))
	f.Add(canonical[:len(canonical)-1])
	// Obtain the prior commitment through the real admitting producer.
	admitted, err := server.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{Request: seed, ExpectedVerifier: verifier})
	if err != nil {
		f.Fatalf("VerifyRegistrationAuthority(seed) error = %v, want nil", err)
	}
	prior, disposition, err := admitted.Replay()
	if err != nil || disposition != controlwire.ReplayDispositionFresh {
		f.Fatalf("Replay(seed) = (%v, %v), want fresh and nil", disposition, err)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var candidate controlplane.RegistrationRequest
		if err := candidate.UnmarshalJSON(canonical); err != nil {
			t.Fatalf("UnmarshalJSON(populated receiver) error = %v, want nil", err)
		}
		original := candidate
		defer original.Token.Destroy()
		decodeErr := candidate.UnmarshalJSON(data)
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrControlPlaneRegistration) || candidate != original {
				t.Fatalf("UnmarshalJSON(rejected) = (%v, %v), want preserved receiver and JSON/registration refusal", candidate, decodeErr)
			}
			var fresh controlplane.RegistrationRequest
			freshErr := fresh.UnmarshalJSON(data)
			if !errors.Is(freshErr, core.ErrControlPlaneRegistration) || fresh != (controlplane.RegistrationRequest{}) {
				t.Fatalf("UnmarshalJSON(fresh rejection) = (%v, %v), want zero receiver and typed refusal", fresh, freshErr)
			}
			return
		}
		defer candidate.Token.Destroy()
		identity, err := candidate.Identity()
		if err != nil {
			t.Fatalf("Identity(accepted) error = %v, want nil", err)
		}
		encoded := mustRegistrationRequestProjection(t, candidate)
		defer clear(encoded)
		if len(encoded) > controlplane.RegistrationRequestJSONMaximumBytes {
			t.Fatalf("canonical byte count = %d, want <= %d", len(encoded), controlplane.RegistrationRequestJSONMaximumBytes)
		}
		var roundTrip controlplane.RegistrationRequest
		if err := roundTrip.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("UnmarshalJSON(canonical) error = %v, want nil", err)
		}
		defer roundTrip.Token.Destroy()
		second := mustRegistrationRequestProjection(t, roundTrip)
		defer clear(second)
		secondIdentity, err := roundTrip.Identity()
		if err != nil || secondIdentity != identity || !bytes.Equal(second, encoded) {
			t.Fatalf("canonical closure = (%+v, %v), want exact identity %+v and stable bytes", secondIdentity, err, identity)
		}
		presented, err := candidate.Token.Verifier()
		if err != nil {
			t.Fatalf("Verifier(accepted) error = %v, want nil", err)
		}
		proof, verifyErr := server.VerifyRegistrationAuthority(controlplane.RegistrationAuthorityVerification{Request: candidate, ExpectedVerifier: verifier, PriorReplay: &prior})
		if bytes.Equal(encoded, canonical) {
			gotIdentity, identityErr := proof.Identity()
			gotReplay, gotDisposition, replayErr := proof.Replay()
			if errors.Join(verifyErr, identityErr, replayErr) != nil || gotIdentity != wantIdentity || !gotReplay.Equal(prior) || gotDisposition != controlwire.ReplayDispositionExact {
				t.Fatalf("exact retry = (%+v, %v, %v), want original identity %+v, commitment and exact disposition", gotIdentity, gotDisposition, errors.Join(verifyErr, identityErr, replayErr), wantIdentity)
			}
			return
		}
		wantErr := core.ErrControlWireReplayConflict
		if !verifier.Equal(presented) {
			wantErr = core.ErrControlWireToken
		}
		if !errors.Is(verifyErr, wantErr) || !errors.Is(verifyErr, core.ErrControlPlaneRegistration) || proof != (controlplane.VerifiedRegistrationAuthority{}) {
			t.Fatalf("changed registration = (%v, %v), want zero proof and %v", proof, verifyErr, wantErr)
		}
	})
}

func FuzzRegistrationPayloadExternalDecoder(f *testing.F) {
	issued := issueTestRegistration(f)
	client := issued.client(f)
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
			proof, err := client.VerifyRegistration(request)
			return registrationAuthenticationOracle(proof, err, authentic)
		},
		WantError: core.ErrControlPlaneRegistration,
	})
}

func FuzzCheckInPayloadExternalDecoder(f *testing.F) {
	issued := issueTestCheckIn(f, controlplaneOffering(f, 2), testCheckInWindow())
	server := issued.server(f)
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
			proof, err := server.VerifyCheckIn(controlplane.CheckInVerification{
				Request: request,
			})
			return checkInAuthenticationOracle(proof, err, authentic)
		},
		WantError: core.ErrControlPlaneCheckIn,
	})
}

func FuzzCheckInResponsePayloadExternalDecoder(f *testing.F) {
	issued := issueTestCheckInResponse(f)
	client := issued.client(f)
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
			proof, err := client.VerifyCheckInResponse(request)
			return checkInResponseAuthenticationOracle(proof, err, authentic)
		},
		WantError: core.ErrControlPlaneCheckInResponse,
	})
}

func FuzzResponseHeaderExternalDecoder(f *testing.F) {
	issued := issueTestRegistration(f)
	client := issued.client(f)
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
			proof, err := client.VerifyRegistration(request)
			return registrationAuthenticationOracle(proof, err, authentic)
		},
		WantError: core.ErrControlPlaneResponseHeader,
	})
}

func FuzzUsageWatermarkExternalDecoder(f *testing.F) {
	issued := issueTestCheckInResponse(f)
	client := issued.client(f)
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
			proof, verifyErr := client.VerifyCheckInResponse(request)
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
