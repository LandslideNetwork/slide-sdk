package types

import "sync"

type Atomic[T any] struct {
	lock  sync.RWMutex
	value T
}

func NewAtomic[T any](value T) *Atomic[T] {
	return &Atomic[T]{
		lock:  sync.RWMutex{},
		value: value,
	}
}

func (a *Atomic[T]) Get() T {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return a.value
}

func (a *Atomic[T]) Set(value T) {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.value = value
}
