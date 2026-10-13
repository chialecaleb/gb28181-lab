package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chialecaleb/gb28181-lab/internal/config"
	"github.com/chialecaleb/gb28181-lab/internal/gb2022"
	wire "github.com/chialecaleb/gb28181-lab/internal/sip"
	gosip "github.com/emiago/sipgo/sip"
)

func TestRegisterURIValidationRequiresAnObservedExchange(t *testing.T) {
	for _, mode := range []string{ModeStandard, ModeCompatibility} {
		op := Operation{Kind: "register", State: "cancelled", ProtocolMode: mode}
		if mode == ModeCompatibility {
			op.Compatibility = []string{"COMPAT-001"}
		}
		found := false
		for _, check := range operationChecks(op) {
			if check.Layer == "register-uri" {
				found = true
				if check.Status != "not-run" {
					t.Fatal("an unobserved exchange was presented as URI validation", mode)
				}
			}
		}
		if !found {
			t.Fatal("missing URI evidence status")
		}
	}
}

func TestProtocolModesAreExplicitAndKeepHistoricalEvidence(t *testing.T) {
	auth := wire.NewAuthenticator("3402000000", "34020000001180000001", "mode-fixture-secret", time.Now)
	business := make(chan gb2022.Message, 4)
	peer, err := wire.NewEndpoint("127.0.0.1:0", "legacy-fixture", nil, func(req *gosip.Request, tx gosip.ServerTransaction) {
		if req.Recipient.User != "34020000002000000001" {
			wire.Reply(req, tx, 400, nil)
			return
		}
		header := wire.Header(req, "Authorization")
		if header == "" {
			challenge, err := auth.Challenge(false)
			if err != nil {
				wire.Reply(req, tx, 503, nil)
			} else {
				wire.Reply(req, tx, 401, map[string]string{"WWW-Authenticate": challenge})
			}
			return
		}
		if _, err := auth.Verify(header, "REGISTER", req.Recipient.String()); err != nil {
			wire.Reply(req, tx, 403, nil)
			return
		}
		wire.Reply(req, tx, 200, map[string]string{"Expires": wire.Header(req, "Expires")})
	}, func(req *gosip.Request, tx gosip.ServerTransaction) {
		message, err := gb2022.ParseMessage(req.Body())
		if err != nil {
			wire.Reply(req, tx, 400, nil)
			return
		}
		business <- message
		wire.Reply(req, tx, 200, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	for _, mode := range []string{ModeStandard, ModeCompatibility} {
		t.Run(mode, func(t *testing.T) {
			s, err := Open(Options{
				Runtime: config.Runtime{DataDir: t.TempDir(), DeviceBind: "127.0.0.1:0", PlatformBind: "127.0.0.1:0", Transport: "UDP"},
				External: &Binding{PlatformID: "34020000002000000001", Domain: "3402000000", Target: peer.Addr,
					Username: "34020000001180000001", Password: "mode-fixture-secret", AuthMode: "digest"},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			snap, _ := s.Snapshot(s.token)
			c := snap.Config
			c.BindingID, c.ProtocolMode = "external", mode
			c, err = s.SaveConfig(s.token, c)
			if err != nil {
				t.Fatal(err)
			}
			op, err := s.Execute(s.token, Action{Kind: "register", ExpectedRevision: c.Revision, IdempotencyKey: "mode-register", TimeoutSeconds: 2})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			op, err = s.Wait(ctx, s.token, op.ID)
			if err != nil {
				t.Fatal(err)
			}
			if op.ProtocolMode != mode {
				t.Fatal("operation lost its protocol mode")
			}
			if mode == ModeStandard {
				if op.State != "failed" || op.SIPStatus != 400 || len(op.Compatibility) != 0 {
					t.Fatal("standard mode silently fell back to a legacy format")
				}
				for _, record := range s.trace.Snapshot(0, 1000).Records {
					if len(record.Compatibility) != 0 || record.ProtocolMode != ModeStandard {
						t.Fatal("standard request acquired compatibility flags")
					}
				}
				return
			}
			if op.State != "completed" || len(op.Compatibility) != 1 || op.Compatibility[0] != "COMPAT-001" {
				t.Fatal("explicit compatibility registration failed", op)
			}
			uriCheck := false
			for _, check := range op.Checks {
				if check.Layer == "register-uri" && check.Status == "failed" {
					uriCheck = true
				}
			}
			if !uriCheck {
				t.Fatal("successful interoperability hid the RFC-specific deviation")
			}
			action(t, s, "register") // Refresh must use the same explicit compatibility policy.
			body := []byte(`<?xml version="1.0" encoding="GB2312"?><Query><CmdType>DeviceInfo</CmdType><SN>77</SN><DeviceID>34020000001180000001</DeviceID></Query>`)
			query, err := peer.Build(wire.RequestSpec{Method: "MESSAGE", From: "34020000002000000001", To: c.Device.ID, Domain: "3402000000", Target: s.deviceEP.Addr, Transport: "UDP", CSeq: 99, Body: body})
			if err != nil {
				t.Fatal(err)
			}
			res, err := peer.Send(ctx, query)
			if err != nil || res.StatusCode != 200 {
				t.Fatal("explicit legacy query was not acknowledged", err)
			}
			select {
			case response := <-business:
				if response.Root != "Response" || response.CmdType != "DeviceInfo" || response.SN != "77" || response.Result != "OK" {
					t.Fatal("legacy query did not produce a correlated independent business response")
				}
			case <-ctx.Done():
				t.Fatal("legacy business response missing")
			}
			c.ProtocolMode = ModeStandard
			if _, err := s.SaveConfig(s.token, c); err == nil {
				t.Fatal("mode changed during an active registration")
			}
			action(t, s, "unregister")
			c, err = s.SaveConfig(s.token, c)
			if err != nil {
				t.Fatal(err)
			}
			snap, _ = s.Snapshot(s.token)
			if snap.Config.ProtocolMode != ModeStandard || snap.Operations[0].ProtocolMode != ModeCompatibility {
				t.Fatal("mode switch rewrote history")
			}
			snap.Operations[0].Compatibility[0] = "mutated"
			again, _ := s.Snapshot(s.token)
			if again.Operations[0].Compatibility[0] != "COMPAT-001" {
				t.Fatal("caller could mutate operation evidence")
			}
			seen, seenQuery := false, false
			for _, record := range s.trace.Snapshot(0, 1000).Records {
				if record.ProtocolMode != ModeCompatibility || record.Revision >= c.Revision {
					t.Fatal("mode switch relabeled earlier wire evidence")
				}
				if len(record.Compatibility) > 0 && record.Compatibility[0] == "COMPAT-001" {
					seen = true
					if !strings.HasPrefix(record.Summary, "REGISTER ") {
						t.Fatal("compatibility changed another SIP method")
					}
				}
				if len(record.Compatibility) > 0 && record.Compatibility[0] == "COMPAT-002" {
					seenQuery = true
					if record.Direction != "in" || record.Structure["cmdType"] != "DeviceInfo" {
						t.Fatal("legacy query annotation was not confined to the accepted input")
					}
				}
			}
			if !seen || !seenQuery {
				t.Fatal("wire evidence did not record the applied compatibility rule")
			}
		})
	}
}

func TestProtocolModeDefaultsPersistenceAndBuiltinBoundary(t *testing.T) {
	runtime := config.Runtime{DataDir: t.TempDir(), DeviceBind: "127.0.0.1:0", PlatformBind: "127.0.0.1:0", Transport: "UDP"}
	s, err := Open(Options{Runtime: runtime})
	if err != nil {
		t.Fatal(err)
	}
	c := s.config
	if c.ProtocolMode != ModeStandard {
		t.Fatal("new configurations must default to standard")
	}
	c.ProtocolMode = "auto"
	if _, err := s.SaveConfig(s.token, c); err == nil {
		t.Fatal("unknown mode accepted")
	}
	c.ProtocolMode = ModeCompatibility
	c, err = s.SaveConfig(s.token, c)
	if err != nil {
		t.Fatal(err)
	}
	op := action(t, s, "register")
	if len(op.Compatibility) != 0 {
		t.Fatal("external-only compatibility leaked into the builtin registrar")
	}
	action(t, s, "unregister")
	s.Close()
	s, err = Open(Options{Runtime: runtime})
	if err != nil {
		t.Fatal(err)
	}
	if s.config.ProtocolMode != ModeCompatibility || s.state.Running {
		t.Fatal("mode was lost or an in-flight registration was restored")
	}
	// Preserve a real pre-mode schema-1 record and verify additive compatibility.
	revision, data, err := s.store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var old map[string]any
	json.Unmarshal(data, &old)
	delete(old, "protocolMode")
	data, _ = json.Marshal(old)
	if _, err = s.store.Save(context.Background(), revision, data); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(Options{Runtime: runtime})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.config.ProtocolMode != ModeStandard {
		t.Fatal("pre-mode configuration did not retain standard behavior")
	}
}
