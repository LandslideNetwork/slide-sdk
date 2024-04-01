package database

import (
	"context"
	"sync/atomic"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/consideritdone/landslidevm/proto/rpcdb"
)

var (
	_ dbm.DB = (*Database)(nil)
)

type (
	Database struct {
		client rpcdb.DatabaseClient
		closed atomic.Bool
	}
)

func New(db rpcdb.DatabaseClient) *Database {
	return &Database{
		client: db,
	}
}

func (db *Database) Close() error {
	db.closed.Store(true)
	resp, err := db.client.Close(context.Background(), &rpcdb.CloseRequest{})
	if err != nil {
		return err
	}
	return ErrEnumToError[resp.Err]
}

func (db *Database) Has(key []byte) (bool, error) {
	resp, err := db.client.Has(context.Background(), &rpcdb.HasRequest{
		Key: key,
	})
	if err != nil {
		return false, err
	}
	return resp.Has, ErrEnumToError[resp.Err]
}

func (db *Database) Get(key []byte) ([]byte, error) {
	resp, err := db.client.Get(context.Background(), &rpcdb.GetRequest{
		Key: key,
	})
	if err != nil {
		return nil, err
	}
	return resp.Value, ErrEnumToError[resp.Err]
}

func (db *Database) Set(key []byte, value []byte) error {
	resp, err := db.client.Put(context.Background(), &rpcdb.PutRequest{
		Key:   key,
		Value: value,
	})
	if err != nil {
		return err
	}
	return ErrEnumToError[resp.Err]
}

func (db *Database) SetSync(key []byte, value []byte) error {
	return db.Set(key, value)
}

func (db *Database) Delete(key []byte) error {
	resp, err := db.client.Delete(context.Background(), &rpcdb.DeleteRequest{
		Key: key,
	})
	if err != nil {
		return err
	}
	return ErrEnumToError[resp.Err]
}

func (db *Database) DeleteSync(key []byte) error {
	return db.Delete(key)
}

func (db *Database) Iterator(start, end []byte) (dbm.Iterator, error) {
	resp, err := db.client.NewIteratorWithStartAndPrefix(context.Background(), &rpcdb.NewIteratorWithStartAndPrefixRequest{
		Start:  start,
		Prefix: nil,
	})
	if err != nil {
		return nil, err
	}
	it := Iterator{
		db:    db,
		id:    resp.Id,
		start: start,
		end:   end,

		fetchedData:    make(chan []*rpcdb.PutRequest),
		reqUpdateError: make(chan chan struct{}),
		onClose:        make(chan struct{}),
		onClosed:       make(chan struct{}),
	}
	go it.fetch()
	return &it, nil
}

func (db *Database) ReverseIterator(start, end []byte) (dbm.Iterator, error) {
	return db.Iterator(start, end)
}

func (db *Database) NewBatch() dbm.Batch {
	return &Batch{db: db}
}

func (db *Database) Print() error {
	//TODO implement me
	return nil
}

func (db *Database) Stats() map[string]string {
	//TODO implement me
	return nil
}
