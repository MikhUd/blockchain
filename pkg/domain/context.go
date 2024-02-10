package domain

type Context struct {
	message any
}

func (context *Context) Message() any {
	return context.message
}
