package main

import (
	"flag"
	"github.com/MikhUd/blockchain/pkg/cluster"
	"github.com/MikhUd/blockchain/pkg/config"
	"log/slog"
)

var (
	clusterPort = flag.String("cluster_port", ":8080", "cluster port")
	configPath  = flag.String("config_path", "", "path to local config")
)

func main() {
	flag.Parse()
	cfg := config.MustLoad(*configPath)
	c := cluster.New(*cfg, *clusterPort)
	if err := c.Start(); err != nil {
		slog.Error(err.Error())
	}
}
