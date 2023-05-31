package app

import (
	"encoding/json"

	"fivebit.co.uk/spacetraders/api"
)

type APIErrorResponse struct {
	Error *APIError `json:"error"`
}

type APIError struct {
	Code    int32                      `json:"code"`
	Message string                      `json:"message"`
	Data    map[string]json.RawMessage `json:"data"`
}

func (ae *APIError) decodeData(field string, into any) error {
	return json.Unmarshal(ae.Data[field], into)
}

func getAPIError(err error) (*APIError, error) {
	openAPIErr, ok := err.(*api.GenericOpenAPIError)
	if !ok {
		return nil, err
	}
	resp := &APIErrorResponse{}
	if err := json.Unmarshal(openAPIErr.Body(), resp); err != nil {
		return nil, err
	}
	return resp.Error, nil
}
