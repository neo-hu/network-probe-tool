package http

import (
	"fmt"
	"net/http"
	"time"
)

type Result struct {
	Addr             string // ConnectStart
	Proto            string
	Length           int64
	Status           int
	DNSLookup        time.Duration
	TCPConnection    time.Duration
	TLSHandshake     time.Duration
	ServerProcessing time.Duration
	ContentTransfer  time.Duration
	Header           http.Header
	Total            time.Duration
}

func (r Result) Format(s fmt.State, verb rune) {
	fmt.Fprintf(s, "DNS lookup:        %4d ms\n",
		int(r.DNSLookup/time.Millisecond))
	fmt.Fprintf(s, "TCP connection:    %4d ms\n",
		int(r.TCPConnection/time.Millisecond))
	if r.TLSHandshake > 0 {
		fmt.Fprintf(s, "TLS handshake:     %4d ms\n",
			int(r.TLSHandshake/time.Millisecond))
	}
	fmt.Fprintf(s, "Server processing: %4d ms\n",
		int(r.ServerProcessing/time.Millisecond))
	if r.ContentTransfer > 0 {
		fmt.Fprintf(s, "Content transfer:  %4d ms\n",
			int(r.ContentTransfer/time.Millisecond))
	}
	fmt.Fprintf(s, "Total:             %4d ms\n\n",
		int(r.Total/time.Millisecond))
	fmt.Fprintf(s, "Addr:              %s\n", r.Addr)
	fmt.Fprintf(s, "Status:            %d\n", r.Status)
	fmt.Fprintf(s, "Length:            %d\n", r.Length)
	fmt.Fprintf(s, "Header:            %v\n", r.Header)
}