package utils

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

func GetRandomListenAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", rand.Intn(50000)+10000)
}

func GetPortFromAddr(addr string) int {
	parts := strings.Split(addr, ":")
	port, _ := strconv.Atoi(parts[1])
	return port
}
