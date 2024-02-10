package cluster

import (
	"github.com/MikhUd/blockchain/pkg/grpcapi/message"
	"github.com/MikhUd/blockchain/pkg/infrastructure/context"
	"log/slog"
)

type Engine struct {
	PID    *message.PID
	writer Sender
}

func NewEngine(PID *message.PID, writer Sender) *Engine {
	return &Engine{PID: PID, writer: writer}
}

func (e *Engine) Send(msg any) error {
	const op = "engine.Send"
	slog.Info(op)
	ctx := context.NewContext(nil, nil, nil, msg)
	err := e.writer.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error("engine send error")
		return err
	}
	return nil
}
