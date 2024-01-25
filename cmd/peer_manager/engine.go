package peer_manager

import (
	bcproto "github.com/MikhUd/blockchain/protos/blockchain"
	"log/slog"
)

type Engine struct {
	PID    *bcproto.PID
	writer Sender
}

func NewEngine(PID *bcproto.PID, writer Sender) *Engine {
	return &Engine{PID: PID, writer: writer}
}

func (e *Engine) Send(msg any) error {
	const op = "engine.Send"
	ctx := &Context{msg: msg}
	err := e.writer.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error("engine send error")
		return err
	}
	return nil
}
