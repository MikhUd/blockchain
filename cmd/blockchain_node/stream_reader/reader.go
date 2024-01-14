package stream_reader

import (
	"context"
	"errors"
	"github.com/MikhUd/blockchain/cmd/blockchain_node"
	"github.com/MikhUd/blockchain/cmd/blockchain_node/peer_manager"
	"github.com/MikhUd/blockchain/protos/blockchain"
	"log/slog"
)

type Reader struct {
	blockchain.DRPCBlockchainNodeUnimplementedServer
	pm           *peer_manager.PeerManager
	deserializer blockchain_node.Deserializer
}

func New(pm *peer_manager.PeerManager) *Reader {
	return &Reader{
		pm: pm,
	}
}

func (r *Reader) Receive(stream blockchain.DRPCBlockchainNode_ReceiveStream) error {
	const op = "reader.Receive"
	for {
		pack, err := stream.Recv()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				slog.With(slog.String("op", op)).Debug("context cancelled")
				break
			}
			slog.With(slog.String("op", op)).Error("receive stream error")
			return err
		}
		for _, msg := range pack.Messages {
			typeName := pack.GetTypeNames()[msg.TypeNameIndex]
			slog.Debug(typeName)
			//payload, err := r.deserializer.Deserialize(msg.Data, typeName)
			if err != nil {
				slog.With(slog.String("op", op)).Error("deserialize by typename error")
			}
			//sender := pack.Senders[msg.SenderIndex]
		}
	}

	return nil
}
