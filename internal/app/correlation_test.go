package app

import (
	"context"
	"github.com/chialecaleb/gb28181-lab/internal/gb2022"
	wire "github.com/chialecaleb/gb28181-lab/internal/sip"
	gosip "github.com/emiago/sipgo/sip"
	"testing"
	"time"
)

func TestBusinessCorrelationRejectsWrongSNTargetAndSource(t *testing.T) {
	s := testService(t, "UDP")
	action(t, s, "register")
	addr := s.deviceEP.Addr
	s.deviceEP.Close()
	result := make(chan []int, 1)
	handler := func(r *gosip.Request, tx gosip.ServerTransaction) {
		m, _ := gb2022.ParseMessage(r.Body())
		wire.Reply(r, tx, 200, nil)
		go func() {
			codes := []int{}
			for i := 0; i < 3; i++ {
				reply := gb2022.Message{Root: "Response", CmdType: "DeviceInfo", DeviceID: m.DeviceID, SN: m.SN, Result: "OK"}
				if i == 0 {
					reply.SN = "999999"
				}
				if i == 1 {
					reply.DeviceID = "34020000001310000001"
				}
				body, _ := gb2022.XML(reply)
				req, e := s.nextMessage("device", "", body, "34020000002000000001", s.platformEP.Addr, "3402000000", "UDP")
				if e != nil {
					result <- nil
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				res, e := s.deviceEP.Send(ctx, req)
				cancel()
				if e != nil {
					result <- nil
					return
				}
				codes = append(codes, res.StatusCode)
			}
			result <- codes
		}()
	}
	ep, e := wire.NewEndpoint(addr, "device", s.trace.Observe, nil, handler)
	if e != nil {
		t.Fatal(e)
	}
	s.mu.Lock()
	s.deviceEP = ep
	s.mu.Unlock()
	action(t, s, "deviceinfo")
	select {
	case codes := <-result:
		if len(codes) != 3 || codes[0] != 481 || codes[1] != 481 || codes[2] != 200 {
			t.Fatal("incorrect business correlation", codes)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("response test timed out")
	}
	stranger, e := wire.NewEndpoint("127.0.0.1:0", "stranger", nil, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer stranger.Close()
	body, _ := gb2022.XML(gb2022.Message{Root: "Notify", CmdType: "Keepalive", DeviceID: s.config.Device.ID, SN: "1", Status: "OK"})
	req, _ := stranger.Build(wire.RequestSpec{Method: "MESSAGE", From: s.config.Device.ID, To: "34020000002000000001", Domain: "3402000000", Target: s.platformEP.Addr, Transport: "UDP", CSeq: 1, Body: body})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, e := stranger.Send(ctx, req)
	if e != nil || res.StatusCode != 403 {
		t.Fatal("unbound source accepted", e)
	}
	action(t, s, "unregister")
}
