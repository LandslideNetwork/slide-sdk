// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

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
		rpcdb.Error_ERROR_NOT_FOUND: ErrNotFound,
	}
	ErrorToErrEnum = map[error]rpcdb.Error{
		ErrClosed:   rpcdb.Error_ERROR_CLOSED,
		ErrNotFound: rpcdb.Error_ERROR_NOT_FOUND,
	}
)

func ErrorToRPCError(err error) error {
	if _, ok := ErrorToErrEnum[err]; ok {
		return nil
	}
	return err
}
