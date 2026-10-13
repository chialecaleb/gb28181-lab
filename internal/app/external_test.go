package app

import (
	"context"
	"encoding/json"
	"github.com/chialecaleb/gb28181-lab/internal/config"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Explicit opt-in only. The private input is created after independent gates and
// read-only parameter discovery. CI never enables this test.
func TestControlledExternalInterop(t *testing.T) {
	path := os.Getenv("LAB_M0_EXTERNAL_TEST")
	if path == "" {
		t.Skip("controlled P6 test requires explicit private input")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal("private test input unavailable")
	}
	var input struct {
		Binding                                                   Binding
		DeviceID, ChannelID, Transport, EvidenceDir, ProtocolMode string
	}
	if json.Unmarshal(b, &input) != nil {
		t.Fatal("invalid private test input")
	}
	if input.Transport != "UDP" && input.Transport != "TCP" {
		t.Fatal("transport required")
	}
	s, e := Open(Options{Runtime: config.Runtime{DataDir: t.TempDir(), DeviceBind: "127.0.0.1:0", PlatformBind: "127.0.0.1:0", Transport: input.Transport}, External: &input.Binding})
	if e != nil {
		t.Fatal("external test engine failed to start")
	}
	defer func() {
		if e := s.Close(); e != nil {
			t.Error("test engine close failed")
		}
		final, _ := s.Snapshot(s.token)
		if final.ActiveOperations != 0 || final.Resources != (Resources{}) {
			t.Errorf("test resources remain: operations=%d resources=%+v", final.ActiveOperations, final.Resources)
		}
		for _, addr := range []string{s.deviceEP.Addr, s.platformEP.Addr} {
			udp, err := net.ListenPacket("udp4", addr)
			if err != nil {
				t.Error("test UDP port was not released")
			} else {
				udp.Close()
			}
			tcp, err := net.Listen("tcp4", addr)
			if err != nil {
				t.Error("test TCP port was not released")
			} else {
				tcp.Close()
			}
		}
		exported, err := s.Evidence(s.token)
		if err != nil {
			t.Error("test evidence unavailable")
		}
		data, err := json.MarshalIndent(exported, "", "  ")
		if err != nil || os.WriteFile(filepath.Join(input.EvidenceDir, "evidence.json"), data, 0600) != nil {
			t.Error("test evidence could not be saved")
		}
		for i, raw := range s.TraceLocal().Raw() {
			if os.WriteFile(filepath.Join(input.EvidenceDir, time.Unix(int64(i), 0).UTC().Format("150405")+".sip"), raw, 0600) != nil {
				t.Error("original test bytes could not be saved")
			}
		}
	}()
	snap, _ := s.Snapshot(s.token)
	c := snap.Config
	c.BindingID = "external"
	c.ProtocolMode = input.ProtocolMode
	if c.ProtocolMode == "" {
		c.ProtocolMode = ModeStandard
	}
	c.Device.ID = input.DeviceID
	c.Device.Channels[0].ID = input.ChannelID
	c.Device.Name = "Lab M0 isolated test"
	c.Device.Channels[0].Name = "Lab M0 test channel"
	c, e = s.SaveConfig(s.token, c)
	if e != nil {
		t.Fatal("test profile rejected")
	}
	run := func(kind string) Operation {
		op, e := s.Execute(s.token, Action{Kind: kind, ExpectedRevision: c.Revision, IdempotencyKey: kind + time.Now().String(), TimeoutSeconds: 8})
		if e != nil {
			t.Fatal("test action rejected")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 13*time.Second)
		defer cancel()
		op, e = s.Wait(ctx, s.token, op.ID)
		if e != nil {
			t.Fatal("test action wait failed")
		}
		return op
	}
	op := run("register")
	t.Logf("registration state=%s SIP=%d", op.State, op.SIPStatus)
	if op.State != "completed" {
		if op.Error != nil {
			t.Logf("registration error code=%s", op.Error.Code)
		}
		t.FailNow()
	}
	defer func() {
		op := run("unregister")
		t.Logf("cleanup state=%s SIP=%d", op.State, op.SIPStatus)
		if op.State != "completed" {
			t.Error("deregistration unconfirmed")
		}
	}()
	op = run("register")
	t.Logf("refresh state=%s SIP=%d", op.State, op.SIPStatus)
	if op.State != "completed" {
		t.Fatal("registration refresh failed")
	}
	if os.WriteFile(filepath.Join(input.EvidenceDir, "ready"), []byte("registered"), 0600) != nil {
		t.Fatal("test readiness could not be saved")
	}
	// Real wall-clock heartbeat schedule, not accelerated by the test.
	deadline := time.Now().Add(190 * time.Second)
	lastAcknowledged := 0
	for time.Now().Before(deadline) {
		time.Sleep(time.Second)
		if _, err := os.Stat(filepath.Join(input.EvidenceDir, "stop")); err == nil {
			t.Error("test cancelled; performing protocol cleanup")
			break
		}
		snap, _ = s.Snapshot(s.token)
		if snap.Device.HeartbeatAcknowledged != lastAcknowledged {
			lastAcknowledged = snap.Device.HeartbeatAcknowledged
			t.Logf("normal heartbeat acknowledged=%d", lastAcknowledged)
		}
		if snap.Device.HeartbeatAcknowledged >= 3 {
			break
		}
	}
	snap, _ = s.Snapshot(s.token)
	t.Logf("heartbeat sent=%d acknowledged=%d version=%s", snap.Device.HeartbeatSent, snap.Device.HeartbeatAcknowledged, snap.Device.VersionCheck)
	if snap.Device.HeartbeatAcknowledged < 3 {
		t.Error("three normal heartbeat cycles were not confirmed")
	}
}
