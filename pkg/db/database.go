package db

import (
	"context"
	"github.com/gocql/gocql"
)

type Database interface {
	Connect(ctx context.Context) error
	IsConnected() bool
	Close(ctx context.Context) error
	Query(ctx context.Context, query string, args ...interface{}) error
}

type Scanner interface {
	Scan(ctx context.Context, query string, dst interface{}, args ...interface{}) error
	Iter(ctx context.Context, query string, dst interface{}, args ...interface{}) (*gocql.Iter, error)
}
