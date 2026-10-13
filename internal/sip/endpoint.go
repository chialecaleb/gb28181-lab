// Package sip constrains sipgo at the wire boundary; it does not implement a
// second SIP transaction stack. sipgo owns UDP retransmission and SIP matching.
package sip

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chialecaleb/gb28181-lab/internal/gb2022"
	"github.com/chialecaleb/gb28181-lab/internal/storage"
	"github.com/emiago/sipgo"
	gosip "github.com/emiago/sipgo/sip"
)

type Event struct {
	At                                               time.Time
	Role, Direction, Transport, Local, Remote, Error string
	Bytes                                            []byte
}
type Observer func(Event)
type Handler = sipgo.RequestHandler
type streamState struct {
	framer  Framer
	updated time.Time
}
type responseExpectation struct {
	callID, from, to, method, source string
	cseq                             uint32
}
type Endpoint struct {
	Addr, Role       string
	ua               *sipgo.UserAgent
	client           *sipgo.Client
	server           *sipgo.Server
	udp              net.PacketConn
	tcp              net.Listener
	observer         Observer
	mu               sync.Mutex
	streams          map[string]*streamState
	incoming         map[string]time.Time
	expected         map[string]responseExpectation
	closed           bool
	clients, servers atomic.Int64
	connections      atomic.Int64
	done             chan struct{}
	wg               sync.WaitGroup
}

// The upstream wire tracer is global. Install it once at package initialisation,
// then route through a locked registry. Never change debug globals while running.
var wireMux = struct {
	sync.RWMutex
	owners map[string]*Endpoint
}{owners: map[string]*Endpoint{}}

type wireTracer struct{}

func init() {
	gosip.SetDefaultLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	gosip.SIPDebugTracer(wireTracer{})
	gosip.SIPDebug = true
}
func (wireTracer) SIPTraceRead(string, string, string, []byte) {} // read filter captures before parsing
func (wireTracer) SIPTraceWrite(network, local, remote string, b []byte) {
	wireMux.RLock()
	e := wireMux.owners[local]
	if e == nil { // First write on a dialled TCP connection uses the explicit Via.
		for addr, owner := range wireMux.owners {
			if bytes.Contains(b, []byte(" "+addr+";branch=")) {
				e = owner
				break
			}
		}
	}
	wireMux.RUnlock()
	if e != nil {
		wireMux.Lock()
		wireMux.owners[local] = e
		wireMux.Unlock()
		e.record("out", network, local, remote, b, "")
	}
}

func NewEndpoint(bind, role string, observer Observer, register, message Handler) (*Endpoint, error) {
	_, port, _ := net.SplitHostPort(bind)
	var udp net.PacketConn
	var tcp net.Listener
	var e error
	var addr string
	for attempt := 0; attempt < 16; attempt++ {
		udp, e = net.ListenPacket("udp4", bind)
		if e != nil {
			return nil, errors.New("SIP UDP port unavailable")
		}
		addr = udp.LocalAddr().String()
		tcp, e = net.Listen("tcp4", addr)
		if e == nil {
			break
		}
		udp.Close()
		if port != "0" {
			return nil, errors.New("SIP TCP port unavailable")
		}
	}
	if e != nil {
		return nil, errors.New("no shared ephemeral UDP/TCP port available")
	}

	ep := &Endpoint{Addr: addr, Role: role, udp: udp, observer: observer, streams: map[string]*streamState{}, incoming: map[string]time.Time{}, expected: map[string]responseExpectation{}, done: make(chan struct{})}
	ep.tcp = &limitedListener{Listener: tcp, endpoint: ep}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	parser := gosip.NewParser()
	parser.MaxMessageLength = MaxMessage
	// Keep sipgo's default transaction lifetime. Its experimental connection-
	// close termination can race the asynchronous delivery of a final response
	// already read before EOF. Caller deadlines and Close still bound pending
	// work; the registration state machine observes TCP loss independently.
	ua, e := sipgo.NewUA(sipgo.WithUserAgent("GB28181-Lab/M0"), sipgo.WithUserAgentParser(parser), sipgo.WithUserAgentTransportLayerOptions(gosip.WithTransportLayerLogger(quiet), gosip.WithTransportLayerReadFilter(ep.filter), gosip.WithTransportLayerTransports(gosip.TransportsConfig{TCP: &gosip.TransportTCP{ReadTimeout: 185 * time.Second, WriteTimeout: 5 * time.Second, DialerCreate: func(local net.Addr) net.Dialer {
		return net.Dialer{Timeout: 3 * time.Second, LocalAddr: local, KeepAlive: 60 * time.Second}
	}}})), sipgo.WithUserAgentTransactionLayerOptions(gosip.WithTransactionLayerLogger(quiet), gosip.WithTransactionLayerUnhandledResponseHandler(func(*gosip.Response) {})))
	if e != nil {
		udp.Close()
		tcp.Close()
		return nil, errors.New("SIP agent creation failed")
	}
	ep.ua = ua
	ep.server, e = sipgo.NewServer(ua, sipgo.WithServerLogger(quiet))
	if e != nil {
		ep.Close()
		return nil, e
	}
	ep.client, e = sipgo.NewClient(ua, sipgo.WithClientLogger(quiet))
	if e != nil {
		ep.Close()
		return nil, e
	}
	wrap := func(h Handler) Handler {
		return func(r *gosip.Request, tx gosip.ServerTransaction) {
			ep.servers.Add(1)
			var once sync.Once
			finish := func() { once.Do(func() { ep.servers.Add(-1) }) }
			if !tx.OnTerminate(func(string, error) { finish() }) {
				finish()
			}
			if h == nil {
				Reply(r, tx, 405, nil)
				return
			}
			h(r, tx)
		}
	}
	ep.server.OnRegister(wrap(register))
	ep.server.OnMessage(wrap(message))
	ep.server.OnNoRoute(wrap(nil))
	wireMux.Lock()
	wireMux.owners[addr] = ep
	wireMux.Unlock()
	readyUDP, readyTCP := make(chan struct{}), make(chan struct{})
	ep.udp = &readyPacket{PacketConn: udp, ready: readyUDP}
	ep.tcp = &readyListener{Listener: ep.tcp, ready: readyTCP}
	ep.wg.Add(3)
	go func() { defer ep.wg.Done(); _ = ep.server.ServeUDP(ep.udp) }()
	go func() { defer ep.wg.Done(); _ = ep.server.ServeTCP(ep.tcp) }()
	go ep.sweep()
	select {
	case <-readyUDP:
	case <-time.After(3 * time.Second):
		ep.Close()
		return nil, errors.New("SIP UDP startup timeout")
	}
	select {
	case <-readyTCP:
	case <-time.After(3 * time.Second):
		ep.Close()
		return nil, errors.New("SIP TCP startup timeout")
	}
	return ep, nil
}
func (e *Endpoint) record(direction, network, local, remote string, b []byte, reason string) {
	if e.observer != nil {
		e.observer(Event{At: time.Now().UTC(), Role: e.Role, Direction: direction, Transport: network, Local: local, Remote: remote, Bytes: bytes.Clone(b), Error: reason})
	}
}

func (e *Endpoint) filter(p gosip.TransportReadProps, b []byte) ([]byte, error) {
	local, remote := p.LocalAddr.String(), p.RemoteAddr.String()
	wireMux.Lock()
	wireMux.owners[local] = e
	wireMux.Unlock()
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, errors.New("endpoint closed")
	}
	var frames [][]byte
	var err error
	if p.Transport == "UDP" {
		err = Datagram(b)
		if err == nil {
			frames = [][]byte{b}
		}
	} else {
		key := local + "|" + remote
		s := e.streams[key]
		if s == nil {
			if len(e.streams) >= 8 {
				return nil, errors.New("TCP stream budget exhausted")
			}
			s = &streamState{}
			e.streams[key] = s
		}
		s.updated = time.Now()
		frames, err = s.framer.Feed(b)
	}
	if err != nil {
		e.record("in", p.Transport, local, remote, b, err.Error())
		return nil, err
	}
	var complete []byte
	parser := gosip.NewParser()
	parser.MaxMessageLength = MaxMessage
	for _, raw := range frames {
		responseMismatch := false
		err = nil
		m, parseErr := parser.ParseSIP(raw)
		if parseErr != nil {
			err = errors.New("invalid SIP syntax")
		} else if m.Via() == nil || m.From() == nil || m.To() == nil || m.CSeq() == nil || m.CallID() == nil {
			err = errors.New("missing SIP transaction fields")
		}
		if err == nil {
			if branch, ok := m.Via().Params.Get("branch"); !ok || !strings.HasPrefix(branch, "z9hG4bK") {
				err = errors.New("M0 requires RFC3261 branch cookie")
			}
		}
		if res, ok := m.(*gosip.Response); ok && err == nil {
			branch, _ := res.Via().Params.Get("branch")
			expected, found := e.expected[branch]
			if !found || res.CSeq().SeqNo != expected.cseq || string(res.CSeq().MethodName) != expected.method || res.CallID().Value() != expected.callID || res.From().Address.User != expected.from || res.To().Address.User != expected.to || remote != expected.source {
				err = errors.New("unmatched or stale SIP response ignored")
				responseMismatch = true
			}
		}
		if r, ok := m.(*gosip.Request); ok && err == nil {
			if r.CSeq().MethodName != r.Method || r.MaxForwards() == nil || r.From().Address.User == "" || r.To().Address.User == "" {
				err = errors.New("invalid SIP request fields")
			} else {
				branch, _ := r.Via().Params.Get("branch")
				key := branch + "|" + r.Via().Host + "|" + strconv.Itoa(r.Via().Port) + "|" + string(r.Method)
				if _, ok := e.incoming[key]; !ok {
					if len(e.incoming) >= 128 {
						err = errors.New("SIP transaction budget exhausted")
					} else {
						e.incoming[key] = time.Now().Add(33 * time.Second)
					}
				}
			}
		}
		reason := ""
		if err != nil {
			reason = err.Error()
		}
		e.record("in", p.Transport, local, remote, raw, reason)
		if err != nil {
			if p.Transport == "TCP" && !responseMismatch {
				return nil, err
			}
			continue
		}
		complete = append(complete, raw...)
	}
	return complete, nil
}

func (e *Endpoint) sweep() {
	defer e.wg.Done()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-e.done:
			return
		case now := <-tick.C:
			e.mu.Lock()
			for k, t := range e.incoming {
				if !now.Before(t) {
					delete(e.incoming, k)
				}
			}
			for k, s := range e.streams {
				_, remote, _ := strings.Cut(k, "|")
				conn, err := e.ua.TransportLayer().GetConnection("tcp", remote)
				if conn != nil {
					conn.TryClose()
				}
				if err != nil || conn == nil || now.Sub(s.updated) > 190*time.Second {
					delete(e.streams, k)
				}
			}
			// Alias keys for disconnected ephemeral TCP sockets must not grow
			// for the lifetime of a desktop session.
			activeLocals := map[string]bool{e.Addr: true}
			for key := range e.streams {
				local, _, _ := strings.Cut(key, "|")
				activeLocals[local] = true
			}
			wireMux.Lock()
			for addr, owner := range wireMux.owners {
				if owner == e && !activeLocals[addr] {
					delete(wireMux.owners, addr)
				}
			}
			wireMux.Unlock()
			e.mu.Unlock()
		}
	}
}
func (e *Endpoint) Stats() (clients, servers, connections int64) {
	return e.clients.Load(), e.servers.Load(), e.connections.Load()
}
func (e *Endpoint) AliveTCP(remote string) bool {
	c, err := e.ua.TransportLayer().GetConnection("tcp", remote)
	if err != nil || c == nil {
		return false
	}
	c.TryClose()
	return true
}
func (e *Endpoint) DropTCP(remote string) {
	c, _ := e.ua.TransportLayer().GetConnection("tcp", remote)
	if c != nil {
		if tcp, ok := c.(*gosip.TCPConnection); ok {
			tcp.Conn.SetReadDeadline(time.Now()) // Let the read loop remove and close its pool entry.
		} else {
			c.Close()
		}
		c.TryClose()
		deadline := time.Now().Add(200 * time.Millisecond)
		for time.Now().Before(deadline) {
			next, _ := e.ua.TransportLayer().GetConnection("tcp", remote)
			if next == nil {
				break
			}
			next.TryClose()
			time.Sleep(time.Millisecond)
		}
	}
}
func (e *Endpoint) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	close(e.done)
	e.mu.Unlock()
	if e.udp != nil {
		e.udp.Close()
	}
	if e.tcp != nil {
		e.tcp.Close()
	}
	if e.ua != nil {
		e.ua.Close()
	}
	e.wg.Wait()
	wireMux.Lock()
	for addr, owner := range wireMux.owners {
		if owner == e {
			delete(wireMux.owners, addr)
		}
	}
	wireMux.Unlock()
}

type RequestSpec struct {
	Method, From, To, Domain, Target, Transport, CallID, Tag, Branch, Authorization string
	CSeq                                                                            uint32
	Expires                                                                         *int
	Body                                                                            []byte
	// COMPAT-001 is an explicit application choice, never a retry fallback.
	RegisterUserinfo bool
}

func (e *Endpoint) Build(s RequestSpec) (*gosip.Request, error) {
	if s.Method != "REGISTER" && s.Method != "MESSAGE" {
		return nil, errors.New("unsupported outgoing SIP method")
	}
	if s.RegisterUserinfo && s.Method != "REGISTER" {
		return nil, errors.New("REGISTER compatibility cannot change another method")
	}
	if !gb2022.ValidID(s.From) || !gb2022.ValidID(s.To) || strings.ContainsAny(s.Domain, "\r\n\x00 <>@;\"") || s.Domain == "" {
		return nil, errors.New("invalid SIP identity or domain")
	}
	host, port, err := net.SplitHostPort(s.Target)
	portNumber, portError := strconv.Atoi(port)
	if err != nil || net.ParseIP(host) == nil || portError != nil || portNumber < 1 || portNumber > 65535 {
		return nil, errors.New("invalid SIP target")
	}
	if s.Transport != "UDP" && s.Transport != "TCP" {
		return nil, errors.New("unsupported SIP transport")
	}
	if s.CSeq < 1 || s.CSeq > 2147483647 {
		return nil, errors.New("invalid CSeq")
	}
	for _, v := range []string{s.CallID, s.Tag, s.Branch, s.Authorization} {
		if strings.ContainsAny(v, "\r\n\x00") {
			return nil, errors.New("SIP header injection rejected")
		}
	}
	if s.CallID == "" {
		s.CallID = storage.RandomID()
	}
	if s.Tag == "" {
		s.Tag = storage.RandomID()[:12]
	}
	if s.Branch == "" {
		s.Branch = "z9hG4bK-" + storage.RandomID()
	}
	// GB 6.1.2 permits the numeric domain in From/To. Routing URIs still
	// use an RFC3261 hostport; the authentication realm is not a network host.
	uri := "sip:" + s.To + "@" + s.Target
	if s.Method == "REGISTER" && !s.RegisterUserinfo {
		uri = "sip:" + s.Target
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s SIP/2.0\r\nVia: SIP/2.0/%s %s;branch=%s;rport\r\nFrom: <sip:%s@%s>;tag=%s\r\nTo: <sip:%s@%s>\r\nCall-ID: %s\r\nCSeq: %d %s\r\nMax-Forwards: 70\r\n", s.Method, uri, s.Transport, e.Addr, s.Branch, s.From, s.Domain, s.Tag, s.To, s.Domain, s.CallID, s.CSeq, s.Method)
	if s.Method == "REGISTER" {
		// REGISTER To is the registering address-of-record, not registrar ID.
		text := b.String()
		text = strings.Replace(text, "To: <sip:"+s.To+"@", "To: <sip:"+s.From+"@", 1)
		b.Reset()
		b.WriteString(text)
		fmt.Fprintf(&b, "Contact: <sip:%s@%s;transport=%s>\r\nX-GB-Ver: 3.0\r\n", s.From, e.Addr, strings.ToLower(s.Transport))
		if s.Expires != nil {
			fmt.Fprintf(&b, "Expires: %d\r\n", *s.Expires)
		}
	}
	if s.Authorization != "" {
		b.WriteString("Authorization: " + s.Authorization + "\r\n")
	}
	if len(s.Body) > 0 {
		b.WriteString("Content-Type: " + gb2022.ContentType + "\r\n")
	}
	fmt.Fprintf(&b, "Content-Length: %d\r\n\r\n", len(s.Body))
	b.Write(s.Body)
	raw := []byte(b.String())
	if err := Datagram(raw); err != nil {
		return nil, err
	}
	p := gosip.NewParser()
	p.MaxMessageLength = MaxMessage
	m, err := p.ParseSIP(raw)
	if err != nil {
		return nil, errors.New("cannot build SIP request")
	}
	r := m.(*gosip.Request)
	r.SetTransport(s.Transport)
	r.SetDestination(s.Target)
	if s.Transport == "UDP" {
		host, port, _ := net.SplitHostPort(e.Addr)
		n, _ := strconv.Atoi(port)
		r.Laddr = gosip.Addr{IP: net.ParseIP(host), Port: n}
	}
	return r, nil
}

var ErrUseTCP = errors.New("message exceeds 1300 bytes; choose TCP")

func (e *Endpoint) Send(ctx context.Context, r *gosip.Request) (*gosip.Response, error) {
	if r.Transport() == "UDP" && len(r.String()) > 1300 {
		return nil, ErrUseTCP
	}
	if e.clients.Add(1) > 16 {
		e.clients.Add(-1)
		return nil, errors.New("SIP client transaction budget exhausted")
	}
	defer e.clients.Add(-1)
	branch, _ := r.Via().Params.Get("branch")
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, errors.New("endpoint closed")
	}
	if _, exists := e.expected[branch]; exists {
		e.mu.Unlock()
		return nil, errors.New("duplicate outgoing transaction")
	}
	e.expected[branch] = responseExpectation{callID: r.CallID().Value(), from: r.From().Address.User, to: r.To().Address.User, method: string(r.Method), source: r.Destination(), cseq: r.CSeq().SeqNo}
	e.mu.Unlock()
	defer func() { e.mu.Lock(); delete(e.expected, branch); e.mu.Unlock() }()
	// Explicit no-op disables sipgo's implicit CSeq increment/header generation.
	tx, err := e.client.TransactionRequest(ctx, r, func(*sipgo.Client, *gosip.Request) error { return nil })
	if err != nil {
		return nil, errors.New("SIP transport unavailable")
	}
	defer tx.Terminate()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tx.Done():
			return nil, errors.New("SIP transaction timed out or connection closed")
		case res := <-tx.Responses():
			if res == nil {
				continue
			}
			if res.StatusCode < 200 {
				continue
			}
			if res.CSeq() == nil || res.CSeq().SeqNo != r.CSeq().SeqNo || res.CallID() == nil || res.CallID().Value() != r.CallID().Value() || res.From() == nil || res.From().Address.User != r.From().Address.User || res.To() == nil || res.To().Address.User != r.To().Address.User || res.Source() != r.Destination() {
				continue
			}
			return res, nil
		}
	}
}

func Header(m gosip.Message, name string) string {
	if headers := m.GetHeaders(name); len(headers) > 0 {
		return headers[0].Value()
	}
	return ""
}
func Reply(r *gosip.Request, tx gosip.ServerTransaction, code int, headers map[string]string) error {
	reasons := map[int]string{200: "OK", 400: "Bad Request", 401: "Unauthorized", 403: "Forbidden", 404: "Not Found", 405: "Method Not Allowed", 415: "Unsupported Media Type", 423: "Interval Too Brief", 481: "Call/Transaction Does Not Exist", 500: "Server Internal Error", 503: "Service Unavailable", 501: "Not Implemented"}
	res := gosip.NewResponseFromRequest(r, code, reasons[code], nil)
	if r.Method == gosip.REGISTER {
		res.AppendHeader(gosip.NewHeader("X-GB-Ver", "3.0"))
	}
	if code == 405 {
		res.AppendHeader(gosip.NewHeader("Allow", "REGISTER, MESSAGE"))
	}
	for k, v := range headers {
		res.AppendHeader(gosip.NewHeader(k, v))
	}
	return tx.Respond(res)
}

type readyPacket struct {
	net.PacketConn
	once  sync.Once
	ready chan struct{}
}

func (c *readyPacket) ReadFrom(b []byte) (int, net.Addr, error) {
	c.once.Do(func() { close(c.ready) })
	return c.PacketConn.ReadFrom(b)
}

type readyListener struct {
	net.Listener
	once  sync.Once
	ready chan struct{}
}

func (l *readyListener) Accept() (net.Conn, error) {
	l.once.Do(func() { close(l.ready) })
	return l.Listener.Accept()
}

type limitedListener struct {
	net.Listener
	endpoint *Endpoint
}

func (l *limitedListener) Accept() (net.Conn, error) {
	for {
		c, e := l.Listener.Accept()
		if e != nil {
			return nil, e
		}
		if l.endpoint.connections.Add(1) > 8 {
			l.endpoint.connections.Add(-1)
			c.Close()
			continue
		}
		return &countedConn{Conn: c, endpoint: l.endpoint}, nil
	}
}

type countedConn struct {
	net.Conn
	endpoint *Endpoint
	once     sync.Once
}

func (c *countedConn) Close() error {
	var e error
	c.once.Do(func() { e = c.Conn.Close(); c.endpoint.connections.Add(-1) })
	return e
}
