package database

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/consideritdone/landslidevm/proto/rpcdb"
	"github.com/consideritdone/landslidevm/utils/set"
)

var (
	_ dbm.DB = (*Database)(nil)
)

type (
	Database struct {
		client rpcdb.DatabaseClient
		closed atomic.Bool
	}
	Iterator struct {
		db    *Database
		id    uint64
		start []byte
		end   []byte

		data        []*rpcdb.PutRequest
		fetchedData chan []*rpcdb.PutRequest

		errLock sync.RWMutex
		err     error

		reqUpdateError chan chan struct{}

		once     sync.Once
		onClose  chan struct{}
		onClosed chan struct{}
	}
	BatchOp struct {
		Key    []byte
		Value  []byte
		Delete bool
	}
	Batch struct {
		Ops  []BatchOp
		size int
		db   *Database
	}
)

func NewDB(db rpcdb.DatabaseClient) *Database {
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

// Invariant: fetch is the only thread with access to send requests to the
// server's iterator. This is needed because iterators are not thread safe and
// the server expects the client (us) to only ever issue one request at a time
// for a given iterator id.
func (it *Iterator) fetch() {
	defer func() {
		resp, err := it.db.client.IteratorRelease(context.Background(), &rpcdb.IteratorReleaseRequest{
			Id: it.id,
		})
		if err != nil {
			it.setError(err)
		} else {
			it.setError(ErrEnumToError[resp.Err])
		}

		close(it.fetchedData)
		close(it.onClosed)
	}()

	for {
		resp, err := it.db.client.IteratorNext(context.Background(), &rpcdb.IteratorNextRequest{
			Id: it.id,
		})
		if err != nil {
			it.setError(err)
			return
		}

		if len(resp.Data) == 0 {
			return
		}

		for {
			select {
			case it.fetchedData <- resp.Data:
			case onUpdated := <-it.reqUpdateError:
				it.updateError()
				close(onUpdated)
				continue
			case <-it.onClose:
				return
			}
			break
		}
	}
}

func (it *Iterator) getError() error {
	it.errLock.RLock()
	defer it.errLock.RUnlock()

	return it.err
}

func (it *Iterator) setError(err error) {
	if err == nil {
		return
	}

	it.errLock.Lock()
	defer it.errLock.Unlock()

	if it.err == nil {
		it.err = err
	}
}

func (it *Iterator) updateError() {
	resp, err := it.db.client.IteratorError(context.Background(), &rpcdb.IteratorErrorRequest{
		Id: it.id,
	})
	if err != nil {
		it.setError(err)
	} else {
		it.setError(ErrEnumToError[resp.Err])
	}
}

func (iter *Iterator) Domain() (start []byte, end []byte) {
	return iter.start, iter.end
}

func (iter *Iterator) Valid() bool {
	return iter.Error() == nil && len(iter.Key()) > 0
}

func (iter *Iterator) Next() {
	if iter.db.closed.Load() {
		iter.data = nil
		iter.setError(ErrClosed)
	}
	if len(iter.data) > 1 {
		iter.data[0] = nil
		iter.data = iter.data[1:]
	}

	iter.data = <-iter.fetchedData
}

func (iter *Iterator) Key() (key []byte) {
	if len(iter.data) == 0 {
		return nil
	}
	return iter.data[0].Key
}

func (iter *Iterator) Value() (value []byte) {
	if len(iter.data) == 0 {
		return nil
	}
	return iter.data[0].Value
}

func (iter *Iterator) Error() error {
	if err := iter.getError(); err != nil {
		return err
	}

	onUpdated := make(chan struct{})
	select {
	case iter.reqUpdateError <- onUpdated:
		<-onUpdated
	case <-iter.onClosed:
	}

	return iter.getError()
}

func (iter *Iterator) Close() error {
	iter.once.Do(func() {
		close(iter.onClose)
		<-iter.onClosed
	})
	return iter.Error()
}

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
	keySet := set.NewSet[string](len(b.Ops))
	for i := len(b.Ops) - 1; i >= 0; i-- {
		op := b.Ops[i]
		key := string(op.Key)
		if keySet.Contains(key) {
			continue
		}
		keySet.Add(key)

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
