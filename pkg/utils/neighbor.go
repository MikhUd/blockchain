package utils

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"time"
)

var (
	PATTERN   = regexp.MustCompile(`((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?\.){3})(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)`)
	LOCALHOST = "127.0.0.1"
)

func IsFoundHost(host string, port uint16) bool {
	timeout := 2 * time.Second

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(int(port))), timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	return true
}

func FindNeighbors(myHost string, myPort uint16, startIp uint8, endIp uint8, startPort uint16, endPort uint16) []string {
	address := fmt.Sprintf("%s:%d", myHost, myPort)

	fmt.Printf("find neighbors loop, address: %s\n", address)
	matched := PATTERN.FindStringSubmatch(myHost)
	if matched == nil {
		return nil
	}
	prefixHost := matched[1]
	lastIp, _ := strconv.Atoi(matched[len(matched)-1])
	neighbors := make([]string, 0)
	for port := startPort; port <= endPort; port += 1 {
		for ip := startIp; ip <= endIp; ip += 1 {
			guessHost := fmt.Sprintf("%s%d", prefixHost, lastIp+int(ip))
			guessTarget := fmt.Sprintf("%s:%d", guessHost, port)
			if guessTarget != address && IsFoundHost(guessHost, port) {
				neighbors = append(neighbors, guessTarget)
			}
		}
	}
	return neighbors
}

func GetHost() string {
	hostname, err := os.Hostname()
	if err != nil {
		return LOCALHOST
	}
	address, err := net.LookupHost(hostname)
	if err != nil {
		return LOCALHOST
	}
	return address[0]
}
