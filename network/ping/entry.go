package ping

import (
	"errors"
	"fmt"
	"github.com/neo-hu/network-probe-tool/pkg/icmp"
	"math"
	"net"
	"syscall"
	"time"
)

type EVType int

const (
	DefaultCount    = 1

	EVPing    EVType = 0
	EVTimeout EVType = 1

	ResultUnUsed = time.Duration(0)
	ResultError  = time.Duration(-1)
)

type reply struct {
	elapsed  time.Duration
	sendTime time.Time
}

type entry struct {
	host string
	ip   net.IP
	sa   syscall.Sockaddr
	mode icmp.Mode

	dataSize int
	count    int
	timeout  time.Duration
	interval time.Duration

	evTime time.Time
	index  int
	send   int
	recv   int
	typ    EVType

	result []*reply

	// dev 标准差
	oldMean float64
	m2      float64
}

func (e entry) Dev() float64 {
	if e.recv <= 0 {
		return 0
	}
	return math.Sqrt(e.m2 / float64(e.recv))
}
func (e entry) String() string {
	return fmt.Sprintf("<entry %s[%d], send:%d>", e.host, e.index, e.send)
}

func newEntry(host string, opts ...AddressOption) (*entry, error) {
	ns, err := net.LookupHost(host)
	if err != nil {
		return nil, err
	}
	e := &entry{host: host, dataSize: icmp.DefaultDataSize,
		count:    DefaultCount,
		interval: icmp.DefaultInterval,
		timeout:  icmp.DefaultTimeout,
		typ:      EVPing,
	}
	for _, ipAddr := range ns {
		addr := net.ParseIP(ipAddr)
		if addr == nil {
			err = fmt.Errorf("parse ip %q is nil", ipAddr)
			continue
		}
		e.ip = addr.To4()
		if e.ip != nil {
			e.mode = icmp.IPV4Address
			var sa = &syscall.SockaddrInet4{}
			copy(sa.Addr[:], e.ip)
			e.sa = sa
			break
		} else {
			e.ip = addr.To16()
			if e.ip != nil {
				e.mode = icmp.IPV4Address
				var sa = &syscall.SockaddrInet6{}
				copy(sa.Addr[:], e.ip)
				e.sa = sa
				break
			}
		}
	}
	if e.mode == 0 {
		return nil, errors.New("host ip is nil")
	}
	for _, opt := range opts {
		opt(e)
	}
	return e, nil
}

type AddressOption func(*entry)

func TimeoutOption(timeout time.Duration) AddressOption {
	return func(e *entry) {
		if timeout > 0 {
			e.timeout = timeout
		}
	}
}
func AddressIntervalOption(interval time.Duration) AddressOption {
	return func(e *entry) {
		e.interval = interval
	}
}

func CountOpt(count int) AddressOption {
	return func(e *entry) {
		if count > 0 {
			e.count = count
		}
	}
}
func DataSizeOption(size int) AddressOption {
	return func(e *entry) {
		if size >= 0 {
			e.dataSize = size
		}
	}
}
