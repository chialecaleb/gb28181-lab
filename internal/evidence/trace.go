package evidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chialecaleb/gb28181-lab/internal/gb2022"
	wire "github.com/chialecaleb/gb28181-lab/internal/sip"
)

type Record struct {
	Sequence       uint64            `json:"sequence"`
	At             time.Time         `json:"at"`
	Role           string            `json:"role"`
	Direction      string            `json:"direction"`
	Transport      string            `json:"transport"`
	Summary        string            `json:"summary"`
	OperationID    string            `json:"operationId,omitempty"`
	ProtocolMode   string            `json:"protocolMode"`
	Revision       int64             `json:"revision"`
	Compatibility  []string          `json:"compatibility"`
	Length         int               `json:"length"`
	SHA256         string            `json:"sha256"`
	Error          string            `json:"error,omitempty"`
	Text           string            `json:"text"`
	Hex            string            `json:"hex"`
	Redacted       bool              `json:"redacted"`
	Truncated      bool              `json:"truncated"`
	Structure      map[string]string `json:"structure"`
	raw            []byte
	transactionKey string
}
type Snapshot struct {
	Records      []Record `json:"records"`
	Dropped      uint64   `json:"dropped"`
	Bytes        int      `json:"bytes"`
	LastSequence uint64   `json:"lastSequence"`
	Limit        int      `json:"limit"`
	ByteLimit    int      `json:"byteLimit"`
}
type Trace struct {
	mu                sync.Mutex
	records           []Record
	bytes             int
	sequence, dropped uint64
	links             map[string]string
	linkOrder         []string
	secrets           [][]byte
	limit, byteLimit  int
	protocolMode      string
	revision          int64
}

func New() *Trace {
	return &Trace{links: map[string]string{}, limit: 1000, byteLimit: 8 * 1024 * 1024, protocolMode: "standard"}
}
func (t *Trace) SetContext(mode string, revision int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.protocolMode, t.revision = mode, revision
}
func (t *Trace) AddSecrets(values ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, v := range values {
		if len(v) >= 3 {
			t.secrets = append(t.secrets, []byte(v))
			if b, e := gb2022.Encode(v); e == nil && !bytes.Equal(b, []byte(v)) {
				t.secrets = append(t.secrets, b)
			}
		}
	}
}
func (t *Trace) Bind(key, op string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.links[key]; !ok {
		t.linkOrder = append(t.linkOrder, key)
	}
	t.links[key] = op
	for len(t.linkOrder) > 256 {
		k := t.linkOrder[0]
		t.linkOrder = t.linkOrder[1:]
		delete(t.links, k)
	}
}

var secretHeader = regexp.MustCompile(`(?im)^(?:Authorization|Proxy-Authorization|WWW-Authenticate|Proxy-Authenticate|Cookie|Set-Cookie):[^\r\n]*`)
var passwordXML = regexp.MustCompile(`(?is)<Password(?:\s[^>]*)?>.*?</Password>`)
var snField = regexp.MustCompile(`<SN>([0-9]+)</SN>`)
var cmdField = regexp.MustCompile(`<CmdType>([A-Za-z]+)</CmdType>`)
var viaBranch = regexp.MustCompile(`(?:^|;)branch=([^;\s]+)`)

func mask(b []byte, pattern *regexp.Regexp) {
	for _, pos := range pattern.FindAllIndex(b, -1) {
		for i := pos[0]; i < pos[1]; i++ {
			if b[i] != '\r' && b[i] != '\n' {
				b[i] = '*'
			}
		}
	}
}
func (t *Trace) redact(b []byte) ([]byte, bool) {
	out := bytes.Clone(b)
	mask(out, secretHeader)
	mask(out, passwordXML)
	secrets := append([][]byte{}, t.secrets...)
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	for _, s := range secrets {
		out = bytes.ReplaceAll(out, s, bytes.Repeat([]byte("*"), len(s)))
	}
	return out, !bytes.Equal(out, b)
}
func (t *Trace) Observe(e wire.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sequence++
	hash := sha256.Sum256(e.Bytes)
	raw := e.Bytes
	truncated := len(raw) > 256*1024
	if truncated {
		raw = raw[:256*1024]
	}
	r := Record{Sequence: t.sequence, At: e.At, Role: e.Role, Direction: e.Direction, Transport: e.Transport, Length: len(e.Bytes), SHA256: hex.EncodeToString(hash[:]), Error: e.Error, raw: bytes.Clone(raw), Truncated: truncated}
	r.ProtocolMode, r.Revision, r.Compatibility = t.protocolMode, t.revision, []string{}
	first, _, _ := bytes.Cut(raw, []byte("\r\n"))
	fields := strings.Fields(string(first))
	if t.protocolMode == "compatibility" && e.Direction == "out" && len(fields) == 3 && fields[0] == "REGISTER" && strings.Contains(fields[1], "@") {
		r.Compatibility = []string{"COMPAT-001"}
	}
	// Match only pre-bound operation keys. A late/unknown SN is never turned
	// into application success by trace association.
	var call, sequence, branch string
	head, _, _ := bytes.Cut(raw[:min(len(raw), wire.MaxHeader)], []byte("\r\n\r\n"))
	for _, line := range strings.Split(string(head), "\r\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(k) {
		case "call-id", "i":
			call = strings.TrimSpace(v)
		case "cseq":
			sequence = strings.TrimSpace(v)
		case "via", "v":
			if match := viaBranch.FindStringSubmatch(v); len(match) > 1 {
				branch = match[1]
			}
		}
	}
	r.OperationID = t.links["call:"+call]
	r.transactionKey = call + "\x00" + sequence + "\x00" + branch
	if r.OperationID == "" {
		sn, cmd := snField.FindSubmatch(raw), cmdField.FindSubmatch(raw)
		if len(sn) > 1 && len(cmd) > 1 {
			r.OperationID = t.links["sn:"+string(cmd[1])+":"+string(sn[1])]
		}
	}
	t.records = append(t.records, r)
	t.bytes += len(raw)
	for len(t.records) > t.limit || t.bytes > t.byteLimit {
		t.bytes -= len(t.records[0].raw)
		t.records[0] = Record{}
		t.records = t.records[1:]
		t.dropped++
	}
}

// MarkCompatibility adds interpretation metadata after the service accepts a
// legacy input. It never modifies the captured bytes or relabels another mode.
func (t *Trace) MarkCompatibility(call, sequence, branch, rule string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := call + "\x00" + sequence + "\x00" + branch
	for i := range t.records {
		r := &t.records[i]
		if r.Direction != "in" || r.ProtocolMode != "compatibility" || r.transactionKey != key || !bytes.HasPrefix(r.raw, []byte("MESSAGE ")) {
			continue
		}
		found := false
		for _, existing := range r.Compatibility {
			found = found || existing == rule
		}
		if !found {
			r.Compatibility = append(r.Compatibility, rule)
		}
	}
}
func (t *Trace) view(r Record) Record {
	r.Compatibility = append([]string{}, r.Compatibility...)
	b, redacted := t.redact(r.raw)
	r.Redacted = redacted
	r.Hex = hex.EncodeToString(b)
	text, e := gb2022.Decode(b)
	if e != nil {
		text = strings.ToValidUTF8(string(b), "�")
	}
	r.Text = text
	lines := strings.Split(text, "\r\n")
	if len(lines) > 0 {
		r.Summary = lines[0]
	}
	r.Structure = map[string]string{"encoding": "GB18030 wire / UTF-8 display", "body": "not parsed"}
	if i := bytes.Index(r.raw, []byte("\r\n\r\n")); i >= 0 && len(r.raw) > i+4 {
		m, err := gb2022.ParseMessage(r.raw[i+4:])
		for _, rule := range r.Compatibility {
			if rule == "COMPAT-002" {
				m, _, err = gb2022.ParseIncomingQuery(r.raw[i+4:], true)
				r.Structure["encoding"] = "GB2312-declared ASCII query / UTF-8 display; COMPAT-002"
			}
		}
		if err == nil {
			for key, value := range map[string]string{"root": m.Root, "cmdType": m.CmdType, "sn": m.SN, "target": m.DeviceID} {
				masked, _ := t.redact([]byte(value))
				r.Structure[key] = string(masked)
			}
			r.Structure["body"] = "parsed"
		}
	}
	r.raw = nil
	return r
}
func (t *Trace) Snapshot(after uint64, limit int) Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	if limit < 1 || limit > 1000 {
		limit = 100
	}
	s := Snapshot{Records: []Record{}, Dropped: t.dropped, Bytes: t.bytes, LastSequence: t.sequence, Limit: t.limit, ByteLimit: t.byteLimit}
	for _, r := range t.records {
		if r.Sequence > after {
			s.Records = append(s.Records, t.view(r))
			if len(s.Records) == limit {
				break
			}
		}
	}
	return s
}

// Raw is only used by the local test evidence writer. It is never bound to a
// browser/desktop/API method. Returned bytes remain subject to local handling.
func (t *Trace) Raw() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([][]byte, len(t.records))
	for i, r := range t.records {
		out[i] = bytes.Clone(r.raw)
	}
	return out
}
