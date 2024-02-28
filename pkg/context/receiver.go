package context

type Receiver interface {
	Receive(ctx *Context) error
}
