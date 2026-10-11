package twilio

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

func webhookDigestField(ctx context.Context, source io.ReaderAt, field webhookFormField) (bool, error) {
	name := strings.NewReader(core.TwilioWebhookBodySHA256QueryName)
	order, err := compareWebhookSpans(ctx, source, field.Name, name, webhookByteSpan{Bytes: int64(name.Len())})
	return order == core.ComparisonEqual, err
}

func validateWebhookQuery(ctx context.Context, query string, requireDigest bool) (core.SHA256Digest, error) {
	source := strings.NewReader(query)
	var digest core.SHA256Digest
	found := false
	for record, err := range walkWebhookFields(ctx, source) {
		if err != nil {
			return core.SHA256Digest{}, bindingError(err)
		}
		field, err := splitWebhookField(source, record)
		if err != nil {
			return core.SHA256Digest{}, bindingError(err)
		}
		if err := errors.Join(writeDecodedWebhookSpan(ctx, source, field.Name, io.Discard), writeDecodedWebhookSpan(ctx, source, field.Value, io.Discard)); err != nil {
			return core.SHA256Digest{}, bindingError(err)
		}
		isDigest, err := webhookDigestField(ctx, source, field)
		if err != nil {
			return core.SHA256Digest{}, bindingError(err)
		}
		if !isDigest {
			continue
		}
		if !requireDigest || found {
			return core.SHA256Digest{}, core.ErrTwilioBinding
		}
		reader := decodedWebhookSpan(source, field.Value)
		var encoded [64]byte
		for index := range encoded {
			value, err := reader.ReadByte()
			if err != nil {
				return core.SHA256Digest{}, bindingError(err)
			}
			encoded[index] = value
		}
		if _, err := reader.ReadByte(); err != io.EOF {
			return core.SHA256Digest{}, bindingError(err)
		}
		if err := digest.UnmarshalText(encoded[:]); err != nil {
			return core.SHA256Digest{}, bindingError(err)
		}
		canonical, err := digest.Hex()
		if err != nil || string(encoded[:]) != canonical {
			return core.SHA256Digest{}, bindingError(err)
		}
		found = true
	}
	if requireDigest != found {
		return core.SHA256Digest{}, core.ErrTwilioBinding
	}
	return digest, nil
}

// Query values preserve their arrival order within each decoded name, matching
// Go's URL query semantics. Repeated scans borrow only source coordinates, never
// maps or field strings. Work is quadratic in field count plus comparison cost;
// retained memory is constant and no query or field quota is selected here.
func sameWebhookQueries(ctx context.Context, configured, observed string) (bool, error) {
	left, right := strings.NewReader(configured), strings.NewReader(observed)
	var leftCount, rightCount uint64
	for record, err := range walkWebhookFields(ctx, right) {
		if err != nil {
			return false, err
		}
		field, err := splitWebhookField(right, record)
		if err != nil {
			return false, err
		}
		isDigest, err := webhookDigestField(ctx, right, field)
		if err != nil {
			return false, err
		}
		if !isDigest {
			rightCount++
		}
	}
	for record, err := range walkWebhookFields(ctx, left) {
		if err != nil {
			return false, err
		}
		field, err := splitWebhookField(left, record)
		if err != nil {
			return false, err
		}
		leftCount++
		ordinal, err := webhookQueryOrdinal(ctx, left, field, int64(record.Offset))
		if err != nil {
			return false, err
		}
		matches, err := webhookQueryValueMatches(ctx, left, field, right, observed, ordinal)
		if err != nil || !matches {
			return false, err
		}
	}
	return leftCount == rightCount, nil
}

func webhookQueryOrdinal(ctx context.Context, source *strings.Reader, field webhookFormField, before int64) (uint64, error) {
	var ordinal uint64
	for record, err := range walkWebhookFields(ctx, io.NewSectionReader(source, 0, before)) {
		if err != nil {
			return 0, err
		}
		candidate, err := splitWebhookField(source, record)
		if err != nil {
			return 0, err
		}
		order, err := compareWebhookSpans(ctx, source, field.Name, source, candidate.Name)
		if err != nil {
			return 0, err
		}
		if order == core.ComparisonEqual {
			ordinal++
		}
	}
	return ordinal, nil
}

func webhookQueryValueMatches(ctx context.Context, left io.ReaderAt, field webhookFormField, right *strings.Reader, query string, ordinal uint64) (bool, error) {
	for record, err := range walkWebhookFields(ctx, io.NewSectionReader(right, 0, int64(len(query)))) {
		if err != nil {
			return false, err
		}
		candidate, err := splitWebhookField(right, record)
		if err != nil {
			return false, err
		}
		order, err := compareWebhookSpans(ctx, left, field.Name, right, candidate.Name)
		if err != nil {
			return false, err
		}
		if order != core.ComparisonEqual {
			continue
		}
		if ordinal != 0 {
			ordinal--
			continue
		}
		order, err = compareWebhookSpans(ctx, left, field.Value, right, candidate.Value)
		return order == core.ComparisonEqual, err
	}
	return false, nil
}
