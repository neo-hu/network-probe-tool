package main

import (
	"flag"
	"fmt"
	"github.com/neo-hu/network-probe-tool/network/mtr"
	"github.com/neo-hu/network-probe-tool/pkg/icmp"
	"log"
	"os"
	"strings"
	"time"
)

type StringSet map[string]struct{}

func (ms StringSet) Add(mk string) {
	ms[mk] = struct{}{}
}

func (ms StringSet) String() string {
	var values []string
	for metric, _ := range ms {
		values = append(values, string(metric))
	}
	return strings.Join(values, ",")
}

func main() {
	var count = 3
	var maxTTL = 30
	var timeout = icmp.DefaultTimeout
	var interval = icmp.DefaultInterval
	var dataSize = icmp.DefaultDataSize
	flag.IntVar(&maxTTL, "max-ttl", maxTTL, "Specifies the maximum number of hops (max time-to-live value) traceroute will probe")
	flag.IntVar(&count, "c", count, "count of pings to send to each target")
	flag.DurationVar(&timeout, "t", timeout, "individual target initial timeout")
	flag.DurationVar(&interval, "i", interval, "interval between sending ping packets")
	flag.IntVar(&dataSize, "d", dataSize, "amount of ping data to send, in bytes")
	flag.Parse()
	target := flag.Arg(0)
	if target == "" {
		fmt.Printf("Usage of %s www.ip8.me\n", os.Args[0])
		flag.PrintDefaults()
		return
	}
	m, err := mtr.NewMtr(target,
		mtr.CountOption(count),
		mtr.TimeoutOption(timeout),
		mtr.DataSizeOption(uint32(dataSize)),
		mtr.MaxTTLOption(maxTTL),
		mtr.IntervalOption(interval))
	if err != nil {
		log.Fatal(err)
	}
	result, err := m.Start()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s => %s\n", result.LocalIp, result.TargetIp)
	fmt.Println("ttl", "host", "max", "min", "avg", "loss")
	for i, ttlResult := range result.TTL {
		if len(ttlResult.Entries) <= 0 {
			continue
		}
		ips := make(StringSet)
		var (
			sum   time.Duration
			max   time.Duration
			min   time.Duration
			num   time.Duration
			avg   time.Duration
			reply float64
		)
		for _, entry := range ttlResult.Entries {
			if len(entry.IP) != 0 {
				reply += 1
				ips.Add(entry.IP.String())
				sum += entry.Elapsed
				num += 1
				if entry.Elapsed > max {
					max = entry.Elapsed
				}
				if min == 0 || entry.Elapsed < min {
					min = entry.Elapsed
				}
			}

		}
		if num > 0 {
			avg = sum / num
		}
		fmt.Println(i+1, ips.String(), max, min, avg,
			(float64(len(ttlResult.Entries))-reply)/float64(len(ttlResult.Entries))*100)
	}
}
