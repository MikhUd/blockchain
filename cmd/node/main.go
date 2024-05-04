package main

import (
	"flag"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/consts"
	"github.com/MikhUd/blockchain/pkg/domain/blockchain"
	"github.com/MikhUd/blockchain/pkg/node"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
)

var (
	nodePort    = flag.String("node_port", "", "node port")
	clusterPort = flag.String("cluster_port", "", "cluster port")
	configPath  = flag.String("config_path", "", "path to local config")
)

func main() {
	go func() {
		http.ListenAndServe("localhost:6061", nil)
	}()
	flag.Parse()
	cfg := config.MustLoad(*configPath)
	hostStr := os.Getenv("HOST")
	portStr := os.Getenv("PORT")
	if hostStr != "" && portStr != "" {
		*nodePort = net.JoinHostPort(hostStr, portStr)
	}
	if cfg.Env == consts.EnvDev {
		*clusterPort = fmt.Sprintf("cluster%s", *clusterPort)
	}
	bc := blockchain.New(*cfg)
	n := node.New(*nodePort, bc, *cfg)
	if err := n.Start(*clusterPort); err != nil {
		log.Fatal(err)
	}
}
