package database

import (
	"context"
	"slices"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/consideritdone/landslidevm/proto/rpcdb"
)

var (
	_ dbm.Batch = (*Batch)(nil)
)

type (
	Batch struct {
		Ops  []BatchOp
		size int
		db   *Database
	}

	BatchOp struct {
		Key    []byte
		Value  []byte
		Delete bool
	}
)

func (b *Batch) Set(key, value []byte) error {
	b.Ops = append(b.Ops, BatchOp{
		Key:   slices.Clone(key),
		Value: slices.Clone(value),
	})
	b.size += len(key) + len(value)
	return nil
}

func (b *Batch) Delete(key []byte) error {
	b.Ops = append(b.Ops, BatchOp{
		Key:    slices.Clone(key),
		Delete: true,
	})
	b.size += len(key)
	return nil
}

func (b *Batch) Write() error {
	request := &rpcdb.WriteBatchRequest{}
	keySet := make(map[string]bool, len(b.Ops))
	for i := len(b.Ops) - 1; i >= 0; i-- {
		op := b.Ops[i]
		key := string(op.Key)
		if keySet[key] {
			continue
		}
		keySet[key] = true

		if op.Delete {
			request.Deletes = append(request.Deletes, &rpcdb.DeleteRequest{
				Key: op.Key,
			})
		} else {
			request.Puts = append(request.Puts, &rpcdb.PutRequest{
				Key:   op.Key,
				Value: op.Value,
			})
		}
	}

	resp, err := b.db.client.WriteBatch(context.Background(), request)
	if err != nil {
		return err
	}
	return ErrEnumToError[resp.Err]
}

func (b *Batch) WriteSync() error {
	return b.Write()
}

func (b *Batch) Close() error {
	return nil
}
