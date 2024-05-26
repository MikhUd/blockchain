package stream

import (
	"fmt"
	"github.com/MikhUd/blockchain/pkg/context"
	"log/slog"
	"sync"
)

type Engine struct {
	sender  string
	writers map[string]Sender
	mu      sync.RWMutex
}

func NewEngine(sender string) *Engine {
	return &Engine{sender: sender, writers: make(map[string]Sender)}
}

func (e *Engine) Send(ctx *context.Context) error {
	var (
		op   = "engine.Send"
		addr = ctx.Receiver().GetAddr()
	)
	e.addWriter(addr)
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

func (e *Engine) Sender() string {
	return e.sender
}

func (e *Engine) addWriter(addr string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.writers[addr]
	if ok {
		return
	}
	e.writers[addr] = NewWriter(addr)
}
