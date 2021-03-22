package main

import (
	"context"
	"fmt"
	"github.com/neo-hu/network-probe-tool/network/http"
	"log"
	gohttp "net/http"
)

func main() {
	req, err := gohttp.NewRequest("GET", "https://www.ip8.me/", nil)
	if err != nil {
		log.Fatal(err)
	}
	t, err := http.NewTrace(context.Background(), req, http.MaxBodyOption(1024 * 1024 * 10))
	if err != nil {
		log.Fatal(err)
	}
	rs, err := t.Start()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", rs)
}