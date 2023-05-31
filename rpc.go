package progrock

import (
	"context"
	"net"
	"sync"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RPCWriter struct {
	Conn    *grpc.ClientConn
	Updates ProgressService_WriteUpdatesClient
}

func DialRPC(ctx context.Context, target string) (Writer, error) {
	conn, err := grpc.DialContext(ctx, target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := NewProgressServiceClient(conn)

	updates, err := client.WriteUpdates(ctx)
	if err != nil {
		return nil, err
	}

	return &RPCWriter{
		Conn:    conn,
		Updates: updates,
	}, nil
}

func (w *RPCWriter) WriteStatus(status *StatusUpdate) error {
	return w.Updates.Send(status)
}

func (w *RPCWriter) Close() error {
	_, err := w.Updates.CloseAndRecv()
	return err
}

type RPCReceiver struct {
	w               Writer
	attachedClients *sync.WaitGroup

	UnimplementedProgressServiceServer
}

func (recv *RPCReceiver) WriteUpdates(srv ProgressService_WriteUpdatesServer) error {
	for {
		update, err := srv.Recv()
		if err != nil {
			return err
		}
		if err := recv.w.WriteStatus(update); err != nil {
			return err
		}
	}
}

func ServeRPC(l net.Listener, w Writer) (Writer, error) {
	wg := new(sync.WaitGroup)

	recv := &RPCReceiver{
		w:               w,
		attachedClients: wg,
	}

	srv := grpc.NewServer()
	RegisterProgressServiceServer(srv, recv)

	go srv.Serve(l)

	return WaitWriter{
		Writer: w,
		srv:    srv,
	}, nil
}

type WaitWriter struct {
	Writer

	srv *grpc.Server
}

func (ww WaitWriter) Close() error {
	ww.srv.GracefulStop()
	return ww.Writer.Close()
}
