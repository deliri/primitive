package id

const (
	// UUIDv7JSONMaximumBytes bounds the complete input token, including whitespace.
	// It admits every canonical text byte as a six-byte JSON Unicode escape.
	UUIDv7JSONMaximumBytes = uuidTextBytes*len("\\u0000") + len("\"\"")
	// ULIDJSONMaximumBytes bounds the complete input token, including whitespace.
	// It admits every canonical text byte as a six-byte JSON Unicode escape.
	ULIDJSONMaximumBytes = ulidTextBytes*len("\\u0000") + len("\"\"")
)

const identityJSONLengthDiagnostic = "identity JSON exceeds its byte limit"
