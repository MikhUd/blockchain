package context

import (
	"errors"
	"github.com/MikhUd/blockchain/pkg/api/message"
)

type Context struct {
	sender    *message.PID
	receiver  *message.PID
	msg       any
	parentCtx *Context
}

var Refused = errors.New("connection refused")

func New(msg any) *Context {
	return &Context{
		msg: msg,
	}
}

func (c *Context) WithParent(parentCtx *Context) *Context {
	c.parentCtx = parentCtx
	return c
}

func (c *Context) Sender() *message.PID {
	return c.sender
}

func (c *Context) Receiver() *message.PID {
	return c.receiver
}

func (c *Context) Msg() any {
	return c.msg
}

func (c *Context) WithSender(s *message.PID) *Context {
	c.sender = s
	return c
}

func (c *Context) WithReceiver(r *message.PID) *Context {
	c.receiver = r
	return c
}
