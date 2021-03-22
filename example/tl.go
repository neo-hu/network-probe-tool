package main

import (
	"fmt"
	"github.com/neo-hu/network-probe-tool/network/mtr"
	"log"
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
	m, err := mtr.NewMtr("www.ip8.me",
		mtr.CountOption(10),
		mtr.TimeoutOption(time.Millisecond),
		mtr.DataSizeOption(uint32(62)),
		mtr.MaxTTLOption(30),
		mtr.IntervalOption(time.Millisecond))
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
			(float64(len(ttlResult.Entries)) - reply)/float64(len(ttlResult.Entries)) * 100)
	}
}
