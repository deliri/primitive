package timeproof

import (
	"bytes"
	"encoding/json"
	"strconv"
)

type enumValue interface {
	comparable
	Validate() error
	String() string
}

// decodeJSONToken is the single quoted-token ingress for every Timeproof value
// persisted as a JSON string. It refuses any document that is not a bounded
// quoted string, so literals such as null cannot decode to an empty token.
func decodeJSONToken(data []byte, maximum int) (string, error) {
	if len(data) < 2 || len(data) > maximum ||
		data[0] != '"' || data[len(data)-1] != '"' {
		return "", errorsJSON()
	}
	var token string
	if err := json.Unmarshal(data, &token); err != nil {
		return "", errorsJSON()
	}
	return token, nil
}

func marshalEnum[T enumValue](value T) ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, contractError(err)
	}
	return strconv.AppendQuote(nil, value.String()), nil
}

func unmarshalEnum[T enumValue](
	data []byte,
	receiver *T,
	parse func(string) (T, error),
) error {
	if receiver == nil {
		return errorsJSON()
	}
	token, err := decodeJSONToken(data, enumJSONMaximumBytes)
	if err != nil {
		return err
	}
	parsed, err := parse(token)
	if err != nil {
		return errorsJSON(err)
	}
	canonical, err := marshalEnum(parsed)
	if err != nil || !bytes.Equal(data, canonical) {
		return errorsJSON()
	}
	*receiver = parsed
	return nil
}
