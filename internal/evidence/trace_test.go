package evidence

import (
	"bytes"
	"fmt"
	wire "github.com/chialecaleb/gb28181-lab/internal/sip"
	"strings"
	"testing"
)

func TestBoundedRedactedWireCopies(t *testing.T) {
	tr := New()
	tr.limit = 2
	tr.AddSecrets("synthetic-secret")
	raw := []byte("REGISTER sip:lab SIP/2.0\r\nAuthorization: Digest response=secret\r\nX-Test: synthetic-secret\r\n\r\n")
	for i := 0; i < 4; i++ {
		tr.Observe(wire.Event{Bytes: raw})
	}
	s := tr.Snapshot(0, 10)
	if len(s.Records) != 2 || s.Dropped != 2 {
		t.Fatal("trace bounds")
	}
	for _, r := range s.Records {
		if !r.Redacted || strings.Contains(r.Text, "secret") || strings.Contains(r.Hex, "736563726574") {
			t.Fatal("secret leak")
		}
	}
	if !bytes.Equal(tr.Raw()[0], raw) {
		t.Fatal("redaction changed original")
	}
}

func TestModeAndConfigurationStayWithOriginalRecords(t *testing.T) {
	tr := New()
	tr.SetContext("compatibility", 2)
	tr.Observe(wire.Event{Direction: "out", Bytes: []byte("REGISTER sip:server@127.0.0.1:15060 SIP/2.0\r\nContent-Length: 0\r\n\r\n")})
	tr.SetContext("standard", 3)
	tr.Observe(wire.Event{Direction: "out", Bytes: []byte("REGISTER sip:127.0.0.1:15060 SIP/2.0\r\nContent-Length: 0\r\n\r\n")})
	snapshot := tr.Snapshot(0, 10)
	if snapshot.Records[0].ProtocolMode != "compatibility" || snapshot.Records[0].Revision != 2 || len(snapshot.Records[0].Compatibility) != 1 || len(snapshot.Records[1].Compatibility) != 0 {
		t.Fatal("trace lost mode or rewrote its provenance")
	}
	snapshot.Records[0].Compatibility[0] = "mutated"
	if tr.Snapshot(0, 10).Records[0].Compatibility[0] != "COMPAT-001" {
		t.Fatal("snapshot mutated original mode evidence")
	}
}

func TestAcceptedLegacyQueryMetadataKeepsRawBytesAndRedactsStructure(t *testing.T) {
	tr := New()
	tr.SetContext("compatibility", 2)
	identity := "34020000001180000001"
	tr.AddSecrets(identity)
	body := `<?xml version="1.0" encoding="GB2312"?><Query><CmdType>DeviceInfo</CmdType><SN>7</SN><DeviceID>` + identity + `</DeviceID></Query>`
	raw := []byte(fmt.Sprintf("MESSAGE sip:%s@127.0.0.1:15061 SIP/2.0\r\nVia: SIP/2.0/UDP 127.0.0.1:15060;branch=z9hG4bK-legacy\r\nCall-ID: legacy\r\nCSeq: 7 MESSAGE\r\nContent-Length: %d\r\n\r\n%s", identity, len(body), body))
	tr.Observe(wire.Event{Direction: "in", Bytes: raw})
	tr.MarkCompatibility("legacy", "7 MESSAGE", "different", "COMPAT-002")
	if len(tr.Snapshot(0, 10).Records[0].Compatibility) != 0 {
		t.Fatal("annotation matched a different SIP transaction")
	}
	tr.MarkCompatibility("legacy", "7 MESSAGE", "z9hG4bK-legacy", "COMPAT-002")
	r := tr.Snapshot(0, 10).Records[0]
	if len(r.Compatibility) != 1 || r.Structure["cmdType"] != "DeviceInfo" || r.Structure["body"] != "parsed" || r.Structure["target"] != strings.Repeat("*", len(identity)) {
		t.Fatal("legacy structure or its redaction was lost", r.Structure)
	}
	if strings.Contains(r.Text, identity) || !bytes.Equal(raw, tr.Raw()[0]) {
		t.Fatal("original bytes changed or a private identity escaped redaction")
	}
}
