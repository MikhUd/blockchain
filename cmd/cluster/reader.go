package cluster

import (
	"github.com/MikhUd/blockchain/pkg/grpcapi/cluster"
	"github.com/MikhUd/blockchain/pkg/grpcapi/message"
	"github.com/MikhUd/blockchain/pkg/infrastructure/context"
	"log"
	"log/slog"
	"reflect"
)

type reader struct {
	cluster.DRPCPeerManagerUnimplementedServer
	c            *Cluster
	deserializer Deserializer
}

func newReader(c *Cluster) *reader {
	return &reader{
		c: c,
	}
}

func (r *reader) Receive(stream cluster.DRPCPeerManager_ReceiveStream) error {
	var ser = ProtoSerializer{}
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}

		data, err := ser.Deserialize(msg.Data, msg.TypeName)

		if err != nil {
			log.Fatal(err)
			return err
		}

		log.Println("Received Data Type:", reflect.TypeOf(data))

		switch data.(type) {
		case *message.TransactionRequest:
			ctx := context.NewContext(nil, nil, nil, data)
			err = r.c.Receive(*ctx)
			if err != nil {
				slog.Error(err.Error())
				return err
			}
		}
	}

	return nil
}
