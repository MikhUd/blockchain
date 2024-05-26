package main

import (
	"flag"
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/domain/blockchain"
	"github.com/MikhUd/blockchain/pkg/node"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
)

var (
	nodePort   = flag.String("node_port", "", "node port")
	configPath = flag.String("config_path", "", "path to local config")
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
	bc := blockchain.New(*cfg)
	n, err := node.New(*nodePort, bc, *cfg)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	if err := n.Start(); err != nil {
		log.Fatal(err)
	}
}
