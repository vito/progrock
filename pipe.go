package progrock

import (
	"errors"
	"sync"
)

func Pipe() (Reader, Writer) {
	pipe := &unboundedPipe{
		cond: sync.NewCond(&sync.Mutex{}),
	}
	return pipe, pipe
}

type unboundedPipe struct {
	cond   *sync.Cond
	buffer []*StatusUpdate
	closed bool
}

func (p *unboundedPipe) WriteStatus(value *StatusUpdate) error {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()

	if p.closed {
		return errors.New("pipe is closed")
	}

	p.buffer = append(p.buffer, value)
	p.cond.Signal()
	return nil
}

func (p *unboundedPipe) ReadStatus() (*StatusUpdate, bool) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()

	for len(p.buffer) == 0 && !p.closed {
		p.cond.Wait()
	}

	if len(p.buffer) == 0 && p.closed {
		return nil, false
	}

	value := p.buffer[0]
	p.buffer = p.buffer[1:]
	return value, true
}

func (p *unboundedPipe) Close() error {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()

	p.closed = true
	p.cond.Broadcast()
	return nil
}
