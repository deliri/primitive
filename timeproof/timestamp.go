package timeproof

import (
	"bytes"
	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// AuthoritativeTimestamp is a proof-carrying verified RFC 3161 conclusion.
type AuthoritativeTimestamp struct {
	evidence AuthorityEvidence
	time     AuthoritativeTime
	signer   core.SHA256Digest
	serial   SerialNumber
	policy   TimestampPolicy
}

// AuthoritativeTime binds the RFC 3161 nominal generation time to the
// authority's declared maximum deviation. A zero Accuracy means the token did
// not declare accuracy; it never means the authority proved exact time.
type AuthoritativeTime struct {
	Generation temporal.Instant  `json:"generation_nanoseconds"`
	Accuracy   temporal.Duration `json:"accuracy_nanoseconds"`
}

// Validate checks the nominal generation and bounded optional accuracy.
func (t AuthoritativeTime) Validate() error {
	if err := t.Generation.Validate(); err != nil {
		return contractError(err)
	}
	if err := t.Accuracy.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// AccuracyDeclared reports whether the authority bounded its generation-time
// deviation. False means the uncertainty is unspecified, not zero.
func (t AuthoritativeTime) AccuracyDeclared() bool {
	return !t.Accuracy.IsZero()
}

type authoritativeTimestampWire struct {
	Evidence AuthorityEvidence `json:"evidence"`
	Time     AuthoritativeTime `json:"time"`
	Signer   core.SHA256Digest `json:"signer_sha256"`
	Serial   SerialNumber      `json:"serial"`
	Policy   TimestampPolicy   `json:"policy"`
}

type authoritativeTimestampWireJSON authoritativeTimestampWire

func (w authoritativeTimestampWire) MarshalJSON() ([]byte, error) {
	if err := w.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(authoritativeTimestampWireJSON(w))
}

// VerifyRequest pairs one prepared request with the exact bounded response and
// a separately supplied expected digest.
type VerifyRequest struct {
	Response       []byte
	Request        Request
	ExpectedDigest core.SHA256Digest
}

// Validate checks the response boundary and independent digest binding.
func (r VerifyRequest) Validate() error {
	if err := r.Request.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.ExpectedDigest.Validate(); err != nil {
		return contractError(err)
	}
	if r.Request.Digest() != r.ExpectedDigest {
		return invalidError(nil)
	}
	if len(r.Response) == 0 || len(r.Response) > ResponseMaximumBytes {
		return contractError(nil)
	}
	return nil
}

// Verify checks RFC 3161, CMS, signer, chain, policy, nonce, and message
// imprint binding, then owns the exact request and response as evidence.
func Verify(request VerifyRequest) (AuthoritativeTimestamp, error) {
	if err := request.Validate(); err != nil {
		return AuthoritativeTimestamp{}, err
	}
	digest, err := request.ExpectedDigest.Bytes()
	if err != nil {
		return AuthoritativeTimestamp{}, contractError(err)
	}
	token, err := parseAndVerifyToken(timestampTokenVerification{
		response: request.Response, digest: digest,
		nonce: request.Request.Nonce(), authority: request.Request.Authority(),
	})
	if err != nil {
		return AuthoritativeTimestamp{}, err
	}
	evidence, err := newAuthorityEvidence(authorityEvidenceInput{
		Request: request.Request, Response: request.Response,
	})
	if err != nil {
		return AuthoritativeTimestamp{}, err
	}
	return authoritativeFromVerified(evidence, token)
}

func authoritativeFromVerified(
	evidence AuthorityEvidence,
	token verifiedToken,
) (AuthoritativeTimestamp, error) {
	serial, err := newSerialNumber(token.Serial)
	if err != nil {
		return AuthoritativeTimestamp{}, err
	}
	timestamp := AuthoritativeTimestamp{
		evidence: evidence,
		time:     token.Time,
		signer:   core.NewSHA256Digest(token.SignerSHA256),
		serial:   serial,
		policy:   token.Policy,
	}
	if err := timestamp.Validate(); err != nil {
		return AuthoritativeTimestamp{}, err
	}
	return timestamp, nil
}

// Validate checks the closed verified-value representation.
func (t AuthoritativeTimestamp) Validate() error {
	if err := t.evidence.Validate(); err != nil {
		return contractError(err)
	}
	if err := t.time.Validate(); err != nil {
		return contractError(err)
	}
	if err := t.signer.Validate(); err != nil {
		return contractError(err)
	}
	if err := t.serial.Validate(); err != nil {
		return contractError(err)
	}
	registry, err := authorityRegistry(t.evidence.Authority())
	if err != nil || t.policy != registry.policy {
		return contractError(err)
	}
	return nil
}

// Time returns the nominal generation time together with the authority's
// declared uncertainty. Consumers must apply both facts to clock policy.
func (t AuthoritativeTimestamp) Time() AuthoritativeTime {
	return t.time
}

// Evidence returns owned proof custody with copy-returning byte accessors.
func (t AuthoritativeTimestamp) Evidence() AuthorityEvidence {
	return t.evidence
}

// Signer returns the verified timestamp signer certificate digest.
func (t AuthoritativeTimestamp) Signer() core.SHA256Digest { return t.signer }

// Serial returns the verified RFC 3161 serial number.
func (t AuthoritativeTimestamp) Serial() SerialNumber { return t.serial }

// Policy returns the verified authority policy.
func (t AuthoritativeTimestamp) Policy() TimestampPolicy { return t.policy }

// MarshalJSON emits canonical evidence plus independently checked derived
// facts. No local observation or acquisition failure can enter this wire form.
func (t AuthoritativeTimestamp) MarshalJSON() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return core.EncodeValidatedJSON(authoritativeTimestampWire{
		Evidence: t.evidence, Time: t.time, Signer: t.signer,
		Serial: t.serial, Policy: t.policy,
	}, core.DefaultStrictJSONLimits())
}

// UnmarshalJSON re-verifies the exact evidence before constructing a value.
func (t *AuthoritativeTimestamp) UnmarshalJSON(data []byte) error {
	if t == nil {
		return errorsJSON()
	}
	wire, err := core.DecodeStrictJSON[authoritativeTimestampWire](
		bytes.NewReader(data), core.DefaultStrictJSONLimits(),
	)
	if err != nil {
		return errorsJSON()
	}
	verified, err := Verify(VerifyRequest{
		Request: wire.Evidence.Request(), Response: wire.Evidence.ResponseBytes(),
		ExpectedDigest: wire.Evidence.Digest(),
	})
	if err != nil || !authoritativeWireMatches(verified, wire) {
		return invalidError(err)
	}
	canonical, err := verified.MarshalJSON()
	if err != nil || !bytes.Equal(data, canonical) {
		return errorsJSON()
	}
	*t = verified
	return nil
}

func (w authoritativeTimestampWire) Validate() error {
	if err := w.Evidence.Validate(); err != nil {
		return err
	}
	if err := w.Time.Validate(); err != nil {
		return err
	}
	if err := w.Signer.Validate(); err != nil {
		return err
	}
	if err := w.Serial.Validate(); err != nil {
		return err
	}
	return w.Policy.Validate()
}

func authoritativeWireMatches(
	timestamp AuthoritativeTimestamp,
	wire authoritativeTimestampWire,
) bool {
	return timestamp.time == wire.Time &&
		timestamp.signer == wire.Signer &&
		timestamp.serial == wire.Serial &&
		timestamp.policy == wire.Policy
}

func (t AuthoritativeTimestamp) isZero() bool {
	return t.evidence.isZero() &&
		t.time == (AuthoritativeTime{}) &&
		t.policy == TimestampPolicyUnknown
}
