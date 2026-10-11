package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"iter"
	"net/url"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Borrowed query facts are selected directly from one request. No map, sorted
// list, key cache or provider session survives an operation. Selection takes
// quadratic work and retains one candidate pair regardless of query count.
type v4QueryName string
type v4QueryValue string

type v4QueryPair struct {
	name    v4QueryName
	value   v4QueryValue
	ordinal int
}
type v4Timestamp string
type v4CredentialScope string
type v4Credential string
type v4SignedHeaders string

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
		if !yield(v4QueryPair{name: v4QueryName(core.SignatureV4QueryFieldCredential.String()), value: v4QueryValue(facts.credential), ordinal: 1}, nil) {
			return
		}
		if !yield(v4QueryPair{name: v4QueryName(core.SignatureV4QueryFieldDate.String()), value: v4QueryValue(facts.stamp), ordinal: 2}, nil) {
			return
		}
		if !yield(v4QueryPair{name: v4QueryName(core.SignatureV4QueryFieldSignedHeaders.String()), value: v4QueryValue(facts.signedHeaders), ordinal: 3}, nil) {
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
			name, value, _ := strings.Cut(part, core.SignatureV4ValueSeparator)
			name, err := url.QueryUnescape(name)
			if err != nil {
				yield(v4QueryPair{}, requestError(err))
				return
			}
			value, err = url.QueryUnescape(value)
			if err != nil {
				yield(v4QueryPair{}, requestError(err))
				return
			}
			switch v4QueryField(name) {
			case core.SignatureV4QueryFieldAlgorithm, core.SignatureV4QueryFieldCredential, core.SignatureV4QueryFieldDate, core.SignatureV4QueryFieldSignedHeaders:
				continue
			case core.SignatureV4QueryFieldSignature:
				yield(v4QueryPair{}, core.ErrExchangeRequest)
				return
			}
			if !yield(v4QueryPair{name: v4QueryName(name), value: v4QueryValue(value), ordinal: ordinal}, nil) {
				return
			}
			ordinal++
		}
	}
}

func v4QueryPairLess(left, right v4QueryPair) bool {
	leftName, rightName := v4QueryEncoding(string(left.name)), v4QueryEncoding(string(right.name))
	if leftName != rightName {
		return leftName < rightName
	}
	leftValue, rightValue := v4QueryEncoding(string(left.value)), v4QueryEncoding(string(right.value))
	if leftValue != rightValue {
		return leftValue < rightValue
	}
	return left.ordinal < right.ordinal
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
		v4WriteEscaped(&query, string(next.name), false)
		query.WriteByte('=')
		v4WriteEscaped(&query, string(next.value), false)
		previous = next
		hasPrevious = true
	}
}

func v4WriteEscaped(target io.Writer, text string, preserveSlash bool) {
	var escaped [3]byte
	escaped[0] = '%'
	for index := range len(text) {
		c := text[index]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~' || preserveSlash && c == '/' {
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
		default:
			for _, header := range intent.Headers.Values {
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
		}
		_, _ = io.WriteString(target, core.SignatureV4LineSeparator)
		previous = name
	}
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

func v4PresignedEndpoint(ctx context.Context, intent V4PresignRequest) (core.HTTPEndpoint, error) {
	stamp, err := intent.SignedAt.CompactUTCSeconds()
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	facts := v4SigningFacts{stamp: v4Timestamp(stamp), scope: v4CredentialScope(stamp[:8] + core.SignatureV4PathSeparator + intent.Region + core.SignatureV4PathSeparator + intent.Service + core.SignatureV4PathSeparator + core.SignatureV4ScopeTerminator), signedHeaders: v4SignedHeaders(v4SignedHeaderNames(intent))}
	facts.credential = v4Credential(string(intent.AccessKey) + core.SignatureV4PathSeparator + string(facts.scope))
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
	_, _ = io.WriteString(digest, string(facts.signedHeaders))
	_, _ = io.WriteString(digest, core.SignatureV4LineSeparator)
	_, _ = io.WriteString(digest, intent.PayloadHash)
	key := v4SecretDateMAC(intent.SecretKey, stamp[:8])
	key = v4MAC(key, intent.Region)
	key = v4MAC(key, intent.Service)
	key = v4MAC(key, core.SignatureV4ScopeTerminator)
	signature := v4MAC(key, core.SignatureV4Algorithm+core.SignatureV4LineSeparator+stamp+core.SignatureV4LineSeparator+string(facts.scope)+core.SignatureV4LineSeparator+hex.EncodeToString(digest.Sum(nil)))
	endpoint.RawQuery = query + core.SignatureV4QuerySeparator + core.SignatureV4SignatureQuery + core.SignatureV4ValueSeparator + hex.EncodeToString(signature)
	if err := contextstate.Validate(ctx); err != nil {
		return core.HTTPEndpoint{}, err
	}
	return core.ParseHTTPEndpoint(endpoint.String())
}
