package jsonrpc

import (
	"encoding/json"
	"fmt"
)

var (
	_ json.Marshaler = (*Response)(nil)
	_ error          = (*Error)(nil)

	ErrParseError     = Error{Code: -32700, Message: "Parse error"}
	ErrInvalidRequest = Error{Code: -32600, Message: "Invalid Request"}
	ErrInvalidMethod  = Error{Code: -32601, Message: "Method not found"}
	ErrInvalidParams  = Error{Code: -32602, Message: "Invalid params"}
	ErrInternalError  = Error{Code: -32603, Message: "Internal error"}
)

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (err Error) Error() string {
	return fmt.Sprintf("[%d]: %s", err.Code, err.Message)
}
