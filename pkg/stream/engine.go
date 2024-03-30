package stream

import (
	"fmt"
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/context"
	"log/slog"
	"sync"
)

type Engine struct {
	PID     *message.PID
	writers map[string]Sender
	mu      sync.RWMutex
}

func NewEngine(PID *message.PID) *Engine {
	return &Engine{PID: PID, writers: make(map[string]Sender)}
}

func (e *Engine) AddWriter(writer Sender) {
	e.mu.RLock()
	_, ok := e.writers[writer.Addr()]
	if ok {
		return
	}
	defer e.mu.RUnlock()
	e.writers[writer.Addr()] = writer
}

func (e *Engine) Send(ctx *context.Context) error {
	var op = "engine.Send"
	addr := ctx.Receiver().GetAddr()
	if len(e.writers) == 0 {
		slog.With(slog.String("op", op)).Error("empty writers")
		return fmt.Errorf("no writers available")
	}
	writer, ok := e.writers[addr]
	if !ok {
		slog.With(slog.String("op", op)).Error(fmt.Sprintf("no writers found by addr:%s", addr))
	}
	err := writer.Send(ctx)
	if err != nil {
		slog.With(slog.String("op", op)).Error("engine send error")
		return err
	}
	return nil
}

func (e *Engine) GetWriter(addr string) Sender {
	if writer, ok := e.writers[addr]; ok == true {
		return writer
	}
	return nil
}
