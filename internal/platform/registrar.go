package platform

import (
	"fmt"
	"github.com/chialecaleb/gb28181-lab/internal/gb2022"
	wire "github.com/chialecaleb/gb28181-lab/internal/sip"
	gosip "github.com/emiago/sipgo/sip"
	"math"
	"net"
	"strconv"
	"sync"
	"time"
)

type Registration struct {
	Registered        bool      `json:"registered"`
	Online            bool      `json:"online"`
	Reason            string    `json:"reason,omitempty"`
	PeerVersion       string    `json:"peerVersion"`
	VersionCheck      string    `json:"versionCheck"`
	ExpiresAt         time.Time `json:"expiresAt"`
	LastHeartbeat     time.Time `json:"lastHeartbeat"`
	HeartbeatReceived int       `json:"heartbeatReceived"`
	Source            string    `json:"-"`
	Transport         string    `json:"transport"`
	Contact           string    `json:"-"`
	CallID            string    `json:"-"`
	CSeq              uint32    `json:"-"`
	registeredAt      time.Time
}
type Registrar struct {
	mu             sync.Mutex
	domain, device string
	auth           *wire.Authenticator
	state          Registration
	heartbeat      time.Duration
	now            func() time.Time
}

func New(domain, device, password string, heartbeat time.Duration, now func() time.Time) *Registrar {
	if now == nil {
		now = time.Now
	}
	return &Registrar{domain: domain, device: device, auth: wire.NewAuthenticator(domain, device, password, now), heartbeat: heartbeat, now: now}
}
func (r *Registrar) Snapshot() Registration { r.mu.Lock(); defer r.mu.Unlock(); return r.state }
func (r *Registrar) Handle(req *gosip.Request, tx gosip.ServerTransaction, target string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	host, port, err := net.SplitHostPort(target)
	number, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || len(req.Body()) != 0 || req.From().Address.User != r.device || req.To().Address.User != r.device || req.From().Address.Host != r.domain || req.To().Address.Host != r.domain || req.Recipient.User != "" || req.Recipient.Host != host || req.Recipient.Port != number {
		wire.Reply(req, tx, 403, nil)
		return
	}
	if len(req.GetHeaders("Require")) > 0 {
		wire.Reply(req, tx, 400, nil)
		return
	}
	header := wire.Header(req, "Authorization")
	if header == "" {
		c, e := r.auth.Challenge(false)
		if e != nil {
			wire.Reply(req, tx, 503, nil)
		} else {
			wire.Reply(req, tx, 401, map[string]string{"WWW-Authenticate": c})
		}
		return
	}
	stale, e := r.auth.Verify(header, "REGISTER", req.Recipient.String())
	if e != nil {
		if stale {
			c, _ := r.auth.Challenge(true)
			wire.Reply(req, tx, 401, map[string]string{"WWW-Authenticate": c})
		} else {
			wire.Reply(req, tx, 403, nil)
		}
		return
	}
	if r.state.CallID == req.CallID().Value() && req.CSeq().SeqNo <= r.state.CSeq {
		wire.Reply(req, tx, 500, nil)
		return
	}
	expires := 86400
	value := wire.Header(req, "Expires")
	contact := req.Contact()
	if contact != nil {
		if s, ok := contact.Params.Get("expires"); ok {
			value = s
		}
	}
	if value != "" {
		expires, e = strconv.Atoi(value)
		if e != nil || expires < 0 || expires > 86400*30 {
			wire.Reply(req, tx, 400, nil)
			return
		}
	}
	if expires > 0 && expires < 3600 {
		wire.Reply(req, tx, 423, map[string]string{"Min-Expires": "3600"})
		return
	}
	if contact == nil {
		wire.Reply(req, tx, 200, r.responseHeaders())
		return
	}
	if contact.Address.Wildcard {
		if expires != 0 {
			wire.Reply(req, tx, 400, nil)
			return
		}
	} else {
		ip := net.ParseIP(contact.Address.Host)
		if contact.Address.User != r.device || ip == nil || !ip.IsLoopback() || contact.Address.Port < 1 {
			wire.Reply(req, tx, 400, nil)
			return
		}
	}
	r.state.CallID = req.CallID().Value()
	r.state.CSeq = req.CSeq().SeqNo
	r.state.PeerVersion = wire.Header(req, "X-GB-Ver")
	r.state.VersionCheck = "passed"
	if r.state.PeerVersion != "3.0" {
		r.state.VersionCheck = "failed"
		if r.state.PeerVersion == "" {
			r.state.PeerVersion = "missing"
		}
	}
	if expires == 0 {
		r.state.Registered = false
		r.state.Online = false
		r.state.Reason = "deregistered"
		r.state.ExpiresAt = time.Time{}
		r.state.Contact = ""
	} else {
		r.state.Registered = true
		r.state.Online = true
		r.state.Reason = ""
		r.state.Source = req.Source()
		r.state.Transport = req.Transport()
		r.state.Contact = contact.Address.String()
		r.state.ExpiresAt = r.now().Add(time.Duration(expires) * time.Second)
		r.state.registeredAt = r.now()
		r.state.LastHeartbeat = time.Time{}
	}
	wire.Reply(req, tx, 200, r.responseHeaders())
}
func (r *Registrar) responseHeaders() map[string]string {
	h := map[string]string{}
	if r.state.Registered {
		seconds := max(0, int(math.Ceil(r.state.ExpiresAt.Sub(r.now()).Seconds())))
		h["Contact"] = fmt.Sprintf("<%s>;expires=%d", r.state.Contact, seconds)
		h["Expires"] = strconv.Itoa(seconds)
	} else {
		h["Expires"] = "0"
	}
	return h
}
func (r *Registrar) Accept(source, transport, from string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.state.Registered || !r.now().Before(r.state.ExpiresAt) || from != r.device {
		return false
	}
	if transport == r.state.Transport {
		return source == r.state.Source
	}
	host, _, _ := net.SplitHostPort(source)
	registeredHost, _, _ := net.SplitHostPort(r.state.Source)
	return transport == "TCP" && host == registeredHost
}
func (r *Registrar) Heartbeat(m gb2022.Message) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.state.Registered || m.DeviceID != r.device || !r.now().Before(r.state.ExpiresAt) {
		return false
	}
	r.state.LastHeartbeat = r.now()
	r.state.HeartbeatReceived++
	r.state.Online = true
	r.state.Reason = ""
	if m.Status != "OK" {
		r.state.Reason = "device reported fault"
	}
	return true
}
func (r *Registrar) Tick(tcpAlive bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.state.Registered {
		return
	}
	now := r.now()
	if !now.Before(r.state.ExpiresAt) {
		r.state.Registered = false
		r.state.Online = false
		r.state.Reason = "registration expired"
		return
	}
	if r.state.Transport == "TCP" && !tcpAlive {
		r.state.Registered = false
		r.state.Online = false
		r.state.Reason = "TCP connection closed"
		return
	}
	last := r.state.LastHeartbeat
	if last.IsZero() {
		last = r.state.registeredAt
	}
	if now.Sub(last) >= 3*r.heartbeat {
		r.state.Online = false
		r.state.Reason = "three heartbeat receive periods missed"
	}
}
