package icmp

import (
	"fmt"
	"golang.org/x/sys/unix"
	"net"
	"syscall"
	"time"
)

const (
	DefaultDataSize = 64
	DefaultTimeout  = time.Second
	DefaultInterval = time.Second
)
type Mode int

var (
	IPV4Address Mode = 4
	IPV6Address Mode = 6
)



func Listen(m Mode) (int, error) {
	var (
		proto  int
		domain int
	)
	switch m {
	case IPV4Address:
		proto = syscall.IPPROTO_ICMP
		domain = syscall.AF_INET
	case IPV6Address:
		proto = syscall.IPPROTO_ICMPV6
		domain = syscall.AF_INET6
	default:
		return 0, fmt.Errorf("unexpected proto %v", m)
	}
	sock, err := syscall.Socket(
		domain,
		syscall.SOCK_RAW,
		proto,
	)
	if err != nil {
		return 0, err
	}
	if err := unix.SetNonblock(sock, true); err != nil {
		return 0, err
	}
	return sock, err
}
func StripIPv4Header(b []byte) (net.IP, int) {
	if len(b) < 20 {
		return nil, 0
	}
	l := int(b[0]&0x0f) << 2
	if b[0]>>4 != 4 {
		return nil, 0
	}
	src := net.IPv4(b[12], b[13], b[14], b[15])
	return src, l
}
