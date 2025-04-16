// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"context"
	"sync"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/landslidenetwork/slide-sdk/utils/avalanche/message"

	"go.uber.org/zap"
)

// const initialQueueSize = 64
var (
	_ MessageQueue = (*blockingMessageQueue)(nil)
)

type SendFailedCallback interface {
	SendFailed(message.OutboundMessage)
}

type SendFailedFunc func(message.OutboundMessage)

func (f SendFailedFunc) SendFailed(msg message.OutboundMessage) {
	f(msg)
}

type MessageQueue interface {
	// Push attempts to add the message to the queue. If the context is
	// canceled, then pushing the message will return `false` and the message
	// will not be added to the queue.
	Push(ctx context.Context, msg message.OutboundMessage) bool

	// Pop blocks until a message is available and then returns the message. If
	// the queue is closed, then `false` is returned.
	Pop() (message.OutboundMessage, bool)

	// PopNow attempts to return a message without blocking. If a message is not
	// available or the queue is closed, then `false` is returned.
	PopNow() (message.OutboundMessage, bool)

	// Close empties the queue and prevents further messages from being pushed
	// onto it. After calling close once, future calls to close will do nothing.
	Close()
}

type blockingMessageQueue struct {
	onFailed SendFailedCallback
	log      log.Logger

	closeOnce   sync.Once
	closingLock sync.RWMutex
	closing     chan struct{}

	// queue of the messages
	queue chan message.OutboundMessage
}

func NewBlockingMessageQueue(
	onFailed SendFailedCallback,
	log log.Logger,
	bufferSize int,
) MessageQueue {
	return &blockingMessageQueue{
		onFailed: onFailed,
		log:      log,

		closing: make(chan struct{}),
		queue:   make(chan message.OutboundMessage, bufferSize),
	}
}

func (q *blockingMessageQueue) Push(ctx context.Context, msg message.OutboundMessage) bool {
	q.closingLock.RLock()
	defer q.closingLock.RUnlock()

	ctxDone := ctx.Done()
	select {
	case <-q.closing:
		q.log.Debug(
			"dropping message",
			zap.String("reason", "closed queue"),
			zap.Stringer("messageOp", msg.Op()),
		)
		q.onFailed.SendFailed(msg)
		return false
	case <-ctxDone:
		q.log.Debug(
			"dropping message",
			zap.String("reason", "cancelled context"),
			zap.Stringer("messageOp", msg.Op()),
		)
		q.onFailed.SendFailed(msg)
		return false
	default:
	}

	select {
	case q.queue <- msg:
		return true
	case <-ctxDone:
		q.log.Debug(
			"dropping message",
			zap.String("reason", "cancelled context"),
			zap.Stringer("messageOp", msg.Op()),
		)
		q.onFailed.SendFailed(msg)
		return false
	case <-q.closing:
		q.log.Debug(
			"dropping message",
			zap.String("reason", "closed queue"),
			zap.Stringer("messageOp", msg.Op()),
		)
		q.onFailed.SendFailed(msg)
		return false
	}
}

func (q *blockingMessageQueue) Pop() (message.OutboundMessage, bool) {
	select {
	case msg := <-q.queue:
		return msg, true
	case <-q.closing:
		return nil, false
	}
}

func (q *blockingMessageQueue) PopNow() (message.OutboundMessage, bool) {
	select {
	case msg := <-q.queue:
		return msg, true
	default:
		return nil, false
	}
}

func (q *blockingMessageQueue) Close() {
	q.closeOnce.Do(func() {
		close(q.closing)

		q.closingLock.Lock()
		defer q.closingLock.Unlock()

		for {
			select {
			case msg := <-q.queue:
				q.onFailed.SendFailed(msg)
			default:
				return
			}
		}
	})
}
