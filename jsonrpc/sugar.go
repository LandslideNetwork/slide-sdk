package jsonrpc

import (
	"context"
	"encoding/json"
)

func TypedHandler[REQ, RES any](h interface {
	Dispatch(context.Context, REQ) (RES, error)
}) Handler {
	return HandlerFunc(func(ctx context.Context, rm json.RawMessage) (json.RawMessage, error) {
		var req REQ
		if err := json.Unmarshal(rm, &req); err != nil {
			e := ErrParseError
			e.Data = err
			return nil, err
		}
		res, err := h.Dispatch(ctx, req)
		if err != nil {
			e := ErrInternalError
			e.Data = err
			return nil, err
		}
		return json.Marshal(res)
	})
}

func TypedHandlerFunc[REQ, RES any](f func(context.Context, REQ) (RES, error)) HandlerFunc {
	return func(ctx context.Context, rm json.RawMessage) (json.RawMessage, error) {
		var req REQ
		if err := json.Unmarshal(rm, &req); err != nil {
			e := ErrParseError
			e.Data = err
			return nil, e
		}
		res, err := f(ctx, req)
		if err != nil {
			e := ErrInternalError
			e.Data = err
			return nil, e
		}
		return json.Marshal(res)
	}
}

func TypedCall[REQ, RES any](client Client, method string, params REQ) (*RES, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	resp, err := client.Call(method, data)
	if err != nil {
		return nil, err
	}
	var res RES
	return &res, json.Unmarshal(resp, &res)
}
