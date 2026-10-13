package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chialecaleb/gb28181-lab/internal/config"
	"github.com/chialecaleb/gb28181-lab/internal/device"
	"github.com/chialecaleb/gb28181-lab/internal/evidence"
	"github.com/chialecaleb/gb28181-lab/internal/gb2022"
	"github.com/chialecaleb/gb28181-lab/internal/platform"
	wire "github.com/chialecaleb/gb28181-lab/internal/sip"
	"github.com/chialecaleb/gb28181-lab/internal/storage"
	gosip "github.com/emiago/sipgo/sip"
)

type Options struct {
	Runtime  config.Runtime
	External *Binding
	Now      func() time.Time
}
type pendingQuery struct {
	opID, target, cmd, sn, binding string
	generation                     uint64
	catalog                        gb2022.Catalog
	result                         chan queryResult
}
type queryResult struct {
	value any
	err   error
}
type runningOperation struct {
	op     Operation
	cancel context.CancelFunc
	done   chan struct{}
	digest [32]byte
	key    string
	action Action
}
type Service struct {
	mu                       sync.Mutex
	registerMu               sync.Mutex
	config                   Config
	state                    device.State
	store                    storage.Repository
	trace                    *evidence.Trace
	deviceEP, platformEP     *wire.Endpoint
	registrar                *platform.Registrar
	external                 *Binding
	password, token          string
	now                      func() time.Time
	ctx                      context.Context
	cancel                   context.CancelFunc
	wg                       sync.WaitGroup
	closed                   bool
	ops                      map[string]*runningOperation
	order                    []string
	keys                     map[string]string
	pending                  map[string]*pendingQuery
	active                   int
	registerCallID           string
	registerCSeq             uint32
	sn                       uint64
	deviceCSeq, platformCSeq uint32
	responders               chan struct{}
	responseCancels          map[string]context.CancelFunc
}

func Open(o Options) (*Service, error) {
	store, e := storage.Open(o.Runtime.DataDir)
	if e != nil {
		return nil, e
	}
	ok := false
	defer func() {
		if !ok {
			store.Close()
		}
	}()
	if o.Now == nil {
		o.Now = time.Now
	}
	token, e := storage.LocalSecret(o.Runtime.DataDir, "access.token")
	if e != nil {
		return nil, e
	}
	password, e := storage.LocalSecret(o.Runtime.DataDir, "builtin-sip.secret")
	if e != nil {
		return nil, e
	}
	s := &Service{store: store, trace: evidence.New(), state: device.Initial(), password: password, token: token, now: o.Now, ops: map[string]*runningOperation{}, keys: map[string]string{}, pending: map[string]*pendingQuery{}, registerCallID: storage.RandomID(), responders: make(chan struct{}, 16), responseCancels: map[string]context.CancelFunc{}}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.trace.AddSecrets(password, token)
	if o.External != nil {
		ext := *o.External
		if !gb2022.ValidID(ext.PlatformID) || ext.Domain == "" || strings.ContainsAny(ext.Domain, "\r\n\x00 <>@;\"") || ext.AuthMode != "digest" || ext.Username == "" || ext.Password == "" {
			return nil, errors.New("external binding needs verified Digest parameters")
		}
		if _, e = config.LocalEndpoint(ext.Target); e != nil {
			return nil, errors.New("M0 external binding must be loopback")
		}
		s.external = &ext
		s.trace.AddSecrets(ext.PlatformID, ext.Domain, ext.Target, ext.Username, ext.Password)
	}
	revision, b, e := store.Load(context.Background())
	if e != nil {
		return nil, e
	}
	if revision == 0 {
		s.config = Config{SchemaVersion: "1.0", ProtocolMode: ModeStandard, Device: device.Default(), BindingID: "builtin", Transport: o.Runtime.Transport, Expires: 86400, HeartbeatSeconds: 60, RetrySeconds: 60, CatalogBatchSize: 16}
		if s.config.Transport == "" {
			s.config.Transport = "UDP"
		}
		s.config.Device.Stamp(device.Definition{}, s.now())
		b, _ = json.Marshal(s.config)
		revision, e = store.Save(context.Background(), 0, b)
		if e != nil {
			return nil, e
		}
	} else {
		if e = json.Unmarshal(b, &s.config); e != nil {
			return nil, errors.New("stored configuration is invalid")
		}
	}
	s.config.Revision = revision
	if s.config.ProtocolMode == "" { // Existing schema-1 configurations remain standard.
		s.config.ProtocolMode = ModeStandard
	}
	if e = validateConfig(s.config); e != nil {
		return nil, e
	}
	s.trace.SetContext(s.config.ProtocolMode, revision)
	s.registrar = platform.New("3402000000", s.config.Device.ID, password, time.Duration(s.config.HeartbeatSeconds)*time.Second, s.now)
	platformEP, e := wire.NewEndpoint(o.Runtime.PlatformBind, "platform", s.trace.Observe, s.onRegister, s.onPlatformMessage)
	if e != nil {
		return nil, e
	}
	s.mu.Lock()
	s.platformEP = platformEP
	s.mu.Unlock()
	s.deviceEP, e = wire.NewEndpoint(o.Runtime.DeviceBind, "device", s.trace.Observe, nil, s.onDeviceMessage)
	if e != nil {
		s.platformEP.Close()
		return nil, e
	}
	s.wg.Add(1)
	go s.maintain()
	ok = true
	return s, nil
}
func validateConfig(c Config) error {
	if c.SchemaVersion != "1.0" {
		return failure("invalid", "unsupported config schema")
	}
	if c.ProtocolMode != ModeStandard && c.ProtocolMode != ModeCompatibility {
		return failure("invalid", "protocolMode must be standard or compatibility")
	}
	if e := c.Device.Validate(); e != nil {
		return failure("invalid", e.Error())
	}
	if c.BindingID != "builtin" && c.BindingID != "external" {
		return failure("invalid", "unknown binding")
	}
	if c.Transport != "UDP" && c.Transport != "TCP" {
		return failure("invalid", "transport must be UDP or TCP")
	}
	if c.Expires < 3600 || c.Expires > 86400*30 || c.HeartbeatSeconds < 60 || c.HeartbeatSeconds > 3600 || c.RetrySeconds < 60 || c.RetrySeconds > 3600 || c.CatalogBatchSize < 1 || c.CatalogBatchSize > 32 {
		return failure("invalid", "timing or batch size is outside M0 limits")
	}
	return nil
}
func (s *Service) authorize(token string) error {
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
		return failure("unauthorized", "local access token required")
	}
	return nil
}

// LocalToken is only for trusted host adapters. It is not a bound API method.
func (s *Service) LocalToken() string          { return s.token }
func (s *Service) TraceLocal() *evidence.Trace { return s.trace }
func (s *Service) bindingLocked() (Binding, error) {
	if s.config.BindingID == "external" {
		if s.external == nil {
			return Binding{}, failure("not_configured", "external binding has no verified local credentials")
		}
		return *s.external, nil
	}
	return Binding{PlatformID: "34020000002000000001", Domain: "3402000000", Target: s.platformEP.Addr, Username: s.config.Device.ID, Password: s.password, AuthMode: "digest"}, nil
}

func (s *Service) Snapshot(token string) (Snapshot, error) {
	if e := s.authorize(token); e != nil {
		return Snapshot{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked(), nil
}
func (s *Service) snapshotLocked() Snapshot {
	c := s.config
	c.Device = c.Device.Clone()
	snapshot := Snapshot{SchemaVersion: "1.0", At: s.now().UTC(), Config: c, Device: s.state, Platform: s.registrar.Snapshot(), ExternalConfigured: s.external != nil, ActiveOperations: s.active, Operations: []Operation{}, Capabilities: []Check{{"signaling", "implemented", "REGISTER / Keepalive / DeviceInfo / Catalog; UDP and TCP"}, {"media", "not-run", "Media and PTZ are not implemented"}, {"security", "not-run", "GB35114 and GB28181 Date/Note are not implemented"}, {"capacity", "not-run", "One-device M0 profile; no capacity claim"}, {"mcp", "not-run", "SDK/spec researched; adapter deferred"}}}
	for _, id := range s.order {
		snapshot.Operations = append(snapshot.Operations, s.ops[id].op.clone())
	}
	dc, ds, dt := s.deviceEP.Stats()
	pc, ps, pt := s.platformEP.Stats()
	snapshot.Resources = Resources{ClientTransactions: dc + pc, ServerTransactions: ds + ps, AcceptedTCPConnections: dt + pt, PendingQueries: len(s.pending), Listeners: 4}
	if s.closed {
		snapshot.Resources.Listeners = 0
	}
	return snapshot
}
func (s *Service) Traces(token string, after uint64, limit int) (evidence.Snapshot, error) {
	if e := s.authorize(token); e != nil {
		return evidence.Snapshot{}, e
	}
	return s.trace.Snapshot(after, limit), nil
}
func (s *Service) Evidence(token string) (map[string]any, error) {
	snapshot, e := s.Snapshot(token)
	if e != nil {
		return nil, e
	}
	return map[string]any{"schemaVersion": "1.0", "ruleVersion": gb2022.RuleVersion, "snapshot": snapshot, "traces": s.trace.Snapshot(0, 1000), "scope": "independent local observation; no complete GB28181/GB35114 conformity claim", "redaction": "credentials masked; raw SHA256 refers to local original octets"}, nil
}
func (s *Service) SaveConfig(token string, c Config) (Config, error) {
	if e := s.authorize(token); e != nil {
		return Config{}, e
	}
	if c.ProtocolMode == "" {
		c.ProtocolMode = ModeStandard
	}
	if e := validateConfig(c); e != nil {
		return Config{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Config{}, failure("closed", "engine is closing")
	}
	if s.state.Running || s.active > 0 || len(s.responders) > 0 {
		return Config{}, failure("busy", "unregister and finish active operations before changing configuration")
	}
	if c.Revision != s.config.Revision {
		return Config{}, failure("conflict", "configuration revision changed")
	}
	if c.BindingID == "external" && s.external == nil {
		return Config{}, failure("not_configured", "verified external binding is not configured")
	}
	c.Device = c.Device.Clone()
	c.Device.Stamp(s.config.Device, s.now())
	b, _ := json.Marshal(c)
	rev, e := s.store.Save(s.ctx, c.Revision, b)
	if e != nil {
		return Config{}, failure("storage", "configuration could not be committed")
	}
	c.Revision = rev
	s.config = c
	s.trace.SetContext(c.ProtocolMode, rev)
	s.state = device.Initial()
	s.registrar = platform.New("3402000000", c.Device.ID, s.password, time.Duration(c.HeartbeatSeconds)*time.Second, s.now)
	s.registerCallID = storage.RandomID()
	s.registerCSeq = 0
	c.Device = c.Device.Clone()
	return c, nil
}
func (s *Service) Execute(token string, a Action) (Operation, error) {
	if e := s.authorize(token); e != nil {
		return Operation{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.executeLocked(a)
}
func (s *Service) executeLocked(a Action) (Operation, error) {
	if s.closed {
		return Operation{}, failure("closed", "engine is closing")
	}
	if !supported(a.Kind) {
		return Operation{}, failure("unsupported", "action is not implemented in M0")
	}
	if len(a.IdempotencyKey) < 1 || len(a.IdempotencyKey) > 128 {
		return Operation{}, failure("invalid", "idempotencyKey is required (1..128 characters)")
	}
	b, _ := json.Marshal(a)
	digest := sha256.Sum256(b)
	if id, ok := s.keys[a.IdempotencyKey]; ok {
		r := s.ops[id]
		if r.digest != digest {
			return Operation{}, failure("conflict", "idempotency key was used with different input")
		}
		return r.op.clone(), nil
	}
	if a.ExpectedRevision != s.config.Revision {
		return Operation{}, failure("conflict", "configuration revision changed")
	}
	if s.active >= 16 {
		return Operation{}, failure("limit", "active operation limit reached")
	}
	if _, e := s.bindingLocked(); e != nil {
		return Operation{}, e
	}
	if a.Target == "" {
		a.Target = s.config.Device.ID
	}
	if !gb2022.ValidID(a.Target) {
		return Operation{}, failure("invalid", "target identity must contain 20 digits")
	}
	if a.TimeoutSeconds == 0 {
		a.TimeoutSeconds = 40
	}
	if a.TimeoutSeconds < 1 || a.TimeoutSeconds > 120 {
		return Operation{}, failure("invalid", "timeout must be 1..120 seconds")
	}
	if _, _, e := gb2022.TimeRange(a.StartTime, a.EndTime); e != nil {
		return Operation{}, failure("invalid", e.Error())
	}
	if a.Kind != "catalog" && (a.StartTime != "" || a.EndTime != "") {
		return Operation{}, failure("invalid", "time filters are only valid for Catalog")
	}
	for _, r := range s.ops {
		if r.op.State == "running" && (r.op.Kind == a.Kind || (a.Kind == "register" && r.op.Kind == "unregister")) {
			return Operation{}, failure("busy", "this action is already in progress")
		}
	}
	if a.Kind != "register" && a.Kind != "unregister" && s.state.Registration != "registered" {
		return Operation{}, failure("not_registered", "register the device before this action")
	}
	if (a.Kind == "deviceinfo" || a.Kind == "catalog") && s.config.BindingID != "builtin" {
		return Operation{}, failure("external_initiator", "external queries must be initiated by the bound platform")
	}
	before := s.state
	if a.Kind == "register" && !s.state.Running {
		s.state.Start()
	}
	if a.Kind == "unregister" {
		s.state.Stop()
		for _, cancel := range s.responseCancels {
			cancel()
		}
		for _, r := range s.ops {
			if r.op.State == "running" {
				r.cancel()
			}
		}
		s.pending = map[string]*pendingQuery{}
	}
	id := storage.RandomID()
	ctx, cancel := context.WithTimeout(s.ctx, time.Duration(a.TimeoutSeconds)*time.Second)
	op := Operation{ID: id, Kind: a.Kind, Target: a.Target, BindingID: s.config.BindingID, ProtocolMode: s.config.ProtocolMode, Compatibility: []string{}, Revision: s.config.Revision, Generation: s.state.Generation, State: "running", StartedAt: s.now().UTC(), Business: "unknown", Before: before, Cleanup: "pending"}
	if s.config.ProtocolMode == ModeCompatibility && s.config.BindingID == "external" && (a.Kind == "register" || a.Kind == "unregister") {
		op.Compatibility = []string{"COMPAT-001"}
	}
	if a.Kind == "register" || a.Kind == "unregister" || a.Kind == "keepalive" {
		op.Business = "not-applicable"
	}
	r := &runningOperation{op: op, cancel: cancel, done: make(chan struct{}), digest: digest, key: a.IdempotencyKey, action: a}
	s.ops[id] = r
	s.order = append(s.order, id)
	s.keys[a.IdempotencyKey] = id
	s.active++
	for len(s.order) > 128 {
		index := -1
		for i, key := range s.order {
			if s.ops[key].op.State != "running" {
				index = i
				break
			}
		}
		if index < 0 {
			break
		}
		key := s.order[index]
		delete(s.keys, s.ops[key].key)
		delete(s.ops, key)
		s.order = append(s.order[:index], s.order[index+1:]...)
	}
	s.wg.Add(1)
	go s.run(ctx, r)
	return op, nil
}
func (s *Service) Wait(ctx context.Context, token, id string) (Operation, error) {
	if e := s.authorize(token); e != nil {
		return Operation{}, e
	}
	s.mu.Lock()
	r := s.ops[id]
	s.mu.Unlock()
	if r == nil {
		return Operation{}, failure("not_found", "operation not found")
	}
	select {
	case <-ctx.Done():
		return Operation{}, ctx.Err()
	case <-r.done:
		s.mu.Lock()
		defer s.mu.Unlock()
		return r.op.clone(), nil
	}
}
func (s *Service) Cancel(token, id string) (Operation, error) {
	if e := s.authorize(token); e != nil {
		return Operation{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.ops[id]
	if r == nil {
		return Operation{}, failure("not_found", "operation not found")
	}
	if r.op.State == "running" {
		if r.op.Kind == "unregister" {
			return Operation{}, failure("cleanup_required", "deregistration is already the bounded cleanup action")
		}
		r.cancel()
		if r.op.Kind == "register" {
			s.state.Stop()
		}
		delete(s.pending, r.op.SN)
	}
	return r.op.clone(), nil
}

func (s *Service) run(ctx context.Context, r *runningOperation) {
	defer s.wg.Done()
	defer r.cancel()
	var result any
	var err error
	switch r.op.Kind {
	case "register":
		err = s.register(ctx, r, false)
	case "unregister":
		err = s.register(ctx, r, true)
	case "keepalive":
		err = s.keepalive(ctx, r)
	case "deviceinfo", "catalog":
		result, err = s.query(ctx, r)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r.op.FinishedAt = s.now().UTC()
	r.op.After = s.state
	r.op.Result = result
	r.op.State = "completed"
	if r.op.Cleanup == "pending" {
		r.op.Cleanup = "in-flight context released"
	}
	if err != nil {
		r.op.State = "failed"
		r.op.Error = AsError(err)
		if errors.Is(err, context.Canceled) {
			r.op.State = "cancelled"
			r.op.Error = &Error{Code: "cancelled", Message: "operation cancelled; late results ignored"}
		} else if errors.Is(err, context.DeadlineExceeded) {
			r.op.Error = &Error{Code: "timeout", Message: "operation deadline exceeded"}
		}
		if r.op.Business == "unknown" {
			r.op.Business = "failed"
		}
	}
	if r.op.SN != "" {
		delete(s.pending, r.op.SN)
	}
	r.op.Checks = operationChecks(r.op)
	s.active--
	close(r.done)
}

func (s *Service) register(ctx context.Context, r *runningOperation, remove bool) error {
	s.registerMu.Lock()
	defer s.registerMu.Unlock()
	err := s.registerWire(ctx, r, remove)
	if err != nil && !remove {
		s.mu.Lock()
		c := s.config
		s.state.Failed(r.op.Generation, s.now(), AsError(err).Message, time.Duration(c.RetrySeconds)*time.Second)
		s.mu.Unlock()
		// A 200 can be lost or arrive after cancellation. Explicitly remove the
		// potentially accepted binding before another REGISTER is allowed to start.
		cleanupCtx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
		cleanup := &runningOperation{op: Operation{ID: r.op.ID, Generation: r.op.Generation}}
		cleanupErr := s.registerWire(cleanupCtx, cleanup, true)
		cancel()
		s.mu.Lock()
		if cleanupErr == nil {
			r.op.Cleanup = "compensating unregister acknowledged"
		} else {
			r.op.Cleanup = "compensating unregister unconfirmed; peer expiry may be required"
		}
		s.mu.Unlock()
	}
	return err
}
func (s *Service) registerWire(ctx context.Context, r *runningOperation, remove bool) error {
	s.mu.Lock()
	c := s.config
	b, e := s.bindingLocked()
	if e != nil {
		s.mu.Unlock()
		return e
	}
	expires := c.Expires
	if remove {
		expires = 0
	}
	call := s.registerCallID
	gen := r.op.Generation
	s.mu.Unlock()
	s.trace.Bind("call:"+call, r.op.ID)
	var authorization string
	for attempt := 0; attempt < 3; attempt++ {
		if e = ctx.Err(); e != nil {
			return e
		}
		s.mu.Lock()
		s.registerCSeq++
		seq := s.registerCSeq
		s.mu.Unlock()
		req, e := s.deviceEP.Build(wire.RequestSpec{Method: "REGISTER", From: c.Device.ID, To: b.PlatformID, Domain: b.Domain, Target: b.Target, Transport: c.Transport, CallID: call, CSeq: seq, Expires: &expires, Authorization: authorization, RegisterUserinfo: c.ProtocolMode == ModeCompatibility && c.BindingID == "external"})
		if e != nil {
			return failure("protocol", e.Error())
		}
		res, e := s.deviceEP.Send(ctx, req)
		if e != nil {
			s.mu.Lock()
			if !remove {
				s.state.Failed(gen, s.now(), e.Error(), time.Duration(c.RetrySeconds)*time.Second)
			}
			s.mu.Unlock()
			return e
		}
		s.mu.Lock()
		r.op.SIPStatus = res.StatusCode
		s.mu.Unlock()
		if res.StatusCode == 401 {
			authorization, e = wire.Authorize(wire.Header(res, "WWW-Authenticate"), b.Domain, b.Username, b.Password, "REGISTER", req.Recipient.String(), 1, "")
			if e != nil {
				s.mu.Lock()
				if !remove {
					s.state.Failed(gen, s.now(), e.Error(), time.Duration(c.RetrySeconds)*time.Second)
				}
				s.mu.Unlock()
				return failure("authentication", e.Error())
			}
			continue
		}
		if res.StatusCode != 200 {
			reason := fmt.Sprintf("registration rejected: SIP %d", res.StatusCode)
			if res.StatusCode >= 300 && res.StatusCode < 400 {
				reason = "registration redirection is not implemented in M0"
			}
			s.mu.Lock()
			if !remove {
				s.state.Failed(gen, s.now(), reason, time.Duration(c.RetrySeconds)*time.Second)
			}
			s.mu.Unlock()
			return failure("sip_rejected", reason)
		}
		if len(res.Body()) != 0 {
			return failure("protocol", "REGISTER success unexpectedly contains a body")
		}
		if remove {
			s.deviceEP.DropTCP(b.Target)
			return nil
		}
		actual := expires
		if v := wire.Header(res, "Expires"); v != "" {
			actual, e = strconv.Atoi(v)
		}
		if contact := res.Contact(); contact != nil {
			if v, ok := contact.Params.Get("expires"); ok {
				actual, e = strconv.Atoi(v)
			}
		}
		if e != nil || actual < 3600 || actual > c.Expires {
			return failure("protocol", "registrar returned an invalid registration expiry")
		}
		s.mu.Lock()
		accepted := ctx.Err() == nil && s.state.Registered(gen, s.now(), time.Duration(actual)*time.Second, time.Duration(c.HeartbeatSeconds)*time.Second, wire.Header(res, "X-GB-Ver"))
		s.mu.Unlock()
		if !accepted {
			return context.Canceled
		}
		return nil
	}
	s.mu.Lock()
	if !remove {
		s.state.Failed(gen, s.now(), "Digest challenge retry limit", time.Duration(c.RetrySeconds)*time.Second)
	}
	s.mu.Unlock()
	return failure("authentication", "Digest challenge retry limit reached")
}

func (s *Service) nextMessage(role, op string, body []byte, targetID, targetAddr, domain, transport string) (*gosip.Request, error) {
	s.mu.Lock()
	from := s.config.Device.ID
	var seq uint32
	if role == "platform" {
		s.platformCSeq++
		seq = s.platformCSeq
		from = "34020000002000000001"
	} else {
		s.deviceCSeq++
		seq = s.deviceCSeq
	}
	s.mu.Unlock()
	call := storage.RandomID()
	s.trace.Bind("call:"+call, op)
	ep := s.deviceEP
	if role == "platform" {
		ep = s.platformEP
	}
	req, e := ep.Build(wire.RequestSpec{Method: "MESSAGE", From: from, To: targetID, Domain: domain, Target: targetAddr, Transport: transport, CallID: call, CSeq: seq, Body: body})
	if e != nil {
		return nil, e
	}
	if transport == "UDP" && len(req.String()) > 1300 {
		req, e = ep.Build(wire.RequestSpec{Method: "MESSAGE", From: from, To: targetID, Domain: domain, Target: targetAddr, Transport: "TCP", CallID: call, CSeq: seq, Body: body})
	}
	return req, e
}
func (s *Service) keepalive(ctx context.Context, r *runningOperation) error {
	s.mu.Lock()
	s.sn++
	sn := strconv.FormatUint(s.sn, 10)
	r.op.SN = sn
	c := s.config
	b, e := s.bindingLocked()
	gen := r.op.Generation
	s.mu.Unlock()
	if e != nil {
		return e
	}
	s.trace.Bind("sn:Keepalive:"+sn, r.op.ID)
	body, e := gb2022.XML(gb2022.Message{Root: "Notify", CmdType: "Keepalive", SN: sn, DeviceID: c.Device.ID, Status: "OK"})
	if e != nil {
		return e
	}
	req, e := s.nextMessage("device", r.op.ID, body, b.PlatformID, b.Target, b.Domain, c.Transport)
	if e != nil {
		return e
	}
	res, e := s.deviceEP.Send(ctx, req)
	if e == nil && (res.StatusCode != 200 || len(res.Body()) != 0) {
		e = failure("sip_rejected", "Keepalive was not acknowledged with an empty SIP 200")
	}
	reason := ""
	if e != nil {
		reason = e.Error()
	}
	s.mu.Lock()
	if res != nil {
		r.op.SIPStatus = res.StatusCode
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		s.state.Beat(gen, s.now(), e == nil, reason, time.Duration(c.HeartbeatSeconds)*time.Second, time.Duration(c.RetrySeconds)*time.Second)
	}
	s.mu.Unlock()
	return e
}
func (s *Service) query(ctx context.Context, r *runningOperation) (any, error) {
	s.mu.Lock()
	s.sn++
	sn := strconv.FormatUint(s.sn, 10)
	r.op.SN = sn
	reg := s.registrar.Snapshot()
	p := &pendingQuery{opID: r.op.ID, target: r.op.Target, cmd: kindCmd(r.op.Kind), sn: sn, binding: r.op.BindingID, generation: r.op.Generation, result: make(chan queryResult, 1), catalog: gb2022.Catalog{SN: sn, Target: r.op.Target}}
	s.pending[sn] = p
	c := s.config
	s.mu.Unlock()
	s.trace.Bind("sn:"+p.cmd+":"+sn, r.op.ID)
	if !reg.Registered || !reg.Online {
		return nil, failure("not_registered", "platform has no online device registration")
	}
	body, e := gb2022.XML(gb2022.Message{Root: "Query", CmdType: p.cmd, SN: sn, DeviceID: p.target, StartTime: r.action.StartTime, EndTime: r.action.EndTime})
	if e != nil {
		return nil, e
	}
	req, e := s.nextMessage("platform", r.op.ID, body, c.Device.ID, reg.Source, "3402000000", c.Transport)
	if e != nil {
		return nil, e
	}
	res, e := s.platformEP.Send(ctx, req)
	if e != nil {
		return nil, e
	}
	s.mu.Lock()
	r.op.SIPStatus = res.StatusCode
	s.mu.Unlock()
	if res.StatusCode != 200 || len(res.Body()) > 0 {
		return nil, failure("sip_rejected", "query did not receive an empty SIP 200")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-p.result:
		if result.err != nil {
			return result.value, result.err
		}
		s.mu.Lock()
		valid := ctx.Err() == nil && p.generation == s.state.Generation && s.state.Running
		if valid {
			r.op.Business = "passed"
		}
		s.mu.Unlock()
		if !valid {
			return nil, context.Canceled
		}
		return result.value, nil
	}
}

func (s *Service) onRegister(r *gosip.Request, tx gosip.ServerTransaction) {
	s.mu.Lock()
	reg := s.registrar
	ep := s.platformEP
	s.mu.Unlock()
	if ep == nil {
		wire.Reply(r, tx, 503, nil)
		return
	}
	reg.Handle(r, tx, ep.Addr)
}
func (s *Service) onPlatformMessage(r *gosip.Request, tx gosip.ServerTransaction) {
	s.mu.Lock()
	reg := s.registrar
	s.mu.Unlock()
	if !strings.EqualFold(wire.Header(r, "Content-Type"), gb2022.ContentType) {
		wire.Reply(r, tx, 415, nil)
		return
	}
	if !reg.Accept(r.Source(), r.Transport(), r.From().Address.User) || r.To().Address.User != "34020000002000000001" {
		wire.Reply(r, tx, 403, nil)
		return
	}
	m, e := gb2022.ParseMessage(r.Body())
	if e != nil {
		wire.Reply(r, tx, 400, nil)
		return
	}
	if m.Root == "Notify" && m.CmdType == "Keepalive" {
		if !reg.Heartbeat(m) {
			wire.Reply(r, tx, 403, nil)
			return
		}
		wire.Reply(r, tx, 200, nil)
		return
	}
	if m.Root != "Response" {
		wire.Reply(r, tx, 400, nil)
		return
	}
	s.mu.Lock()
	p := s.pending[m.SN]
	if p == nil || p.cmd != m.CmdType || p.target != m.DeviceID || p.generation != s.state.Generation || !s.state.Running || p.binding != s.config.BindingID {
		s.mu.Unlock()
		wire.Reply(r, tx, 481, nil)
		return
	}
	result := queryResult{value: m}
	done := true
	if m.CmdType == "Catalog" {
		done, e = p.catalog.Add(m)
		result.value = append([]gb2022.Item{}, p.catalog.Items...)
		result.err = e
	}
	if m.CmdType == "DeviceInfo" && m.Result != "OK" {
		result.err = failure("business_rejected", "device returned DeviceInfo ERROR")
	}
	if e != nil {
		result.err = failure("protocol", e.Error())
		done = true
	}
	if done {
		select {
		case p.result <- result:
		default:
		}
		delete(s.pending, m.SN)
	}
	s.mu.Unlock()
	if e != nil {
		wire.Reply(r, tx, 400, nil)
	} else {
		wire.Reply(r, tx, 200, nil)
	}
}
func (s *Service) onDeviceMessage(r *gosip.Request, tx gosip.ServerTransaction) {
	s.mu.Lock()
	b, e := s.bindingLocked()
	c := s.config
	c.Device = c.Device.Clone()
	gen := s.state.Generation
	registered := s.state.Running && s.state.Registration == "registered"
	s.mu.Unlock()
	if e != nil || !registered || r.From().Address.User != b.PlatformID || r.To().Address.User != c.Device.ID || r.Source() != b.Target {
		wire.Reply(r, tx, 403, nil)
		return
	}
	if !strings.EqualFold(wire.Header(r, "Content-Type"), gb2022.ContentType) {
		wire.Reply(r, tx, 415, nil)
		return
	}
	m, compatibility, e := gb2022.ParseIncomingQuery(r.Body(), c.ProtocolMode == ModeCompatibility && c.BindingID == "external")
	if e != nil {
		wire.Reply(r, tx, 400, nil)
		return
	}
	if compatibility != "" {
		branch, _ := r.Via().Params.Get("branch")
		s.trace.MarkCompatibility(wire.Header(r, "Call-ID"), wire.Header(r, "CSeq"), branch, compatibility)
	}
	var bodies [][]byte
	if m.CmdType == "DeviceInfo" {
		info, err := c.Device.Info(m.SN, m.DeviceID)
		if err != nil {
			wire.Reply(r, tx, 404, nil)
			return
		}
		raw, err := gb2022.XML(info)
		if err != nil {
			wire.Reply(r, tx, 500, nil)
			return
		}
		bodies = [][]byte{raw}
	} else {
		items, err := c.Device.Catalog(b.PlatformID, m.DeviceID, m.StartTime, m.EndTime, s.now())
		if err != nil {
			wire.Reply(r, tx, 404, nil)
			return
		}
		bodies, err = gb2022.Batches(m.SN, m.DeviceID, items, c.CatalogBatchSize)
		if err != nil {
			wire.Reply(r, tx, 500, nil)
			return
		}
	}
	select {
	case s.responders <- struct{}{}:
	default:
		wire.Reply(r, tx, 503, nil)
		return
	}
	if e = wire.Reply(r, tx, 200, nil); e != nil {
		<-s.responders
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		<-s.responders
		return
	}
	s.wg.Add(1)
	ctx, cancel := context.WithTimeout(s.ctx, 40*time.Second)
	responseID := storage.RandomID()
	s.responseCancels[responseID] = cancel
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer func() { <-s.responders; s.mu.Lock(); delete(s.responseCancels, responseID); s.mu.Unlock() }()
		defer cancel()
		for _, body := range bodies {
			s.mu.Lock()
			valid := s.state.Running && s.state.Generation == gen
			s.mu.Unlock()
			if !valid {
				return
			}
			req, err := s.nextMessage("device", "", body, b.PlatformID, b.Target, b.Domain, c.Transport)
			if err != nil {
				return
			}
			res, err := s.deviceEP.Send(ctx, req)
			if err != nil || res.StatusCode != 200 {
				return
			}
		}
	}()
}
func (s *Service) maintain() {
	defer s.wg.Done()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-tick.C:
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				return
			}
			reg := s.registrar.Snapshot()
			s.registrar.Tick(reg.Transport != "TCP" || s.platformEP.AliveTCP(reg.Source))
			b, e := s.bindingLocked()
			if e != nil {
				s.mu.Unlock()
				continue
			}
			action := s.state.Due(s.now(), s.config.Transport != "TCP" || s.deviceEP.AliveTCP(b.Target), time.Duration(s.config.RetrySeconds)*time.Second)
			if action != "" {
				_, _ = s.executeLocked(Action{Kind: action, ExpectedRevision: s.config.Revision, IdempotencyKey: "timer-" + storage.RandomID()})
			}
			s.mu.Unlock()
		}
	}
}

func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	running := s.state.Running
	revision := s.config.Revision
	s.mu.Unlock()
	if running {
		op, e := s.Execute(s.token, Action{Kind: "unregister", ExpectedRevision: revision, IdempotencyKey: "shutdown-" + storage.RandomID(), TimeoutSeconds: 5})
		if e == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			s.Wait(ctx, s.token, op.ID)
			cancel()
		}
	}
	s.mu.Lock()
	s.closed = true
	s.state.Stop()
	s.cancel()
	for _, r := range s.ops {
		r.cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
	s.deviceEP.Close()
	s.platformEP.Close()
	return s.store.Close()
}

// Addresses is local host diagnostics, not a browser configuration response.
func (s *Service) Addresses() (string, string) { return s.deviceEP.Addr, s.platformEP.Addr }
func (s *Service) DropDeviceTCP() {
	s.mu.Lock()
	b, _ := s.bindingLocked()
	s.mu.Unlock()
	s.deviceEP.DropTCP(b.Target)
}
func isLoopbackPeer(addr string) bool {
	host, _, e := net.SplitHostPort(addr)
	return e == nil && net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()
}
