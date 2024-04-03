package database

import (
	"context"
	"sync"

	dbm "github.com/cometbft/cometbft-db"
	"github.com/consideritdone/landslidevm/proto/rpcdb"
)

var (
	_ dbm.Iterator = (*Iterator)(nil)
)

type (
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
)

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

func (it *Iterator) Domain() (start []byte, end []byte) {
	return it.start, it.end
}

func (it *Iterator) Valid() bool {
	return it.Error() == nil && len(it.Key()) > 0
}

func (it *Iterator) Next() {
	if it.db.closed.Load() {
		it.data = nil
		it.setError(ErrClosed)
	}
	if len(it.data) > 1 {
		it.data[0] = nil
		it.data = it.data[1:]
	}

	it.data = <-it.fetchedData
}

func (it *Iterator) Key() (key []byte) {
	if len(it.data) == 0 {
		return nil
	}
	return it.data[0].Key
}

func (it *Iterator) Value() (value []byte) {
	if len(it.data) == 0 {
		return nil
	}
	return it.data[0].Value
}

func (it *Iterator) Error() error {
	if err := it.getError(); err != nil {
		return err
	}

	onUpdated := make(chan struct{})
	select {
	case it.reqUpdateError <- onUpdated:
		<-onUpdated
	case <-it.onClosed:
	}

	return it.getError()
}

func (it *Iterator) Close() error {
	it.once.Do(func() {
		close(it.onClose)
		<-it.onClosed
	})
	return it.Error()
}
