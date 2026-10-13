package sip

import (
	"encoding/json"
	"github.com/icholy/digest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIndependentDigestVectors(t *testing.T) {
	b, e := os.ReadFile("../../protocol-assets/m0-vectors.json")
	if e != nil {
		t.Fatal(e)
	}
	var v struct {
		Digest  struct{ Username, Realm, Password, Method, URI, Nonce, Cnonce, Legacy, Auth string }
		RFC2617 struct{ Username, Realm, Password, Method, URI, Nonce, Cnonce, Response string }
	}
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	d := v.Digest
	for _, qop := range []string{"", "auth"} {
		c := digest.Challenge{Realm: d.Realm, Nonce: d.Nonce, Algorithm: "MD5"}
		if qop != "" {
			c.QOP = []string{qop}
		}
		header, e := Authorize(c.String(), d.Realm, d.Username, d.Password, d.Method, d.URI, 1, d.Cnonce)
		if e != nil {
			t.Fatal(e)
		}
		cred, _ := digest.ParseCredentials(header)
		want := d.Legacy
		if qop != "" {
			want = d.Auth
		}
		if cred.Response != want {
			t.Fatal("independent SIP digest mismatch")
		}
	}
	r := v.RFC2617
	c := digest.Challenge{Realm: r.Realm, Nonce: r.Nonce, QOP: []string{"auth"}}
	h, e := Authorize(c.String(), r.Realm, r.Username, r.Password, r.Method, r.URI, 1, r.Cnonce)
	if e != nil {
		t.Fatal(e)
	}
	cred, _ := digest.ParseCredentials(h)
	if cred.Response != r.Response {
		t.Fatal("RFC 2617 vector mismatch")
	}
}
func TestDigestRejectionAndExpiry(t *testing.T) {
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	a := NewAuthenticator("lab", "device", "synthetic", func() time.Time { return now })
	c, _ := a.Challenge(false)
	wrong, _ := Authorize(c, "lab", "device", "wrong-password", "REGISTER", "sip:lab", 1, "wrong-secret-vector")
	if _, err := a.Verify(wrong, "REGISTER", "sip:lab"); err == nil {
		t.Fatal("wrong password accepted")
	}
	h, e := Authorize(c, "lab", "device", "synthetic", "REGISTER", "sip:lab", 1, "fixed-offline-vector")
	if e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(h, `realm="lab"`, `realm="wrong"`, 1), strings.Replace(h, `uri="sip:lab"`, `uri="sip:elsewhere"`, 1), h + `, nonce="again"`} {
		if _, e = a.Verify(bad, "REGISTER", "sip:lab"); e == nil {
			t.Fatal("bad credential accepted")
		}
	}
	if _, e = a.Verify(h, "REGISTER", "sip:lab"); e != nil {
		t.Fatal(e)
	}
	if _, e = a.Verify(h, "REGISTER", "sip:lab"); e == nil {
		t.Fatal("replay accepted")
	}
	now = now.Add(121 * time.Second)
	stale, e := a.Verify(h, "REGISTER", "sip:lab")
	if e == nil || !stale {
		t.Fatal("expired nonce accepted")
	}
}
