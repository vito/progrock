package progrock

import (
	"context"
	"errors"
	"io"
	"net"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// ServeRPC serves a ProgressService over the given listener.
func ServeRPC(l net.Listener, w Writer) (Writer, error) {
	recv := NewRPCReceiver(w)

	srv := grpc.NewServer()
	RegisterProgressServiceServer(srv, recv)

	go srv.Serve(l)

	return WaitWriter{
		Writer: w,
		srv:    srv,
	}, nil
}

// DialRPC dials a ProgressService at the given target.
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

	return NewRPCWriter(conn, updates), nil
}

// RPCWriter is a Writer that writes to a ProgressService.
type RPCWriter struct {
	Conn    *grpc.ClientConn
	Updates ProgressService_WriteUpdatesClient
}

// NewRPCWriter returns a new RPCWriter.
func NewRPCWriter(conn *grpc.ClientConn, updates ProgressService_WriteUpdatesClient) *RPCWriter {
	return &RPCWriter{
		Conn:    conn,
		Updates: updates,
	}
}

// WriteStatus implements Writer.
func (w *RPCWriter) WriteStatus(status *StatusUpdate) error {
	return w.Updates.Send(status)
}

// Close closes the underlying RPC connection.
func (w *RPCWriter) Close() error {
	_, err := w.Updates.CloseAndRecv()
	return err
}

// RPCReceiver is a ProgressServiceServer that writes to a Writer.
type RPCReceiver struct {
	w Writer

	UnimplementedProgressServiceServer
}

// NewRPCReceiver returns a new RPCReceiver.
func NewRPCReceiver(w Writer) *RPCReceiver {
	return &RPCReceiver{w: w}
}

// WriteUpdates implements ProgressServiceServer.
func (recv *RPCReceiver) WriteUpdates(srv ProgressService_WriteUpdatesServer) error {
	for {
		update, err := srv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return srv.SendAndClose(&emptypb.Empty{})
			}
			return err
		}
		if err := recv.w.WriteStatus(update); err != nil {
			return err
		}
	}
}

// WaitWriter is a Writer that waits for the RPC server to stop before closing
// the underlying Writer.
type WaitWriter struct {
	Writer

	srv *grpc.Server
}

// Close waits for the RPC server to stop and closes the underlying Writer.
func (ww WaitWriter) Close() error {
	ww.srv.GracefulStop()
	return ww.Writer.Close()
}
