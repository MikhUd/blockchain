package context

import (
	"github.com/MikhUd/blockchain/pkg/grpcapi/message"
)

type Context struct {
	sender    *message.PID
	receiver  Receiver
	msg       any
	parentCtx *Context
}

func NewContext(pid *message.PID, sender *message.PID, receiver Receiver, msg any) *Context {
	return &Context{
		sender:   sender,
		receiver: receiver,
		msg:      msg,
	}
}

func (c *Context) Invoke() error {
	return c.receiver.Receive(c)
}

func (c *Context) WithParent(parentCtx *Context) *Context {
	c.parentCtx = parentCtx
	return c
}

func (c *Context) Sender() *message.PID {
	return c.sender
}

func (c *Context) Receiver() Receiver {
	return c.receiver
}

func (c *Context) Msg() any {
	return c.msg
}

func (c *Context) WithSender(s *message.PID) *Context {
	c.sender = s
	return c
}

func (c *Context) WithReceiver(r Receiver) *Context {
	c.receiver = r
	return c
}

func (c *Context) WithMsg(msg any) *Context {
	c.msg = msg
	return c
}
