package transport

import (
	"bytes"
	"encoding/json"
	"github.com/chialecaleb/gb28181-lab/internal/app"
	"github.com/chialecaleb/gb28181-lab/internal/config"
	"io/fs"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHTTPBoundaryAndDesktopContract(t *testing.T) {
	s, e := app.Open(app.Options{Runtime: config.Runtime{DataDir: t.TempDir(), DeviceBind: "127.0.0.1:0", PlatformBind: "127.0.0.1:0", Transport: "UDP"}})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	var files fs.FS = fstest.MapFS{"index.html": {Data: []byte("workbench")}}
	h := NewHTTP(s, "127.0.0.1:18081", files)
	tests := []struct {
		name, remote, host, origin, token, method, path, body string
		status                                                int
	}{
		{"anonymous", "127.0.0.1:42", "127.0.0.1:18081", "", "", "GET", "/api/v1/snapshot", "", 401},
		{"rebind", "127.0.0.1:42", "evil.test", "", s.LocalToken(), "GET", "/api/v1/snapshot", "", 403},
		{"cross origin", "127.0.0.1:42", "127.0.0.1:18081", "https://evil.test", s.LocalToken(), "GET", "/api/v1/snapshot", "", 403},
		{"remote", "192.0.2.2:42", "127.0.0.1:18081", "", s.LocalToken(), "GET", "/api/v1/snapshot", "", 403},
		{"unknown field", "127.0.0.1:42", "127.0.0.1:18081", "", s.LocalToken(), "POST", "/api/v1/actions", `{"unknown":1}`, 400},
		{"trailing json", "127.0.0.1:42", "127.0.0.1:18081", "", s.LocalToken(), "POST", "/api/v1/actions", `{} {}`, 400},
		{"valid", "127.0.0.1:42", "127.0.0.1:18081", "", s.LocalToken(), "GET", "/api/v1/snapshot", "", 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, "http://"+tt.host+tt.path, bytes.NewBufferString(tt.body))
			r.RemoteAddr = tt.remote
			r.Header.Set("Origin", tt.origin)
			r.Header.Set("Authorization", "Bearer "+tt.token)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.status {
				t.Fatalf("status %d body %s", w.Code, w.Body.String())
			}
			if bytes.Contains(w.Body.Bytes(), []byte(s.LocalToken())) {
				t.Fatal("token exposed")
			}
			if tt.name == "valid" {
				var dto app.Snapshot
				if e := json.Unmarshal(w.Body.Bytes(), &dto); e != nil {
					t.Fatal(e)
				}
				desktop, e := NewDesktop(s).Snapshot()
				if e != nil || dto.Config.Revision != desktop.Config.Revision || dto.Device != desktop.Device {
					t.Fatal("host contract mismatch")
				}
			}
		})
	}
}
