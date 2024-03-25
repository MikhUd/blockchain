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
)

type Writer struct {
	writeAddr  string
	nconn      net.Conn
	dconn      *drpcconn.Conn
	stream     remote.DRPCRemote_ReceiveStream
	serializer serializer.Serializer
	state      atomic.Uint32
	tlsConfig  *tls.Config
}

func NewWriter(writeAddr string) Sender {
	return &Writer{writeAddr: writeAddr}
}

func (w *Writer) Addr() string {
	return w.writeAddr
}

func (w *Writer) Send(ctx *clusterContext.Context) error {
	const op = "writer.Send"
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
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("writer send error: %s", err.Error()))
		return err
	}
	return nil
}

func (w *Writer) start(addr string) error {
	const op = "writer.Start"
	//TODO: implement tls
	nconn, err := net.Dial("tcp", addr)
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("nconn init error: %s", err.Error()))
		return clusterContext.Refused
	}
	w.nconn = nconn

	dconn := drpcconn.New(nconn)
	client := remote.NewDRPCRemoteClient(dconn)
	stream, err := client.Receive(context.Background())
	if err != nil {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("dconn creating stream error: %s", err.Error()))
	}
	w.dconn = dconn
	w.stream = stream

	go func() {
		<-w.dconn.Closed()
	}()

	return nil
}
