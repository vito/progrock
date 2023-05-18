package progrock

import (
	"fmt"
	"net"
	"net/rpc"
	"sync"
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

func (w *RPCWriter) WriteStatus(status *StatusUpdate) error {
	var res NoResponse
	return w.c.Call("RPCReceiver.Write", status, &res)
}

func (w *RPCWriter) Close() error {
	var res NoResponse
	return w.c.Call("RPCReceiver.Detach", &NoArgs{}, &res)
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

func (recv *RPCReceiver) Write(status *StatusUpdate, res *NoResponse) error {
	return recv.w.WriteStatus(status)
}

func (recv *RPCReceiver) Detach(*NoArgs, *NoResponse) error {
	recv.attachedClients.Done()
	return nil
}

func ServeRPC(l net.Listener, w Writer) (Writer, error) {
	wg := new(sync.WaitGroup)

	recv := &RPCReceiver{
		w:               w,
		attachedClients: wg,
	}

	srv := rpc.NewServer()
	err := srv.Register(recv)
	if err != nil {
		return nil, err
	}

	// avoid using srv.Accept() because it logs when the listener closes
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}

			srv.ServeConn(conn)
		}
	}()

	return WaitWriter{
		Writer: w,

		attachedClients: wg,
	}, nil
}

type WaitWriter struct {
	Writer
	attachedClients *sync.WaitGroup
}

func (ww WaitWriter) Close() error {
	ww.attachedClients.Wait()
	return ww.Writer.Close()
}
