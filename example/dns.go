package main

import (
	"fmt"
	"github.com/neo-hu/network-probe-tool/network/dns"
	"log"
)

func main() {
	elapsed, rs, err := dns.NewDNS("8.8.8.8", dns.NetworkOption("tcp")).
		Exchange("ip8.me", dns.TypeANY)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(elapsed, rs)
}
