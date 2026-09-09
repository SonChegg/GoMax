package connection

import (
	"sync"

	"github.com/SonChegg/PyMax/protocol"
)

// PendingRequests correlates outbound requests to their eventual inbound
// response by sequence number, a port of pymax's connection.pending.PendingRequests.
type PendingRequests struct {
	mu      sync.Mutex
	pending map[int]chan pendingResult
}

type pendingResult struct {
	frame protocol.InboundFrame
	err   error
}

// NewPendingRequests builds an empty request/response correlation table.
func NewPendingRequests() *PendingRequests {
	return &PendingRequests{pending: make(map[int]chan pendingResult)}
}

// Create registers a new pending request for seq and returns the channel
// that will receive its result exactly once.
func (p *PendingRequests) Create(seq int) <-chan pendingResult {
	ch := make(chan pendingResult, 1)
	p.mu.Lock()
	p.pending[seq] = ch
	p.mu.Unlock()
	return ch
}

// Resolve delivers frame to the pending request for seq, if any. It reports
// whether a waiter was found.
func (p *PendingRequests) Resolve(seq int, frame protocol.InboundFrame) bool {
	p.mu.Lock()
	ch, ok := p.pending[seq]
	if ok {
		delete(p.pending, seq)
	}
	p.mu.Unlock()

	if !ok {
		return false
	}
	ch <- pendingResult{frame: frame}
	return true
}

// Discard cancels the pending request for seq without delivering a result,
// used when a caller stops waiting (e.g. on context cancellation).
func (p *PendingRequests) Discard(seq int) {
	p.mu.Lock()
	ch, ok := p.pending[seq]
	if ok {
		delete(p.pending, seq)
	}
	p.mu.Unlock()

	if ok {
		close(ch)
	}
}

// CancelAll fails every pending request with err (or simply closes them if
// err is nil), used when the connection is lost or explicitly closed.
func (p *PendingRequests) CancelAll(err error) {
	p.mu.Lock()
	pending := p.pending
	p.pending = make(map[int]chan pendingResult)
	p.mu.Unlock()

	for _, ch := range pending {
		if err != nil {
			ch <- pendingResult{err: err}
		} else {
			close(ch)
		}
	}
}
