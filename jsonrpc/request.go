package jsonrpc

import (
	"bytes"
	"encoding/json"
)

var (
	_ json.Marshaler   = (*Request)(nil)
	_ json.Unmarshaler = (*Request)(nil)
)

type (
	RequestParams struct {
		JsonRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	Request struct {
		single bool
		params []RequestParams
	}
)

func (req Request) MarshalJSON() ([]byte, error) {
	if req.single {
		return json.Marshal(req.params[0])
	}
	return json.Marshal(req.params)
}

func (req *Request) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	req.single = !bytes.HasPrefix(data, []byte("["))

	if !req.single {
		req.params = make([]RequestParams, 0)
		return json.Unmarshal(data, &req.params)
	}

	req.params = make([]RequestParams, 1)
	return json.Unmarshal(data, &req.params[0])
}
