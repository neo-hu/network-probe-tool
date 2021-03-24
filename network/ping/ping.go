package ping

import (
	"container/heap"
	"fmt"
	"github.com/neo-hu/network-probe-tool/network"
	icmp2 "github.com/neo-hu/network-probe-tool/pkg/icmp"
	_select "github.com/neo-hu/network-probe-tool/pkg/select"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"
	"os"
	"sync/atomic"
	"syscall"
	"time"
)

type entryReply struct {
	r *reply
	e *entry
}
type Ping struct {
	ipv4Fd int
	ipv6Fd int
	s      *_select.Select

	ident        int
	interval     time.Duration
	entryHeap    EntryHeap
	entries      []*entry
	startingFlag int32
	closeFlag    int32

	buffer []byte

	seqPool *icmp2.SeqPool
	//seq    int
	//seqMap map[int]*entryReply
}

type Option func(*Ping)

func IdentOption(ident uint16) Option {
	return func(ping *Ping) {
		ping.ident = int(ident)
	}
}

func IntervalOption(interval time.Duration) Option {
	return func(ping *Ping) {
		ping.interval = interval
	}
}

func NewPing(opts ...Option) *Ping {
	p := &Ping{
		ident:    os.Getpid() & 0xFFFF,
		interval: time.Millisecond,
		buffer:   make([]byte, 4096),
		s:        _select.NewSelect(),
	}

	for _, opt := range opts {
		opt(p)
	}
	p.seqPool = icmp2.NewSeqPool(uint16(p.ident))
	return p
}

func (p *Ping) Add(host string, opts ...AddressOption) error {
	if atomic.LoadInt32(&p.startingFlag) == 1 {
		return nil
	}
	e, err := newEntry(host, opts...)
	if err != nil {
		return err
	}
	if e.mode == icmp2.IPV6Address {
		if p.ipv6Fd == 0 {
			p.ipv6Fd, err = icmp2.Listen(e.mode)
			if err != nil {
				return err
			}
			p.s.Add(p.ipv6Fd)
		}
	} else {
		if p.ipv4Fd == 0 {
			p.ipv4Fd, err = icmp2.Listen(e.mode)
			if err != nil {
				return err
			}
			p.s.Add(p.ipv4Fd)
		}
	}
	p.enqueue(e)
	p.entries = append(p.entries, e)
	return nil
}

func (p *Ping) enqueue(e *entry) {
	heap.Push(&p.entryHeap, e)
}
func (p *Ping) dequeue() *entry {
	e := heap.Pop(&p.entryHeap)
	return e.(*entry)
}
func (p *Ping) remove(e *entry) {
	heap.Remove(&p.entryHeap, e.index)
}

func (p *Ping) send(e *entry, r *reply) error {
	e.send += 1
	var (
		typ icmp.Type
		fd  int
		err error
	)
	if e.mode == icmp2.IPV6Address {
		typ = ipv6.ICMPTypeEchoRequest
		fd = p.ipv6Fd
	} else {
		typ = ipv4.ICMPTypeEcho
		fd = p.ipv4Fd
	}
	ident, seq := p.seqPool.Apply(&entryReply{
		r: r,
		e: e,
	})
	if e.dataSize > len(p.buffer) {
		p.buffer = make([]byte, e.dataSize)
	}
	bytes, err := (&icmp.Message{
		Type: typ, Code: 0,
		Body: &icmp.Echo{
			ID:   int(ident),
			Seq:  int(seq),
			Data: p.buffer[:e.dataSize],
		},
	}).Marshal(nil)
	if err != nil {
		p.seqPool.Free(ident, seq)
		return err
	}
	err = syscall.Sendto(fd, bytes, 0, e.sa)
	if err != nil {
		p.seqPool.Free(ident, seq)
	}
	return err
}

func (p *Ping) Stop() (err error) {
	if atomic.LoadInt32(&p.startingFlag) != 1 {
		return network.ErrNotRunning
	}

	if !atomic.CompareAndSwapInt32(&p.closeFlag, 0, 1) {
		return network.ErrAlreadyClosed
	}

	if p.ipv6Fd != 0 {
		if cErr := unix.Close(p.ipv6Fd); cErr != nil {
			err = cErr
		}
	}
	if p.ipv4Fd != 0 {
		if cErr := unix.Close(p.ipv4Fd); cErr != nil {
			err = cErr
		}
	}
	return
}

func (p *Ping) Start() ([]Result, error) {
	if atomic.SwapInt32(&p.startingFlag, 1) == 1 {
		return nil, network.ErrAlreadyRunning
	}
	currentTime := time.Now()
	var lastSendTime time.Time
	var waitTime time.Duration
	for !p.isClosing() && p.entryHeap.Len() != 0 {
		e := p.entryHeap.Peek().(*entry)
		if e.evTime.Before(currentTime) {
			if e.typ == EVPing {
				if currentTime.Sub(lastSendTime) < p.interval {
					// TODO 判断发送的间隔
					goto waitForReply
				}
				e := p.dequeue()
				lastSendTime = time.Now()
				r := &reply{
					sendTime: lastSendTime,
					elapsed:  ResultUnUsed,
				}

				e.result = append(e.result, r)
				err := p.send(e, r)
				if err != nil {
					r.elapsed = ResultError
				}
				if e.send < e.count {
					e.typ = EVPing
					e.evTime = lastSendTime.Add(e.interval)
				} else {
					e.typ = EVTimeout
					e.evTime = lastSendTime.Add(e.timeout)
				}
				p.enqueue(e)
			} else if e.typ == EVTimeout {
				p.remove(e)
			}
		}
	waitForReply:
		if p.entryHeap.Len() != 0 {
			e := p.entryHeap.Peek().(*entry)
			if e.evTime.IsZero() {
				waitTime = 0
			} else {
				waitTime = e.evTime.Sub(currentTime)
				if waitTime < 0 {
					waitTime = 0
				}
			}
			if e.typ == EVPing {
				if waitTime < p.interval {
					lt := currentTime.Sub(lastSendTime)
					if lt < p.interval {
						waitTime = p.interval - lt
					} else {
						waitTime = p.interval
					}
				}
			}
		} else {
			waitTime = 0
		}

		for !p.isClosing() {
			if w, _ := p.waitForReply(waitTime); !w {
				break
			}
			waitTime = 0
		}
		currentTime = time.Now()
	}
	if p.isClosing() {
		return nil, network.ErrAlreadyClosed
	}
	results := make([]Result, len(p.entries))
	for index, e := range p.entries {
		rs := Result{
			Packets:  e.send,
			Received: e.recv,
			IP:       e.ip,
			Dev:      e.Dev(),
		}
		rs.Host = e.host
		for _, r := range e.result {
			rs.Times = append(rs.Times, r.elapsed)
		}
		results[index] = rs
	}
	return results, nil
}

func (p *Ping) waitForReply(waitTime time.Duration) (bool, error) {
	s, err := p.s.CanRead(waitTime)
	if err != nil {
		return false, err
	}
	if s == nil {
		// todo timeout
		return false, nil
	}

	n, ra, err := s.Read(p.buffer[:])
	if err != nil {
		return false, err
	}
	var proto int
	var start int
	switch ra := ra.(type) {
	case *syscall.SockaddrInet4:
		proto = ipv4.ICMPType(0).Protocol()
		_, start = icmp2.StripIPv4Header(p.buffer[:n])
	case *syscall.SockaddrInet6:
		proto = ipv6.ICMPType(0).Protocol()
	default:
		return false, fmt.Errorf("%T type err", ra)
	}
	m, err := icmp.ParseMessage(proto, p.buffer[start:n])
	if err != nil {
		return false, err
	}
	if pkt, ok := m.Body.(*icmp.Echo); ok {
		v := p.seqPool.Free(uint16(pkt.ID), uint16(pkt.Seq))
		if v == nil {
			return false, nil
		}
		r := v.(*entryReply)
		if r.r.elapsed == ResultUnUsed {
			r.r.elapsed = time.Since(r.r.sendTime)
			elapsed := float64(r.r.elapsed) / float64(time.Millisecond)
			if r.e.recv == 0 {
				r.e.oldMean = elapsed
			} else {
				newMean := r.e.oldMean + (elapsed-r.e.oldMean)/(float64(r.e.recv)+1)
				r.e.m2 += (elapsed - r.e.oldMean) * (elapsed - newMean)
				r.e.oldMean = newMean
			}
			r.e.recv += 1
			if r.e.recv >= r.e.count {
				// todo 探测完成
				p.remove(r.e)
			}
		}

	}
	return true, nil
}

func (p *Ping) isClosing() bool {
	return atomic.LoadInt32(&p.closeFlag) == 1
}
