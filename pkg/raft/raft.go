package raft

import (
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/raft/member"
	"math/rand/v2"
)

type Raft struct {
	cfg        config.Config
	memberPool member.Pool
}

func RandElection(cfg config.Config) int {
	return rand.IntN(cfg.MaxLeaderElectionMs-cfg.MinLeaderElectionMs) + cfg.MinLeaderElectionMs
}
