// Package config implements the literal local dotenv contract. TEST_* values
// are recognised for file validation, but are never returned to the application.
package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

type Runtime struct {
	HTTPBind, DeviceBind, PlatformBind, Transport, DataDir, EvidenceDir string
	StandardsRoot                                                       string
}

var known = strings.Fields(`LAB_PROFILE LAB_HTTP_BIND LAB_DEVICE_SIP_BIND LAB_PLATFORM_SIP_BIND LAB_WEB_DEV_BIND LAB_SIP_TRANSPORT LAB_DATA_DIR LAB_EVIDENCE_DIR GB28181_STANDARDS_ROOT GB28181_REFERENCE_ROOT TEST_LIVEGBS_HTTP_URL TEST_LIVEGBS_USERNAME TEST_LIVEGBS_PASSWORD TEST_LIVEGBS_SIP_HOST TEST_LIVEGBS_SIP_PORT TEST_LIVEGBS_SIP_ID TEST_LIVEGBS_SIP_REALM TEST_LIVEGBS_SIP_AUTH_MODE TEST_LIVEGBS_SIP_USERNAME TEST_LIVEGBS_SIP_PASSWORD TEST_LIVEGBS_SOURCE_ROOT TEST_LIVEGBS_API_DOC_URL`)
var keyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func Parse(data []byte) (map[string]string, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(data) {
		return nil, errors.New("dotenv is not UTF-8")
	}
	values := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || !keyPattern.MatchString(key) {
			return nil, fmt.Errorf("invalid dotenv assignment at line %d", n)
		}
		if _, ok := values[key]; ok {
			return nil, fmt.Errorf("duplicate dotenv key at line %d", n)
		}
		if strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
			if len(value) < 2 || value[len(value)-1] != value[0] {
				return nil, fmt.Errorf("unclosed quote at line %d", n)
			}
			value = value[1 : len(value)-1]
		}
		if strings.ContainsRune(value, 0) {
			return nil, fmt.Errorf("invalid dotenv value at line %d", n)
		}
		values[key] = value
	}
	if scanner.Err() != nil {
		return nil, errors.New("dotenv line exceeds size limit")
	}
	return values, nil
}

func Load(root string, lookup func(string) (string, bool)) (Runtime, error) {
	defaults := map[string]string{"LAB_PROFILE": "local-test", "LAB_HTTP_BIND": "127.0.0.1:18081", "LAB_DEVICE_SIP_BIND": "127.0.0.1:15061", "LAB_PLATFORM_SIP_BIND": "127.0.0.1:15062", "LAB_WEB_DEV_BIND": "127.0.0.1:5173", "LAB_SIP_TRANSPORT": "UDP", "LAB_DATA_DIR": ".local/runtime", "LAB_EVIDENCE_DIR": "artifacts/m0"}
	allow := make(map[string]bool)
	for _, k := range known {
		allow[k] = true
	}
	for _, name := range []string{".env", ".env.local"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return Runtime{}, errors.New("cannot read local configuration")
		}
		values, err := Parse(data)
		if err != nil {
			return Runtime{}, err
		}
		for k, v := range values {
			if !allow[k] {
				return Runtime{}, errors.New("unknown dotenv key; compare .env.example")
			}
			if strings.HasPrefix(k, "LAB_") || k == "GB28181_STANDARDS_ROOT" {
				defaults[k] = v
			}
		}
	}
	for _, k := range known {
		if strings.HasPrefix(k, "LAB_") || k == "GB28181_STANDARDS_ROOT" {
			if v, ok := lookup(k); ok {
				defaults[k] = v
			}
		}
	}
	if defaults["LAB_PROFILE"] != "local-test" {
		return Runtime{}, errors.New("LAB_PROFILE must be local-test")
	}
	if defaults["LAB_SIP_TRANSPORT"] != "UDP" && defaults["LAB_SIP_TRANSPORT"] != "TCP" {
		return Runtime{}, errors.New("invalid LAB_SIP_TRANSPORT")
	}
	ports := map[uint16]bool{}
	for _, k := range []string{"LAB_HTTP_BIND", "LAB_DEVICE_SIP_BIND", "LAB_PLATFORM_SIP_BIND", "LAB_WEB_DEV_BIND"} {
		addr, err := LocalEndpoint(defaults[k])
		if err != nil {
			return Runtime{}, fmt.Errorf("invalid %s: requires loopback IPv4 and nonzero port", k)
		}
		if ports[addr.Port()] {
			return Runtime{}, errors.New("local ports must be distinct")
		}
		ports[addr.Port()] = true
	}
	path := func(key string) string {
		p := defaults[key]
		if p == "" {
			return ""
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		return filepath.Clean(p)
	}
	if defaults["LAB_DATA_DIR"] == "" || defaults["LAB_EVIDENCE_DIR"] == "" {
		return Runtime{}, errors.New("missing runtime or evidence directory")
	}
	return Runtime{HTTPBind: defaults["LAB_HTTP_BIND"], DeviceBind: defaults["LAB_DEVICE_SIP_BIND"], PlatformBind: defaults["LAB_PLATFORM_SIP_BIND"], Transport: defaults["LAB_SIP_TRANSPORT"], DataDir: path("LAB_DATA_DIR"), EvidenceDir: path("LAB_EVIDENCE_DIR"), StandardsRoot: path("GB28181_STANDARDS_ROOT")}, nil
}

func LocalEndpoint(value string) (netip.AddrPort, error) {
	a, e := netip.ParseAddrPort(value)
	if e != nil || !a.Addr().Is4() || !a.Addr().IsLoopback() || a.Port() == 0 {
		return netip.AddrPort{}, errors.New("requires loopback IPv4 endpoint")
	}
	return a, nil
}
