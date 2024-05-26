package service_discovery

import (
	"context"
	"errors"
	"github.com/MikhUd/blockchain/pkg/db"
	"github.com/gocql/gocql"
	"time"
)

const registryTable = "registry"

type Registry struct {
	source db.Database
}

func NewRegistry(source db.Database) *Registry {
	return &Registry{source: source}
}

type Member struct {
	Id        string
	Address   string
	PublicKey string
	Timestamp int64
}

func (r *Registry) RegisterMember(ctx context.Context, id, address, pubKey string) error {
	if !r.source.IsConnected() {
		if err := r.source.Connect(ctx); err != nil {
			return err
		}
	}
	ts := time.Now().Unix()
	query := `INSERT INTO ` + registryTable + ` (id, address, pub_key, timestamp) VALUES (?, ?, ?, ?)`
	return r.source.Query(ctx, query, id, address, pubKey, ts)
}

func (r *Registry) UnRegisterMember(ctx context.Context, id string) error {
	if !r.source.IsConnected() {
		if err := r.source.Connect(ctx); err != nil {
			return err
		}
	}
	query := `DELETE FROM ` + registryTable + ` WHERE id = ?`
	return r.source.Query(ctx, query, id)
}

func (r *Registry) GetMembers(ctx context.Context) ([]Member, error) {
	var (
		members []Member
		member  Member
	)
	if !r.source.IsConnected() {
		if err := r.source.Connect(ctx); err != nil {
			return nil, err
		}
	}
	query := `SELECT * FROM ` + registryTable
	iter, err := r.source.(db.Scanner).Iter(ctx, query, &member)
	if err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	for iter.Scan(&member.Id, &member.Address, &member.PublicKey, &member.Timestamp) {
		members = append(members, member)
	}
	if err = iter.Close(); err != nil {
		return nil, err
	}
	return members, nil
}
