package peer_manager

import (
	bcproto "github.com/MikhUd/blockchain/protos/blockchain"
	"log"
	"reflect"
)

type reader struct {
	bcproto.DRPCPeerManagerUnimplementedServer
	pm           *PeerManager
	deserializer Deserializer
}

func newReader(pm *PeerManager) *reader {
	return &reader{
		pm: pm,
	}
}

func (r *reader) Receive(stream bcproto.DRPCPeerManager_ReceiveStream) error {
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
		case *bcproto.TransactionRequest:
			ctx := &Context{msg: data}
			err = r.pm.Receive(*ctx)
			if err != nil {
				r.pm.Logger.Error(err.Error())
				return err
			}
		}
	}

	return nil
}
