package peer_manager

import (
	bcproto "github.com/MikhUd/blockchain/protos/blockchain"
)

type Context struct {
	sender    *bcproto.PID
	receiver  Receiver
	msg       any
	parentCtx *Context
}

func NewContext(pid *bcproto.PID, sender *bcproto.PID, receiver Receiver, msg any) *Context {
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

func (c *Context) Sender() *bcproto.PID {
	return c.sender
}

func (c *Context) Receiver() Receiver {
	return c.receiver
}

func (c *Context) Msg() any {
	return c.msg
}

func (c *Context) WithSender(s *bcproto.PID) *Context {
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
