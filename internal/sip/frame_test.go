package sip

import (
	"bytes"
	"strings"
	"testing"
)

func frameFixture() []byte {
	return []byte("MESSAGE sip:34020000001180000001@3402000000 SIP/2.0\r\nVia: SIP/2.0/TCP 127.0.0.1:15062;branch=z9hG4bK-fixture\r\nFrom: <sip:34020000002000000001@3402000000>;tag=x\r\nTo: <sip:34020000001180000001@3402000000>\r\nCall-ID: independent\r\nCSeq: 1 MESSAGE\r\nContent-Length: 4\r\n\r\n\xd6\xd0\xce\xc4")
}
func TestTCPEverySplitAndCoalescing(t *testing.T) {
	b := frameFixture()
	for i := 1; i < len(b); i++ {
		var f Framer
		a, e := f.Feed(b[:i])
		if e != nil || len(a) != 0 {
			t.Fatal("partial accepted", i, e)
		}
		a, e = f.Feed(append(bytes.Clone(b[i:]), b...))
		if e != nil || len(a) != 2 || !bytes.Equal(a[0], b) || !bytes.Equal(a[1], b) || f.Pending() != 0 {
			t.Fatal("framing", i, e)
		}
	}
}
func TestInvalidFrameLengthsAndInjection(t *testing.T) {
	b := string(frameFixture())
	for _, s := range []string{strings.Replace(b, "Content-Length: 4", "Content-Length: 4\r\nl: 4", 1), strings.Replace(b, "Content-Length: 4", "Content-Length: 16777216", 1), strings.Replace(b, "Content-Length: 4", "Content-Length: -1", 1), strings.Replace(b, "CSeq: 1 MESSAGE", "CSeq: 1 MESSAGE\nInjected: yes", 1), b[:len(b)-1], b + "x"} {
		if e := Datagram([]byte(s)); e == nil {
			t.Fatal("invalid wire accepted")
		}
	}
	var f Framer
	if _, e := f.Feed(bytes.Repeat([]byte("a"), MaxHeader+1)); e == nil {
		t.Fatal("unbounded headers")
	}
}
