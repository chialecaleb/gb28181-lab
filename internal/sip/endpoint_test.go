package sip

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	gosip "github.com/emiago/sipgo/sip"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTCPFinalResponseBeforePeerClose(t *testing.T) {
	// An independent TCP peer writes a complete final response, then closes
	// immediately. The received response must win over transport EOF.
	for i := 0; i < 48; i++ {
		t.Run(fmt.Sprintf("exchange-%d", i), func(t *testing.T) {
			ep, err := NewEndpoint("127.0.0.1:0", "final-close", nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ep.Close()
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			code := []int{200, 401, 403}[i%3]
			finished := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					finished <- err
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(2 * time.Second))
				reader := bufio.NewReader(conn)
				var request strings.Builder
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						finished <- err
						return
					}
					request.WriteString(line)
					if line == "\r\n" {
						break
					}
				}
				_, err = conn.Write([]byte(responseFixture(request.String(), code)))
				finished <- err
			}()
			req, err := ep.Build(RequestSpec{Method: "REGISTER", From: "34020000001180000001", To: "34020000002000000001", Domain: "3402000000", Target: listener.Addr().String(), Transport: "TCP", CSeq: uint32(i + 1)})
			if err != nil {
				listener.Close()
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			response, err := ep.Send(ctx, req)
			cancel()
			listener.Close()
			peerErr := <-finished
			ep.DropTCP(req.Destination())
			if peerErr != nil {
				t.Fatal(peerErr)
			}
			if err != nil || response.StatusCode != code {
				t.Fatalf("received final response %d was lost on peer close (iteration %d): %v", code, i, err)
			}
		})
	}
}

func TestTCPPeerCloseWithoutResponseStillHonorsDeadline(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
	}()
	ep, err := NewEndpoint("127.0.0.1:0", "empty-close", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ep.Close()
	req, err := ep.Build(RequestSpec{Method: "MESSAGE", From: "34020000001180000001", To: "34020000002000000001", Domain: "3402000000", Target: listener.Addr().String(), Transport: "TCP", CSeq: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	started := time.Now()
	if response, err := ep.Send(ctx, req); response != nil || err == nil || time.Since(started) > time.Second {
		t.Fatalf("close without a response must remain bounded by the caller deadline: %v", err)
	}
	<-finished
	if clients, _, _ := ep.Stats(); clients != 0 {
		t.Fatal("closed connection retained an active client transaction")
	}
}

func TestUDPWireRetransmissionAndServerDedup(t *testing.T) {
	peer, e := net.ListenPacket("udp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	ep, e := NewEndpoint("127.0.0.1:0", "test", nil, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer ep.Close()
	result := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		peer.SetReadDeadline(time.Now().Add(3 * time.Second))
		n, _, e := peer.ReadFrom(buf)
		if e != nil {
			result <- e
			return
		}
		first := bytes.Clone(buf[:n])
		n, addr, e := peer.ReadFrom(buf)
		if e != nil {
			result <- e
			return
		}
		if !bytes.Equal(first, buf[:n]) {
			result <- fmt.Errorf("UDP retry changed bytes")
			return
		}
		res := responseFixture(string(first), 200)
		// Wrong/out-of-order final responses must not complete this transaction.
		peer.WriteTo([]byte(strings.Replace(res, "CSeq: 1 MESSAGE", "CSeq: 99 MESSAGE", 1)), addr)
		_, e = peer.WriteTo([]byte(res), addr)
		result <- e
	}()
	req, e := ep.Build(RequestSpec{Method: "MESSAGE", From: "34020000001180000001", To: "34020000002000000001", Domain: "3402000000", Target: peer.LocalAddr().String(), Transport: "UDP", CSeq: 1})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	res, e := ep.Send(ctx, req)
	if e != nil || res.StatusCode != 200 {
		t.Fatal("retry failed", e)
	}
	if e = <-result; e != nil {
		t.Fatal(e)
	}
	var calls atomic.Int32
	server, e := NewEndpoint("127.0.0.1:0", "dedup", nil, nil, func(r *gosip.Request, tx gosip.ServerTransaction) { calls.Add(1); Reply(r, tx, 200, nil) })
	if e != nil {
		t.Fatal(e)
	}
	defer server.Close()
	addr, _ := net.ResolveUDPAddr("udp4", server.Addr)
	req.SetDestination(server.Addr)
	for i := 0; i < 2; i++ {
		peer.WriteTo([]byte(req.String()), addr)
		buf := make([]byte, 4096)
		peer.SetReadDeadline(time.Now().Add(time.Second))
		if _, _, e = peer.ReadFrom(buf); e != nil {
			t.Fatal(e)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("retransmission invoked business handler %d times", calls.Load())
	}
}

// Response fixture deliberately does not call sipgo's response builder.
func responseFixture(request string, code int) string {
	var out strings.Builder
	fmt.Fprintf(&out, "SIP/2.0 %d Test\r\n", code)
	for _, line := range strings.Split(request, "\r\n") {
		for _, name := range []string{"Via:", "From:", "To:", "Call-ID:", "CSeq:"} {
			if strings.HasPrefix(line, name) {
				if name == "To:" {
					line += ";tag=independent"
				}
				out.WriteString(line + "\r\n")
			}
		}
	}
	out.WriteString("Content-Length: 0\r\n\r\n")
	return out.String()
}

func TestRequestRoutingIsSeparateFromGBDomain(t *testing.T) {
	ep := &Endpoint{Addr: "127.0.0.1:15061"}
	for _, method := range []string{"REGISTER", "MESSAGE"} {
		req, err := ep.Build(RequestSpec{Method: method, From: "34020000001180000001", To: "34020000002000000001", Domain: "3402000000", Target: "127.0.0.1:15062", Transport: "UDP", CSeq: 1})
		if err != nil {
			t.Fatal(err)
		}
		if req.Recipient.Host != "127.0.0.1" || req.Recipient.Port != 15062 {
			t.Fatal("routing URI incorrectly used a GB identity domain as an RFC host")
		}
		if req.From().Address.Host != "3402000000" || req.To().Address.Host != "3402000000" {
			t.Fatal("routing address replaced the GB address-of-record domain")
		}
		if method == "REGISTER" && (req.Recipient.User != "" || req.To().Address.User != req.From().Address.User) {
			t.Fatal("standard REGISTER must omit userinfo and register the device AoR")
		}
		if method == "MESSAGE" && req.Recipient.User != "34020000002000000001" {
			t.Fatal("MESSAGE lost the recipient identity")
		}
	}
}

func TestRequestRoutingTargetValidation(t *testing.T) {
	ep := &Endpoint{Addr: "127.0.0.1:15061"}
	for _, target := range []string{"127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:invalid", "user@127.0.0.1:15062", "127.0.0.1;transport=tcp:15062", "127.0.0.1\r\n:15062"} {
		if _, err := ep.Build(RequestSpec{Method: "REGISTER", From: "34020000001180000001", To: "34020000002000000001", Domain: "3402000000", Target: target, Transport: "UDP", CSeq: 1}); err == nil {
			t.Fatal("invalid routing target accepted")
		}
	}
}

func TestExplicitRegisterCompatibilityOnlyChangesRequestURI(t *testing.T) {
	ep := &Endpoint{Addr: "127.0.0.1:15061"}
	spec := RequestSpec{Method: "REGISTER", From: "34020000001180000001", To: "34020000002000000001", Domain: "3402000000", Target: "127.0.0.1:15062", Transport: "UDP", CSeq: 1, CallID: "fixture", Tag: "fixture", Branch: "z9hG4bK-fixture"}
	standard, err := ep.Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.RegisterUserinfo = true
	compatibility, err := ep.Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	if compatibility.Recipient.String() != "sip:34020000002000000001@127.0.0.1:15062" {
		t.Fatal("legacy URI does not follow the reviewed 2016 form")
	}
	if strings.Replace(compatibility.String(), compatibility.Recipient.String(), standard.Recipient.String(), 1) != standard.String() {
		t.Fatal("compatibility unexpectedly changed other SIP fields")
	}
	spec.Method = "MESSAGE"
	if _, err := ep.Build(spec); err == nil {
		t.Fatal("REGISTER compatibility was accepted for another method")
	}
}
func TestRealTCPSplitCoalescedMalformedAndReconnect(t *testing.T) {
	var calls atomic.Int32
	ep, e := NewEndpoint("127.0.0.1:0", "tcp", nil, nil, func(r *gosip.Request, tx gosip.ServerTransaction) { calls.Add(1); Reply(r, tx, 200, nil) })
	if e != nil {
		t.Fatal(e)
	}
	defer ep.Close()
	c, e := net.Dial("tcp4", ep.Addr)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	raw := strings.Replace(string(frameFixture()), "Content-Length: 4", "Max-Forwards: 70\r\nContent-Length: 4", 1)
	second := strings.ReplaceAll(strings.Replace(raw, "CSeq: 1", "CSeq: 2", 1), "z9hG4bK-fixture", "z9hG4bK-second")
	c.Write([]byte(raw[:37]))
	time.Sleep(10 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatal("partial dispatched")
	}
	c.Write([]byte(raw[37:] + second))
	deadline := time.Now().Add(time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if calls.Load() != 2 {
		t.Fatal("TCP coalescing dropped message")
	}
	c.Close()
	for _, bad := range []string{strings.Replace(raw, "Content-Length: 4", "Content-Length: 99999999", 1), strings.Replace(raw, "Content-Length: 4", "Content-Length: 4\r\nl: 4", 1), raw[:len(raw)-2]} {
		conn, e := net.Dial("tcp4", ep.Addr)
		if e != nil {
			t.Fatal(e)
		}
		conn.Write([]byte(bad))
		if len(bad) > len(raw) {
			conn.SetReadDeadline(time.Now().Add(time.Second))
			buf := make([]byte, 32)
			if _, e = conn.Read(buf); e == nil {
				t.Fatal("invalid frame connection kept usable")
			}
		}
		conn.Close()
	}
	if calls.Load() != 2 {
		t.Fatal("invalid frame dispatched")
	}
}
