package setupapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

var errInvalidSetupInput = errors.New("invalid setup input")

type setupInput struct {
	Username string
	Password string
}

func decodeSetupInput(body io.Reader) (setupInput, error) {
	raw, err := io.ReadAll(body)
	if err != nil || !utf8.Valid(raw) {
		return setupInput{}, errInvalidSetupInput
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return setupInput{}, errInvalidSetupInput
	}
	values := make(map[string]string, 2)
	for decoder.More() {
		token, err := decoder.Token()
		key, valid := token.(string)
		_, duplicate := values[key]
		if err != nil || !valid || duplicate || (key != "username" && key != "password") {
			return setupInput{}, errInvalidSetupInput
		}
		var value *string
		if err := decoder.Decode(&value); err != nil || value == nil {
			return setupInput{}, errInvalidSetupInput
		}
		values[key] = *value
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') || len(values) != 2 {
		return setupInput{}, errInvalidSetupInput
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return setupInput{}, errInvalidSetupInput
	}
	return setupInput{Username: values["username"], Password: values["password"]}, nil
}
