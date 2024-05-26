package stream

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/api/remote"
	clusterContext "github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/serializer"
	"io"
	"log/slog"
	"net"
	"storj.io/drpc/drpcconn"
	"time"
)

type Writer struct {
	writeAddr  string
	conn       net.Conn
	dconn      *drpcconn.Conn
	stream     remote.DRPCRemote_ReceiveStream
	serializer serializer.Serializer
	tlsConfig  *tls.Config
}

const connTimeout = time.Minute * 10

func NewWriter(writeAddr string) Sender {
	return &Writer{writeAddr: writeAddr, serializer: &serializer.ProtoSerializer{}}
}

func (w *Writer) Send(ctx *clusterContext.Context) error {
	var op = "writer.Send"
	err := w.init(w.writeAddr)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("writer start error: %s", err.Error()))
		return err
	}
	data, err := w.serializer.Serialize(ctx.Msg())
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("serialize error: %s", err.Error()))
		return err
	}
	msg := &message.BlockchainMessage{Data: data, TypeName: w.serializer.TypeName(ctx.Msg())}
	if err = w.stream.Send(msg); err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("stream sending failed: %s, addr: %s", err.Error(), w.writeAddr))
		switch true {
		case errors.Is(err, io.EOF):
			_ = w.dconn.Close()
			return err
		case errors.Is(err, net.ErrClosed):
			w.Shutdown()
			return err
		}
	}
	return w.conn.SetDeadline(time.Now().Add(connTimeout))
}

func (w *Writer) Addr() string {
	return w.writeAddr
}

func (w *Writer) init(addr string) error {
	if w.dconn == nil {
		return w.initConn(addr)
	}
	select {
	case <-w.dconn.Closed():
		w.Shutdown()
		//TODO: implement tls
		return w.initConn(addr)
	default:
		return nil
	}
}

func (w *Writer) initConn(addr string) error {
	var (
		err        error
		conn       net.Conn
		op         = "writer.Start"
		delay      = time.Millisecond * 500
		maxRetries = 3
	)
	for i := 0; i < maxRetries; i++ {
		rDelay := delay * time.Duration(i*2)
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			time.Sleep(rDelay)
			continue
		}
		break
	}
	w.conn = conn
	dconn := drpcconn.New(conn)
	client := remote.NewDRPCRemoteClient(dconn)
	stream, err := client.Receive(context.Background())
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("dconn creating stream error: %s", err.Error()))
		return err
	}
	w.dconn = dconn
	w.stream = stream
	return nil
}

func (w *Writer) Shutdown() {
	if w.stream != nil {
		w.stream.Close()
	}
}
