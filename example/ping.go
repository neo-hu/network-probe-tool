package main

import (
	"flag"
	"fmt"
	"github.com/neo-hu/network-probe-tool/network/ping"
	"github.com/neo-hu/network-probe-tool/pkg/icmp"
	"log"
	"os"
)

func main() {
	var count = 3
	var timeout = icmp.DefaultTimeout
	var interval = icmp.DefaultInterval
	var dataSize = icmp.DefaultDataSize

	flag.IntVar(&count, "count", count, "count of pings to send to each target")
	flag.DurationVar(&timeout, "timeout", timeout, "individual target initial timeout")
	flag.DurationVar(&interval, "interval", interval, "interval between sending ping packets")
	flag.IntVar(&dataSize, "data-size", dataSize,"amount of ping data to send, in bytes")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Printf("Usage of %s www.ip8.me\n", os.Args[0])
		flag.PrintDefaults()
		return
	}

	p := ping.NewPing(ping.IntervalOption(interval))
	for _, host := range flag.Args() {
		err := p.Add(host,
			ping.CountOpt(count),
			ping.TimeoutOption(timeout),
			ping.DataSizeOption(dataSize),
			ping.AddressIntervalOption(interval))
		if err != nil {
			log.Fatal(err)
		}
	}
	rs, err := p.Start()
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range rs {
		fmt.Println(r.String())
	}
}
