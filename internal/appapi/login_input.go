package appapi

import (
	"encoding/json"
	"errors"
	"io"
)

var errInvalidLoginRequest = errors.New("invalid login request")

func decodeLoginRequest(body io.Reader) (loginReq, error) {
	decoder := json.NewDecoder(body)
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return loginReq{}, errInvalidLoginRequest
	}
	var request loginReq
	present := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil || key != "code" || present {
			return loginReq{}, errInvalidLoginRequest
		}
		var code *string
		if err := decoder.Decode(&code); err != nil || code == nil || *code == "" {
			return loginReq{}, errInvalidLoginRequest
		}
		request.Code = *code
		present = true
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') || !present {
		return loginReq{}, errInvalidLoginRequest
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return loginReq{}, errInvalidLoginRequest
	}
	return request, nil
}
