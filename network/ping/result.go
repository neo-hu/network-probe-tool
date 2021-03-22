package ping

import (
	"fmt"
	"net"
	"time"
)

type Result struct {
	Host     string
	Dev      float64
	Packets  int
	Received int
	IP       net.IP
	Times    []time.Duration
}

func (r Result) String() string {
	var rt string
	if r.Received > 0 {
		var (
			min   time.Duration
			avg   time.Duration
			max   time.Duration
			sum   time.Duration
			count time.Duration
		)
		for _, duration := range r.Times {
			if duration <= 0 {
				continue
			}
			sum += duration
			count += 1
			if min == 0 || min > duration {
				min = duration
			}

			if max == 0 || max < duration {
				max = duration
			}
		}
		if count > 0 {
			avg = max / count
		}
		rt = fmt.Sprintf("\nround-trip min/avg/max/mdev = %v/%v/%v/%.2f", min, avg, max, r.Dev)
	}
	return fmt.Sprintf("[%s(%s)]%d packets transmitted, %d packets received, %.2f%% packet loss%s",
		r.Host, r.IP, r.Packets, r.Received, r.Loss(), rt)
}

func (r Result) Loss() float64 {
	var loss float64 = 0
	if r.Packets > 0 {
		loss = float64((r.Packets-r.Received)*100) / float64(r.Packets)
	}
	return loss
}
