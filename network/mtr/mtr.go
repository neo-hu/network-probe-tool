package mtr

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/neo-hu/network-probe-tool/network"
	icmp2 "github.com/neo-hu/network-probe-tool/pkg/icmp"
	_select "github.com/neo-hu/network-probe-tool/pkg/select"
	"github.com/neo-hu/network-probe-tool/pkg/udp"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type seqValue struct {
	ttl   int // 第几跳
	index int // count index

	evPrev *seqValue /* double linked list for the event-queue */
	evNext *seqValue /* double linked list for the event-queue */
	evTime time.Time
}

type Mtr struct {
	socketFd int
	s        *_select.Select
	target   string
	sa       syscall.Sockaddr
	ip       net.IP
	localIp  net.IP

	mode         icmp2.Mode
	currentCount int
	currentTTL   int

	pingTTL       map[int]int // 如果已经到达对端，当前ping循环不用不用继续发送ttl
	currentMaxTTL int

	seq    int32
	seqMap map[int]*seqValue

	forceIPv4, forceIPv6 bool
	ident                int           // icmp ident
	maxTTL               int           //  最大多少跳 , default 60
	count                int           //  每一跳发包的数量 , default 3
	interval             time.Duration //  发包的间隔 , default 1 Millisecond
	timeout              time.Duration //  超时时间 , default 1 Second
	dataSize             int           //  发包的大小 , default 64

	evFirst *seqValue
	evLast  *seqValue
	buffer  []byte

	result []*seqResult

	startingFlag int32
	closeFlag    int32
}

func NewMtr(target string, opts ...Option) (*Mtr, error) {
	m := &Mtr{
		s:          _select.NewSelect(),
		target:     target,
		ident:      os.Getpid() & 0xFFFF,
		interval:   icmp2.DefaultInterval,
		maxTTL:     60,
		dataSize:   icmp2.DefaultDataSize,
		count:      3,
		timeout:    icmp2.DefaultTimeout,
		seqMap:     map[int]*seqValue{},
		pingTTL:    map[int]int{},
		currentTTL: 1,
		buffer:     make([]byte, 4096),
	}
	for _, opt := range opts {
		opt(m)
	}

	ips, err := net.LookupIP(target)

	for _, ip := range ips {
		m.ip = ip
		if IsIPv4(ip) && !m.forceIPv6 {
			sa1 := &syscall.SockaddrInet4{}
			copy(sa1.Addr[:], ip.To4())
			m.sa = sa1
			m.mode = icmp2.IPV4Address
			break
		} else if IsIPv6(ip) && !m.forceIPv4 {
			sa1 := &syscall.SockaddrInet6{}
			copy(sa1.Addr[:], ip.To16())
			m.sa = sa1
			m.mode = icmp2.IPV6Address
			break
		}
	}
	if m.sa == nil {
		return nil, errors.New("there is not A or AAAA record")
	}
	m.socketFd, err = icmp2.Listen(m.mode)
	if err != nil {
		return nil, err
	}

	localIp, err := udp.GetLocalAddr(target)
	if err == nil {
		m.localIp = localIp
	}

	m.s.Add(m.socketFd)
	m.result = make([]*seqResult, m.maxTTL+1, m.maxTTL+1)
	return m, nil
}

type Option func(*Mtr)

func IdentOption(ident uint16) Option {
	return func(m *Mtr) {
		m.ident = int(ident)
	}
}
func DataSizeOption(dataSize uint32) Option {
	return func(m *Mtr) {
		m.dataSize = int(dataSize)
	}
}
func ForceIPv4Option() Option {
	return func(m *Mtr) {
		m.forceIPv4 = true
	}
}
func ForceIPv6Option() Option {
	return func(m *Mtr) {
		m.forceIPv6 = true
	}
}
func TimeoutOption(timeout time.Duration) Option {
	return func(m *Mtr) {
		m.timeout = timeout
	}
}
func CountOption(count int) Option {
	return func(m *Mtr) {
		m.count = count
	}
}

func MaxTTLOption(maxTTL int) Option {
	return func(m *Mtr) {
		m.maxTTL = maxTTL
	}
}

func IntervalOption(interval time.Duration) Option {
	return func(m *Mtr) {
		m.interval = interval
	}
}

func IsIPv4(ip net.IP) bool {
	return len(ip.To4()) == net.IPv4len
}

// IsIPv6 returns true if ip version is v6
func IsIPv6(ip net.IP) bool {
	if r := strings.Index(ip.String(), ":"); r != -1 {
		return true
	}
	return false
}

func (m *Mtr) Close() error {
	if m.socketFd > 0 {
		return unix.Close(m.socketFd)
	}
	return nil
}

func (m *Mtr) Start() (*Result, error) {
	if atomic.SwapInt32(&m.startingFlag, 1) == 1 {
		return nil, network.ErrAlreadyRunning
	}
	var lastSendTime time.Time
	var waitTime time.Duration
	for m.count > m.currentCount {
		ld := time.Now().Sub(lastSendTime)
		if ld < m.interval {
			// 前一个包发送的间隔没到
			waitTime = m.interval - ld
			goto waitForReply
		}
		lastSendTime = time.Now()
		m.send(lastSendTime)
		m.currentTTL += 1
		if _, ok := m.pingTTL[m.currentCount]; ok || m.currentTTL > m.maxTTL {
			// todo 当前这轮ping已经到达对端了 or 已经到上限，发送下一轮ping
			m.currentTTL = 1
			m.currentCount += 1
		}
		waitTime = m.interval
	waitForReply:
		for {
			if w, _ := m.waitForReply(waitTime); !w {
				break
			}
			// 如果已经接收到一个数据，继续接收
			waitTime = 0
		}
	}

	for m.evFirst != nil {
		waitTime = m.evFirst.evTime.Sub(time.Now())
		if waitTime < 0 {
			// todo 接收超时了
			m.evRemove(m.evFirst)
			continue
		}
		for {
			if w, _ := m.waitForReply(waitTime); !w {
				break
			}
			// 如果已经接收到一个数据，继续接收
			waitTime = 0
		}
	}

	result := &Result{
		TargetIp: m.ip,
		LocalIp:  m.localIp,
	}

	//for ttl, r := range m.result {
	//	if ttl == 0 {
	//		continue
	//	}
	//	fmt.Println(ttl, len(r.entries))
	//	for _, entry := range r.entries {
	//		fmt.Println(entry.replyTime)
	//	}
	//	fmt.Println()
	//}
	for ttl, r := range m.result[:m.currentMaxTTL+1] {
		if ttl == 0 {
			continue
		}
		if r == nil {
			break
		}
		tr := TTLResult{}
		for i, entry := range r.entries {
			maxTTL := m.pingTTL[i]
			if maxTTL > 0 && maxTTL < ttl {
				continue
			}
			e := TTLResultEntry{IP: entry.ip}
			if len(entry.ip) != 0 {
				e.Elapsed = entry.replyTime.Sub(entry.t)
			}
			tr.Entries = append(tr.Entries, e)
		}
		if len(tr.Entries) == 0 {
			break
		}
		result.TTL = append(result.TTL, tr)
	}
	return result, nil
}

func (m *Mtr) waitForReply(waitTime time.Duration) (bool, error) {
	s, err := m.s.CanRead(waitTime)
	if err != nil {
		return false, err
	}
	if s == nil {
		// todo timeout
		return false, nil
	}
	n, ra, err := s.Read(m.buffer[:])
	var proto int
	var start int
	var src net.IP
	switch sockaddr := ra.(type) {
	case *syscall.SockaddrInet4:
		proto = ipv4.ICMPType(0).Protocol()
		src, start = icmp2.StripIPv4Header(m.buffer[:n])
	case *syscall.SockaddrInet6:
		proto = ipv6.ICMPType(0).Protocol()
		src = net.IP(ra.(*syscall.SockaddrInet6).Addr[:])
	default:
		return false, fmt.Errorf("%T type err", sockaddr)
	}
	msg, err := icmp.ParseMessage(proto, m.buffer[start:n])
	if err != nil {
		return false, err
	}
	switch msg.Type {
	case ipv6.ICMPTypeTimeExceeded:
	case ipv4.ICMPTypeTimeExceeded:
		// todo ttl 超时
		// todo StripIPv4Header(len: start) + ParseMessage (len: 4) +  TimeExceeded (len: 4) + 24 + ident (len:2)  + seq (len:2)
		pos := start + 4 + 4 + 24
		if n < pos+4 {
			break
		}
		if int(binary.BigEndian.Uint16(m.buffer[pos:n])) != m.ident {
			// todo 非本程序发送的
			break
		}
		pkgSeq := binary.BigEndian.Uint16(m.buffer[pos+2 : n])
		val, ok := m.seqMap[int(pkgSeq)]
		if !ok {
			break
		}
		e := m.result[val.ttl].entries[val.index]
		e.ip = src
		e.replyTime = time.Now()
		m.result[val.ttl].entries[val.index] = e
		m.result[val.ttl].reply += 1
		m.evRemove(val)
		delete(m.seqMap, int(pkgSeq))
	case ipv4.ICMPTypeEchoReply, ipv6.ICMPTypeEchoReply:
		if pkt, ok := msg.Body.(*icmp.Echo); ok {
			if pkt.ID != m.ident {
				// todo 非本程序发送的
				break
			}
			val, ok := m.seqMap[pkt.Seq]
			if !ok {
				break
			}
			e := m.result[val.ttl].entries[val.index]
			m.result[val.ttl].reply += 1
			e.ip = src
			e.replyTime = time.Now()
			e.end = true
			m.result[val.ttl].entries[val.index] = e
			// todo 已经到目的了
			delete(m.seqMap, pkt.Seq)
			m.evRemove(val)
			if m.pingTTL[val.index] == 0 || m.pingTTL[val.index] > val.ttl {
				m.pingTTL[val.index] = val.ttl
				if m.currentMaxTTL == 0 || val.ttl > m.currentMaxTTL {
					m.currentMaxTTL = val.ttl
				}
			}
		}
	}
	return true, nil
}

func (m *Mtr) send(lastSendTime time.Time) error {
	seq := m.incr()
	if m.result[m.currentTTL] == nil {
		m.result[m.currentTTL] = &seqResult{}
	}
	sv := &seqValue{
		ttl:    m.currentTTL,
		index:  len(m.result[m.currentTTL].entries),
		evTime: lastSendTime.Add(m.timeout),
	}
	m.result[m.currentTTL].entries = append(m.result[m.currentTTL].entries, seqEntry{
		t: lastSendTime,
	})

	m.seqMap[seq] = sv
	m.evEnqueue(sv)
	var b []byte
	var err error
	if m.dataSize > len(m.buffer) {
		m.buffer = make([]byte, m.dataSize)
	}
	if m.mode == icmp2.IPV4Address {
		b, err = (&icmp.Message{
			Type: ipv4.ICMPTypeEcho, Code: 0,
			Body: &icmp.Echo{
				ID: m.ident, Seq: seq,
				Data: m.buffer[:m.dataSize],
			},
		}).Marshal(nil)
		if err != nil {
			return err
		}
		err = syscall.SetsockoptInt(m.socketFd, syscall.IPPROTO_IP, syscall.IP_TTL, m.currentTTL)
		if err != nil {
			return err
		}
	} else {
		b, err = (&icmp.Message{
			Type: ipv6.ICMPTypeEchoRequest, Code: 0,
			Body: &icmp.Echo{
				ID: m.ident, Seq: seq,
				Data: m.buffer[:m.dataSize],
			},
		}).Marshal(nil)
		if err != nil {
			return err
		}
		err = syscall.SetsockoptInt(m.socketFd, syscall.IPPROTO_IPV6, syscall.IPV6_UNICAST_HOPS, m.currentTTL)
		if err != nil {
			return err
		}
	}
	return syscall.Sendto(m.socketFd, b, 0, m.sa)
}

func (m *Mtr) evRemove(h *seqValue) {
	if m.evFirst == h {
		m.evFirst = h.evNext
	}
	if m.evLast == h {
		m.evLast = h.evPrev
	}
	if h.evPrev != nil {
		h.evPrev.evNext = h.evNext
	}
	if h.evNext != nil {
		h.evNext.evPrev = h.evPrev
	}
	h.evPrev = nil
	h.evNext = nil
}
func (m *Mtr) evEnqueue(h *seqValue) {
	var i *seqValue
	var iPrev *seqValue
	/* Empty list */
	if m.evLast == nil {
		h.evNext = nil
		h.evPrev = nil
		m.evFirst = h
		m.evLast = h
		return
	}
	if h.evTime.After(m.evLast.evTime) {
		h.evNext = nil
		h.evPrev = m.evLast
		m.evLast.evNext = h
		m.evLast = h
		return
	}
	i = m.evLast
	for {
		iPrev = i.evPrev
		if iPrev == nil || h.evTime.After(iPrev.evTime) {
			h.evPrev = iPrev
			h.evNext = i
			i.evPrev = h
			if iPrev != nil {
				iPrev.evNext = h
			} else {
				m.evFirst = h
			}
			return
		}
		i = iPrev
	}
}

func (m *Mtr) incr() int {
	return int(atomic.AddInt32(&m.seq, 1))
}
