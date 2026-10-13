package sip

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
)

const MaxHeader = 64 * 1024
const MaxMessage = 16 * 1024 * 1024

// FrameLength validates the wire envelope before sipgo sees it. A missing body
// returns the eventual frame size; no partial frame may reach the SIP parser.
func FrameLength(b []byte) (int, error) {
	end := bytes.Index(b, []byte("\r\n\r\n"))
	if end < 0 {
		if len(b) > MaxHeader {
			return 0, errors.New("SIP headers exceed limit")
		}
		return 0, nil
	}
	if end+4 > MaxHeader {
		return 0, errors.New("SIP headers exceed limit")
	}
	h := b[:end]
	for i, c := range h {
		if c == 0 || c >= 128 || c == '\n' && (i == 0 || h[i-1] != '\r') || c == '\r' && (i+1 >= len(h) || h[i+1] != '\n') {
			return 0, errors.New("invalid SIP header octets")
		}
	}
	lines := strings.Split(string(h), "\r\n")
	if len(lines) < 2 {
		return 0, errors.New("missing SIP headers")
	}
	seen := map[string]bool{}
	length := -1
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			return 0, errors.New("folded SIP headers unsupported in M0")
		}
		name, value, ok := strings.Cut(line, ":")
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.TrimSpace(value)
		if !ok || name == "" {
			return 0, errors.New("invalid SIP header")
		}
		switch name {
		case "l":
			name = "content-length"
		case "v":
			name = "via"
		case "f":
			name = "from"
		case "t":
			name = "to"
		case "i":
			name = "call-id"
		case "c":
			name = "content-type"
		case "m":
			name = "contact"
		}
		if name != "via" && seen[name] {
			return 0, errors.New("duplicate singleton SIP header")
		}
		seen[name] = true
		if name == "content-length" {
			if value == "" {
				return 0, errors.New("invalid Content-Length")
			}
			for _, c := range value {
				if c < '0' || c > '9' {
					return 0, errors.New("invalid Content-Length")
				}
			}
			n, e := strconv.Atoi(value)
			if e != nil || n > MaxMessage {
				return 0, errors.New("Content-Length exceeds limit")
			}
			length = n
		}
	}
	if length < 0 {
		return 0, errors.New("M0 requires Content-Length")
	}
	for _, key := range []string{"via", "from", "to", "call-id", "cseq"} {
		if !seen[key] {
			return 0, errors.New("missing required SIP header")
		}
	}
	n := end + 4 + length
	if n > MaxMessage {
		return 0, errors.New("SIP frame exceeds limit")
	}
	return n, nil
}

type Framer struct{ buffer []byte }

func (f *Framer) Feed(b []byte) ([][]byte, error) {
	if len(f.buffer)+len(b) > MaxMessage+32768 {
		return nil, errors.New("SIP stream exceeds limit")
	}
	f.buffer = append(f.buffer, b...)
	var frames [][]byte
	for len(f.buffer) > 0 {
		// CRLF transport keepalive is not a GB Keepalive business message.
		if bytes.HasPrefix(f.buffer, []byte("\r\n")) {
			f.buffer = f.buffer[2:]
			continue
		}
		n, e := FrameLength(f.buffer)
		if e != nil {
			f.buffer = nil
			return frames, e
		}
		if n == 0 || len(f.buffer) < n {
			break
		}
		frames = append(frames, bytes.Clone(f.buffer[:n]))
		f.buffer = f.buffer[n:]
	}
	if len(f.buffer) == 0 {
		f.buffer = nil
	}
	return frames, nil
}
func (f *Framer) Pending() int { return len(f.buffer) }
func Datagram(b []byte) error {
	n, e := FrameLength(b)
	if e != nil {
		return e
	}
	if n == 0 || n != len(b) {
		return errors.New("SIP datagram length mismatch")
	}
	return nil
}
