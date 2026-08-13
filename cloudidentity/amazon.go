package cloudidentity

import (
	"context"
	"encoding/xml"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

const (
	amazonActionQuery           = "Action"
	amazonActionValue           = "GetWebIdentityToken"
	amazonVersionQuery          = "Version"
	amazonVersionValue          = "2011-06-15"
	amazonAudienceQuery         = "Audience.member.1"
	amazonSigningAlgorithmQuery = "SigningAlgorithm"
	amazonSigningAlgorithmValue = "RS256"
	amazonDurationQuery         = "DurationSeconds"
	amazonDurationValue         = "300"
	amazonSigAlgorithmQuery     = "X-Amz-Algorithm"
	amazonSigAlgorithmValue     = "AWS4-HMAC-SHA256"
	// #nosec G101 -- AWS-published SigV4 query name, not a credential value.
	amazonCredentialQuery    = "X-Amz-Credential"
	amazonDateQuery          = "X-Amz-Date"
	amazonExpiresQuery       = "X-Amz-Expires"
	amazonSignedHeadersQuery = "X-Amz-SignedHeaders"
	amazonSignedHeadersValue = "host"
	amazonSignatureQuery     = "X-Amz-Signature"
	// #nosec G101 -- AWS-published SigV4 query name, not a token value.
	amazonSecurityTokenQuery   = "X-Amz-Security-Token"
	amazonCredentialService    = "sts"
	amazonCredentialTerminal   = "aws4_request"
	amazonDateLayout           = "20060102T150405Z"
	amazonSignedURLMaximumSecs = 300
	// amazonResponseNamespace is the XML namespace AWS publishes for the STS
	// API version this package signs for. It is composed from the same version
	// constant the request carries, so the two cannot name different versions.
	amazonResponseNamespace = "https://sts.amazonaws.com/doc/" +
		amazonVersionValue + "/"
	// amazonSingleElement is the exact number of times each element of the
	// published envelope may appear.
	amazonSingleElement = 1
	// AmazonEnvelopeMaximumBytes bounds the published XML envelope that
	// surrounds one token.
	AmazonEnvelopeMaximumBytes = 4 * 1024
	// AmazonResponseMaximumBytes bounds one complete AWS STS response: the
	// common token bound plus the envelope that carries it.
	AmazonResponseMaximumBytes = TokenMaximumBytes + AmazonEnvelopeMaximumBytes
)

// amazonResponse is the published AWS STS GetWebIdentityToken envelope. Both
// nested elements decode as slices so a document carrying more than one of
// either is refused rather than silently resolving to whichever copy the
// decoder read last.
type amazonResponse struct {
	XMLName xml.Name       `xml:"GetWebIdentityTokenResponse"`
	Results []amazonResult `xml:"GetWebIdentityTokenResult"`
}

type amazonResult struct {
	XMLName xml.Name             `xml:"GetWebIdentityTokenResult"`
	Tokens  []amazonTokenElement `xml:"WebIdentityToken"`
}

type amazonUnexpectedElement struct {
	XMLName xml.Name
}

type amazonTokenElement struct {
	XMLName    xml.Name                  `xml:"WebIdentityToken"`
	Value      string                    `xml:",chardata"`
	Unexpected []amazonUnexpectedElement `xml:",any"`
}

// amazonToken projects the one token a published response carries.
//
// The namespace is checked because encoding/xml matches an unqualified element
// name in any namespace: without this, a document produced by a different STS
// API version, or by no AWS API at all, satisfies a request that pinned
// amazonVersionValue.
func amazonToken(document amazonResponse) (string, error) {
	if document.XMLName.Space != amazonResponseNamespace ||
		len(document.Results) != amazonSingleElement ||
		document.Results[0].XMLName.Space != amazonResponseNamespace ||
		len(document.Results[0].Tokens) != amazonSingleElement ||
		document.Results[0].Tokens[0].XMLName.Space !=
			amazonResponseNamespace ||
		len(document.Results[0].Tokens[0].Unexpected) != 0 {
		return "", core.ErrCloudIdentityContract
	}
	return document.Results[0].Tokens[0].Value, nil
}

// amazonQueryField is the closed set of query fields one signed regional STS
// GetWebIdentityToken capability may carry. The domain is the single authority
// for membership and for required-field arithmetic, so admitting a new field
// cannot leave a copied count behind.
type amazonQueryField uint8

const (
	amazonQueryFieldUnknown amazonQueryField = iota
	amazonQueryFieldAction
	amazonQueryFieldVersion
	amazonQueryFieldAudience
	amazonQueryFieldSigningAlgorithm
	amazonQueryFieldDuration
	amazonQueryFieldSignatureAlgorithm
	amazonQueryFieldCredential
	amazonQueryFieldDate
	amazonQueryFieldExpires
	amazonQueryFieldSignedHeaders
	amazonQueryFieldSignature
	amazonQueryFieldSecurityToken
	amazonQueryFieldLimit
)

const (
	// amazonQueryFieldCount is the size of the closed query domain, derived
	// from the domain itself so no copied count can drift from it.
	amazonQueryFieldCount = int(amazonQueryFieldLimit) - 1
)

// validate closes the query-field domain.
func (f amazonQueryField) validate() error {
	if f <= amazonQueryFieldUnknown || f >= amazonQueryFieldLimit ||
		amazonQueryFieldNames()[f] == "" {
		return core.ErrCloudIdentityContract
	}
	return nil
}

// name returns the exact AWS-published query name.
func (f amazonQueryField) name() string {
	if f >= amazonQueryFieldLimit {
		return ""
	}
	return amazonQueryFieldNames()[f]
}

func amazonQueryFieldNames() [amazonQueryFieldLimit]string {
	return [...]string{
		"",
		amazonActionQuery,
		amazonVersionQuery,
		amazonAudienceQuery,
		amazonSigningAlgorithmQuery,
		amazonDurationQuery,
		amazonSigAlgorithmQuery,
		amazonCredentialQuery,
		amazonDateQuery,
		amazonExpiresQuery,
		amazonSignedHeadersQuery,
		amazonSignatureQuery,
		amazonSecurityTokenQuery,
	}
}

// optional reports whether a valid capability may omit the field. Only the
// session security token is optional, because a long-lived signing identity
// produces no session token to carry.
func (f amazonQueryField) optional() bool {
	return f == amazonQueryFieldSecurityToken
}

// parseAmazonQueryField admits one query name into the closed domain.
func parseAmazonQueryField(value string) (amazonQueryField, error) {
	for field := amazonQueryFieldAction; field < amazonQueryFieldLimit; field++ {
		if field.name() == value {
			return field, nil
		}
	}
	return amazonQueryFieldUnknown, core.ErrCloudIdentityContract
}

// AcquireAmazonWebServices submits one caller-signed regional AWS STS
// GetWebIdentityToken request and returns its opaque bearer.
//
// Ingress refusals are returned as themselves, because an invalid client or
// capability is a caller defect that names no request material. Every failure
// from the outbound effect onward is wrapped in the redacting AWS request error,
// so no step can print the signed capability by omission.
func AcquireAmazonWebServices(
	ctx context.Context,
	client Client,
	request AmazonWebServicesRequest,
) (Token, error) {
	if err := validateAcquisition(client, request); err != nil {
		return Token{}, err
	}
	limit, err := amazonResponseLimit()
	if err != nil {
		return Token{}, err
	}
	response, err := acquire(acquisitionCall{
		context:       ctx,
		client:        client,
		target:        *request.endpoint,
		responseLimit: limit,
		policy:        request.request.Policy,
	})
	if err != nil {
		return Token{}, amazonFailure(err)
	}
	return amazonResponseToken(response.Body)
}

func amazonResponseToken(body []byte) (Token, error) {
	var document amazonResponse
	if err := xml.Unmarshal(body, &document); err != nil {
		return Token{}, amazonFailure(contractError(err))
	}
	value, err := amazonToken(document)
	if err != nil {
		return Token{}, amazonFailure(err)
	}
	token, err := newToken(ProviderAmazonWebServices, value)
	if err != nil {
		return Token{}, amazonFailure(err)
	}
	return token, nil
}

func amazonResponseLimit() (core.ByteCount, error) {
	limit, err := core.NewByteCount(AmazonResponseMaximumBytes)
	if err != nil {
		return core.ByteCount{}, contractError(err)
	}
	return limit, nil
}

func validateAmazonWebServicesEndpoint(
	target url.URL,
	audience Audience,
) error {
	region, ok := amazonSTSRegion(target)
	if !ok {
		return core.ErrCloudIdentityContract
	}
	query := target.Query()
	if !validAmazonActionQuery(query, audience) ||
		!validAmazonSignatureQuery(query, region) ||
		!exactAmazonQueryDomain(query) {
		return core.ErrCloudIdentityContract
	}
	return nil
}

func amazonSTSRegion(target url.URL) (string, bool) {
	if target.Scheme != core.SchemeHTTPS ||
		target.Port() != "" ||
		(target.EscapedPath() != "" && target.EscapedPath() != "/") {
		return "", false
	}
	host := strings.ToLower(target.Hostname())
	for _, prefix := range [...]string{"sts.", "sts-fips."} {
		if !strings.HasPrefix(host, prefix) {
			continue
		}
		remainder := strings.TrimPrefix(host, prefix)
		for _, suffix := range [...]string{
			".api.amazonwebservices.com.cn",
			".amazonaws.com.cn",
			".amazonaws.com",
			".api.aws",
		} {
			region, found := strings.CutSuffix(remainder, suffix)
			if found && validAmazonRegion(region) {
				return region, true
			}
		}
	}
	return "", false
}

func validAmazonRegion(region string) bool {
	if region == "" || strings.Contains(region, ".") {
		return false
	}
	for index := range len(region) {
		value := region[index]
		if value >= 'a' && value <= 'z' ||
			value >= '0' && value <= '9' ||
			value == '-' {
			continue
		}
		return false
	}
	return true
}

func validAmazonActionQuery(
	query url.Values,
	audience Audience,
) bool {
	return exactQueryValue(query, amazonActionQuery, amazonActionValue) &&
		exactQueryValue(query, amazonVersionQuery, amazonVersionValue) &&
		exactQueryValue(query, amazonAudienceQuery, audience.String()) &&
		exactQueryValue(
			query,
			amazonSigningAlgorithmQuery,
			amazonSigningAlgorithmValue,
		) &&
		exactQueryValue(query, amazonDurationQuery, amazonDurationValue)
}

func validAmazonSignatureQuery(
	query url.Values,
	region string,
) bool {
	credential, ok := uniqueQueryValue(query, amazonCredentialQuery)
	if !ok || !validAmazonCredential(credential, region) {
		return false
	}
	date, ok := uniqueQueryValue(query, amazonDateQuery)
	if !ok || !validAmazonDate(date, credential) {
		return false
	}
	expires, ok := uniqueQueryValue(query, amazonExpiresQuery)
	if !ok || !validAmazonExpiry(expires) {
		return false
	}
	signature, ok := uniqueQueryValue(query, amazonSignatureQuery)
	return ok &&
		exactQueryValue(
			query,
			amazonSigAlgorithmQuery,
			amazonSigAlgorithmValue,
		) &&
		exactQueryValue(
			query,
			amazonSignedHeadersQuery,
			amazonSignedHeadersValue,
		) &&
		validAmazonSignature(signature)
}

func validAmazonCredential(value, region string) bool {
	parts := strings.Split(value, "/")
	return len(parts) == 5 &&
		parts[0] != "" &&
		len(parts[1]) == 8 &&
		allDecimal(parts[1]) &&
		parts[2] == region &&
		parts[3] == amazonCredentialService &&
		parts[4] == amazonCredentialTerminal
}

func validAmazonDate(value, credential string) bool {
	credentialParts := strings.Split(credential, "/")
	if len(credentialParts) != 5 || len(value) < 8 {
		return false
	}
	parsed, err := time.Parse(amazonDateLayout, value)
	return err == nil &&
		parsed.Format(amazonDateLayout) == value &&
		credentialParts[1] == value[:8]
}

func validAmazonExpiry(value string) bool {
	seconds, err := strconv.ParseUint(value, 10, 64)
	return err == nil &&
		strconv.FormatUint(seconds, 10) == value &&
		seconds > 0 &&
		seconds <= amazonSignedURLMaximumSecs
}

func validAmazonSignature(value string) bool {
	const signatureBytes = 32
	decoded := make([]byte, signatureBytes)
	return core.DecodeCanonicalHex(decoded, value) == nil
}

// exactAmazonQueryDomain admits only fields of the closed query domain. Every
// required field is separately proved present exactly once by the action and
// signature checks, so membership alone closes the capability: no count
// arithmetic is copied here to drift from the domain.
func exactAmazonQueryDomain(query url.Values) bool {
	for name := range query {
		field, err := parseAmazonQueryField(name)
		if err != nil {
			return false
		}
		if !field.optional() {
			continue
		}
		if value, ok := uniqueQueryValue(query, name); !ok || value == "" {
			return false
		}
	}
	return true
}

func exactQueryValue(
	query url.Values,
	name string,
	expected string,
) bool {
	value, ok := uniqueQueryValue(query, name)
	return ok && value == expected
}

func uniqueQueryValue(
	query url.Values,
	name string,
) (string, bool) {
	values := query[name]
	if len(values) != 1 || values[0] == "" {
		return "", false
	}
	return values[0], true
}

func allDecimal(value string) bool {
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}
