package cassandra

import (
	"context"
	"errors"
	"github.com/gocql/gocql"
	"log/slog"
	"time"
)

type DB struct {
	keySpace string
	hosts    []string
	cluster  *gocql.ClusterConfig
	session  *gocql.Session
}

var defaultTimeout = time.Second * 2

func NewDB(keySpace string, hosts []string) *DB {
	return &DB{keySpace: keySpace, hosts: hosts}
}

func (db *DB) Connect(ctx context.Context) error {
	var err error
	db.cluster = gocql.NewCluster(db.hosts...)
	db.cluster.Consistency = gocql.Quorum
	db.cluster.Keyspace = db.keySpace
	db.cluster.Timeout = defaultTimeout
	done := make(chan struct{})
	go func() {
		defer close(done)
		if db.session, err = db.cluster.CreateSession(); err != nil {
			slog.Error("error session create", err.Error())
			return
		}
	}()
	select {
	case <-done:
		slog.Info("connect to cassandra")
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (db *DB) IsConnected() bool {
	return db.session != nil
}

func (db *DB) Close(ctx context.Context) error {
	db.cluster.Timeout = defaultTimeout
	if db.session != nil {
		done := make(chan struct{})
		defer close(done)
		go func() {
			defer close(done)
			db.session.Close()
		}()
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (db *DB) Query(ctx context.Context, query string, args ...interface{}) error {
	db.cluster.Timeout = defaultTimeout
	if db.session == nil {
		return errors.New("session is nil")
	}
	q := db.session.Query(query, args...)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := q.Exec()
		if err != nil {
			slog.Error("query execution error:", err.Error())
		}
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (db *DB) Scan(ctx context.Context, query string, dst interface{}, args ...interface{}) error {
	var errorCh = make(chan error)
	db.cluster.Timeout = defaultTimeout
	if db.session == nil {
		return errors.New("session is nil")
	}
	q := db.session.Query(query, args...)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := q.Scan(dst)
		if err != nil {
			errorCh <- err
		}
	}()
	select {
	case err := <-errorCh:
		return err
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (db *DB) Iter(ctx context.Context, query string, dst interface{}, args ...interface{}) (*gocql.Iter, error) {
	if db.session == nil {
		return nil, errors.New("session is nil")
	}

	q := db.session.Query(query, args...)
	iter := q.Iter()
	go func() {
		defer iter.Close()
		select {
		case <-ctx.Done():
			return
		}
	}()
	return iter, nil
}
