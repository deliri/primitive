package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"iter"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Borrowed query facts are selected directly from one request. No map, sorted
// list, key cache or provider session survives an operation. Selection takes
// quadratic work and retains one candidate pair regardless of query count.
type v4QueryName string
type v4QueryValue string

// Query components are decoded native byte strings, not UTF-8 text domains.
// Go admits empty strings and every byte after percent decoding. The encoded
// ingress owns malformed-escape refusal; imposing decoded-value restrictions
// would change the canonical signature of an admitted URL.
func parseV4QueryName(encoded string) (v4QueryName, error) {
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return "", requestError(err)
	}
	return v4QueryName(decoded), nil
}

func parseV4QueryValue(encoded string) (v4QueryValue, error) {
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return "", requestError(err)
	}
	return v4QueryValue(decoded), nil
}

func (n v4QueryName) String() string  { return string(n) }
func (v v4QueryValue) String() string { return string(v) }

type v4QueryPair struct {
	name    v4QueryName
	value   v4QueryValue
	ordinal int
}
type v4Timestamp string
type v4CredentialScope string
type v4Credential string
type v4SignedHeaders string

func parseV4Timestamp(text string) (v4Timestamp, error) {
	value := v4Timestamp(text)
	if err := value.Validate(); err != nil {
		return "", err
	}
	return value, nil
}

func (v v4Timestamp) Validate() error {
	_, parseErr := temporal.ParseCompactUTC(v.String())
	if parseErr == nil {
		return nil
	}
	// CompactUTCSeconds deliberately drops fractional precision. At the
	// minimum signed-nanosecond instant its preceding complete second cannot
	// be represented by Instant, but is an actual owning producer output.
	minimum, err := temporal.InstantFromNanoseconds(math.MinInt64).CompactUTCSeconds()
	if err != nil {
		return requestError(err)
	}
	if v.String() == minimum {
		return nil
	}
	return requestError(parseErr)
}

func (v v4Timestamp) String() string { return string(v) }

func parseV4CredentialScope(text string) (v4CredentialScope, error) {
	value := v4CredentialScope(text)
	if err := value.Validate(); err != nil {
		return "", err
	}
	return value, nil
}

func (v v4CredentialScope) Validate() error {
	date, rest, datePresent := strings.Cut(v.String(), core.SignatureV4PathSeparator)
	region, rest, regionPresent := strings.Cut(rest, core.SignatureV4PathSeparator)
	service, terminal, servicePresent := strings.Cut(rest, core.SignatureV4PathSeparator)
	if !datePresent || !regionPresent || !servicePresent || len(date) != 8 ||
		region == "" || service == "" || terminal != core.SignatureV4ScopeTerminator ||
		strings.ContainsAny(region, "\r\n") || strings.ContainsAny(service, "\r\n") {
		return core.ErrExchangeRequest
	}
	_, err := temporal.ParseCivilDate(date[:4] + "-" + date[4:6] + "-" + date[6:])
	if err != nil {
		return requestError(err)
	}
	return nil
}

func (v v4CredentialScope) String() string { return string(v) }

func parseV4Credential(text string) (v4Credential, error) {
	value := v4Credential(text)
	if err := value.Validate(); err != nil {
		return "", err
	}
	return value, nil
}

func (v v4Credential) Validate() error {
	split := len(v.String())
	// The access-key bytes are borrowed and may themselves contain slashes.
	// Only the final four separators belong to the credential-scope grammar.
	for range 4 {
		split = strings.LastIndex(v.String()[:split], core.SignatureV4PathSeparator)
		if split < 0 {
			return core.ErrExchangeRequest
		}
	}
	if split == 0 {
		return core.ErrExchangeRequest
	}
	return v4CredentialScope(v.String()[split+1:]).Validate()
}

func (v v4Credential) String() string { return string(v) }

func parseV4SignedHeaders(text string) (v4SignedHeaders, error) {
	value := v4SignedHeaders(text)
	if err := value.Validate(); err != nil {
		return "", err
	}
	return value, nil
}

func (v v4SignedHeaders) Validate() error {
	previous, host := "", false
	for name := range strings.SplitSeq(v.String(), core.SignatureV4HeaderSeparator) {
		header, err := core.ParseHTTPHeaderName(name)
		if err != nil {
			return requestError(err)
		}
		if name != strings.ToLower(header.String()) || name <= previous || v4HeaderIgnored(name) {
			return core.ErrExchangeRequest
		}
		host = host || name == core.SignatureV4HostHeader
		previous = name
	}
	if !host {
		return core.ErrExchangeRequest
	}
	return nil
}

func (v v4SignedHeaders) String() string { return string(v) }

type v4SigningFacts struct {
	stamp         v4Timestamp
	scope         v4CredentialScope
	credential    v4Credential
	signedHeaders v4SignedHeaders
}

// Only this wire boundary classifies external names; canonical control flow
// operates on the closed signer-owned vocabulary.
func v4QueryField(name string) core.SignatureV4QueryField {
	switch name {
	case core.SignatureV4AlgorithmQuery:
		return core.SignatureV4QueryFieldAlgorithm
	case core.SignatureV4CredentialQuery:
		return core.SignatureV4QueryFieldCredential
	case core.SignatureV4DateQuery:
		return core.SignatureV4QueryFieldDate
	case core.SignatureV4SignedHeadersQuery:
		return core.SignatureV4QueryFieldSignedHeaders
	case core.SignatureV4SignatureQuery:
		return core.SignatureV4QueryFieldSignature
	default:
		return core.SignatureV4QueryFieldUnknown
	}
}

func v4QueryFacts(target url.URL, facts v4SigningFacts) iter.Seq2[v4QueryPair, error] {
	return func(yield func(v4QueryPair, error) bool) {
		if !yield(v4QueryPair{name: v4QueryName(core.SignatureV4QueryFieldAlgorithm.String()), value: v4QueryValue(core.SignatureV4Algorithm), ordinal: 0}, nil) {
			return
		}
		if !yield(v4QueryPair{name: v4QueryName(core.SignatureV4QueryFieldCredential.String()), value: v4QueryValue(facts.credential.String()), ordinal: 1}, nil) {
			return
		}
		if !yield(v4QueryPair{name: v4QueryName(core.SignatureV4QueryFieldDate.String()), value: v4QueryValue(facts.stamp.String()), ordinal: 2}, nil) {
			return
		}
		if !yield(v4QueryPair{name: v4QueryName(core.SignatureV4QueryFieldSignedHeaders.String()), value: v4QueryValue(facts.signedHeaders.String()), ordinal: 3}, nil) {
			return
		}
		ordinal := 4
		for part := range strings.SplitSeq(target.RawQuery, core.SignatureV4QuerySeparator) {
			if part == "" {
				continue
			}
			// Go Query silently discards malformed fields. A signing boundary instead
			// refuses incomplete canonical input rather than signing a partial request.
			if strings.Contains(part, core.SignatureV4HeaderSeparator) {
				yield(v4QueryPair{}, core.ErrExchangeRequest)
				return
			}
			encodedName, encodedValue, _ := strings.Cut(part, core.SignatureV4ValueSeparator)
			name, err := parseV4QueryName(encodedName)
			if err != nil {
				yield(v4QueryPair{}, err)
				return
			}
			value, err := parseV4QueryValue(encodedValue)
			if err != nil {
				yield(v4QueryPair{}, err)
				return
			}
			switch v4QueryField(name.String()) {
			case core.SignatureV4QueryFieldAlgorithm, core.SignatureV4QueryFieldCredential, core.SignatureV4QueryFieldDate, core.SignatureV4QueryFieldSignedHeaders:
				continue
			case core.SignatureV4QueryFieldSignature:
				yield(v4QueryPair{}, core.ErrExchangeRequest)
				return
			case core.SignatureV4QueryFieldUnknown:
				// Provider-owned names remain opaque data, not signer controls.
			default:
				yield(v4QueryPair{}, core.ErrExchangeRequest)
				return
			}
			if !yield(v4QueryPair{name: name, value: value, ordinal: ordinal}, nil) {
				return
			}
			ordinal++
		}
	}
}

func v4QueryPairLess(left, right v4QueryPair) bool {
	if order := v4QueryEncodingCompare(left.name.String(), right.name.String()); order != 0 {
		return order < 0
	}
	if order := v4QueryEncodingCompare(left.value.String(), right.value.String()); order != 0 {
		return order < 0
	}
	return left.ordinal < right.ordinal
}

// v4QueryEncodingCompare orders two texts by their canonical query encodings.
// Unreserved bytes encode to themselves, so two wholly unreserved texts compare
// directly; any other byte takes the complete encoding path. Selection
// compares each pair repeatedly, so the common case must not scan or rebuild
// both encodings on every comparison.
func v4QueryEncodingCompare(left, right string) int {
	if v4QueryUnreserved(left) && v4QueryUnreserved(right) {
		return strings.Compare(left, right)
	}
	return strings.Compare(v4QueryEncoding(left), v4QueryEncoding(right))
}

func v4QueryUnreserved(text string) bool {
	for index := range len(text) {
		if !v4UnreservedByte(text[index]) {
			return false
		}
	}
	return true
}

// v4UnreservedByte is the RFC 3986 unreserved set shared by Go's query
// escaping and the canonical escaper below.
func v4UnreservedByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~'
}

func v4QueryEncoding(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), core.SignatureV4FormSpace, core.SignatureV4SpaceEncoding)
}

func v4CanonicalQuery(ctx context.Context, target url.URL, facts v4SigningFacts) (string, error) {
	var query strings.Builder
	var previous v4QueryPair
	hasPrevious := false
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return "", err
		}
		var next v4QueryPair
		found := false
		for pair, err := range v4QueryFacts(target, facts) {
			if err != nil {
				return "", err
			}
			if hasPrevious && !v4QueryPairLess(previous, pair) {
				continue
			}
			if !found || v4QueryPairLess(pair, next) {
				next = pair
				found = true
			}
		}
		if !found {
			return query.String(), nil
		}
		if hasPrevious {
			query.WriteByte('&')
		}
		v4WriteEscaped(&query, next.name.String(), false)
		query.WriteByte('=')
		v4WriteEscaped(&query, next.value.String(), false)
		previous = next
		hasPrevious = true
	}
}

func v4WriteEscaped(target io.Writer, text string, preserveSlash bool) {
	var escaped [3]byte
	escaped[0] = '%'
	for index := range len(text) {
		c := text[index]
		if v4UnreservedByte(c) || preserveSlash && c == '/' {
			escaped[0] = c
			_, _ = target.Write(escaped[:1])
			escaped[0] = '%'
		} else {
			escaped[1] = core.SignatureV4EscapeHex[c>>4]
			escaped[2] = core.SignatureV4EscapeHex[c&15]
			_, _ = target.Write(escaped[:])
		}
	}
}

func v4SignedHeaderNames(intent V4PresignRequest) string {
	var names strings.Builder
	previous := ""
	for {
		next := v4NextHeaderName(intent, previous)
		if next == "" {
			return names.String()
		}
		if previous != "" {
			names.WriteByte(';')
		}
		names.WriteString(next)
		previous = next
	}
}

func v4HeaderIgnored(name string) bool {
	switch name {
	case core.SignatureV4AuthorizationHeader, core.SignatureV4UserAgentHeader, core.SignatureV4TraceHeader, core.SignatureV4ExpectHeader, core.SignatureV4TransferEncodingHeader:
		return true
	}
	return false
}

func v4NextHeaderName(intent V4PresignRequest, previous string) string {
	next := ""
	consider := func(name string) {
		if name > previous && (next == "" || name < next) {
			next = name
		}
	}
	consider(core.SignatureV4HostHeader)
	if !intent.ContentType.IsZero() {
		consider(core.SignatureV4ContentTypeHeader)
	}
	if !intent.ContentLength.IsZero() {
		consider(core.SignatureV4ContentLengthHeader)
	}
	for _, header := range intent.Headers.Values {
		name := strings.ToLower(header.Name.String())
		if !v4HeaderIgnored(name) {
			consider(name)
		}
	}
	return next
}

func v4WriteHeaderValue(target io.Writer, value string) {
	value = strings.TrimSpace(value)
	for index := range len(value) {
		if value[index] == ' ' && index > 0 && value[index-1] == ' ' {
			continue
		}
		_, _ = io.WriteString(target, value[index:index+1])
	}
}

func v4WriteCanonicalHeaders(target hash.Hash, intent V4PresignRequest) error {
	previous := ""
	for {
		name := v4NextHeaderName(intent, previous)
		if name == "" {
			return nil
		}
		_, _ = io.WriteString(target, name)
		_, _ = io.WriteString(target, core.SignatureV4HeaderValueSeparator)
		switch name {
		case core.SignatureV4HostHeader:
			endpoint := intent.Target.HTTPURL()
			host := endpoint.Host
			if endpoint.Port() == core.SignatureV4HTTPSPort {
				host = endpoint.Hostname()
			}
			v4WriteHeaderValue(target, host)
		case core.SignatureV4ContentTypeHeader:
			v4WriteHeaderValue(target, intent.ContentType.String())
		case core.SignatureV4ContentLengthHeader:
			_, _ = io.WriteString(target, strconv.FormatUint(intent.ContentLength.Bytes().Uint64(), 10))
		default:
			if err := v4WriteSelectedHeaderValues(target, intent.Headers, name); err != nil {
				return err
			}
		}
		_, _ = io.WriteString(target, core.SignatureV4LineSeparator)
		previous = name
	}
}

func v4WriteSelectedHeaderValues(target hash.Hash, headers Headers, name string) error {
	for _, header := range headers.Values {
		if strings.ToLower(header.Name.String()) != name {
			continue
		}
		for index, value := range header.Values {
			if index > 0 {
				_, _ = io.WriteString(target, core.SignatureV4ValueListSeparator)
			}
			text, err := value.Value()
			if err != nil {
				return err
			}
			v4WriteHeaderValue(target, text)
		}
	}
	return nil
}

func v4MAC(key []byte, message string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = io.WriteString(mac, message)
	return mac.Sum(nil)
}

// HMAC's native SHA-256 block size determines key normalization; it is not an
// input quota. Long credentials are hashed from borrowed bytes so prefixing
// does not clone their extent into a second secret string/byte inventory.
func v4SecretDateMAC(secret []byte, date string) []byte {
	var keyBlock [sha256.BlockSize]byte
	if len(secret) <= len(keyBlock)-len(core.SignatureV4KeyPrefix) {
		prefix := copy(keyBlock[:], core.SignatureV4KeyPrefix)
		copy(keyBlock[prefix:], secret)
		return v4MAC(keyBlock[:prefix+len(secret)], date)
	}
	digest := sha256.New()
	_, _ = io.WriteString(digest, core.SignatureV4KeyPrefix)
	_, _ = digest.Write(secret)
	var key [sha256.Size]byte
	return v4MAC(digest.Sum(key[:0]), date)
}

func newV4SigningFacts(intent V4PresignRequest) (v4SigningFacts, error) {
	stampText, err := intent.SignedAt.CompactUTCSeconds()
	if err != nil {
		return v4SigningFacts{}, err
	}
	stamp, err := parseV4Timestamp(stampText)
	if err != nil {
		return v4SigningFacts{}, err
	}
	scope, err := parseV4CredentialScope(stamp.String()[:8] + core.SignatureV4PathSeparator + intent.Region + core.SignatureV4PathSeparator + intent.Service + core.SignatureV4PathSeparator + core.SignatureV4ScopeTerminator)
	if err != nil {
		return v4SigningFacts{}, err
	}
	headers, err := parseV4SignedHeaders(v4SignedHeaderNames(intent))
	if err != nil {
		return v4SigningFacts{}, err
	}
	credential, err := parseV4Credential(string(intent.AccessKey) + core.SignatureV4PathSeparator + scope.String())
	if err != nil {
		return v4SigningFacts{}, err
	}
	return v4SigningFacts{stamp: stamp, scope: scope, credential: credential, signedHeaders: headers}, nil
}

func v4PresignedEndpoint(ctx context.Context, intent V4PresignRequest) (core.HTTPEndpoint, error) {
	facts, err := newV4SigningFacts(intent)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	endpoint := intent.Target.HTTPURL()
	query, err := v4CanonicalQuery(ctx, endpoint, facts)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	digest := sha256.New()
	_, _ = io.WriteString(digest, intent.Method.String())
	_, _ = io.WriteString(digest, core.SignatureV4LineSeparator)
	path := endpoint.EscapedPath()
	if path == "" {
		path = core.SignatureV4PathSeparator
	}
	if intent.DisableURIPathEscaping {
		_, _ = io.WriteString(digest, path)
	} else {
		v4WriteEscaped(digest, path, true)
	}
	_, _ = io.WriteString(digest, core.SignatureV4LineSeparator)
	_, _ = io.WriteString(digest, query)
	_, _ = io.WriteString(digest, core.SignatureV4LineSeparator)
	if err := v4WriteCanonicalHeaders(digest, intent); err != nil {
		return core.HTTPEndpoint{}, err
	}
	_, _ = io.WriteString(digest, core.SignatureV4LineSeparator)
	_, _ = io.WriteString(digest, facts.signedHeaders.String())
	_, _ = io.WriteString(digest, core.SignatureV4LineSeparator)
	_, _ = io.WriteString(digest, intent.PayloadHash)
	key := v4SecretDateMAC(intent.SecretKey, facts.stamp.String()[:8])
	key = v4MAC(key, intent.Region)
	key = v4MAC(key, intent.Service)
	key = v4MAC(key, core.SignatureV4ScopeTerminator)
	signature := v4MAC(key, core.SignatureV4Algorithm+core.SignatureV4LineSeparator+facts.stamp.String()+core.SignatureV4LineSeparator+facts.scope.String()+core.SignatureV4LineSeparator+hex.EncodeToString(digest.Sum(nil)))
	endpoint.RawQuery = query + core.SignatureV4QuerySeparator + core.SignatureV4SignatureQuery + core.SignatureV4ValueSeparator + hex.EncodeToString(signature)
	if err := contextstate.Validate(ctx); err != nil {
		return core.HTTPEndpoint{}, err
	}
	return core.ParseHTTPEndpoint(endpoint.String())
}
