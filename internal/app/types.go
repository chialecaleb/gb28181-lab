package app

import (
	"errors"
	"fmt"
	"github.com/chialecaleb/gb28181-lab/internal/device"
	"github.com/chialecaleb/gb28181-lab/internal/platform"
	"time"
)

type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	FieldPath string `json:"fieldPath,omitempty"`
	Retryable bool   `json:"retryable"`
}

func (e *Error) Error() string           { return e.Message }
func failure(code, message string) error { return &Error{Code: code, Message: message} }
func AsError(e error) *Error {
	var a *Error
	if errors.As(e, &a) {
		return a
	}
	return &Error{Code: "internal", Message: "operation failed; inspect local diagnostics"}
}

type Binding struct {
	PlatformID string `json:"platformId"`
	Domain     string `json:"domain"`
	Target     string `json:"target"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	AuthMode   string `json:"authMode"`
}

const ModeStandard = "standard"
const ModeCompatibility = "compatibility"

type Config struct {
	SchemaVersion    string            `json:"schemaVersion"`
	Revision         int64             `json:"revision"`
	ProtocolMode     string            `json:"protocolMode"`
	Device           device.Definition `json:"device"`
	BindingID        string            `json:"bindingId"`
	Transport        string            `json:"transport"`
	Expires          int               `json:"expires"`
	HeartbeatSeconds int               `json:"heartbeatSeconds"`
	RetrySeconds     int               `json:"retrySeconds"`
	CatalogBatchSize int               `json:"catalogBatchSize"`
}
type Action struct {
	Kind             string `json:"kind"`
	Target           string `json:"target,omitempty"`
	IdempotencyKey   string `json:"idempotencyKey"`
	ExpectedRevision int64  `json:"expectedRevision"`
	TimeoutSeconds   int    `json:"timeoutSeconds,omitempty"`
	StartTime        string `json:"startTime,omitempty"`
	EndTime          string `json:"endTime,omitempty"`
}
type Check struct {
	Layer  string `json:"layer"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}
type Operation struct {
	ID            string       `json:"id"`
	Kind          string       `json:"kind"`
	Target        string       `json:"target"`
	BindingID     string       `json:"bindingId"`
	ProtocolMode  string       `json:"protocolMode"`
	Compatibility []string     `json:"compatibility"`
	Revision      int64        `json:"revision"`
	Generation    uint64       `json:"generation"`
	State         string       `json:"state"`
	StartedAt     time.Time    `json:"startedAt"`
	FinishedAt    time.Time    `json:"finishedAt"`
	SIPStatus     int          `json:"sipStatus"`
	Business      string       `json:"business"`
	SN            string       `json:"sn,omitempty"`
	Error         *Error       `json:"error,omitempty"`
	Result        any          `json:"result,omitempty"`
	Checks        []Check      `json:"checks"`
	Before        device.State `json:"before"`
	After         device.State `json:"after"`
	Cleanup       string       `json:"cleanup"`
}
type Snapshot struct {
	SchemaVersion      string                `json:"schemaVersion"`
	At                 time.Time             `json:"at"`
	Config             Config                `json:"config"`
	Device             device.State          `json:"device"`
	Platform           platform.Registration `json:"platform"`
	ExternalConfigured bool                  `json:"externalConfigured"`
	Operations         []Operation           `json:"operations"`
	ActiveOperations   int                   `json:"activeOperations"`
	Resources          Resources             `json:"resources"`
	Capabilities       []Check               `json:"capabilities"`
}
type Resources struct {
	ClientTransactions     int64 `json:"clientTransactions"`
	ServerTransactions     int64 `json:"serverTransactions"`
	AcceptedTCPConnections int64 `json:"acceptedTcpConnections"`
	PendingQueries         int   `json:"pendingQueries"`
	Listeners              int   `json:"listeners"`
}

func (o Operation) clone() Operation {
	o.Checks = append([]Check{}, o.Checks...)
	o.Compatibility = append([]string{}, o.Compatibility...)
	return o
}
func supported(kind string) bool {
	switch kind {
	case "register", "unregister", "keepalive", "deviceinfo", "catalog":
		return true
	}
	return false
}
func kindCmd(kind string) string {
	if kind == "deviceinfo" {
		return "DeviceInfo"
	}
	if kind == "catalog" {
		return "Catalog"
	}
	return "Keepalive"
}
func operationChecks(op Operation) []Check {
	sipStatus := "unknown"
	if op.SIPStatus >= 200 && op.SIPStatus < 300 {
		sipStatus = "passed"
	} else if op.SIPStatus >= 300 {
		sipStatus = "failed"
	}
	state := "unknown"
	if op.State == "completed" {
		state = "passed"
	} else if op.State == "failed" || op.State == "cancelled" {
		state = "failed"
	}
	checks := []Check{{"sip", sipStatus, fmt.Sprintf("SIP status %d", op.SIPStatus)}, {"business", op.Business, "Independent MESSAGE response for DeviceInfo/Catalog"}, {"state", state, op.State}, {"media", "not-run", "M0 has no media implementation"}, {"security", "not-run", "Digest is not GB35114 authentication"}}
	if op.Kind == "register" || op.Kind == "unregister" {
		status, detail := "passed", "RFC3261 10.2: REGISTER Request-URI has no userinfo"
		if len(op.Compatibility) > 0 {
			status, detail = "failed", "COMPAT-001: explicit REGISTER userinfo exception; interoperability is not RFC3261 conformity"
		}
		if op.SIPStatus == 0 {
			status, detail = "not-run", "No completed SIP exchange; inspect the wire trace for any transmitted request"
		}
		checks = append(checks, Check{"register-uri", status, detail})
	}
	return checks
}
