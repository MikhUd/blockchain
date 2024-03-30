package utils

import (
	"github.com/MikhUd/blockchain/pkg/stream"
	"net"
	"sync"
)

type ConnAble interface {
	GetMutex() *sync.RWMutex
	GetConnPool() map[string]net.Conn
	GetEngine() *stream.Engine
}

func SaveConn(saveTo ConnAble, connAddr string, conn net.Conn) {
	var (
		mu       = saveTo.GetMutex()
		connPool = saveTo.GetConnPool()
	)
	mu.Lock()
	defer mu.Unlock()
	connPool[connAddr] = conn
}

func CloseConn(saveTo ConnAble, connAddr string) error {
	var (
		mu       = saveTo.GetMutex()
		connPool = saveTo.GetConnPool()
		engine   = saveTo.GetEngine()
	)
	mu.Lock()
	defer mu.Unlock()
	if conn, ok := connPool[connAddr]; ok == true {
		if err := conn.Close(); err == nil {
			if writer := engine.GetWriter(connAddr); writer != nil {
				writer.Shutdown()
			}
		}
		delete(connPool, connAddr)
	}
	return nil
}
