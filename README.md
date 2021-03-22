## Install 

Use `go get`

```bash
$ go get github.com/neo-hu/network-probe-tool
```

### ping
```go
p := ping.NewPing(ping.IntervalOption(time.Millisecond))
for _, host := range []string{"www.ip8.me", "www.sina.com.cn"} {
    err := p.Add(host,
        ping.CountOpt(10),
        ping.TimeoutOption(time.Second),
        ping.DataSizeOption(64),
        ping.AddressIntervalOption(time.Millisecond))
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
```
```
[www.ip8.me(47.254.33.50)]10 packets transmitted, 10 packets received, 0.00% packet loss
round-trip min/avg/max/mdev = 182.29158ms/18.935195ms/189.35195ms/2.12
[www.sina.com.cn(123.125.104.150)]10 packets transmitted, 9 packets received, 10.00% packet loss
round-trip min/avg/max/mdev = 5.220302ms/1.513171ms/13.618541ms/2.82
```
### http
```go
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
```
```
DNS lookup:           7 ms
TCP connection:     183 ms
TLS handshake:      396 ms
Server processing:  186 ms
Content transfer:     1 ms
Total:              775 ms

Addr:              47.254.33.50:443
Status:            200
Length:            3079
Header:            map[Content-Type:[text/html; charset=utf-8] Connection:[keep-alive] Date:[Mon, 22 Mar 2021 08:54:03 GMT]]
```


### mtr
```go
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
```
```
10.222.32.140 => 47.254.33.50
ttl host max min avg loss
1 10.222.33.254 7.750451ms 4.525032ms 5.819714ms 0
2 10.211.254.5 6.536012ms 3.976379ms 5.068225ms 0
3 10.235.254.101 10.929905ms 4.055806ms 6.736914ms 40
4 61.135.152.129 9.974369ms 5.640932ms 7.939829ms 0
5 36.51.127.33 8.326009ms 8.326009ms 8.326009ms 90
6 123.125.248.81 13.805313ms 6.868211ms 8.554925ms 20
7 61.148.152.17 13.71672ms 5.552559ms 9.452621ms 0
8 202.96.12.77 11.10152ms 6.377124ms 8.369387ms 0
9 219.158.5.146 16.093866ms 8.570245ms 12.849743ms 0
10 219.158.3.30 18.090777ms 7.696596ms 12.565781ms 0
11 219.158.16.94 165.728169ms 158.264125ms 162.276089ms 40
12 219.158.40.190 183.927293ms 179.120053ms 181.355027ms 50
13 63.243.250.54 164.118387ms 161.486486ms 162.75227ms 40
14 63.243.250.61 188.067948ms 181.589465ms 184.133623ms 50
15 209.58.86.74 186.169455ms 180.995911ms 183.420383ms 50
16 66.198.127.194 185.744145ms 182.652905ms 184.934764ms 50
17  0s 0s 0s 100
18 11.48.200.57 168.292699ms 165.400456ms 166.926276ms 40
19 11.48.160.1 187.873378ms 182.501121ms 183.95195ms 50
20 47.254.33.50 186.589539ms 182.952204ms 184.231067ms 50
```

### dns
```go
elapsed, rs, err := dns.NewDNS("8.8.8.8", dns.NetworkOption("tcp")).
    Exchange("ip8.me", dns.TypeANY)
if err != nil {
    log.Fatal(err)
}
fmt.Println(elapsed, rs)
```
