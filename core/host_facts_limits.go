package core

// TimeZoneNameMaximumBytes bounds one explicit IANA database lookup name.
// The caller selects a zone; Hostfacts performs the native lookup. Sharing
// this ceiling lets callers validate intent before reaching the capability.
const TimeZoneNameMaximumBytes = 255
