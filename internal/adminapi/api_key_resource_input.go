package adminapi

import (
	"bytes"
	"encoding/json"
	"errors"
)

type apiKeyResourceInput struct {
	FarmIDs   apiKeyResourceList[int64]  `json:"farm_ids"`
	PondIDs   apiKeyResourceList[int64]  `json:"pond_ids"`
	DeviceNos apiKeyResourceList[string] `json:"device_nos"`
}

func (r *apiKeyResourceInput) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("resource scope must be an object")
	}
	type wireInput apiKeyResourceInput
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode((*wireInput)(r))
}

type apiKeyResourceList[T any] []T

func (r *apiKeyResourceList[T]) UnmarshalJSON(data []byte) error {
	var values []T
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if values == nil {
		return errors.New("resource list must be an array")
	}
	*r = values
	return nil
}
