package sip

import (
	"crypto/subtle"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/chialecaleb/gb28181-lab/internal/storage"
	"github.com/icholy/digest"
)

// GuardDigest adds multiplicity/size checks absent from the upstream parameter
// parser. Error strings deliberately contain no header values or secrets.
func GuardDigest(s string) error {
	if len(s) > 4096 || !strings.HasPrefix(s, "Digest ") || strings.ContainsAny(s, "\r\n\x00") {
		return errors.New("invalid Digest header")
	}
	seen := map[string]bool{}
	parts := []string{}
	start := 7
	quote, escape := false, false
	for i := 7; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' && quote {
			escape = true
			continue
		}
		if c == '"' {
			quote = !quote
		}
		if c == ',' && !quote {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if quote || escape {
		return errors.New("invalid Digest quoting")
	}
	parts = append(parts, s[start:])
	for _, p := range parts {
		k, _, ok := strings.Cut(strings.TrimSpace(p), "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" || seen[k] {
			return errors.New("duplicate or invalid Digest parameter")
		}
		seen[k] = true
	}
	return nil
}
func Authorize(challenge, realm, user, password, method, uri string, count int, cnonce string) (string, error) {
	if e := GuardDigest(challenge); e != nil {
		return "", e
	}
	c, e := digest.ParseChallenge(challenge)
	if e != nil || c.Realm != realm || c.Nonce == "" || c.Userhash || c.Charset != "" || (c.Algorithm != "" && !strings.EqualFold(c.Algorithm, "MD5")) {
		return "", errors.New("unsupported Digest challenge or realm mismatch")
	}
	if len(c.QOP) > 0 {
		found := false
		for _, q := range c.QOP {
			if strings.TrimSpace(q) == "auth" {
				found = true
			}
		}
		if !found {
			return "", errors.New("unsupported Digest qop")
		}
		c.QOP = []string{"auth"}
	}
	if cnonce == "" {
		cnonce = storage.RandomID()
	}
	if count < 1 {
		count = 1
	}
	cred, e := digest.Digest(c, digest.Options{Method: method, URI: uri, Username: user, Password: password, Count: count, Cnonce: cnonce})
	if e != nil {
		return "", errors.New("Digest calculation failed")
	}
	return cred.String(), nil
}

type nonceState struct {
	expires    time.Time
	opaque     string
	nc         int
	legacyUsed bool
}
type Authenticator struct {
	mu                    sync.Mutex
	realm, user, password string
	nonces                map[string]nonceState
	now                   func() time.Time
}

func NewAuthenticator(realm, user, password string, now func() time.Time) *Authenticator {
	if now == nil {
		now = time.Now
	}
	return &Authenticator{realm: realm, user: user, password: password, now: now, nonces: map[string]nonceState{}}
}
func (a *Authenticator) Challenge(stale bool) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	for n, v := range a.nonces {
		if !now.Before(v.expires) {
			delete(a.nonces, n)
		}
	}
	if len(a.nonces) >= 128 {
		return "", errors.New("authentication challenge budget exhausted")
	}
	n, o := storage.RandomID(), storage.RandomID()
	a.nonces[n] = nonceState{expires: now.Add(2 * time.Minute), opaque: o}
	return (&digest.Challenge{Realm: a.realm, Nonce: n, Opaque: o, Algorithm: "MD5", QOP: []string{"auth"}, Stale: stale}).String(), nil
}

// Verify returns stale separately. Only a valid digest advances replay state.
func (a *Authenticator) Verify(header, method, uri string) (stale bool, err error) {
	if e := GuardDigest(header); e != nil {
		return false, e
	}
	c, e := digest.ParseCredentials(header)
	if e != nil || c.Username != a.user || c.Realm != a.realm || c.URI != uri || c.Userhash || (c.Algorithm != "" && !strings.EqualFold(c.Algorithm, "MD5")) {
		return false, errors.New("Digest identity, realm or URI rejected")
	}
	if c.QOP != "" && c.QOP != "auth" {
		return false, errors.New("unsupported Digest qop")
	}
	if c.QOP == "auth" && (c.Cnonce == "" || c.Nc < 1) {
		return false, errors.New("invalid Digest replay fields")
	}
	if c.QOP == "" && (c.Cnonce != "" || c.Nc != 0) {
		return false, errors.New("legacy Digest cannot contain qop fields")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	n, ok := a.nonces[c.Nonce]
	if !ok || !a.now().Before(n.expires) {
		return true, errors.New("Digest nonce expired or unknown")
	}
	if c.Opaque != n.opaque {
		return false, errors.New("Digest opaque rejected")
	}
	chal := &digest.Challenge{Realm: a.realm, Nonce: c.Nonce, Opaque: n.opaque, Algorithm: "MD5"}
	if c.QOP != "" {
		chal.QOP = []string{"auth"}
	}
	want, e := digest.Digest(chal, digest.Options{Method: method, URI: uri, Username: a.user, Password: a.password, Count: c.Nc, Cnonce: c.Cnonce})
	if e != nil || subtle.ConstantTimeCompare([]byte(strings.ToLower(c.Response)), []byte(want.Response)) != 1 {
		return false, errors.New("Digest credentials rejected")
	}
	if (c.QOP == "auth" && c.Nc <= n.nc) || (c.QOP == "" && n.legacyUsed) {
		return false, errors.New("Digest replay rejected")
	}
	if c.QOP == "auth" {
		n.nc = c.Nc
	} else {
		n.legacyUsed = true
	}
	a.nonces[c.Nonce] = n
	return false, nil
}
