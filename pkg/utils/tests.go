package utils

import "github.com/MikhUd/blockchain/pkg/config"

const (
	LocalPath = "../config/local.yaml"
)

func LoadConfig(path string) *config.Config {
	return config.MustLoad(path)
}
