package closer

import (
	"errors"
	"io"
	"sync"
)

// Closer is a nice utility for closing a group of objects while reporting an
// error if one occurs.
type Closer struct {
	lock    sync.Mutex
	closers []io.Closer
}

// Add a new object to be closed.
func (c *Closer) Add(closer io.Closer) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.closers = append(c.closers, closer)
}

// Close closes each of the closers add to [c] and returns the first error
// that occurs or nil if no error occurs.
func (c *Closer) Close() error {
	c.lock.Lock()
	closers := c.closers
	c.closers = nil
	c.lock.Unlock()

	var errs []error
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
