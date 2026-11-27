package gmail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

// PrepareMessage builds the exact RFC 5322 message before any provider call.
func (c Client) PrepareMessage(ctx context.Context, r MessagePrepareRequest) (PreparedMessage, error) {
	if err := c.admit(ctx, r); err != nil {
		return PreparedMessage{}, err
	}
	raw, err := encodeMessage(r)
	if err != nil {
		return PreparedMessage{}, err
	}
	if len(raw) > SendMediaMaximumBytes {
		return PreparedMessage{}, core.ErrExchangeBodyLimit
	}
	p := PreparedMessage{raw: raw, from: r.From.Address, digest: core.SHA256Of(raw)}
	if r.Reply != nil {
		p.thread = r.Reply.Thread
	}
	return p, nil
}

func encodeMessage(r MessagePrepareRequest) ([]byte, error) {
	var body bytes.Buffer
	extent := len(r.Text) + len(r.HTML)
	if r.PDF != nil {
		extent += int(r.PDF.Size.Uint64())
	}
	body.Grow(base64.StdEncoding.EncodedLen(extent) + extent/28 + 2048)
	if err := writeMessageHeaders(&body, r); err != nil {
		return nil, err
	}
	boundary, err := r.ContentSHA.Hex()
	if err != nil {
		return nil, err
	}
	altBoundary := "alt-" + boundary[:32]
	if r.PDF == nil {
		alternative := multipart.NewWriter(&body)
		if err := startMIME(&body, alternative, "alternative", altBoundary); err != nil {
			return nil, err
		}
		return bodyResult(&body, writeAlternative(alternative, r))
	}
	mixed := multipart.NewWriter(&body)
	if err := startMIME(&body, mixed, "mixed", "mixed-"+boundary[32:]); err != nil {
		return nil, err
	}
	part, err := mixed.CreatePart(mimeHeaders(mime.FormatMediaType("multipart/alternative", map[string]string{"boundary": altBoundary}), "", ""))
	if err != nil {
		return nil, err
	}
	alternative := multipart.NewWriter(part)
	if err := alternative.SetBoundary(altBoundary); err != nil {
		return nil, err
	}
	if err := writeAlternative(alternative, r); err != nil {
		return nil, err
	}
	if err := writePDF(mixed, *r.PDF); err != nil {
		return nil, err
	}
	return bodyResult(&body, mixed.Close())
}

func bodyResult(body *bytes.Buffer, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func writeMessageHeaders(out *bytes.Buffer, r MessagePrepareRequest) error {
	at, err := r.At.Time()
	if err != nil {
		return err
	}
	digest, err := r.ContentSHA.Hex()
	if err != nil {
		return err
	}
	headers := [...]struct{ name, value string }{
		{"From", formatRecipient(r.From)},
		{"To", formatRecipients(r.Recipients.To)},
		{"Cc", formatRecipients(r.Recipients.Cc)},
		{"Bcc", formatRecipients(r.Recipients.Bcc)},
		{"Subject", mime.QEncoding.Encode("UTF-8", string(r.Subject))},
		{"Message-ID", string(r.RFCID)},
		{"Date", at.Format(time.RFC1123Z)},
		{"MIME-Version", "1.0"},
		{sendDigestHeader, digest},
	}
	for _, header := range headers {
		if header.value != "" {
			if err := writeHeader(out, header.name, header.value); err != nil {
				return err
			}
		}
	}
	if r.Reply != nil {
		return writeReplyHeaders(out, *r.Reply)
	}
	return nil
}

func writeReplyHeaders(out *bytes.Buffer, r ReplyReference) error {
	var chain strings.Builder
	for _, id := range r.References {
		chain.WriteString(string(id))
		chain.WriteByte(' ')
	}
	chain.WriteString(string(r.ParentRFCID))
	return errors.Join(writeHeader(out, "In-Reply-To", string(r.ParentRFCID)), writeHeader(out, "References", chain.String()))
}

func writeHeader(out *bytes.Buffer, name, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return core.ErrGmailContract
	}
	line := name + ": " + value
	for len(line) > subjectMaximumBytes {
		fold := strings.LastIndexByte(line[1:subjectMaximumBytes+1], ' ') + 1
		if fold <= 1 {
			return core.ErrGmailContract
		}
		out.WriteString(line[:fold])
		out.WriteString("\r\n")
		line = line[fold:]
	}
	out.WriteString(line)
	out.WriteString("\r\n")
	return nil
}

func formatRecipient(recipient Recipient) string {
	return (&mail.Address{Name: string(recipient.Name), Address: recipient.Address.String()}).String()
}

func formatRecipients(list []Recipient) string {
	var out strings.Builder
	for i, recipient := range list {
		if i != 0 {
			out.WriteString(", ")
		}
		out.WriteString(formatRecipient(recipient))
	}
	return out.String()
}

func startMIME(out *bytes.Buffer, writer *multipart.Writer, subtype, boundary string) error {
	if err := writer.SetBoundary(boundary); err != nil {
		return err
	}
	out.WriteString("Content-Type: ")
	out.WriteString(mime.FormatMediaType("multipart/"+subtype, map[string]string{"boundary": boundary}))
	out.WriteString("\r\n\r\n")
	return nil
}

func writeAlternative(writer *multipart.Writer, r MessagePrepareRequest) error {
	for _, part := range [...]struct{ contentType, value string }{
		{"text/plain; charset=UTF-8", string(r.Text)}, {"text/html; charset=UTF-8", string(r.HTML)},
	} {
		body, err := writer.CreatePart(mimeHeaders(part.contentType, "base64", ""))
		if err != nil {
			return err
		}
		if err := writeBase64(body, strings.NewReader(part.value)); err != nil {
			return err
		}
	}
	return writer.Close()
}

func mimeHeaders(contentType, encoding, disposition string) textproto.MIMEHeader {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Type", contentType)
	if encoding != "" {
		header.Set("Content-Transfer-Encoding", encoding)
	}
	if disposition != "" {
		header.Set("Content-Disposition", disposition)
	}
	return header
}

func writePDF(writer *multipart.Writer, pdf SendPDF) error {
	hash := sha256.New()
	source := &io.LimitedReader{R: io.TeeReader(pdf.Source, hash), N: int64(pdf.Size.Uint64())}
	prefix := [5]byte{}
	if _, err := io.ReadFull(source, prefix[:]); err != nil || string(prefix[:]) != "%PDF-" {
		return core.ErrGmailContract
	}
	name := mime.FormatMediaType("attachment", map[string]string{"filename": string(pdf.Name)})
	part, err := writer.CreatePart(mimeHeaders("application/pdf", "base64", name))
	if err != nil {
		return err
	}
	if err := writeBase64(part, io.MultiReader(bytes.NewReader(prefix[:]), source)); err != nil {
		return errors.Join(core.ErrGmailContract, cancellationOnly(err))
	}
	var excess [1]byte
	_, readErr := io.ReadFull(pdf.Source, excess[:])
	if source.N != 0 || readErr != io.EOF || core.NewSHA256Digest([sha256.Size]byte(hash.Sum(nil))) != pdf.SHA256 {
		return errors.Join(core.ErrGmailBinding, cancellationOnly(readErr))
	}
	return nil
}

// RFC 2045 caps base64 MIME lines at 76 ASCII characters.
func writeBase64(out io.Writer, source io.Reader) error {
	var input [57]byte
	var line [78]byte
	for {
		n, err := io.ReadFull(source, input[:])
		if err == io.EOF {
			return nil
		}
		if err != nil && err != io.ErrUnexpectedEOF {
			return err
		}
		length := base64.StdEncoding.EncodedLen(n)
		base64.StdEncoding.Encode(line[:length], input[:n])
		line[length], line[length+1] = '\r', '\n'
		if _, writeErr := out.Write(line[:length+2]); writeErr != nil {
			return writeErr
		}
		if err == io.ErrUnexpectedEOF {
			return nil
		}
	}
}
