package app

import (
	"context"
	"encoding/json"
	"github.com/chialecaleb/gb28181-lab/internal/config"
	"github.com/chialecaleb/gb28181-lab/internal/gb2022"
	wire "github.com/chialecaleb/gb28181-lab/internal/sip"
	gosip "github.com/emiago/sipgo/sip"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testService(t *testing.T, transport string) *Service {
	t.Helper()
	s, e := Open(Options{Runtime: config.Runtime{DataDir: t.TempDir(), DeviceBind: "127.0.0.1:0", PlatformBind: "127.0.0.1:0", Transport: transport}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := s.Close(); e != nil {
			t.Error(e)
		}
	})
	return s
}

func TestBusinessTimeoutCancelAndIdempotency(t *testing.T) {
	s := testService(t, "UDP")
	action(t, s, "register")
	addr := s.deviceEP.Addr
	s.deviceEP.Close()
	ep, e := wire.NewEndpoint(addr, "device", s.trace.Observe, nil, func(r *gosip.Request, tx gosip.ServerTransaction) { wire.Reply(r, tx, 200, nil) })
	if e != nil {
		t.Fatal(e)
	}
	s.mu.Lock()
	s.deviceEP = ep
	s.mu.Unlock()
	snap, _ := s.Snapshot(s.token)
	a := Action{Kind: "deviceinfo", ExpectedRevision: snap.Config.Revision, IdempotencyKey: "same-input", TimeoutSeconds: 1}
	op, e := s.Execute(s.token, a)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.Execute(s.token, a)
	if e != nil || op.ID != again.ID {
		t.Fatal("idempotency failed")
	}
	a.Target = "34020000001310000001"
	if _, e = s.Execute(s.token, a); e == nil {
		t.Fatal("conflicting idempotency accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	op, e = s.Wait(ctx, s.token, op.ID)
	if e != nil || op.State != "failed" || op.SIPStatus != 200 || op.Business == "passed" || op.Error.Code != "timeout" {
		t.Fatalf("SIP 200 mistaken for business success: %+v %v", op, e)
	}
	op, e = s.Execute(s.token, Action{Kind: "catalog", ExpectedRevision: snap.Config.Revision, IdempotencyKey: "cancel", TimeoutSeconds: 4})
	if e != nil {
		t.Fatal(e)
	}
	s.Cancel(s.token, op.ID)
	op, e = s.Wait(ctx, s.token, op.ID)
	if e != nil || op.State != "cancelled" {
		t.Fatal("cancel", op, e)
	}
	snap, _ = s.Snapshot(s.token)
	if snap.ActiveOperations != 0 || snap.Resources.PendingQueries != 0 || snap.Resources.ClientTransactions != 0 {
		t.Fatal("query resources leaked", snap.Resources)
	}
	action(t, s, "unregister")
}
func TestRestartConfigurationAndSecretIsolation(t *testing.T) {
	runtime := config.Runtime{DataDir: t.TempDir(), DeviceBind: "127.0.0.1:0", PlatformBind: "127.0.0.1:0", Transport: "UDP"}
	s, e := Open(Options{Runtime: runtime})
	if e != nil {
		t.Fatal(e)
	}
	snap, _ := s.Snapshot(s.token)
	c := snap.Config
	c.Device.Name = "设备中文𠀀"
	c.CatalogBatchSize = 1
	saved, e := s.SaveConfig(s.token, c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveConfig(s.token, c); e == nil {
		t.Fatal("stale revision accepted")
	}
	action(t, s, "register")
	cat := action(t, s, "catalog")
	if len(cat.Result.([]gb2022.Item)) != 2 {
		t.Fatal(cat)
	}
	if _, e = s.SaveConfig(s.token, saved); e == nil {
		t.Fatal("registered configuration mutated")
	}
	exported, _ := s.Evidence(s.token)
	b, _ := json.Marshal(exported)
	for _, secret := range []string{s.token, s.password} {
		if strings.Contains(string(b), secret) {
			t.Fatal("secret exposed")
		}
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = Open(Options{Runtime: runtime})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	snap, _ = s.Snapshot(s.token)
	if snap.Config.Device.Name != "设备中文𠀀" || snap.Config.Revision != saved.Revision || snap.Device.Running || snap.Platform.Registered {
		t.Fatal("restart restored in-flight state or lost configuration")
	}
}
func TestRegistrationRejectionVersionAndCancellation(t *testing.T) {
	for _, code := range []int{403, 302, 200} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			peer, e := wire.NewEndpoint("127.0.0.1:0", "fault", nil, func(r *gosip.Request, tx gosip.ServerTransaction) {
				res := gosip.NewResponseFromRequest(r, code, "Fixture", nil)
				res.AppendHeader(gosip.NewHeader("Expires", "86400"))
				tx.Respond(res)
			}, nil)
			if e != nil {
				t.Fatal(e)
			}
			defer peer.Close()
			s, e := Open(Options{Runtime: config.Runtime{DataDir: t.TempDir(), DeviceBind: "127.0.0.1:0", PlatformBind: "127.0.0.1:0", Transport: "UDP"}, External: &Binding{PlatformID: "34020000002000000001", Domain: "3402000000", Target: peer.Addr, Username: "synthetic", Password: "synthetic-only", AuthMode: "digest"}})
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			snap, _ := s.Snapshot(s.token)
			snap.Config.BindingID = "external"
			c, e := s.SaveConfig(s.token, snap.Config)
			if e != nil {
				t.Fatal(e)
			}
			op, e := s.Execute(s.token, Action{Kind: "register", ExpectedRevision: c.Revision, IdempotencyKey: "fault", TimeoutSeconds: 1})
			if e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			op, e = s.Wait(ctx, s.token, op.ID)
			if e != nil {
				t.Fatal(e)
			}
			snap, _ = s.Snapshot(s.token)
			if code == 200 {
				if op.State != "completed" || snap.Device.VersionCheck != "failed" {
					t.Fatal("missing version hidden", op)
				}
			} else if op.State != "failed" || snap.Device.Registration == "registered" {
				t.Fatal("rejection accepted", op)
			}
		})
	}
	s := testService(t, "UDP")
	snap, _ := s.Snapshot(s.token)
	op, e := s.Execute(s.token, Action{Kind: "register", ExpectedRevision: snap.Config.Revision, IdempotencyKey: "cancel-register", TimeoutSeconds: 4})
	if e != nil {
		t.Fatal(e)
	}
	s.Cancel(s.token, op.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	op, e = s.Wait(ctx, s.token, op.ID)
	if e != nil {
		t.Fatal(e)
	}
	snap, _ = s.Snapshot(s.token)
	if snap.Platform.Registered || snap.Device.Running || snap.ActiveOperations != 0 {
		t.Fatal("cancelled registration left a live binding", op)
	}
}

func TestBuiltinRegistrarRejectsWrongRoutingAndAoR(t *testing.T) {
	s := testService(t, "UDP")
	peer, err := wire.NewEndpoint("127.0.0.1:0", "fixture", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	for _, mutate := range []func(*gosip.Request){
		func(r *gosip.Request) { r.Recipient.User = "34020000002000000001" },
		func(r *gosip.Request) { r.Recipient.Host = "192.0.2.1" },
		func(r *gosip.Request) { r.Recipient.Port++ },
		func(r *gosip.Request) { r.From().Address.Host = "other.example" },
		func(r *gosip.Request) { r.To().Address.Host = "other.example" },
	} {
		req, err := peer.Build(wire.RequestSpec{Method: "REGISTER", From: s.config.Device.ID, To: "34020000002000000001", Domain: "3402000000", Target: s.platformEP.Addr, Transport: "UDP", CSeq: 1})
		if err != nil {
			t.Fatal(err)
		}
		mutate(req)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		res, err := peer.Send(ctx, req)
		cancel()
		if err != nil || res.StatusCode != 403 {
			t.Fatal("registrar accepted an unrelated routing target or address of record", err)
		}
	}
	if s.registrar.Snapshot().Registered {
		t.Fatal("rejected request created a registration")
	}
}
func action(t *testing.T, s *Service, kind string) Operation {
	t.Helper()
	snap, e := s.Snapshot(s.LocalToken())
	if e != nil {
		t.Fatal(e)
	}
	op, e := s.Execute(s.LocalToken(), Action{Kind: kind, ExpectedRevision: snap.Config.Revision, IdempotencyKey: kind + time.Now().String(), TimeoutSeconds: 4})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	op, e = s.Wait(ctx, s.LocalToken(), op.ID)
	if e != nil {
		t.Fatal(e)
	}
	if op.State != "completed" {
		t.Fatalf("%s: %+v", kind, op)
	}
	return op
}
func TestRealLoopbackSignaling(t *testing.T) {
	for _, transport := range []string{"UDP", "TCP"} {
		t.Run(transport, func(t *testing.T) {
			s := testService(t, transport)
			action(t, s, "register")
			action(t, s, "keepalive")
			info := action(t, s, "deviceinfo")
			if info.Business != "passed" {
				t.Fatal(info)
			}
			cat := action(t, s, "catalog")
			items, ok := cat.Result.([]gb2022.Item)
			if !ok || len(items) != 2 {
				t.Fatalf("catalog result: %#v", cat.Result)
			}
			action(t, s, "register") // refresh with same Call-ID and increasing CSeq
			action(t, s, "unregister")
			snap, _ := s.Snapshot(s.LocalToken())
			if snap.Device.Running || snap.Platform.Registered || snap.ActiveOperations != 0 || snap.Resources.PendingQueries != 0 || snap.Resources.ClientTransactions != 0 {
				t.Fatalf("cleanup: %+v", snap)
			}
			da, pa := s.Addresses()
			if e := s.Close(); e != nil {
				t.Fatal(e)
			}
			for _, addr := range []string{da, pa} {
				u, e := net.ListenPacket("udp4", addr)
				if e != nil {
					t.Fatal(e)
				}
				u.Close()
				l, e := net.Listen("tcp4", addr)
				if e != nil {
					t.Fatal(e)
				}
				l.Close()
			}
		})
	}
}

func TestHeartbeatResponseTimeoutsAndRecovery(t *testing.T) {
	s := testService(t, "UDP")
	action(t, s, "register")
	addr := s.platformEP.Addr
	s.platformEP.Close()
	peer, e := wire.NewEndpoint(addr, "platform", s.trace.Observe, s.onRegister, func(*gosip.Request, gosip.ServerTransaction) {})
	if e != nil {
		t.Fatal(e)
	}
	s.mu.Lock()
	s.platformEP = peer
	s.mu.Unlock()
	for i := 0; i < 3; i++ {
		snap, _ := s.Snapshot(s.token)
		op, e := s.Execute(s.token, Action{Kind: "keepalive", ExpectedRevision: snap.Config.Revision, IdempotencyKey: strconv.Itoa(i), TimeoutSeconds: 1})
		if e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		op, e = s.Wait(ctx, s.token, op.ID)
		cancel()
		if e != nil || op.State != "failed" {
			t.Fatal("heartbeat unexpectedly succeeded", e)
		}
	}
	snap, _ := s.Snapshot(s.token)
	if snap.Device.HeartbeatMisses != 3 || snap.Device.Registration != "offline" {
		t.Fatal("response timeouts did not drive offline", snap.Device)
	}
	action(t, s, "register")
	snap, _ = s.Snapshot(s.token)
	if snap.Device.Registration != "registered" || snap.Device.HeartbeatMisses != 0 {
		t.Fatal("re-registration did not recover")
	}
	action(t, s, "unregister")
}

func TestTCPDisconnectAndNewRegistration(t *testing.T) {
	s := testService(t, "TCP")
	action(t, s, "register")
	s.DropDeviceTCP()
	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) {
		snap, _ := s.Snapshot(s.token)
		if snap.Device.Registration == "offline" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	snap, _ := s.Snapshot(s.token)
	if snap.Device.Registration != "offline" {
		t.Fatal("closed TCP did not go offline")
	}
	action(t, s, "register")
	action(t, s, "deviceinfo")
	action(t, s, "unregister")
}
