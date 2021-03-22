package udp

import (
	"fmt"
	"net"
	"strings"
)

func GetLocalAddr(rAddr string) (net.IP, error) {
	conn, err := net.Dial("udp", net.JoinHostPort(rAddr, "80"))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().String()
	ip := strings.Split(localAddr, ":")

	if len(ip) < 1 {
		return nil, fmt.Errorf("local ip addr not found")
	}

	lAddr := net.ParseIP(ip[0])
	return lAddr, nil
}