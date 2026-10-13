package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiteralAndErrors(t *testing.T) {
	v, e := Parse([]byte("\ufeff# comment\r\nLAB_DATA_DIR='中文/C:\\x # $HOME'\r\nTEST_LIVEGBS_PASSWORD=\"a#${X}\\z\"\n"))
	if e != nil || v["LAB_DATA_DIR"] != "中文/C:\\x # $HOME" || v["TEST_LIVEGBS_PASSWORD"] != "a#${X}\\z" {
		t.Fatalf("literal parsing failed: %v", e)
	}
	for _, s := range []string{"A=x\nA=secret", "export A=secret", "A='secret", "A=\x00secret", "A=\xff"} {
		_, e := Parse([]byte(s))
		if e == nil || strings.Contains(e.Error(), "secret") {
			t.Fatal("invalid/sensitive diagnostic")
		}
	}
}

func TestPrecedenceAndWhitelist(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, ".env"), []byte("LAB_SIP_TRANSPORT=TCP\nLAB_DATA_DIR=base"), 0600)
	os.WriteFile(filepath.Join(d, ".env.local"), []byte("LAB_DATA_DIR=local\nTEST_LIVEGBS_PASSWORD=never-return"), 0600)
	lookup := func(k string) (string, bool) {
		if k == "LAB_DATA_DIR" {
			return "process", true
		}
		return "", false
	}
	v, e := Load(d, lookup)
	if e != nil || v.DataDir != filepath.Join(d, "process") || v.Transport != "TCP" {
		t.Fatal("precedence", e)
	}
	_, e = Load(d, func(k string) (string, bool) { return "", k == "LAB_DATA_DIR" })
	if e == nil {
		t.Fatal("empty must override")
	}
	os.WriteFile(filepath.Join(d, ".env.local"), []byte("UNKNOWN=secret"), 0600)
	_, e = Load(d, lookup)
	if e == nil || strings.Contains(e.Error(), "secret") {
		t.Fatal("unknown key")
	}
}

func TestEndpointIsolation(t *testing.T) {
	for _, s := range []string{"0.0.0.0:80", "localhost:80", "[::1]:80", "127.0.0.1:0", "127.0.0.1:65536"} {
		if _, e := LocalEndpoint(s); e == nil {
			t.Fatal("accepted", s)
		}
	}
	if _, e := LocalEndpoint("127.0.0.1:18081"); e != nil {
		t.Fatal(e)
	}
}
