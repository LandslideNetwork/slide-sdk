package database

import (
	"errors"

	"github.com/consideritdone/landslidevm/proto/rpcdb"
)

var (
	ErrClosed      = errors.New("closed")
	ErrNotFound    = errors.New("not found")
	ErrEnumToError = map[rpcdb.Error]error{
		rpcdb.Error_ERROR_CLOSED:    ErrClosed,
		rpcdb.Error_ERROR_NOT_FOUND: nil,
	}
)
