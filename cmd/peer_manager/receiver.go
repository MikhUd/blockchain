package peer_manager

type Receiver interface {
	Receive(ctx *Context) error
}
