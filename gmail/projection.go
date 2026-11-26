package gmail

import (
	"encoding/base64"
	"errors"
	"math"
	"mime"
	"net/mail"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type threadReferenceWire struct {
	ID ThreadID `json:"id"`
}

type threadPageWire struct {
	Threads []threadReferenceWire `json:"threads"`
	Next    PageToken             `json:"nextPageToken"`
}

func (w threadPageWire) project(request ThreadListRequest) (ThreadPage, error) {
	if len(w.Threads) > int(request.Limit) || w.Next != "" && w.Next == request.After {
		return ThreadPage{}, core.ErrGmailResponse
	}
	page := ThreadPage{Threads: make([]ThreadID, 0, len(w.Threads)), Next: w.Next}
	for _, ref := range w.Threads {
		page.Threads = append(page.Threads, ref.ID)
	}
	if err := page.Validate(); err != nil {
		return ThreadPage{}, errors.Join(core.ErrGmailResponse, err)
	}
	return page, nil
}

type threadWire struct {
	ID       ThreadID            `json:"id"`
	Messages []messageHeaderWire `json:"messages"`
}

type messageHeaderWire struct {
	ID      MessageID        `json:"id"`
	Thread  ThreadID         `json:"threadId"`
	Date    internalDateWire `json:"internalDate"`
	Labels  []labelWire      `json:"labelIds"`
	Payload metadataWire     `json:"payload"`
}

type metadataWire struct {
	Headers []headerWire `json:"headers"`
}

type headerWire struct {
	Name  headerNameWire  `json:"name"`
	Value headerValueWire `json:"value"`
}

type internalDateWire string
type labelWire string
type headerNameWire string
type headerValueWire string

const (
	labelDraft  labelWire = "DRAFT"
	labelUnread labelWire = "UNREAD"
)

func (w threadWire) project(request ThreadID) (Thread, error) {
	if w.ID != request {
		return Thread{}, core.ErrGmailBinding
	}
	if len(w.Messages) > ThreadMessageMaximum {
		return Thread{}, errors.Join(core.ErrGmailResponse, core.ErrExchangeBodyLimit)
	}
	out := Thread{ID: w.ID, Messages: make([]MessageHeader, 0, len(w.Messages))}
	for _, row := range w.Messages {
		if row.Thread != w.ID {
			return Thread{}, core.ErrGmailBinding
		}
		if slices.Contains(row.Labels, labelDraft) {
			continue
		}
		header, err := row.project()
		if err != nil {
			return Thread{}, err
		}
		out.Messages = append(out.Messages, header)
	}
	if err := out.Validate(); err != nil {
		return Thread{}, errors.Join(core.ErrGmailResponse, err)
	}
	return out, nil
}

func (w messageHeaderWire) project() (MessageHeader, error) {
	if len(w.Payload.Headers) > 32 || len(w.Labels) > 256 {
		return MessageHeader{}, core.ErrGmailResponse
	}
	at, err := projectDate(w.Date)
	if err != nil {
		return MessageHeader{}, err
	}
	out := MessageHeader{ID: w.ID, At: at, Unread: slices.Contains(w.Labels, labelUnread)}
	var seenFrom, seenSubject bool
	for _, header := range w.Payload.Headers {
		switch {
		case strings.EqualFold(string(header.Name), "From"):
			if seenFrom {
				return MessageHeader{}, core.ErrGmailResponse
			}
			seenFrom = true
			out.Sender, err = projectSender(header.Value)
		case strings.EqualFold(string(header.Name), "Subject"):
			if seenSubject {
				return MessageHeader{}, core.ErrGmailResponse
			}
			seenSubject = true
			var subject string
			subject, err = new(mime.WordDecoder).DecodeHeader(string(header.Value))
			out.Subject = Subject(subject)
		}
		if err != nil {
			return MessageHeader{}, core.ErrGmailResponse
		}
	}
	if err := out.Validate(); err != nil {
		return MessageHeader{}, errors.Join(core.ErrGmailResponse, err)
	}
	return out, nil
}

func projectSender(value headerValueWire) (core.OAuthEmail, error) {
	if len(value) > subjectMaximumBytes {
		return core.OAuthEmail{}, core.ErrGmailResponse
	}
	sender, err := mail.ParseAddress(string(value))
	if err != nil {
		// Multi-author, legacy or malformed From is not a usable match.
		return core.OAuthEmail{}, nil
	}
	address, err := core.ParseOAuthEmail(sender.Address)
	if err != nil {
		return core.OAuthEmail{}, core.ErrGmailResponse
	}
	return address, nil
}

func projectDate(raw internalDateWire) (temporal.Instant, error) {
	millis, err := strconv.ParseUint(string(raw), 10, 64)
	if err != nil || millis == 0 || millis > math.MaxInt64/temporal.NanosecondsPerMillisecond || strconv.FormatUint(millis, 10) != string(raw) {
		return temporal.Instant{}, core.ErrGmailResponse
	}
	return temporal.InstantFromNanoseconds(int64(millis * temporal.NanosecondsPerMillisecond)), nil
}

type messageWire struct {
	Labels  []labelWire  `json:"labelIds"`
	ID      MessageID    `json:"id"`
	Thread  ThreadID     `json:"threadId"`
	Payload mimePartWire `json:"payload"`
}

type mimePartWire struct {
	Type     mimeTypeWire   `json:"mimeType"`
	Filename fileNameWire   `json:"filename"`
	Body     mimeBodyWire   `json:"body"`
	Parts    []mimePartWire `json:"parts"`
}

type mimeBodyWire struct {
	Data base64URLWire `json:"data"`
}

type mimeTypeWire string
type fileNameWire string
type base64URLWire string

func (w messageWire) project(request MessageReadRequest) (Message, error) {
	if w.ID != request.Message || w.Thread != request.Thread || slices.Contains(w.Labels, labelDraft) {
		return Message{}, core.ErrGmailBinding
	}
	text, kind, err := w.Payload.plainText(0)
	if err != nil {
		return Message{}, err
	}
	out := Message{ID: w.ID, Thread: w.Thread, Kind: kind, Text: text}
	if err := out.Validate(); err != nil {
		return Message{}, errors.Join(core.ErrGmailResponse, err)
	}
	return out, nil
}

func (part mimePartWire) plainText(depth int) (PlainText, BodyKind, error) {
	if part.Filename != "" {
		return "", BodyUnavailable, nil
	}
	switch part.Type {
	case "text/plain":
		return part.Body.Data.plainText()
	case "multipart/alternative", "multipart/mixed", "multipart/related", "multipart/signed":
		if depth < 3 {
			for _, child := range part.Parts {
				text, kind, err := child.plainText(depth + 1)
				if kind != BodyUnavailable || err != nil {
					return text, kind, err
				}
			}
		}
	}
	return "", BodyUnavailable, nil
}

func (data base64URLWire) plainText() (PlainText, BodyKind, error) {
	if data == "" {
		return "", BodyUnavailable, nil
	}
	if len(data) > base64.URLEncoding.EncodedLen(MessageTextMaximumBytes) {
		return "", BodyInvalid, errors.Join(core.ErrGmailResponse, core.ErrExchangeBodyLimit)
	}
	for _, c := range data {
		if c == '\r' || c == '\n' {
			return "", BodyInvalid, core.ErrGmailResponse
		}
	}
	encoding := base64.RawURLEncoding.Strict()
	if strings.HasSuffix(string(data), "=") {
		encoding = base64.URLEncoding.Strict()
	}
	decoded, err := encoding.DecodeString(string(data))
	defer clear(decoded)
	if err != nil || !utf8.Valid(decoded) {
		return "", BodyInvalid, core.ErrGmailResponse
	}
	if len(decoded) > MessageTextMaximumBytes {
		return "", BodyInvalid, errors.Join(core.ErrGmailResponse, core.ErrExchangeBodyLimit)
	}
	return PlainText(decoded), BodyPlainText, nil
}
