package objectstore

import (
	"errors"
	"net/url"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	headerIfNoneMatch          = "If-None-Match"
	headerS3ChecksumCRC32C     = "X-Amz-Checksum-Crc32c"
	headerS3ChecksumMode       = "X-Amz-Checksum-Mode"
	headerS3ChecksumType       = "X-Amz-Checksum-Type"
	headerS3Version            = "X-Amz-Version-Id"
	headerGCSHash              = "X-Goog-Hash"
	headerGCSGenerationMatch   = "X-Goog-If-Generation-Match"
	headerGCSGeneration        = "X-Goog-Generation"
	headerContentRange         = "Content-Range"
	headerCreateOnlyValue      = "*"
	headerZeroValue            = "0"
	headerS3ChecksumModeValue  = "ENABLED"
	headerS3ChecksumComposite  = "COMPOSITE"
	headerS3ChecksumFullObject = "FULL_OBJECT"
	headerGCSChecksumPrefix    = "crc32c="
	headerHost                 = "Host"
	headerContentDisposition   = "Content-Disposition"
	headerRange                = "Range"
	queryS3Signature           = "X-Amz-Signature"
	queryS3SignedHeaders       = "X-Amz-SignedHeaders"
	queryGCSSignature          = "X-Goog-Signature"
	queryGCSSignedHeaders      = "X-Goog-SignedHeaders"
	// cloudflareImagesUploadHost is the published one-time upload host.
	cloudflareImagesUploadHost = "upload.imagedelivery.net"
	// signedHeaderTokenSeparator joins the vendor signed-header declaration.
	signedHeaderTokenSeparator = ";"
	// gcsHashComponentSeparator joins the components of one x-goog-hash value.
	gcsHashComponentSeparator = ","
	// headerWireSyntaxBytes is the ": " and CRLF framing each field adds to the
	// HTTP wire extent, beyond its name and value.
	headerWireSyntaxBytes = 4
	// automaticSignedHeaderMaximumCount is the widest set Objectstore and
	// Exchange place on one request without caller-supplied fields.
	automaticSignedHeaderMaximumCount   = 6
	signedHeaderDeclarationMaximumCount = SignedHeaderMaximumCount +
		automaticSignedHeaderMaximumCount
)

// signedHeaderDeclaration is the bounded canonical header-name set a V4
// capability says its signature covers. Both admitted V4 protocols require
// lowercase, strictly sorted, semicolon-separated names.
type signedHeaderDeclaration struct {
	values [signedHeaderDeclarationMaximumCount]core.HTTPHeaderName
	count  uint8
}

func parseSignedHeaderDeclaration(value string) (signedHeaderDeclaration, error) {
	var declaration signedHeaderDeclaration
	previous := ""
	for token := range strings.SplitSeq(value, signedHeaderTokenSeparator) {
		if token == "" || token != strings.ToLower(token) ||
			(previous != "" && token <= previous) ||
			int(declaration.count) >= len(declaration.values) {
			return signedHeaderDeclaration{}, core.ErrObjectStoreContract
		}
		name, err := headerName(token)
		if err != nil {
			return signedHeaderDeclaration{}, err
		}
		declaration.values[declaration.count] = name
		declaration.count++
		previous = token
	}
	if declaration.count == 0 {
		return signedHeaderDeclaration{}, core.ErrObjectStoreContract
	}
	return declaration, nil
}

func (d signedHeaderDeclaration) contains(name core.HTTPHeaderName) bool {
	for index := range int(d.count) {
		if d.values[index] == name {
			return true
		}
	}
	return false
}

func (d signedHeaderDeclaration) each(visitor func(core.HTTPHeaderName) bool) bool {
	for index := range int(d.count) {
		if !visitor(d.values[index]) {
			return false
		}
	}
	return true
}

func validateProviderSignedHeaders(
	provider Provider,
	target UploadTarget,
) error {
	if provider == ProviderCloudflareImages {
		return nil
	}
	signed, err := providerSignedHeaderDeclaration(
		provider,
		target.URL.value.Query(),
	)
	if err != nil {
		return err
	}
	required, err := providerRequiredSignedHeaders(provider)
	if err != nil {
		return err
	}
	for _, name := range required {
		if !semicolonTokenContains(signed, name) {
			return core.ErrObjectStoreContract
		}
	}
	return nil
}

type callerSignedHeaderValidation struct {
	value     url.URL
	headers   SignedHeaders
	provider  Provider
	direction Direction
}

func validateCallerSignedHeaders(validation callerSignedHeaderValidation) error {
	if validation.provider == ProviderCloudflareImages {
		if len(validation.headers.values) != 0 {
			return core.ErrObjectStoreContract
		}
		return nil
	}
	signed, err := providerSignedHeaderDeclaration(validation.provider, validation.value.Query())
	if err != nil {
		return err
	}
	declaration, err := parseSignedHeaderDeclaration(signed)
	if err != nil {
		return err
	}
	for _, header := range validation.headers.values {
		if !declaration.contains(header.name) {
			return core.ErrObjectStoreContract
		}
	}
	sent, err := sentRequestHeaderNames(validation.provider, validation.direction, validation.headers)
	if err != nil {
		return err
	}
	if !declaration.each(sent.contains) {
		return core.ErrObjectStoreContract
	}
	return nil
}

type sentHeaderNames struct {
	values [signedHeaderDeclarationMaximumCount]core.HTTPHeaderName
	count  uint8
}

func sentRequestHeaderNames(
	provider Provider,
	direction Direction,
	caller SignedHeaders,
) (sentHeaderNames, error) {
	if err := provider.Validate(); err != nil {
		return sentHeaderNames{}, err
	}
	if err := direction.Validate(); err != nil {
		return sentHeaderNames{}, err
	}
	var names sentHeaderNames
	names.add(core.HTTPHeaderAcceptEncoding())
	if err := names.addText(headerHost); err != nil {
		return sentHeaderNames{}, err
	}
	if err := addAutomaticDirectionHeaders(&names, provider, direction); err != nil {
		return sentHeaderNames{}, err
	}
	for _, header := range caller.values {
		names.add(header.name)
	}
	return names, nil
}

func addAutomaticDirectionHeaders(
	names *sentHeaderNames,
	provider Provider,
	direction Direction,
) error {
	if direction == DirectionDownload {
		names.add(core.HTTPHeaderAccept())
		if provider == ProviderAmazonS3 {
			return names.addText(headerS3ChecksumMode)
		}
		return nil
	}
	names.add(core.HTTPHeaderContentType())
	names.add(core.HTTPHeaderContentLength())
	required, err := providerRequiredSignedHeaders(provider)
	if err != nil {
		return err
	}
	for _, value := range required {
		if err := names.addText(value); err != nil {
			return err
		}
	}
	return nil
}

func (s *sentHeaderNames) addText(value string) error {
	name, err := headerName(value)
	if err != nil {
		return err
	}
	s.add(name)
	return nil
}

func (s *sentHeaderNames) add(name core.HTTPHeaderName) {
	if s.contains(name) {
		return
	}
	s.values[s.count] = name
	s.count++
}

func (s sentHeaderNames) contains(name core.HTTPHeaderName) bool {
	for index := range int(s.count) {
		if s.values[index] == name {
			return true
		}
	}
	return false
}

func providerSignedHeaderDeclaration(
	provider Provider,
	query url.Values,
) (string, error) {
	name, err := providerSignedHeadersQuery(provider)
	if err != nil {
		return "", err
	}
	value, ok := uniqueQueryValue(query, name)
	if !ok {
		return "", core.ErrObjectStoreContract
	}
	return value, nil
}

func uniqueQueryValue(query url.Values, name string) (string, bool) {
	if name == "" {
		return "", false
	}
	values := query[name]
	if len(values) != 1 || values[0] == "" {
		return "", false
	}
	return values[0], true
}

func validateDownloadSignedHeaders(
	provider Provider,
	target DownloadTarget,
) error {
	if provider != ProviderAmazonS3 {
		return nil
	}
	signed, err := providerSignedHeaderDeclaration(
		provider,
		target.URL.value.Query(),
	)
	if err != nil {
		return err
	}
	if !semicolonTokenContains(signed, headerS3ChecksumMode) {
		return core.ErrObjectStoreContract
	}
	return nil
}

// providerRequiredSignedHeaders returns the fields the vendor must have signed
// for Objectstore's own create-only and checksum request shape to be accepted.
func providerRequiredSignedHeaders(provider Provider) ([]string, error) {
	switch provider {
	case ProviderAmazonS3:
		return []string{headerIfNoneMatch, headerS3ChecksumCRC32C}, nil
	case ProviderGoogleCloudStorage:
		return []string{headerGCSGenerationMatch, headerGCSHash}, nil
	case ProviderCloudflareImages:
		return nil, nil
	case ProviderUnknown, providerLimit:
		return nil, core.ErrObjectStoreContract
	default:
		return nil, core.ErrObjectStoreContract
	}
}

func providerSignedHeadersQuery(provider Provider) (string, error) {
	switch provider {
	case ProviderAmazonS3:
		return queryS3SignedHeaders, nil
	case ProviderGoogleCloudStorage:
		return queryGCSSignedHeaders, nil
	case ProviderCloudflareImages, ProviderUnknown, providerLimit:
		return "", core.ErrObjectStoreContract
	default:
		return "", core.ErrObjectStoreContract
	}
}

// semicolonTokenContains reports whether the vendor declaration lists token as
// one complete component. An empty token never matches, so a declaration with
// an empty component cannot satisfy a lookup by accident.
func semicolonTokenContains(value, token string) bool {
	if token == "" {
		return false
	}
	for candidate := range strings.SplitSeq(value, signedHeaderTokenSeparator) {
		if strings.EqualFold(candidate, token) {
			return true
		}
	}
	return false
}

func uploadHeaders(
	provider Provider,
	target UploadTarget,
	integrity Integrity,
) (exchange.Headers, error) {
	headers, err := signedHeaders(target.Headers)
	if err != nil {
		return exchange.Headers{}, err
	}
	checksum, err := integrity.CRC32C.Base64()
	if err != nil {
		return exchange.Headers{}, errors.Join(
			core.ErrObjectStoreContract,
			err,
		)
	}
	var emitted []providerHeader
	switch provider {
	case ProviderAmazonS3:
		emitted = []providerHeader{
			{name: headerIfNoneMatch, value: headerCreateOnlyValue},
			{name: headerS3ChecksumCRC32C, value: checksum},
		}
	case ProviderGoogleCloudStorage:
		emitted = []providerHeader{
			{name: headerGCSGenerationMatch, value: headerZeroValue},
			{name: headerGCSHash, value: headerGCSChecksumPrefix + checksum},
		}
	case ProviderCloudflareImages:
	case ProviderUnknown, providerLimit:
		return exchange.Headers{}, core.ErrObjectStoreContract
	default:
		return exchange.Headers{}, core.ErrObjectStoreContract
	}
	return appendProviderHeaders(headers, emitted)
}

func downloadHeaders(
	provider Provider,
	target DownloadTarget,
) (exchange.Headers, error) {
	headers, err := signedHeaders(target.Headers)
	if err != nil {
		return exchange.Headers{}, err
	}
	if provider != ProviderAmazonS3 {
		return headers, nil
	}
	return appendProviderHeaders(headers, []providerHeader{
		{name: headerS3ChecksumMode, value: headerS3ChecksumModeValue},
	})
}

// providerHeader is one Objectstore-owned request field before it is parsed
// into a compiler-owned header name.
type providerHeader struct {
	name  string
	value string
}

func appendProviderHeaders(
	headers exchange.Headers,
	emitted []providerHeader,
) (exchange.Headers, error) {
	for _, header := range emitted {
		name, err := headerName(header.name)
		if err != nil {
			return exchange.Headers{}, err
		}
		value, err := exchange.NewHeaderValue(header.value)
		if err != nil {
			return exchange.Headers{}, errors.Join(core.ErrObjectStoreContract, err)
		}
		headers.Values = append(headers.Values, exchange.Header{
			Name: name, Values: []exchange.HeaderValue{value},
		})
	}
	return headers, headers.Validate()
}

func signedHeaders(headers SignedHeaders) (exchange.Headers, error) {
	values := make([]exchange.Header, len(headers.values))
	for index, header := range headers.values {
		value, err := exchange.NewHeaderValue(*header.value)
		if err != nil {
			return exchange.Headers{}, errors.Join(core.ErrObjectStoreContract, err)
		}
		values[index] = exchange.Header{
			Name: header.name, Values: []exchange.HeaderValue{value},
		}
	}
	result := exchange.Headers{Values: values}
	return result, result.Validate()
}

// responseSelection captures exactly the provider metadata Objectstore reads:
// the object identity for both directions, and on download the range marker
// plus the provider's own checksum for an independent comparison.
func responseSelection(
	provider Provider,
	direction Direction,
) (exchange.HeaderSelection, error) {
	if err := direction.Validate(); err != nil {
		return exchange.HeaderSelection{}, err
	}
	values, err := responseSelectionValues(provider, direction)
	if err != nil {
		return exchange.HeaderSelection{}, err
	}
	names := make([]core.HTTPHeaderName, 0, len(values))
	for _, value := range values {
		name, nameErr := headerName(value)
		if nameErr != nil {
			return exchange.HeaderSelection{}, nameErr
		}
		names = append(names, name)
	}
	return exchange.HeaderSelection{Names: names}, nil
}

func responseSelectionValues(
	provider Provider,
	direction Direction,
) ([]string, error) {
	values := make([]string, 0, 3)
	switch provider {
	case ProviderAmazonS3:
		values = append(values, headerS3Version)
		if direction == DirectionDownload {
			values = append(
				values,
				headerS3ChecksumCRC32C,
				headerS3ChecksumType,
			)
		}
	case ProviderGoogleCloudStorage:
		values = append(values, headerGCSGeneration)
		if direction == DirectionDownload {
			values = append(values, headerGCSHash)
		}
	case ProviderCloudflareImages:
	case ProviderUnknown, providerLimit:
		return nil, core.ErrObjectStoreContract
	default:
		return nil, core.ErrObjectStoreContract
	}
	if direction == DirectionDownload {
		values = append(values, headerContentRange)
	}
	return values, nil
}

func capturedVersion(
	headers exchange.CapturedHeaders,
	provider Provider,
) (ProviderVersion, bool, error) {
	if err := headers.Validate(); err != nil {
		return ProviderVersion{}, false, core.ErrObjectStoreContract
	}
	name, err := providerVersionHeader(provider)
	if err != nil || name == "" {
		return ProviderVersion{}, false, err
	}
	value, present, err := uniqueCapturedValue(headers, name)
	if err != nil || !present {
		return ProviderVersion{}, false, err
	}
	version, err := newProviderVersion(provider, value)
	if err != nil {
		return ProviderVersion{}, false, err
	}
	return version, true, nil
}

func providerVersionHeader(provider Provider) (string, error) {
	switch provider {
	case ProviderAmazonS3:
		return headerS3Version, nil
	case ProviderGoogleCloudStorage:
		return headerGCSGeneration, nil
	case ProviderCloudflareImages:
		return "", nil
	case ProviderUnknown, providerLimit:
		return "", core.ErrObjectStoreContract
	default:
		return "", core.ErrObjectStoreContract
	}
}

// providerDownloadCRC32C returns the checksum the provider computed over the
// object it served. It is an independent witness to Objectstore's own streaming
// CRC32C: the caller-supplied expected digests and the local digests are
// produced by the same algorithm, so a provider-side value is the only party
// that can disagree with both. Providers that stored an object without a
// checksum return none, and absence is not corruption.
func providerDownloadCRC32C(
	headers exchange.CapturedHeaders,
	provider Provider,
) (core.CRC32C, bool, error) {
	switch provider {
	case ProviderAmazonS3:
		return amazonS3DownloadCRC32C(headers)
	case ProviderGoogleCloudStorage:
		return googleCloudStorageDownloadCRC32C(headers)
	case ProviderCloudflareImages:
		return core.CRC32C{}, false, nil
	case ProviderUnknown, providerLimit:
		return core.CRC32C{}, false, core.ErrObjectStoreContract
	default:
		return core.CRC32C{}, false, core.ErrObjectStoreContract
	}
}

func amazonS3DownloadCRC32C(
	headers exchange.CapturedHeaders,
) (core.CRC32C, bool, error) {
	value, present, err := uniqueCapturedValue(headers, headerS3ChecksumCRC32C)
	if err != nil || !present {
		return core.CRC32C{}, false, err
	}
	checksumType, typed, err := amazonS3DownloadChecksumType(headers)
	if err != nil {
		return core.CRC32C{}, false, err
	}
	if typed && checksumType == amazonS3ChecksumTypeComposite {
		return core.CRC32C{}, false, nil
	}
	return parseProviderCRC32C(value)
}

type amazonS3ChecksumType uint8

const (
	amazonS3ChecksumTypeUnknown amazonS3ChecksumType = iota
	amazonS3ChecksumTypeComposite
	amazonS3ChecksumTypeFullObject
	amazonS3ChecksumTypeLimit
)

func amazonS3DownloadChecksumType(
	headers exchange.CapturedHeaders,
) (amazonS3ChecksumType, bool, error) {
	value, present, err := uniqueCapturedValue(headers, headerS3ChecksumType)
	if err != nil || !present {
		return amazonS3ChecksumTypeUnknown, false, err
	}
	switch value {
	case headerS3ChecksumComposite:
		return amazonS3ChecksumTypeComposite, true, nil
	case headerS3ChecksumFullObject:
		return amazonS3ChecksumTypeFullObject, true, nil
	default:
		return amazonS3ChecksumTypeUnknown, false, core.ErrObjectStoreIntegrity
	}
}

func googleCloudStorageDownloadCRC32C(
	headers exchange.CapturedHeaders,
) (core.CRC32C, bool, error) {
	value, present, err := uniqueCapturedValue(headers, headerGCSHash)
	if err != nil || !present {
		return core.CRC32C{}, false, err
	}
	encoded, present, err := googleCloudStorageHashComponent(value)
	if err != nil || !present {
		return core.CRC32C{}, false, err
	}
	return parseProviderCRC32C(encoded)
}

// googleCloudStorageHashComponent extracts the one crc32c component of an
// x-goog-hash value. A repeated component is contradictory metadata.
func googleCloudStorageHashComponent(value string) (string, bool, error) {
	found := ""
	count := 0
	for component := range strings.SplitSeq(value, gcsHashComponentSeparator) {
		trimmed := strings.TrimSpace(component)
		if !strings.HasPrefix(trimmed, headerGCSChecksumPrefix) {
			continue
		}
		found = strings.TrimPrefix(trimmed, headerGCSChecksumPrefix)
		count++
	}
	if count == 0 {
		return "", false, nil
	}
	if count != 1 {
		return "", false, core.ErrObjectStoreIntegrity
	}
	return found, true, nil
}

func parseProviderCRC32C(value string) (core.CRC32C, bool, error) {
	var checksum core.CRC32C
	err := checksum.UnmarshalText([]byte(value))
	if err != nil {
		return core.CRC32C{}, false, errors.Join(
			core.ErrObjectStoreIntegrity,
			err,
		)
	}
	return checksum, true, nil
}

// uniqueCapturedValue returns the single value captured for name. A repeated
// field or a multi-valued field is contradictory provider metadata.
func uniqueCapturedValue(
	headers exchange.CapturedHeaders,
	name string,
) (string, bool, error) {
	wanted, err := headerName(name)
	if err != nil {
		return "", false, err
	}
	found := ""
	count := 0
	for _, header := range headers.Values {
		if header.Name != wanted {
			continue
		}
		if len(header.Values) != 1 {
			return "", false, core.ErrObjectStoreContract
		}
		found, err = header.Values[0].Value()
		if err != nil {
			return "", false, errors.Join(core.ErrObjectStoreContract, err)
		}
		count++
	}
	if count == 0 {
		return "", false, nil
	}
	if count != 1 {
		return "", false, core.ErrObjectStoreContract
	}
	return found, true, nil
}

func capturedHeaderPresent(
	headers exchange.CapturedHeaders,
	name core.HTTPHeaderName,
) bool {
	for _, header := range headers.Values {
		if header.Name == name {
			return true
		}
	}
	return false
}

// headerName parses one Objectstore-owned protocol constant. A constant that
// does not parse would otherwise become the zero header name, which silently
// matches nothing and would disable the owned-field guard.
func headerName(value string) (core.HTTPHeaderName, error) {
	name, err := core.ParseHTTPHeaderName(value)
	if err != nil {
		return core.HTTPHeaderName{}, errors.Join(
			core.ErrObjectStoreContract,
			err,
		)
	}
	return name, nil
}

// multipartFileDisposition is the one part disposition Cloudflare Images
// direct creator upload accepts, projected from the owned field name.
func multipartFileDisposition() string {
	return `form-data; name="` + cloudflareImagesFormField +
		`"; filename="` + cloudflareImagesFormField + `"`
}
