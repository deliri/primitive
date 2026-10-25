package core

// Signature V4 wire vocabulary is mechanical protocol grammar, shared by
// canonical hashing and request publication. Provider scope remains supplied.
const (
	SignatureV4Algorithm              = "AWS4-HMAC-SHA256"
	SignatureV4KeyPrefix              = "AWS4"
	SignatureV4ScopeTerminator        = "aws4_request"
	SignatureV4AlgorithmQuery         = "X-Amz-Algorithm"
	SignatureV4CredentialQuery        = "X-Amz-Credential"
	SignatureV4DateQuery              = "X-Amz-Date"
	SignatureV4SignedHeadersQuery     = "X-Amz-SignedHeaders"
	SignatureV4SignatureQuery         = "X-Amz-Signature"
	SignatureV4HostHeader             = "host"
	SignatureV4ContentTypeHeader      = "content-type"
	SignatureV4ContentLengthHeader    = "content-length"
	SignatureV4AuthorizationHeader    = "authorization"
	SignatureV4UserAgentHeader        = "user-agent"
	SignatureV4TraceHeader            = "x-amzn-trace-id"
	SignatureV4ExpectHeader           = "expect"
	SignatureV4TransferEncodingHeader = "transfer-encoding"
	SignatureV4HTTPSPort              = "443"
	SignatureV4EscapeHex              = "0123456789ABCDEF"
)

// SignatureV4QueryField is the closed set of signer-owned query members.
// Caller-owned query names are separate parsed request facts.
type SignatureV4QueryField uint8

// Signer-owned query member identities exclude caller-defined query names.
const (
	SignatureV4QueryFieldUnknown SignatureV4QueryField = iota
	SignatureV4QueryFieldAlgorithm
	SignatureV4QueryFieldCredential
	SignatureV4QueryFieldDate
	SignatureV4QueryFieldSignedHeaders
	SignatureV4QueryFieldSignature
)

// Validate rejects unknown and out-of-domain member identities.
func (field SignatureV4QueryField) Validate() error {
	if field < SignatureV4QueryFieldAlgorithm || field > SignatureV4QueryFieldSignature {
		return ErrExchangeContract
	}
	return nil
}

// OffWireEnum marks internal control vocabulary rather than persisted numbers.
func (SignatureV4QueryField) OffWireEnum() {}

// String projects the canonical protocol member spelling.
func (field SignatureV4QueryField) String() string {
	switch field {
	case SignatureV4QueryFieldAlgorithm:
		return SignatureV4AlgorithmQuery
	case SignatureV4QueryFieldCredential:
		return SignatureV4CredentialQuery
	case SignatureV4QueryFieldDate:
		return SignatureV4DateQuery
	case SignatureV4QueryFieldSignedHeaders:
		return SignatureV4SignedHeadersQuery
	case SignatureV4QueryFieldSignature:
		return SignatureV4SignatureQuery
	default:
		return ""
	}
}

// Signature V4 delimiters are fixed wire grammar, not caller control strings.
const (
	SignatureV4QuerySeparator       = "&"
	SignatureV4ValueSeparator       = "="
	SignatureV4HeaderSeparator      = ";"
	SignatureV4ValueListSeparator   = ","
	SignatureV4HeaderValueSeparator = ":"
	SignatureV4LineSeparator        = "\n"
	SignatureV4PathSeparator        = "/"
	SignatureV4SpaceEncoding        = "%20"
	SignatureV4FormSpace            = "+"
)
