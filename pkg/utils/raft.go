package utils

import (
	"github.com/MikhUd/blockchain/pkg/config"
	"math/rand/v2"
)

func RandElection(cfg config.Config) int {
	return rand.IntN(cfg.MaxLeaderElectionMs-cfg.MinLeaderElectionMs) + cfg.MinLeaderElectionMs
}
