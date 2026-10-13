package device

import (
	"testing"
	"time"
)

func TestVirtualClockRegistrationHeartbeatAndLateResults(t *testing.T) {
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	s := Initial()
	g := s.Start()
	if !s.Registered(g, now, 86400*time.Second, 60*time.Second, "3.0") {
		t.Fatal("register")
	}
	if s.Due(now.Add(59*time.Second), true, time.Minute) != "" || s.Due(now.Add(time.Minute), true, time.Minute) != "keepalive" {
		t.Fatal("heartbeat schedule")
	}
	for i := 1; i <= 3; i++ {
		s.Beat(g, now.Add(time.Duration(i)*time.Minute), false, "timeout", time.Minute, time.Minute)
	}
	if s.Registration != "offline" || s.Due(now.Add(239*time.Second), true, time.Minute) != "" || s.Due(now.Add(240*time.Second), true, time.Minute) != "register" {
		t.Fatal("misses/retry")
	}
	s.Registered(g, now, 86400*time.Second, time.Minute, "2.0")
	if s.VersionCheck != "failed" {
		t.Fatal("version mismatch hidden")
	}
	if s.Due(now.Add(86400*time.Second*4/5), true, time.Minute) != "register" {
		t.Fatal("refresh")
	}
	s.Stop()
	if s.Registered(g, now, time.Hour, time.Minute, "3.0") || s.Due(now.Add(90000*time.Second), true, time.Minute) != "" {
		t.Fatal("old generation revived")
	}
}
func TestCatalogIdentityAndTimeFilter(t *testing.T) {
	d := Default()
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	d.Stamp(Definition{}, now)
	if e := d.Validate(); e != nil {
		t.Fatal(e)
	}
	items, e := d.Catalog("34020000002000000001", d.ID, "", "", now)
	if e != nil || len(items) != 2 {
		t.Fatal("J(c) self+child", e)
	}
	d.Channels = nil
	items, _ = d.Catalog("34020000002000000001", d.ID, "", "", now)
	if len(items) != 1 {
		t.Fatal("zero channels still has self")
	}
	items, e = d.Catalog("34020000002000000001", d.ID, "2027-01-01T00:00:00Z", "2027-01-02T00:00:00Z", now)
	if e != nil || len(items) != 0 {
		t.Fatal("empty filtered result", e)
	}
}
