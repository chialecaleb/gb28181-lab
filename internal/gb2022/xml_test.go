package gb2022

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestIndependentVectors(t *testing.T) {
	b, e := os.ReadFile("../../protocol-assets/m0-vectors.json")
	if e != nil {
		t.Fatal(e)
	}
	var v struct {
		XML []struct {
			Name, Text, Hex string
			Length          int
		}
	}
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	for _, v := range v.XML {
		t.Run(v.Name, func(t *testing.T) {
			raw, _ := hex.DecodeString(v.Hex)
			m, e := ParseMessage(raw)
			if e != nil {
				t.Fatal(e)
			}
			got, e := XML(m)
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(raw, got) || len(got) != v.Length {
				t.Fatalf("independent octets differ: %s", got)
			}
			s, e := Decode(raw)
			if e != nil || s != v.Text {
				t.Fatal("independent decode", e)
			}
		})
	}
}
func TestMalformedXML(t *testing.T) {
	good := `<Query><CmdType>DeviceInfo</CmdType><SN>1</SN><DeviceID>34020000001180000001</DeviceID></Query>`
	for _, s := range []string{strings.Replace(good, "<SN>1</SN>", "<SN>1</SN><SN>2</SN>", 1), strings.Replace(good, "<SN>1", "<SN>0", 1), strings.Replace(good, "DeviceInfo", "Unknown", 1), good + good, "<!DOCTYPE a SYSTEM 'file:///secret'>" + good, strings.Replace(good, "</Query>", "<Unknown>x</Unknown></Query>", 1), strings.Replace(good, "<SN>1</SN>", "<SN><x>1</x></SN>", 1), `<?xml version="1.0" encoding="UTF-8"?>` + good, strings.Replace(good, "Query", "Notify", 2)} {
		if _, e := ParseMessage([]byte(s)); e == nil {
			t.Fatalf("accepted malformed input %s", s)
		}
	}
	if _, e := ParseMessage(append([]byte(good), 0x81)); e == nil {
		t.Fatal("accepted invalid encoding")
	}
}

func TestLegacyQueryDeclarationIsExplicitAndDoesNotRewriteBytes(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="GB2312"?><Query><CmdType>DeviceInfo</CmdType><SN>7</SN><DeviceID>34020000001180000001</DeviceID></Query>`)
	original := bytes.Clone(body)
	if _, _, err := ParseIncomingQuery(body, false); err == nil {
		t.Fatal("standard mode accepted a legacy declaration")
	}
	message, rule, err := ParseIncomingQuery(body, true)
	if err != nil || rule != "COMPAT-002" || message.CmdType != "DeviceInfo" || message.SN != "7" {
		t.Fatal("explicit legacy query failed", err)
	}
	if !bytes.Equal(body, original) {
		t.Fatal("compatibility rewrote original XML bytes")
	}
	for _, bad := range [][]byte{
		bytes.Replace(body, []byte("GB2312"), []byte("UTF-8"), 1),
		bytes.Replace(body, []byte("</Query>"), []byte("<Unknown>1</Unknown></Query>"), 1),
		bytes.Replace(body, []byte("<SN>7</SN>"), []byte("<SN>7</SN><SN>8</SN>"), 1),
		append([]byte(`<?xml version="1.0" encoding="GB2312"?>`), body...),
		bytes.Replace(body, []byte("<Query>"), []byte("<!--\xd6\xd0\xce\xc4--><Query>"), 1),
		bytes.ReplaceAll(body, []byte("Query"), []byte("Response")),
	} {
		if _, _, err := ParseIncomingQuery(bad, true); err == nil {
			t.Fatal("compatibility weakened the supported schema or encoding boundary")
		}
	}
	standard := bytes.Replace(body, []byte("GB2312"), []byte("GB18030"), 1)
	if _, rule, err := ParseIncomingQuery(standard, true); err != nil || rule != "" {
		t.Fatal("ordinary query was mislabeled as a compatibility operation", err)
	}
}
func sample(n int) Item {
	return Item{DeviceID: fmt.Sprintf("3402000000131%07d", n+1), Name: "摄像机", Manufacturer: "Lab", Model: "M0", CivilCode: "340200", Address: "合成点位", ParentID: "34020000001180000001", RegisterWay: 1, Status: "ON", GrassrootsCode: "000000", PointType: 9}
}
func TestCatalogBoundaries(t *testing.T) {
	for _, count := range []int{0, 1, 3, 10000, 10001} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			items := make([]Item, count)
			for i := range items {
				items[i] = sample(i)
			}
			b, e := Batches("3", "34020000001180000001", items, 10000)
			if e != nil {
				t.Fatal(e)
			}
			if len(b) != max(1, (count+9999)/10000) {
				t.Fatal("batch boundary")
			}
			c := Catalog{SN: "3", Target: "34020000001180000001"}
			for j, raw := range b {
				m, e := ParseMessage(raw)
				if e != nil {
					t.Fatal(e)
				}
				done, e := c.Add(m)
				if e != nil || done != (j == len(b)-1) {
					t.Fatal("aggregation", e)
				}
			}
			if count == 0 && bytes.Contains(b[0], []byte("DeviceList")) {
				t.Fatal("empty list")
			}
		})
	}
	items := []Item{sample(0), sample(1)}
	b, e := Batches("4", "34020000001180000001", items, 1)
	if e != nil {
		t.Fatal(e)
	}
	m, _ := ParseMessage(b[0])
	c := Catalog{SN: "4", Target: m.DeviceID}
	if done, e := c.Add(m); e != nil || done {
		t.Fatal("partial cannot complete")
	}
	if _, e = c.Add(m); e == nil {
		t.Fatal("duplicate batch accepted")
	}
	bad := strings.Replace(string(b[0]), `Num="1"`, `Num="2"`, 1)
	if _, e = ParseMessage([]byte(bad)); e == nil {
		t.Fatal("incorrect Num accepted")
	}
}
