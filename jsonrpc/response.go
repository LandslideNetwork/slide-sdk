package jsonrpc

import (
	"bytes"
	"encoding/json"
)

var (
	_ json.Marshaler   = (*Response)(nil)
	_ json.Unmarshaler = (*Response)(nil)
)

type (
	ResponseParams struct {
		JsonRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   *Error          `json:"error,omitempty"`
		Result  json.RawMessage `json:"result,omitempty"`
	}

	Response struct {
		single bool
		params []ResponseParams
	}
)

func (res Response) MarshalJSON() ([]byte, error) {
	if res.single {
		return json.Marshal(res.params[0])
	}
	return json.Marshal(res.params)
}

func (res *Response) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	res.single = !bytes.HasPrefix(data, []byte("["))

	if !res.single {
		res.params = make([]ResponseParams, 0)
		return json.Unmarshal(data, &res.params)
	}

	res.params = make([]ResponseParams, 1)
	return json.Unmarshal(data, &res.params[0])
}
