package http

import (
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptrace"
	"time"
)

type CheckRedirectFunc func(req *http.Request, via []*http.Request) error

type Trace struct {
	req                   *http.Request
	checkRedirect         CheckRedirectFunc
	idleConnTimeout       time.Duration
	tlsHandshakeTimeout   time.Duration
	expectContinueTimeout time.Duration
	maxBody               int64

	tlsClientConfig *tls.Config
	client          *http.Client

	//1.GetConn
	hostPort string
	// 2.DNSStart
	dnsStartTime time.Time
	dnsElapsed   time.Duration
	dnsHost      string

	// 3. DNSDone
	dnsAddrs []net.IPAddr

	// 4.ConnectStart
	connectStartTime time.Time
	connectAddr      string

	// 5.ConnectDone
	connectElapsed  time.Duration
	connectDoneTime time.Time

	// 6.tls
	isTLS        bool
	tlsStartTime time.Time
	tlsDoneTime  time.Time
	tlsHandshakeElapsed  time.Duration

	// 7.GotConn
	gotConnTime time.Time
	// 8.GotFirstResponseByte
	firstResponseByte       time.Time
	serverProcessingElapsed time.Duration
}

type Option func(*Trace)

func TLSClientConfig(tlsClientConfig *tls.Config) Option {
	return func(m *Trace) {
		m.tlsClientConfig = tlsClientConfig
	}
}
func MaxBodyOption(maxBody int64) Option {
	return func(m *Trace) {
		m.maxBody = maxBody
	}
}
func ExpectContinueTimeoutOption(expectContinueTimeout time.Duration) Option {
	return func(m *Trace) {
		m.expectContinueTimeout = expectContinueTimeout
	}
}
func IdleConnTimeoutOption(idleConnTimeout time.Duration) Option {
	return func(m *Trace) {
		m.idleConnTimeout = idleConnTimeout
	}
}
func TLSHandshakeTimeoutOption(tlsHandshakeTimeout time.Duration) Option {
	return func(m *Trace) {
		m.tlsHandshakeTimeout = tlsHandshakeTimeout
	}
}

func CheckRedirectOption(f CheckRedirectFunc) Option {
	return func(m *Trace) {
		m.checkRedirect = f
	}
}

func NewTrace(ctx context.Context, req *http.Request, opts ...Option) (*Trace, error) {
	t := &Trace{
		maxBody:               1024 * 1024,
		idleConnTimeout:       3 * time.Second,
		tlsHandshakeTimeout:   3 * time.Second,
		expectContinueTimeout: 1 * time.Second,
	}
	trace := &httptrace.ClientTrace{
		GetConn:              t.getConn,
		DNSStart:             t.dnsStart,
		DNSDone:              t.dnsDone,
		ConnectStart:         t.connectStart,
		ConnectDone:          t.connectDone,
		TLSHandshakeStart:    t.tlsHandshakeStart,
		TLSHandshakeDone:     t.tlsHandshakeDone,
		GotConn:              t.gotConn,
		GotFirstResponseByte: t.gotFirstResponseByte,
	}
	t.req = req.WithContext(httptrace.WithClientTrace(ctx, trace))
	for _, opt := range opts {
		opt(t)
	}
	tr := &http.Transport{
		IdleConnTimeout:       t.idleConnTimeout,
		TLSHandshakeTimeout:   t.tlsHandshakeTimeout,
		ExpectContinueTimeout: t.expectContinueTimeout,
	}
	switch req.URL.Scheme {
	case "https":
		if t.tlsClientConfig == nil {
			host, _, err := net.SplitHostPort(req.Host)
			if err != nil {
				host = req.Host
			}
			tr.TLSClientConfig = &tls.Config{
				ServerName:         host,
				InsecureSkipVerify: true,
				Certificates:       nil,
			}
		} else {
			tr.TLSClientConfig = t.tlsClientConfig
		}
	}

	t.client = &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if t.checkRedirect != nil {
				return t.checkRedirect(req, via)
			}
			return nil
		},
	}
	return t, nil
}

func (t *Trace) Start() (Result, error) {
	resp, err := t.client.Do(t.req)
	r := Result{
		Addr:             t.connectAddr,
		DNSLookup:        t.dnsElapsed,
		TCPConnection:    t.connectElapsed,
		ServerProcessing: t.serverProcessingElapsed,
	}
	if !t.dnsStartTime.IsZero() {
		r.Total = time.Now().Sub(t.dnsStartTime)
	}
	if err != nil {
		return r, err
	}
	r.Status = resp.StatusCode
	r.Proto = resp.Proto
	r.Header = resp.Header
	if t.req.URL.Scheme == "https" {
		r.TLSHandshake = t.gotConnTime.Sub(t.connectDoneTime)
	}
	if t.maxBody > 0 {
		r.Length, err = readResponseBody(resp, t.maxBody)
		r.ContentTransfer = time.Since(t.firstResponseByte)
	}
	r.Total = time.Now().Sub(t.dnsStartTime)
	return r, err
}

func isRedirect(resp *http.Response) bool {
	return resp.StatusCode > 299 && resp.StatusCode < 400
}

func readResponseBody(resp *http.Response, max int64) (int64, error) {
	if isRedirect(resp) {
		return 0, nil
	}
	w := ioutil.Discard
	//var b bytes.Buffer
	// todo 不需要读完，
	length, err := io.CopyN(w, resp.Body, max)
	if resp.ContentLength > 0 {
		length = resp.ContentLength
	}
	if err == io.EOF {
		return length, nil
	}
	return length, err
}

func (t *Trace) getConn(hostPort string) {
	t.hostPort = hostPort
}

func (t *Trace) dnsStart(info httptrace.DNSStartInfo) {
	t.dnsStartTime = time.Now()
	t.dnsHost = info.Host

}

func (t *Trace) dnsDone(info httptrace.DNSDoneInfo) {
	t.dnsAddrs = info.Addrs
	t.dnsElapsed = time.Since(t.dnsStartTime)
}

func (t *Trace) connectStart(network string, addr string) {
	t.connectAddr = addr
	t.connectStartTime = time.Now()
	// todo ip
	if t.dnsStartTime.IsZero() {
		t.dnsStartTime = t.connectStartTime
	}
}

func (t *Trace) connectDone(network, addr string, err error) {
	t.connectElapsed = time.Since(t.connectStartTime)
	t.connectDoneTime = time.Now()
}

func (t *Trace) tlsHandshakeStart() {
	t.isTLS = true
	t.tlsStartTime = time.Now()
}
func (t *Trace) tlsHandshakeDone(tls.ConnectionState, error) {
	t.tlsDoneTime = time.Now()
	t.tlsHandshakeElapsed = t.tlsDoneTime.Sub(t.tlsStartTime)
}

func (t *Trace) gotConn(info httptrace.GotConnInfo) {
	t.gotConnTime = time.Now()
}

func (t *Trace) gotFirstResponseByte() {
	t.firstResponseByte = time.Now()
	t.serverProcessingElapsed = t.firstResponseByte.Sub(t.gotConnTime)
}
