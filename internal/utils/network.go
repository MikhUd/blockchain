package utils

import (
	"fmt"
	"math/rand"
)

func GetRandomListenAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", rand.Intn(50000)+10000)
}
