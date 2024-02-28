package main

import (
	"flag"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/domain/blockchain"
	"github.com/MikhUd/blockchain/pkg/node"
	"log"
	"net"
	"os"
)

var (
	nodePort    = flag.String("node_port", "", "node port")
	clusterPort = flag.String("cluster_port", "", "cluster port")
	configPath  = flag.String("config_path", "", "path to local config")
)

func main() {
	flag.Parse()
	hostStr := os.Getenv("HOST")
	portStr := os.Getenv("PORT")
	if hostStr != "" && portStr != "" {
		*nodePort = net.JoinHostPort(hostStr, portStr)
	}
	*clusterPort = fmt.Sprintf("cluster%s", *clusterPort)
	cfg := config.MustLoad(*configPath)
	bc := blockchain.New(*cfg)
	n := node.New(*nodePort, *clusterPort, bc)
	if err := n.Start(); err != nil {
		log.Fatal(err)
	}
}
