package awsidentity

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// decodeAmazonResponse keeps Go's typed decoder and requires one complete
// document. Decode alone may silently stop before trailing malformed material.
func decodeAmazonResponse(body []byte, document *amazonResponse) error {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	token, err := nextAmazonXMLContent(decoder, true)
	if err != nil {
		return err
	}
	start, ok := token.(xml.StartElement)
	if !ok {
		return core.ErrAWSIdentityContract
	}
	if err := decoder.DecodeElement(document, &start); err != nil {
		return err
	}
	_, err = nextAmazonXMLContent(decoder, false)
	if !errors.Is(err, io.EOF) {
		return contractError(err)
	}
	return nil
}

// XML permits whitespace, comments and processing instructions around its
// root. A declaration is admitted only in the prolog, never after the root.
func nextAmazonXMLContent(decoder *xml.Decoder, prolog bool) (xml.Token, error) {
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		if amazonXMLPadding(token, prolog) {
			continue
		}
		return token, nil
	}
}

func amazonXMLPadding(token xml.Token, prolog bool) bool {
	switch value := token.(type) {
	case xml.CharData:
		return len(bytes.Trim(value, " \t\r\n")) == 0
	case xml.Comment:
		return true
	case xml.ProcInst:
		return prolog || value.Target != "xml"
	default:
		return false
	}
}
