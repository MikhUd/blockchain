package raft

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"github.com/MikhUd/blockchain/pkg/config"
	discovery "github.com/MikhUd/blockchain/pkg/service_discovery"
	"github.com/MikhUd/blockchain/pkg/utils"
	"time"
)

const (
	Candidate = iota
	Follower
	Leader
)

type ICluster interface {
	Start() error
	Stop() error
	AddMember(id string, addr string, publicKey *rsa.PublicKey) error
	RemoveMember(id string) error
	GetLeaderID() string
	HandleJoin(member Member) error
}

type IRaft interface {
	Start() error
	Stop(ctx context.Context, id string) error
}

type Raft struct {
	cfg         config.Config
	cluster     ICluster
	currentTerm uint64
}

func NewRaft(cfg config.Config, owner Owner, members []discovery.Member) (*Raft, error) {
	c, err := NewCluster(owner, time.Duration(utils.RandElection(cfg))*time.Millisecond, members, cfg)
	if err != nil {
		return nil, err
	}
	return &Raft{
		cfg:     cfg,
		cluster: c,
	}, nil
}

func (r *Raft) Start() error {
	return r.cluster.Start()
}

func (r *Raft) Stop(ctx context.Context, id string) error {
	return nil
}

func (r *Raft) AddMember(id string, addr string, publicKey *ecdsa.PublicKey) error {
	return nil
}
