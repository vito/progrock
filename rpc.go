package progrock

import (
	"fmt"
	"net"
	"net/rpc"
	"sync"

	"github.com/vito/progrock/graph"
)

type RPCWriter struct {
	c *rpc.Client
}

func DialRPC(net, addr string) (Writer, error) {
	c, err := rpc.Dial(net, addr)
	if err != nil {
		return nil, err
	}

	var res NoResponse
	err = c.Call("RPCReceiver.Attach", &NoArgs{}, &res)
	if err != nil {
		return nil, fmt.Errorf("attach: %w", err)
	}

	return &RPCWriter{
		c: c,
	}, nil
}

func (w *RPCWriter) WriteStatus(status *graph.SolveStatus) {
	var res NoResponse
	_ = w.c.Call("RPCReceiver.Write", status, &res)
}

func (w *RPCWriter) Close() {
	var res NoResponse
	_ = w.c.Call("RPCReceiver.Detach", &NoArgs{}, &res)
}

type RPCReceiver struct {
	w               Writer
	attachedClients *sync.WaitGroup
}

type NoArgs struct{}
type NoResponse struct{}

func (recv *RPCReceiver) Attach(*NoArgs, *NoResponse) error {
	recv.attachedClients.Add(1)
	return nil
}

func (recv *RPCReceiver) Write(status *graph.SolveStatus, res *NoResponse) error {
	recv.w.WriteStatus(status)
	return nil
}

func (recv *RPCReceiver) Detach(*NoArgs, *NoResponse) error {
	recv.attachedClients.Done()
	return nil
}

func ServeRPC(l net.Listener) (ChanReader, Writer, error) {
	r, w := Pipe()

	wg := new(sync.WaitGroup)

	recv := &RPCReceiver{
		w:               w,
		attachedClients: wg,
	}

	s := rpc.NewServer()
	err := s.Register(recv)
	if err != nil {
		return nil, nil, err
	}

	go s.Accept(l)

	return r, WaitWriter{
		Writer: w,

		attachedClients: wg,
	}, nil
}

type WaitWriter struct {
	Writer
	attachedClients *sync.WaitGroup
}

func (ww WaitWriter) Close() {
	ww.attachedClients.Wait()
	ww.Writer.Close()
}
