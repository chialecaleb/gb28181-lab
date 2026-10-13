package device

import "time"

type State struct {
	Generation            uint64    `json:"generation"`
	Running               bool      `json:"running"`
	Registration          string    `json:"registration"`
	Reason                string    `json:"reason,omitempty"`
	PeerVersion           string    `json:"peerVersion"`
	VersionCheck          string    `json:"versionCheck"`
	Heartbeat             string    `json:"heartbeat"`
	HeartbeatMisses       int       `json:"heartbeatMisses"`
	HeartbeatSent         int       `json:"heartbeatSent"`
	HeartbeatAcknowledged int       `json:"heartbeatAcknowledged"`
	LastHeartbeat         time.Time `json:"lastHeartbeat"`
	ExpiresAt             time.Time `json:"expiresAt"`
	RefreshAt             time.Time `json:"refreshAt"`
	NextHeartbeat         time.Time `json:"nextHeartbeat"`
	RetryAt               time.Time `json:"retryAt"`
}

func Initial() State {
	return State{Registration: "unregistered", Heartbeat: "unknown", PeerVersion: "unknown", VersionCheck: "unknown"}
}
func (s *State) Start() uint64 {
	s.Generation++
	s.Running = true
	s.Registration = "registering"
	s.Reason = ""
	s.Heartbeat = "unknown"
	return s.Generation
}
func (s *State) Stop() {
	s.Generation++
	s.Running = false
	s.Registration = "unregistered"
	s.Heartbeat = "unknown"
	s.ExpiresAt = time.Time{}
	s.RefreshAt = time.Time{}
	s.NextHeartbeat = time.Time{}
	s.RetryAt = time.Time{}
}
func (s *State) Registered(gen uint64, now time.Time, expires, heartbeat time.Duration, peer string) bool {
	if gen != s.Generation || !s.Running {
		return false
	}
	s.Registration = "registered"
	s.Reason = ""
	s.PeerVersion = peer
	s.VersionCheck = "passed"
	if peer != "3.0" {
		s.VersionCheck = "failed"
		if peer == "" {
			s.PeerVersion = "missing"
		}
	}
	s.ExpiresAt = now.Add(expires)
	s.RefreshAt = now.Add(expires * 4 / 5)
	s.NextHeartbeat = now.Add(heartbeat)
	s.HeartbeatMisses = 0
	s.Heartbeat = "waiting"
	s.RetryAt = time.Time{}
	return true
}
func (s *State) Failed(gen uint64, now time.Time, reason string, retry time.Duration) bool {
	if gen != s.Generation || !s.Running {
		return false
	}
	s.Registration = "offline"
	s.Reason = reason
	s.Heartbeat = "unknown"
	s.RetryAt = now.Add(retry)
	return true
}
func (s *State) Beat(gen uint64, now time.Time, ok bool, reason string, interval, retry time.Duration) {
	if gen != s.Generation || !s.Running || s.Registration != "registered" {
		return
	}
	s.HeartbeatSent++
	s.NextHeartbeat = now.Add(interval)
	if ok {
		s.HeartbeatAcknowledged++
		s.HeartbeatMisses = 0
		s.Heartbeat = "acknowledged"
		s.LastHeartbeat = now
	} else {
		s.HeartbeatMisses++
		s.Heartbeat = "missed"
		s.Reason = reason
		if s.HeartbeatMisses >= 3 {
			s.Failed(gen, now, "three heartbeat responses missed", retry)
			s.Heartbeat = "offline"
		}
	}
}
func (s *State) Due(now time.Time, tcpAlive bool, retry time.Duration) string {
	if !s.Running {
		return ""
	}
	if s.Registration == "registered" {
		if !tcpAlive {
			s.Failed(s.Generation, now, "TCP connection closed", retry)
			return ""
		}
		if !now.Before(s.ExpiresAt) {
			s.Failed(s.Generation, now, "registration expired", retry)
			return ""
		}
		if !now.Before(s.RefreshAt) {
			return "register"
		}
		if !now.Before(s.NextHeartbeat) {
			return "keepalive"
		}
	}
	if s.Registration == "offline" && !now.Before(s.RetryAt) {
		return "register"
	}
	return ""
}
