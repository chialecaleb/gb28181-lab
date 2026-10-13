package transport

import (
	"encoding/json"
	"github.com/chialecaleb/gb28181-lab/internal/app"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"
)

type HTTP struct {
	service *app.Service
	host    string
	static  http.Handler
}

func NewHTTP(s *app.Service, host string, assets fs.FS) http.Handler {
	return &HTTP{service: s, host: host, static: http.FileServer(http.FS(assets))}
}
func NewServer(addr string, h http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 * 1024}
}
func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() || r.Host != h.host {
		http.Error(w, "local host required", 403)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+h.host {
		http.Error(w, "origin rejected", 403)
		return
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "cross-site request rejected", 403)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "method not allowed", 405)
			return
		}
		h.static.ServeHTTP(w, r)
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if _, e := h.service.Snapshot(token); e != nil {
		h.reply(w, nil, e)
		return
	}
	if r.Method == "POST" || r.Method == "PUT" {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "application/json required", 415)
			return
		}
	}
	var value any
	var err error
	switch r.Method + " " + r.URL.Path {
	case "GET /api/v1/snapshot":
		value, err = h.service.Snapshot(token)
	case "GET /api/v1/traces":
		value, err = h.service.Traces(token, 0, 1000)
	case "GET /api/v1/evidence":
		value, err = h.service.Evidence(token)
	case "PUT /api/v1/config":
		var c app.Config
		if !decode(w, r, &c) {
			return
		}
		value, err = h.service.SaveConfig(token, c)
	case "POST /api/v1/actions":
		var a app.Action
		if !decode(w, r, &a) {
			return
		}
		value, err = h.service.Execute(token, a)
	case "POST /api/v1/cancel":
		var c struct {
			ID string `json:"id"`
		}
		if !decode(w, r, &c) {
			return
		}
		value, err = h.service.Cancel(token, c.ID)
	default:
		http.NotFound(w, r)
		return
	}
	h.reply(w, value, err)
}
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 128*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]any{"error": &app.Error{Code: "invalid", Message: "invalid JSON request"}})
		return false
	}
	return true
}
func (h *HTTP) reply(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		e := app.AsError(err)
		status := 400
		switch e.Code {
		case "unauthorized":
			status = 401
		case "conflict", "busy":
			status = 409
		case "not_found":
			status = 404
		case "internal", "storage":
			status = 500
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]any{"error": e})
		return
	}
	json.NewEncoder(w).Encode(value)
}
