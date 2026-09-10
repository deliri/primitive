package objectstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type observationFault uint16

const (
	absentEvidence observationFault = 1 << iota
	foreignVersion
	foreignLength
	foreignChecksum
	absentContentType
	absentOccurrence
	absentVersion
	absentChecksum
	observationFaultLimit
)

type observationPrimaryClass uint8

const (
	observationBoundary observationPrimaryClass = iota
	observationContradiction
	observationRefusal
	observationNeutral
	observationPrimaryClassLimit
)

// A missing evidence carrier is neutral absence; malformed required observation
// fields are refusal; individually valid disagreements are contradiction.
func (f observationFault) primaryClass() observationPrimaryClass {
	if f&absentEvidence != 0 {
		return observationNeutral
	}
	if f&(absentContentType|absentOccurrence|absentVersion|absentChecksum) != 0 {
		return observationRefusal
	}
	if f != 0 {
		return observationContradiction
	}
	return observationBoundary
}

type handoffProviderTransport struct {
	maximum       int64
	versionHeader string
	version       string
	observed      []byte
	calls         int
}

func (p *handoffProviderTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	p.calls++
	data, readErr := io.ReadAll(io.LimitReader(request.Body, p.maximum+1))
	closeErr := request.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, err
	}
	if int64(len(data)) != p.maximum || request.ContentLength != p.maximum {
		return nil, core.ErrObjectStoreIntegrity
	}
	p.observed = data
	headers := make(http.Header)
	headers.Set(p.versionHeader, p.version)
	return &http.Response{StatusCode: http.StatusOK, Header: headers, Body: http.NoBody, Request: request}, nil
}

// This exhausts the 144 realizable combinations of six observation fields,
// (version/checksum: equal, different, absent; other fields: valid or faulty),
// for both version-bearing upload providers and empty/nonempty objects. The
// baseline evidence comes from the actual Upload -> Evidence -> JSON ingress
// chain. The fixture substitutes only the provider transport, never Transfer
// or the verifier. Absence takes precedence over contradictory valid facts.
func TestProviderObservationHandoffExhaustiveFaultMatrix(t *testing.T) {
	t.Parallel()
	for _, provider := range []struct {
		name                    string
		kind                    Provider
		header                  string
		version, changedVersion string
		upload                  func(context.Context, Client, UploadRequest) (Transfer, error)
	}{
		{name: "S3", kind: ProviderAmazonS3, header: headerS3Version, version: "version-a", changedVersion: "version-b", upload: UploadS3},
		{name: "GCS", kind: ProviderGoogleCloudStorage, header: headerGCSGeneration, version: "41", changedVersion: "42", upload: UploadGCS},
	} {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()
			for _, extent := range []struct {
				name string
				data []byte
			}{
				{name: "empty object is valid evidence"},
				{name: "mixed bytes retain exact identity", data: []byte{0, 0xff, 0x81, 0x42}},
			} {
				t.Run(extent.name, func(t *testing.T) {
					t.Parallel()
					transport := &handoffProviderTransport{maximum: int64(len(extent.data)), versionHeader: provider.header, version: provider.version}
					network, err := exchange.NewClient(&http.Client{Transport: transport})
					if err != nil {
						t.Fatal(err)
					}
					client, err := NewClient(network)
					if err != nil {
						t.Fatal(err)
					}
					request := UploadRequest{Source: bytes.NewReader(extent.data), Target: providerUploadTarget(t, provider.kind), Integrity: providerIntegrity(t, extent.data), ContentType: core.HTTPMediaTypeOctetStream(), Policy: providerPolicy(t)}
					transfer, err := provider.upload(t.Context(), client, request)
					if err != nil || transfer.Validate() != nil || transfer.Commitment() != CommitmentConfirmed || !bytes.Equal(transport.observed, extent.data) || transport.calls != 1 {
						t.Fatalf("upload producer = (%v, %v), observed=%x calls=%d; want exact confirmed object %x and one call", transfer, err, transport.observed, transport.calls, extent.data)
					}
					projection, err := transfer.Evidence()
					if err != nil {
						t.Fatal(err)
					}
					encoded, err := projection.MarshalJSON()
					if err != nil {
						t.Fatal(err)
					}
					var evidence TransferEvidence
					if err := evidence.UnmarshalJSON(encoded); err != nil {
						t.Fatal(err)
					}
					observed := providerIntegrity(t, transport.observed)
					checksum, err := observed.CRC32C.Uint32()
					if err != nil {
						t.Fatal(err)
					}
					version, err := newProviderVersion(provider.kind, provider.version)
					if err != nil {
						t.Fatal(err)
					}
					otherVersion, err := newProviderVersion(provider.kind, provider.changedVersion)
					if err != nil {
						t.Fatal(err)
					}
					otherLength, err := core.NewByteLength(observed.Length.Uint64() + 1)
					if err != nil {
						t.Fatal(err)
					}
					base := ProviderUploadObservationRequest{Evidence: evidence, Version: version, Bytes: observed.Length, CRC32C: observed.CRC32C, ContentType: core.HTTPMediaTypeOctetStream(), OccurredAt: temporal.InstantFromNanoseconds(1)}
					wantEvidence := TransferEvidence{provider: provider.kind, direction: DirectionUpload, version: version, bytes: observed.Length, sha256: observed.SHA256, crc32c: observed.CRC32C, set: true}
					if evidence != wantEvidence {
						t.Fatalf("producer handoff evidence=%+v, want independently observed facts %+v", evidence, wantEvidence)
					}
					rows := 0
					var classes [observationPrimaryClassLimit]int
					for fault := range observationFaultLimit {
						// A single field cannot be both absent and different. Exclude
						// impossible fixture states instead of overwriting one mutation.
						if fault&(foreignVersion|absentVersion) == foreignVersion|absentVersion || fault&(foreignChecksum|absentChecksum) == foreignChecksum|absentChecksum {
							continue
						}
						rows++
						primary := fault.primaryClass()
						classes[primary]++
						t.Run(fault.String(), func(t *testing.T) {
							t.Parallel()
							candidate := base
							if fault&absentEvidence != 0 {
								candidate.Evidence = TransferEvidence{}
							}
							if fault&foreignVersion != 0 {
								candidate.Version = otherVersion
							}
							if fault&foreignLength != 0 {
								candidate.Bytes = otherLength
							}
							if fault&foreignChecksum != 0 {
								candidate.CRC32C = core.NewCRC32C(^checksum)
							}
							if fault&absentContentType != 0 {
								candidate.ContentType = core.HTTPMediaType{}
							}
							if fault&absentOccurrence != 0 {
								candidate.OccurredAt = temporal.Instant{}
							}
							if fault&absentVersion != 0 {
								candidate.Version = ProviderVersion{}
							}
							if fault&absentChecksum != 0 {
								candidate.CRC32C = core.CRC32C{}
							}
							if (candidate != base) != (fault != 0) {
								t.Fatalf("faults=%06b changed input=%t, want %t", fault, candidate != base, fault != 0)
							}
							var want error
							if fault != 0 {
								want = core.ErrObjectStoreIntegrity
							}
							missing := fault&(absentEvidence|absentContentType|absentOccurrence|absentVersion|absentChecksum) != 0
							if primary == observationRefusal || primary == observationNeutral {
								want = core.ErrObjectStoreContract
							}
							got, err := VerifyProviderUpload(candidate)
							if !errors.Is(err, want) {
								t.Fatalf("faults=%06b error=%v, want primary %v", fault, err, want)
							}
							if want != nil {
								if got != (VerifiedProviderUpload{}) {
									t.Fatalf("refusal retained proof %v, want zero", got)
								}
								if missing && errors.Is(err, core.ErrObjectStoreIntegrity) {
									t.Fatalf("absent facts also classified as integrity contradiction: %v", err)
								}
								if !missing && !errors.Is(err, core.ErrObjectStoreSource) {
									t.Fatalf("contradiction identities=%v, want source and integrity identities", err)
								}
							} else {
								facts, factsErr := got.Evidence()
								media, mediaErr := got.ContentType()
								instant, instantErr := got.OccurredAt()
								if factsErr != nil || mediaErr != nil || instantErr != nil || got.Validate() != nil || facts != evidence || media != base.ContentType || instant != base.OccurredAt {
									t.Fatalf("verified facts=%v/%v/%v, want exact producer evidence and observation", facts, media, instant)
								}
							}
							// Refusals and prior successful calls cannot contaminate the
							// next use of this immutable, independently owned baseline.
							again, againErr := VerifyProviderUpload(base)
							if againErr != nil || again.request != base {
								t.Fatalf("baseline after faults=%06b = (%v, %v), want original proof", fault, again, againErr)
							}
						})
					}
					// Exclusive counts for the complete six-field fault model:
					// absent evidence dominates 72 rows; among the remaining 72,
					// eight have all required fields, seven of those disagree.
					wantClasses := [observationPrimaryClassLimit]int{1, 7, 64, 72}
					if classes != wantClasses {
						t.Fatalf("primary class counts=%v, want %v", classes, wantClasses)
					}
					if rows != 3*3*2*2*2*2 {
						t.Fatalf("realizable matrix rows = %d, want %d", rows, 3*3*2*2*2*2)
					}
				})
			}
		})
	}
}

func (f observationFault) String() string {
	if f == 0 {
		return "exact producer facts"
	}
	var names []string
	for _, field := range []struct {
		fault observationFault
		name  string
	}{
		{absentEvidence, "absent evidence"}, {foreignVersion, "different version"},
		{foreignLength, "different extent"}, {foreignChecksum, "different checksum"},
		{absentContentType, "absent media type"}, {absentOccurrence, "absent occurrence"},
		{absentVersion, "absent version"}, {absentChecksum, "absent checksum"},
	} {
		if f&field.fault != 0 {
			names = append(names, field.name)
		}
	}
	return strings.Join(names, " + ")
}
