package gmail

import "errors"

// SentEvidenceFailure names the failed Sent evidence boundary without retaining provider data.
type SentEvidenceFailure uint8

const (
	sentEvidenceFailureNone SentEvidenceFailure = iota
	// SentEvidenceFetch is a failed exact message fetch.
	SentEvidenceFetch
	// SentEvidenceMetadata is a malformed provider metadata response.
	SentEvidenceMetadata
	// SentEvidenceMessage marks a provider message ID mismatch.
	SentEvidenceMessage
	// SentEvidenceThread marks a provider thread ID mismatch.
	SentEvidenceThread
	// SentEvidenceSentLabel means the response does not attest a sent message.
	SentEvidenceSentLabel
	// SentEvidenceFrom marks a sender mismatch.
	SentEvidenceFrom
	// SentEvidenceRecipients marks a recipient mismatch.
	SentEvidenceRecipients
	// SentEvidenceMessageIDHeader marks an invalid or mismatched Internet Message-ID.
	SentEvidenceMessageIDHeader
	// SentEvidenceDigestHeader marks an invalid or mismatched content digest header.
	SentEvidenceDigestHeader
	// SentEvidenceSearch is a failed exact legacy attempt lookup.
	SentEvidenceSearch
	// SentEvidenceAccess is a refused read grant.
	SentEvidenceAccess
	sentEvidenceFailureLimit
)

// IsValid reports whether the value names a defined failure.
func (f SentEvidenceFailure) IsValid() bool {
	return f > sentEvidenceFailureNone && f < sentEvidenceFailureLimit
}

// String reports the stable diagnostic class without provider data.
func (f SentEvidenceFailure) String() string {
	labels := [...]string{
		sentEvidenceFailureNone:     "gmail_evidence_unknown",
		SentEvidenceFetch:           "gmail_evidence_fetch",
		SentEvidenceMetadata:        "gmail_evidence_metadata",
		SentEvidenceMessage:         "gmail_evidence_message",
		SentEvidenceThread:          "gmail_evidence_thread",
		SentEvidenceSentLabel:       "gmail_evidence_sent_label",
		SentEvidenceFrom:            "gmail_evidence_from",
		SentEvidenceRecipients:      "gmail_evidence_recipients",
		SentEvidenceMessageIDHeader: "gmail_evidence_message_id_header",
		SentEvidenceDigestHeader:    "gmail_evidence_digest_header",
		SentEvidenceSearch:          "gmail_evidence_search",
		SentEvidenceAccess:          "gmail_evidence_access",
	}
	if !f.IsValid() {
		return labels[sentEvidenceFailureNone]
	}
	return labels[f]
}

// SentFailureOf extracts the closed diagnostic from a failed evidence read.
func SentFailureOf(err error) SentEvidenceFailure {
	var failure sentEvidenceError
	if errors.As(err, &failure) {
		return failure.class
	}
	return sentEvidenceFailureNone
}

type sentEvidenceError struct {
	class SentEvidenceFailure
	cause error
}

func (e sentEvidenceError) Error() string { return e.class.String() }
func (e sentEvidenceError) Unwrap() error { return e.cause }

func (sentEvidenceError) gmailInternalFlow() {}
