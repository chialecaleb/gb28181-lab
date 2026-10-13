package platform

import (
	"github.com/chialecaleb/gb28181-lab/internal/gb2022"
	"testing"
	"time"
)

func TestRegistrationExpiryHeartbeatAndRecovery(t *testing.T) {
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	r := New("3402000000", "34020000001180000001", "synthetic", time.Minute, func() time.Time { return now })
	r.state = Registration{Registered: true, Online: true, registeredAt: now, ExpiresAt: now.Add(time.Hour), Source: "127.0.0.1:12345", Transport: "UDP"}
	now = now.Add(179 * time.Second)
	r.Tick(true)
	if !r.Snapshot().Online {
		t.Fatal("premature heartbeat expiry")
	}
	now = now.Add(time.Second)
	r.Tick(true)
	if r.Snapshot().Online {
		t.Fatal("missed periods did not go offline")
	}
	if !r.Heartbeat(gb2022.Message{DeviceID: r.device, Status: "OK"}) || !r.Snapshot().Online {
		t.Fatal("heartbeat recovery")
	}
	if r.Accept("127.0.0.1:12346", "UDP", r.device) || r.Accept(r.state.Source, "UDP", "34020000001180000002") {
		t.Fatal("unbound source accepted")
	}
	now = now.Add(time.Hour)
	r.Tick(true)
	if r.Snapshot().Registered || r.Heartbeat(gb2022.Message{DeviceID: r.device, Status: "OK"}) {
		t.Fatal("expired registration revived")
	}
}
