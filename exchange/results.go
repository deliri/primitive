package exchange

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// ResponseMetadata is the validated result shared by all client families.
type ResponseMetadata struct {
	// RetryAfter is the native Retry-After projection at receipt time. Zero
	// means no usable positive hint. Retry owners clamp it to their policy.
	RetryAfter temporal.Duration
	Headers    CapturedHeaders
	Bytes      core.ByteLength
	Attempts   uint64
	Status     core.HTTPStatusCode
}

// Validate checks status, bounded captured headers, and attempt count.
func (m ResponseMetadata) Validate() error {
	if err := m.RetryAfter.Validate(); err != nil {
		return responseError(err)
	}
	if err := m.Status.Validate(); err != nil {
		return responseError(err)
	}
	if err := m.Headers.Validate(); err != nil {
		return responseError(err)
	}
	if m.Attempts == 0 {
		return responseError(core.ErrExchangeContract)
	}
	return nil
}

// JSONResponse is one strictly decoded and validated JSON response.
type JSONResponse[Body core.Validatable] struct {
	// FailureBody owns the unexpected-status bytes when the request did not
	// provide a FailureDestination. It is raw wire data alongside the error,
	// never an admitted Body. A supplied sink avoids this whole-value retention.
	FailureBody []byte
	Body        Body
	Metadata    ResponseMetadata
}

// Validate checks metadata and the decoded caller-owned body.
func (r JSONResponse[Body]) Validate() error {
	if err := r.Metadata.Validate(); err != nil {
		return err
	}
	if r.FailureBody != nil {
		return core.ErrExchangeResponse
	}
	if err := validateCallerValue(r.Body); err != nil {
		return responseError(err)
	}
	return nil
}

// BoundedResponse is one aggregate byte response.
type BoundedResponse struct {
	Body     []byte
	Metadata ResponseMetadata
}

// Validate checks metadata and byte accounting.
func (r BoundedResponse) Validate() error {
	if err := r.Metadata.Validate(); err != nil {
		return err
	}
	if r.Metadata.Bytes.Uint64() != uint64(len(r.Body)) {
		return responseError(core.ErrExchangeContract)
	}
	return nil
}

// StreamResponse reports one streaming HTTP response. For Download,
// Metadata.Bytes counts bytes acknowledged by the destination. For Upload,
// Metadata.Bytes counts only bytes acknowledged by an explicit failure sink;
// successful acknowledgments have zero bytes. It never proves request-body delivery.
// DeclaredRequestBytes retains Upload's declared request extent, never a peer
// acknowledgement. RequestLengthKnown distinguishes declared zero from unknown.
// Download leaves both fields zero.
type StreamResponse struct {
	Metadata             ResponseMetadata
	DeclaredRequestBytes core.ByteLength
	RequestLengthKnown   bool
}

// Validate checks the completed transfer metadata.
func (r StreamResponse) Validate() error {
	if err := r.Metadata.Validate(); err != nil {
		return err
	}
	if !r.RequestLengthKnown && r.DeclaredRequestBytes != (core.ByteLength{}) {
		return core.ErrExchangeContract
	}
	_, err := r.DeclaredRequestBytes.Int64()
	return err
}

// StreamRoundTripResponse reports both sides of one completed streamed HTTP
// exchange. Metadata.Bytes is the received response extent.
// DeclaredRequestBytes is the requested upload extent, not proof of delivery;
// Go may return an early response before consuming the request body.
type StreamRoundTripResponse struct {
	Metadata             ResponseMetadata
	DeclaredRequestBytes core.ByteLength
	RequestLengthKnown   bool
}

// Validate checks the completed request and response observations.
func (r StreamRoundTripResponse) Validate() error {
	if err := r.Metadata.Validate(); err != nil {
		return err
	}
	if !r.RequestLengthKnown && r.DeclaredRequestBytes != (core.ByteLength{}) {
		return core.ErrExchangeContract
	}
	_, err := r.DeclaredRequestBytes.Int64()
	return err
}

var (
	_ core.Validatable = ResponseMetadata{}
	_ core.Validatable = BoundedResponse{}
	_ core.Validatable = StreamResponse{}
	_ core.Validatable = StreamRoundTripResponse{}
)
