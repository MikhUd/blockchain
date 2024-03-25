package stream

import (
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/api/remote"
	"github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/serializer"
	"log"
	"log/slog"
)

type Reader struct {
	remote.DRPCRemoteUnimplementedServer
	context.Receiver
	serializer.ProtoSerializer
}

func NewReader(r context.Receiver) *Reader {
	return &Reader{
		Receiver: r,
	}
}

func (r *Reader) Receive(stream remote.DRPCRemote_ReceiveStream) error {
	const op = "reader.Receive"
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		data, err := r.Deserialize(msg.GetData(), msg.GetTypeName())
		if err != nil {
			log.Fatal(err)
			return err
		}
		err = r.Receiver.Receive(context.New(data).WithSender(&message.PID{Addr: data.(Addressable).GetRemote().GetAddr()}))
		if err != nil {
			slog.With(slog.String("op", op)).Error(err.Error())
			return err
		}
	}
	return nil
}
