package stream

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/api/remote"
	"github.com/MikhUd/blockchain/pkg/config"
	clusterContext "github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/serializer"
	"log/slog"
	"net"
	"storj.io/drpc/drpcconn"
	"sync/atomic"
	"time"
)

type Writer struct {
	writeAddr  string
	conn       net.Conn
	dconn      *drpcconn.Conn
	stream     remote.DRPCRemote_ReceiveStream
	serializer serializer.Serializer
	state      atomic.Uint32
	tlsConfig  *tls.Config
}

func NewWriter(writeAddr string) Sender {
	return &Writer{writeAddr: writeAddr}
}

func (w *Writer) WithConn(conn net.Conn) Sender {
	w.conn = conn
	return w
}

func (w *Writer) Addr() string {
	return w.writeAddr
}

func (w *Writer) Send(ctx *clusterContext.Context) error {
	var op = "writer.Send"
	var ser serializer.ProtoSerializer
	if w.state.Load() != config.Running {
		err := w.start(w.writeAddr)
		if err != nil {
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("writer start error: %s", err.Error()))
			return err
		}
	}
	data, err := ser.Serialize(ctx.Msg())
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("serialize error: %s", err.Error()))
		return err
	}
	msg := &message.BlockchainMessage{Data: data, TypeName: ser.TypeName(ctx.Msg())}
	err = w.stream.Send(msg)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("writer send error: %s, addr: %s", err.Error(), w.writeAddr))
		return err
	}
	return nil
}

func (w *Writer) start(addr string) error {
	var (
		err        error
		conn       net.Conn
		op         = "writer.Start"
		delay      = time.Millisecond * 500
		maxRetries = 3
	)
	//TODO: implement tls
	if w.conn == nil {
		for i := 0; i < maxRetries; i++ {
			rDelay := delay * time.Duration(i*2)
			conn, err = net.Dial("tcp", addr)
			if err != nil {
				time.Sleep(rDelay)
				continue
			}
			break
		}
		if conn == nil {
			w.Shutdown()
			return err
		}
		w.conn = conn
	}
	dconn := drpcconn.New(w.conn)
	client := remote.NewDRPCRemoteClient(dconn)
	stream, err := client.Receive(context.Background())
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("dconn creating stream error: %s", err.Error()))
		return err
	}
	w.dconn = dconn
	w.stream = stream
	w.state.Store(config.Running)
	go func() {
		<-w.dconn.Closed()
	}()
	return nil
}

func (w *Writer) Shutdown() {
	fmt.Println("WRITER SHUTDOWN", w.writeAddr)
	if w.stream != nil {
		_ = w.stream.Close()
	}
	if w.conn != nil {
		_ = w.conn.Close()
	}
	if w.dconn != nil {
		_ = w.dconn.Close()
	}
	w.state.Store(config.Stopped)
}
