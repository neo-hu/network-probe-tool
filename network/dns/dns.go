package dns

import (
	"fmt"
	"github.com/miekg/dns"
	"net"
	"time"
)

const (
	TypeA uint16 = dns.TypeA
	TypeANY uint16 = dns.TypeANY
	TypeTXT uint16 = dns.TypeTXT
	TypeCNAME uint16 = dns.TypeCNAME
	TypeMX uint16 = dns.TypeMX
)

type DNS struct {
	nameserver       string
	network          string
	timeout          time.Duration
	recursionDesired bool
}

type Option func(*DNS)

func NetworkOption(network string) Option {
	return func(m *DNS) {
		m.network = network
	}
}
func RecursionDesiredOption(recursionDesired bool) Option {
	return func(m *DNS) {
		m.recursionDesired = recursionDesired
	}
}
func TimeoutOption(timeout time.Duration) Option {
	return func(m *DNS) {
		m.timeout = timeout
	}
}


func NewDNS(nameserver string,opts ...Option) *DNS {
	d := &DNS{network: "udp", timeout: 2 * time.Second, nameserver: nameserver}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *DNS) Exchange(addr string, t uint16) (time.Duration, []string, error) {

	c := &dns.Client{
		Net:     d.network,
		Timeout: d.timeout,
	}
	m := new(dns.Msg)
	m.Compress = true
	m.SetQuestion(dns.Fqdn(addr), t)
	nameserver := d.nameserver
	if net.ParseIP(nameserver) != nil {
		nameserver = net.JoinHostPort(nameserver, "53")
	}
	r, rtt, err := c.Exchange(m, nameserver)
	if err != nil {
		return 0, nil, err
	}
	if r.Rcode != dns.RcodeSuccess {
		return 0, nil, fmt.Errorf("failed to get an valid answer %v %s", r.Rcode, dns.RcodeToString[r.Rcode])
	}
	var result []string
	for _, k := range r.Answer {
		switch t1 := k.(type) {
		case *dns.A:
			if t == TypeA || t == TypeANY  {
				result = append(result, t1.A.String())
			}
		case *dns.TXT:
			if t == TypeTXT || t == TypeANY  {
				result = append(result, t1.Txt...)
			}
		case *dns.MX:
			if t == TypeMX || t == TypeANY  {
				result = append(result, t1.Mx)
			}
		case *dns.CNAME:
			if t == TypeCNAME || t == TypeANY  {
				result = append(result, t1.Target)
			}
		}
	}
	return rtt, result, nil
}
