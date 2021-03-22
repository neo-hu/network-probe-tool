package mtr

import (
	"net"
	"time"
)

type TTLResultEntry struct {
	IP      net.IP
	Elapsed time.Duration
}
type TTLResult struct {
	Entries []TTLResultEntry
}

type Result struct {
	LocalIp  net.IP
	TargetIp net.IP
	TTL      []TTLResult
}

type seqEntry struct {
	t         time.Time
	ip        net.IP
	replyTime time.Time
	end       bool
}

type seqResult struct {
	entries []seqEntry
	reply   int
}
