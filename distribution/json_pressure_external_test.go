package distribution_test

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/objectstore"
	"io"
	"slices"
	"testing"
)

// Members are copied from real typed projections solely to damage wire grammar.
// No production schema or accepted value is reconstructed from a loose map.
type distributionWireMember struct {
	name       string
	key, value []byte
}

func distributionWireMembers(t testing.TB, data []byte) []distributionWireMember {
	t.Helper()
	d := jsontext.NewDecoder(bytes.NewReader(data))
	token, err := d.ReadToken()
	if err != nil || token.Kind() != '{' {
		t.Fatalf("wire object=(%v,%v), want object", token, err)
	}
	var members []distributionWireMember
	for d.PeekKind() != '}' {
		key, err := d.ReadToken()
		if err != nil {
			t.Fatalf("ReadToken(key)=%v, want nil", err)
		}
		name := key.String()
		value, err := d.ReadValue()
		if err != nil {
			t.Fatalf("ReadValue()=%v, want nil", err)
		}
		encodedKey, err := core.MarshalCanonicalJSONString(name)
		if err != nil {
			t.Fatalf("key encoding=%v, want nil", err)
		}
		members = append(members, distributionWireMember{name: name, key: encodedKey, value: bytes.Clone(value)})
	}
	if _, err := d.ReadToken(); err != nil {
		t.Fatalf("ReadToken(close)=%v, want nil", err)
	}
	if _, err := d.ReadToken(); !errors.Is(err, io.EOF) {
		t.Fatalf("ReadToken(end)=%v, want EOF", err)
	}
	return members
}
func distributionWireObject(members []distributionWireMember) []byte {
	b := []byte{'{'}
	for i, m := range members {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, m.key...)
		b = append(b, ':')
		b = append(b, m.value...)
	}
	return append(b, '}')
}

type distributionJSONPressure struct {
	name    string
	wire    []byte
	wantErr error
}

func distributionJSONCases(t testing.TB, wire []byte, maximum int) []distributionJSONPressure {
	t.Helper()
	members := distributionWireMembers(t, wire)
	reversed := slices.Clone(members)
	slices.Reverse(reversed)
	cases := []distributionJSONPressure{
		{name: "canonical structure", wire: wire},
		{name: "reversed distinct fields preserve signed facts", wire: distributionWireObject(reversed)},
		{name: "outer whitespace preserves facts", wire: append(append([]byte(" \n"), wire...), '\t')},
		{name: "empty input emits no value", wantErr: core.ErrJSONContract},
		{name: "null input emits no value", wire: []byte("null"), wantErr: core.ErrJSONContract},
		{name: "array cannot impersonate object", wire: []byte("[]"), wantErr: core.ErrJSONContract},
		{name: "missing close", wire: wire[:len(wire)-1], wantErr: core.ErrJSONContract},
		{name: "trailing document", wire: append(bytes.Clone(wire), []byte("{}")...), wantErr: core.ErrJSONContract},
		{name: "unknown field", wire: distributionWireObject(append(slices.Clone(members), distributionWireMember{key: []byte("\"unknown\""), value: []byte("true")})), wantErr: core.ErrJSONContract},
	}
	for _, m := range []struct {
		name    string
		size    int
		wantErr error
	}{
		{"one below input ceiling", maximum - 1, nil},
		{"exact input ceiling", maximum, nil},
		{"one above input ceiling", maximum + 1, core.ErrJSONContract},
	} {
		if len(wire) > m.size {
			t.Fatalf("fixture size=%d, want below input ceiling %d", len(wire), m.size)
		}
		padded := append(bytes.Clone(wire), bytes.Repeat([]byte{' '}, m.size-len(wire))...)
		cases = append(cases, distributionJSONPressure{name: m.name, wire: padded, wantErr: m.wantErr})
	}
	for i, member := range members {
		if len(member.value) > 0 && member.value[0] == '[' {
			var items []jsontext.Value
			if err := json.Unmarshal(member.value, &items); err != nil || len(items) == 0 {
				t.Fatalf("array fixture=(%d,%v), want nonempty array", len(items), err)
			}
			for _, extent := range []struct {
				name   string
				values []jsontext.Value
			}{
				{"one object missing", items[:len(items)-1]},
				{"one extra object", append(slices.Clone(items), items[0])},
			} {
				changed := slices.Clone(members)
				encoded, err := json.Marshal(extent.values)
				if err != nil {
					t.Fatalf("array encoding=%v, want nil", err)
				}
				changed[i].value = encoded
				cases = append(cases, distributionJSONPressure{name: member.name + " " + extent.name, wire: distributionWireObject(changed), wantErr: core.ErrJSONContract})
			}
		}
		omitted := append(slices.Clone(members[:i]), members[i+1:]...)
		cases = append(cases, distributionJSONPressure{name: "missing " + member.name, wire: distributionWireObject(omitted), wantErr: core.ErrJSONContract})
		duplicate := append(slices.Clone(members), member)
		cases = append(cases, distributionJSONPressure{name: "duplicate " + member.name, wire: distributionWireObject(duplicate), wantErr: core.ErrJSONContract})
		for _, bad := range []struct {
			name  string
			value []byte
		}{{"null", []byte("null")}, {"wrong boolean type", []byte("true")}} {
			mutated := slices.Clone(members)
			mutated[i].value = bad.value
			cases = append(cases, distributionJSONPressure{name: member.name + " " + bad.name, wire: distributionWireObject(mutated), wantErr: core.ErrJSONContract})
		}
	}
	return cases
}

func testDistributionJSONDoor[T comparable](t *testing.T, seed T, maximum int, marshal func(T) ([]byte, error), unmarshal func(*T, []byte) error) {
	t.Helper()
	wire, err := marshal(seed)
	if err != nil {
		t.Fatalf("MarshalJSON(seed)=%v, want nil", err)
	}
	for _, tc := range distributionJSONCases(t, wire, maximum) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, before := range []T{*new(T), seed} {
				got := before
				err := unmarshal(&got, tc.wire)
				want := seed
				if tc.wantErr != nil {
					want = before
				}
				if !errors.Is(err, tc.wantErr) || got != want {
					t.Fatalf("UnmarshalJSON()=(%v,%v), want (%v,%v)", got, err, want, tc.wantErr)
				}
				if tc.wantErr != nil && !errors.Is(err, core.ErrDistributionContract) {
					t.Fatalf("decode refusal=%v, want %v", err, core.ErrDistributionContract)
				}
				if tc.wantErr == nil {
					canonical, err := marshal(got)
					if err != nil || !bytes.Equal(canonical, wire) {
						t.Fatalf("MarshalJSON()=(%d,%v), want exact %d canonical bytes", len(canonical), err, len(wire))
					}
				}
			}
		})
	}
	cases := []struct {
		name string
		wire []byte
	}{{name: "nil receiver before canonical input", wire: wire}, {name: "nil receiver before empty input"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := unmarshal(nil, tc.wire)
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrDistributionContract) {
				t.Fatalf("nil receiver refusal=%v, want JSON and Distribution identities", err)
			}
		})
	}
}

func publicationGrantProjectionFromDocument(d distribution.PublicationGrantDocument) (distribution.PublicationGrantProjection, error) {
	p := distribution.PublicationGrantProjection{Payload: d.Payload, Attestation: d.Attestation}
	for i, c := range d.Capabilities {
		target, err := c.Target()
		if err != nil {
			return distribution.PublicationGrantProjection{}, err
		}
		provider, err := c.Provider()
		if err != nil {
			return distribution.PublicationGrantProjection{}, err
		}
		projection, err := objectstore.NewUploadCapabilityProjection(provider, target)
		if err != nil {
			return distribution.PublicationGrantProjection{}, err
		}
		p.Capabilities[i] = projection
	}
	return p, p.Validate()
}

func upgradeGrantProjectionFromDocument(d distribution.UpgradeGrantDocument) (distribution.UpgradeGrantProjection, error) {
	target, err := d.Capability.Target()
	if err != nil {
		return distribution.UpgradeGrantProjection{}, err
	}
	provider, err := d.Capability.Provider()
	if err != nil {
		return distribution.UpgradeGrantProjection{}, err
	}
	capability, err := objectstore.NewDownloadCapabilityProjection(provider, target)
	if err != nil {
		return distribution.UpgradeGrantProjection{}, err
	}
	p := distribution.UpgradeGrantProjection{Capability: capability, Payload: d.Payload, Attestation: d.Attestation}
	return p, p.Validate()
}

func TestPublicationRequestPayloadJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newPublicationExchangeFixture(t)
	testDistributionJSONDoor(t, p.request, distribution.RequestPayloadJSONMaximumBytes, distribution.PublicationRequestPayload.MarshalJSON, (*distribution.PublicationRequestPayload).UnmarshalJSON)
}

func TestPublicationRequestDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newPublicationExchangeFixture(t)
	testDistributionJSONDoor(t, p.requestDocument, distribution.RequestDocumentJSONMaximumBytes, distribution.PublicationRequestDocument.MarshalJSON, (*distribution.PublicationRequestDocument).UnmarshalJSON)
}

func TestPublicationGrantPayloadJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newPublicationExchangeFixture(t)
	testDistributionJSONDoor(t, p.grantPayload, distribution.ResponsePayloadJSONMaximumBytes, distribution.PublicationGrantPayload.MarshalJSON, (*distribution.PublicationGrantPayload).UnmarshalJSON)
}

func TestPublicationCompletionPayloadJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newPublicationExchangeFixture(t)
	d := completedPublicationDocument(t, p, 0)
	testDistributionJSONDoor(t, d.Payload, distribution.PublicationCompletionPayloadJSONMaximumBytes, distribution.PublicationCompletionPayload.MarshalJSON, (*distribution.PublicationCompletionPayload).UnmarshalJSON)
}

func TestPublicationCompletionDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newPublicationExchangeFixture(t)
	d := completedPublicationDocument(t, p, 0)
	testDistributionJSONDoor(t, d, distribution.ResponseDocumentJSONMaximumBytes, distribution.PublicationCompletionDocument.MarshalJSON, (*distribution.PublicationCompletionDocument).UnmarshalJSON)
}

func TestUpdateRequestPayloadJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newUpdateExchangeFixture(t)
	testDistributionJSONDoor(t, p.request, distribution.RequestPayloadJSONMaximumBytes, distribution.UpdateRequestPayload.MarshalJSON, (*distribution.UpdateRequestPayload).UnmarshalJSON)
}

func TestUpdateRequestDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newUpdateExchangeFixture(t)
	testDistributionJSONDoor(t, p.requestDoc, distribution.RequestDocumentJSONMaximumBytes, distribution.UpdateRequestDocument.MarshalJSON, (*distribution.UpdateRequestDocument).UnmarshalJSON)
}

func TestUpdateResponsePayloadJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newUpdateExchangeFixture(t)
	testDistributionJSONDoor(t, p.responseDoc.Payload, distribution.ResponsePayloadJSONMaximumBytes, distribution.UpdateResponsePayload.MarshalJSON, (*distribution.UpdateResponsePayload).UnmarshalJSON)
}

func TestUpdateResponseDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newUpdateExchangeFixture(t)
	testDistributionJSONDoor(t, p.responseDoc, distribution.ResponseDocumentJSONMaximumBytes, distribution.UpdateResponseDocument.MarshalJSON, (*distribution.UpdateResponseDocument).UnmarshalJSON)
}

func TestUpgradeRequestPayloadJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newUpgradeExchangeFixture(t)
	testDistributionJSONDoor(t, p.request, distribution.RequestPayloadJSONMaximumBytes, distribution.UpgradeRequestPayload.MarshalJSON, (*distribution.UpgradeRequestPayload).UnmarshalJSON)
}

func TestUpgradeRequestDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newUpgradeExchangeFixture(t)
	testDistributionJSONDoor(t, p.requestDoc, distribution.RequestDocumentJSONMaximumBytes, distribution.UpgradeRequestDocument.MarshalJSON, (*distribution.UpgradeRequestDocument).UnmarshalJSON)
}

func TestUpgradeGrantPayloadJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newUpgradeExchangeFixture(t)
	testDistributionJSONDoor(t, p.grantDoc.Payload, distribution.ResponsePayloadJSONMaximumBytes, distribution.UpgradeGrantPayload.MarshalJSON, (*distribution.UpgradeGrantPayload).UnmarshalJSON)
}

func TestRequestCommitmentJSONLayerTriad(t *testing.T) {
	t.Parallel()
	p := newPublicationExchangeFixture(t)
	d, err := distribution.CommitRequest(p.request)
	if err != nil {
		t.Fatalf("CommitRequest()=%v, want nil", err)
	}
	testDistributionJSONDoor(t, d, distribution.RequestPayloadJSONMaximumBytes, distribution.RequestCommitment.MarshalJSON, (*distribution.RequestCommitment).UnmarshalJSON)
}

func TestPublicationGrantDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()
	f := newPublicationExchangeFixture(t)
	projection, err := publicationGrantProjectionFromDocument(f.grantDocument)
	if err != nil {
		t.Fatalf("projection reconstruction=%v, want nil", err)
	}
	wire, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON()=%v, want nil", err)
	}
	for _, tc := range distributionJSONCases(t, wire, distribution.ResponseDocumentJSONMaximumBytes) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, populated := range []bool{false, true} {
				var got distribution.PublicationGrantDocument
				if populated {
					got = f.grantDocument
				}
				err := got.UnmarshalJSON(tc.wire)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("UnmarshalJSON()=%v, want %v", err, tc.wantErr)
				}
				if tc.wantErr != nil {
					if !errors.Is(err, core.ErrDistributionContract) {
						t.Fatalf("UnmarshalJSON()=%v, want %v", err, core.ErrDistributionContract)
					}
					isZero := got.Payload == (distribution.PublicationGrantPayload{}) && got.Attestation == (attest.Envelope[distribution.SigningDomain]{})
					for _, c := range got.Capabilities {
						isZero = isZero && c.IsZero()
					}
					if (populated && !samePublicationGrantDocument(got, f.grantDocument)) || (!populated && !isZero) {
						t.Fatalf("receiver preserved=%t, want true", false)
					}
				} else {
					p, err := publicationGrantProjectionFromDocument(got)
					if err != nil {
						t.Fatalf("projection reconstruction=%v, want nil", err)
					}
					encoded, err := p.MarshalJSON()
					if err != nil || !bytes.Equal(encoded, wire) {
						t.Fatalf("canonical projection=(%d,%v), want exact %d bytes", len(encoded), err, len(wire))
					}
				}
			}
		})
	}
	var absent *distribution.PublicationGrantDocument
	if err := absent.UnmarshalJSON(wire); !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrDistributionContract) {
		t.Fatalf("nil receiver refusal=%v, want JSON and Distribution identities", err)
	}
}

func TestUpgradeGrantDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()
	f := newUpgradeExchangeFixture(t)
	projection, err := upgradeGrantProjectionFromDocument(f.grantDoc)
	if err != nil {
		t.Fatalf("projection reconstruction=%v, want nil", err)
	}
	wire, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON()=%v, want nil", err)
	}
	for _, tc := range distributionJSONCases(t, wire, distribution.ResponseDocumentJSONMaximumBytes) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, populated := range []bool{false, true} {
				var got distribution.UpgradeGrantDocument
				if populated {
					got = f.grantDoc
				}
				err := got.UnmarshalJSON(tc.wire)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("UnmarshalJSON()=%v, want %v", err, tc.wantErr)
				}
				if tc.wantErr != nil {
					if !errors.Is(err, core.ErrDistributionContract) {
						t.Fatalf("UnmarshalJSON()=%v, want %v", err, core.ErrDistributionContract)
					}
					isZero := got.Payload == (distribution.UpgradeGrantPayload{}) && got.Attestation == (attest.Envelope[distribution.SigningDomain]{}) && got.Capability.IsZero()
					if (populated && !sameUpgradeGrantDocument(got, f.grantDoc)) || (!populated && !isZero) {
						t.Fatalf("receiver preserved=%t, want true", false)
					}
				} else {
					p, err := upgradeGrantProjectionFromDocument(got)
					if err != nil {
						t.Fatalf("projection reconstruction=%v, want nil", err)
					}
					encoded, err := p.MarshalJSON()
					if err != nil || !bytes.Equal(encoded, wire) {
						t.Fatalf("canonical projection=(%d,%v), want exact %d bytes", len(encoded), err, len(wire))
					}
				}
			}
		})
	}
	var absent *distribution.UpgradeGrantDocument
	if err := absent.UnmarshalJSON(wire); !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrDistributionContract) {
		t.Fatalf("nil receiver refusal=%v, want JSON and Distribution identities", err)
	}
}
