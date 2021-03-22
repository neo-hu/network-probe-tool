package _select

import (
	"golang.org/x/sys/unix"
	"syscall"
	"time"
)

type Select struct {
	fds  []int
	rfds *syscall.FdSet
}


func NewSelect() *Select {
	return &Select{
		rfds: &syscall.FdSet{},
	}
}

func (s *Select) Add(fd int) {
	s.fds = append(s.fds, fd)
}

type Recv struct {
	fd int
}

func (r *Recv) Fd() int {
	return r.fd
}
func (r *Recv) Read(p []byte) (n int, from syscall.Sockaddr, err error) {
	return syscall.Recvfrom(r.fd, p, 0)
}

func (s *Select) CanRead(waitTime time.Duration) (*Recv, error) {
	var socket int
	FD_ZERO(s.rfds)
	for _, fd := range s.fds {
		if fd > 0 {
			FD_SET(s.rfds, fd)
			if fd > socket {
				socket = fd
			}
		}
	}
	if socket <= 0 {
		return nil, nil
	}
	timeout := syscall.NsecToTimeval(waitTime.Nanoseconds())
selectAgain:
	err := SysSelect(socket+1, s.rfds, nil, nil, &timeout)
	if err != nil {
		if err == unix.EINTR {
			goto selectAgain
		}
	}
	for _, fd := range s.fds {
		if fd > 0 && FD_ISSET(s.rfds, fd) {
			return &Recv{fd}, nil
		}
	}
	return nil, nil
}

func fdget(fd int, fds *syscall.FdSet) (index, offset int) {
	index = fd / (syscall.FD_SETSIZE / len(fds.Bits)) % len(fds.Bits)
	offset = fd % (syscall.FD_SETSIZE / len(fds.Bits))
	return
}

func FD_SET(p *syscall.FdSet, i int) {
	idx, pos := fdget(i, p)
	p.Bits[idx] |= 1 << uint(pos)
}

func FD_ISSET(p *syscall.FdSet, i int) bool {
	idx, pos := fdget(i, p)
	return p.Bits[idx]&(1<<uint(pos)) != 0
}
func FD_ZERO(p *syscall.FdSet) {
	for i := range p.Bits {
		p.Bits[i] = 0
	}
}
