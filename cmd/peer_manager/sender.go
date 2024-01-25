package peer_manager

type Sender interface {
	Send(ctx *Context) error
}
