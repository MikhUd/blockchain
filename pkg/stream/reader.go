package stream

import (
	"github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/grpcapi/message"
	"github.com/MikhUd/blockchain/pkg/grpcapi/remote"
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
		data, err := r.Deserialize(msg.Data, msg.TypeName)
		if err != nil {
			log.Fatal(err)
			return err
		}

		switch data.(type) {
		case *message.JoinRequest:
			err = r.Receiver.Receive(context.New(data).WithSender(&message.PID{Addr: data.(*message.JoinRequest).Address}))
		case *message.JoinResponse:
			err = r.Receiver.Receive(context.New(data).WithSender(&message.PID{Addr: data.(*message.JoinResponse).Address}))
		case *message.HeartbeatRequest:
			err = r.Receiver.Receive(context.New(data).WithSender(&message.PID{Addr: data.(*message.HeartbeatRequest).Address}))
		case *message.HeartbeatResponse:
			err = r.Receiver.Receive(context.New(data).WithSender(&message.PID{Addr: data.(*message.HeartbeatResponse).Address}))
		}
		if err != nil {
			slog.With(slog.String("op", op)).Error(err.Error())
			return err
		}
	}

	return nil
}
