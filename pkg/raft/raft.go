package raft

import (
	"context"
	"crypto/ecdsa"
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
	AddMember(id string, addr string, publicKey *ecdsa.PublicKey) error
	RemoveMember(id string) error
	GetMembers() []*Member
	GetLeaderID() string
}

type IRaft interface {
	Start() error
	Stop(ctx context.Context, id string) error
	GetCluster() ICluster
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
	return r.cluster.Stop()
}

func (r *Raft) GetCluster() ICluster {
	return r.cluster
}
