// Package gb2022 implements the reviewed M0 MANSCDP subset. Raw GB18030
// bytes and display UTF-8 are distinct; no parser output is used as wire evidence.
package gb2022

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const MaxBody = 16 * 1024 * 1024
const ContentType = "Application/MANSCDP+xml"
const RuleVersion = "gb2022-m0-2"

var digits = regexp.MustCompile(`^[0-9]+$`)
var declaration = regexp.MustCompile(`(?i)encoding\s*=\s*['"]([^'"]+)['"]`)

func ValidID(s string) bool { return len(s) == 20 && digits.MatchString(s) }
func ValidSN(s string) bool {
	return len(s) > 0 && len(s) <= 64 && digits.MatchString(s) && strings.TrimLeft(s, "0") != ""
}
func ValidText(s string) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= 256 && !strings.ContainsAny(s, "\x00\r\n\t")
}

func Encode(s string) ([]byte, error) {
	if !utf8.ValidString(s) {
		return nil, errors.New("invalid UTF-8 source")
	}
	b, e := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(s))
	if e != nil {
		return nil, errors.New("GB18030 encoding failed")
	}
	return b, nil
}
func Decode(b []byte) (string, error) {
	if len(b) > MaxBody {
		return "", errors.New("body exceeds resource limit")
	}
	d, e := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
	if e != nil {
		return "", errors.New("invalid GB18030")
	}
	r, e := simplifiedchinese.GB18030.NewEncoder().Bytes(d)
	if e != nil || !bytes.Equal(r, b) {
		return "", errors.New("invalid or noncanonical GB18030")
	}
	return string(d), nil
}

type Node struct {
	Name     string
	Text     string
	Attrs    map[string]string
	Children []*Node
}

func (n *Node) Child(name string) *Node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}
func (n *Node) Value(name string) string {
	if c := n.Child(name); c != nil {
		return c.Text
	}
	return ""
}

func Parse(b []byte) (*Node, error) {
	return parse(b, false)
}

func parse(b []byte, legacyASCII bool) (*Node, error) {
	s, e := Decode(b)
	if e != nil {
		return nil, e
	}

	encodingAllowed := func(name string) bool {
		return strings.EqualFold(name, "GB18030") || (legacyASCII && strings.EqualFold(name, "GB2312"))
	}
	d := xml.NewDecoder(strings.NewReader(s))
	d.CharsetReader = func(name string, r io.Reader) (io.Reader, error) {
		if !encodingAllowed(name) {
			return nil, errors.New("unsupported XML encoding")
		}
		return r, nil
	}
	var root *Node
	var stack []*Node
	elements := 0
	declarationSeen := false
	for {
		t, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, errors.New("invalid XML")
		}
		switch t := t.(type) {
		case xml.StartElement:
			elements++
			if elements > 300000 || len(stack) >= 12 || t.Name.Space != "" {
				return nil, errors.New("XML resource or namespace limit")
			}
			n := &Node{Name: t.Name.Local, Attrs: map[string]string{}}
			for _, a := range t.Attr {
				if a.Name.Space != "" {
					return nil, errors.New("unsupported XML attribute")
				}
				if _, ok := n.Attrs[a.Name.Local]; ok {
					return nil, errors.New("duplicate XML attribute")
				}
				n.Attrs[a.Name.Local] = a.Value
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, errors.New("multiple XML roots")
				}
				root = n
			} else {
				p := stack[len(stack)-1]
				p.Children = append(p.Children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, errors.New("invalid XML nesting")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(string(t)) != "" {
					return nil, errors.New("text outside XML root")
				}
			} else {
				n := stack[len(stack)-1]
				n.Text += string(t)
				if len(n.Text) > 64*1024 {
					return nil, errors.New("XML text exceeds limit")
				}
			}
		case xml.Directive:
			return nil, errors.New("XML directives and entities are not supported")
		case xml.ProcInst:
			if matches := declaration.FindSubmatch(t.Inst); len(matches) > 0 && !encodingAllowed(string(matches[1])) {
				return nil, errors.New("XML declaration must match GB18030 wire encoding")
			}
			if t.Target != "xml" || root != nil || declarationSeen {
				return nil, errors.New("unsupported XML instruction")
			}
			declarationSeen = true
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, errors.New("missing XML root")
	}
	return root, nil
}

// Fields enforces sequence, multiplicity and leaf structure. '?' is optional,
// '*' repeatable; required scalar fields may be empty only when their caller
// permits that value. The schema is independent of Go XML struct defaults.
func fields(n *Node, spec ...string) error {
	if strings.TrimSpace(n.Text) != "" || len(n.Attrs) > 0 {
		return errors.New("unexpected XML text or attribute")
	}
	pos := 0
	for _, f := range spec {
		optional := strings.HasSuffix(f, "?") || strings.HasSuffix(f, "*")
		repeat := strings.HasSuffix(f, "*")
		name := strings.TrimRight(f, "?*")
		count := 0
		for pos < len(n.Children) && n.Children[pos].Name == name {
			count++
			pos++
			if !repeat {
				break
			}
		}
		if count == 0 && !optional {
			return fmt.Errorf("missing %s", name)
		}
	}
	if pos != len(n.Children) {
		return errors.New("unknown, duplicated or out-of-order XML field")
	}
	return nil
}
func scalar(n *Node) error {
	if len(n.Children) > 0 || len(n.Attrs) > 0 {
		return errors.New("invalid scalar XML field")
	}
	return nil
}

type Message struct {
	Root         string   `json:"root"`
	CmdType      string   `json:"cmdType"`
	SN           string   `json:"sn"`
	DeviceID     string   `json:"deviceId"`
	Status       string   `json:"status,omitempty"`
	Result       string   `json:"result,omitempty"`
	DeviceName   string   `json:"deviceName,omitempty"`
	Manufacturer string   `json:"manufacturer,omitempty"`
	Model        string   `json:"model,omitempty"`
	Firmware     string   `json:"firmware,omitempty"`
	Channel      *int     `json:"channel,omitempty"`
	SumNum       int      `json:"sumNum"`
	Items        []Item   `json:"items,omitempty"`
	StartTime    string   `json:"startTime,omitempty"`
	EndTime      string   `json:"endTime,omitempty"`
	Faults       []string `json:"faults,omitempty"`
}
type Item struct {
	DeviceID       string `json:"deviceId"`
	Name           string `json:"name"`
	Manufacturer   string `json:"manufacturer"`
	Model          string `json:"model"`
	CivilCode      string `json:"civilCode"`
	Address        string `json:"address"`
	Parental       int    `json:"parental"`
	ParentID       string `json:"parentId"`
	RegisterWay    int    `json:"registerWay"`
	Secrecy        int    `json:"secrecy"`
	Status         string `json:"status"`
	GrassrootsCode string `json:"grassrootsCode,omitempty"`
	PointType      int    `json:"pointType,omitempty"`
}

func ParseMessage(b []byte) (Message, error) {
	return parseMessage(b, false)
}

// ParseIncomingQuery admits the observed 2016 declaration only for the ASCII
// subset shared by GB2312 and GB18030. M0 query fields need no legacy text
// transcoder. Original bytes and all schema checks remain unchanged.
func ParseIncomingQuery(b []byte, compatibility bool) (Message, string, error) {
	legacy := compatibility && legacyASCIIQuery(b)
	m, err := parseMessage(b, legacy)
	if err != nil {
		return m, "", err
	}
	if m.Root != "Query" {
		return m, "", errors.New("incoming message is not a query")
	}
	if legacy {
		return m, "COMPAT-002", nil
	}
	return m, "", nil
}

func legacyASCIIQuery(b []byte) bool {
	if len(b) > MaxBody {
		return false
	}
	for _, value := range b {
		if value >= 128 {
			return false
		}
	}
	head := bytes.TrimSpace(b)
	end := bytes.Index(head, []byte("?>"))
	if !bytes.HasPrefix(head, []byte("<?xml")) || end < 0 {
		return false
	}
	encoding := declaration.FindSubmatch(head[:end])
	return len(encoding) > 1 && strings.EqualFold(string(encoding[1]), "GB2312")
}

func parseMessage(b []byte, legacyASCII bool) (Message, error) {
	n, e := parse(b, legacyASCII)
	if e != nil {
		return Message{}, e
	}
	m := Message{Root: n.Name, CmdType: n.Value("CmdType"), SN: n.Value("SN"), DeviceID: n.Value("DeviceID")}
	if !ValidSN(m.SN) || !ValidID(m.DeviceID) {
		return m, errors.New("invalid SN or device identity")
	}
	for _, c := range n.Children {
		if c.Name != "Info" && c.Name != "DeviceList" {
			if e = scalar(c); e != nil {
				return m, e
			}
		}
	}
	switch {
	case m.Root == "Query" && m.CmdType == "DeviceInfo":
		e = fields(n, "CmdType", "SN", "DeviceID")
	case m.Root == "Query" && m.CmdType == "Catalog":
		e = fields(n, "CmdType", "SN", "DeviceID", "StartTime?", "EndTime?")
		m.StartTime = n.Value("StartTime")
		m.EndTime = n.Value("EndTime")
		if e == nil {
			_, _, e = TimeRange(m.StartTime, m.EndTime)
		}
	case m.Root == "Notify" && m.CmdType == "Keepalive":
		e = fields(n, "CmdType", "SN", "DeviceID", "Status", "Info")
		m.Status = n.Value("Status")
		if e == nil {
			info := n.Child("Info")
			e = fields(info, "DeviceID*")
			for _, c := range info.Children {
				if scalar(c) != nil || !ValidID(c.Text) {
					return m, errors.New("invalid fault identity")
				}
				m.Faults = append(m.Faults, c.Text)
			}
			if len(m.Faults) > 32 {
				return m, errors.New("fault list exceeds M0 budget")
			}
		}
		if m.Status != "OK" && m.Status != "ERROR" {
			return m, errors.New("invalid Keepalive status")
		}
	case m.Root == "Response" && m.CmdType == "DeviceInfo":
		e = fields(n, "CmdType", "SN", "DeviceID", "DeviceName?", "Result", "Manufacturer?", "Model?", "Firmware?", "Channel?", "ExtraInfo*")
		m.Result = n.Value("Result")
		m.DeviceName = n.Value("DeviceName")
		m.Manufacturer = n.Value("Manufacturer")
		m.Model = n.Value("Model")
		m.Firmware = n.Value("Firmware")
		if m.Result != "OK" && m.Result != "ERROR" {
			return m, errors.New("invalid DeviceInfo result")
		}
		if c := n.Child("Channel"); c != nil {
			v, err := nonnegative(c.Text)
			if err != nil {
				return m, err
			}
			m.Channel = &v
		}
	case m.Root == "Response" && m.CmdType == "Catalog":
		e = fields(n, "CmdType", "SN", "DeviceID", "SumNum", "DeviceList?", "ExtraInfo*")
		if e != nil {
			return m, e
		}
		m.SumNum, e = nonnegative(n.Value("SumNum"))
		if e != nil {
			return m, e
		}
		list := n.Child("DeviceList")
		if m.SumNum == 0 {
			if list != nil {
				return m, errors.New("empty Catalog must omit DeviceList")
			}
		} else {
			if list == nil || len(list.Attrs) != 1 {
				return m, errors.New("Catalog requires DeviceList Num")
			}
			num, err := nonnegative(list.Attrs["Num"])
			if err != nil || num == 0 || num != len(list.Children) || num > 10000 || num > m.SumNum || strings.TrimSpace(list.Text) != "" {
				return m, errors.New("invalid Catalog Num/SumNum")
			}
			ids := map[string]bool{}
			for _, n := range list.Children {
				item, err := parseItem(n)
				if err != nil {
					return m, err
				}
				if ids[item.DeviceID] {
					return m, errors.New("duplicate Catalog identity")
				}
				ids[item.DeviceID] = true
				m.Items = append(m.Items, item)
			}
		}
	default:
		return m, errors.New("unsupported XML root or CmdType")
	}
	if e != nil {
		return m, e
	}
	for _, c := range n.Children {
		if c.Name == "ExtraInfo" && utf8.RuneCountInString(c.Text) > 1024 {
			return m, errors.New("ExtraInfo exceeds standard limit")
		}
	}
	return m, nil
}

func nonnegative(s string) (int, error) {
	if !digits.MatchString(s) {
		return 0, errors.New("invalid nonnegative number")
	}
	v, e := strconv.Atoi(s)
	if e != nil || v < 0 {
		return 0, errors.New("number exceeds product range")
	}
	return v, nil
}
func TimeRange(start, end string) (time.Time, time.Time, error) {
	var a, b time.Time
	var e error
	parse := func(s string) (time.Time, error) {
		if strings.HasSuffix(s, "Z") || len(s) > 19 {
			return time.Parse(time.RFC3339, s)
		}
		return time.ParseInLocation("2006-01-02T15:04:05", s, time.FixedZone("CST", 8*3600))
	}
	if start != "" {
		a, e = parse(start)
		if e != nil {
			return a, b, errors.New("invalid StartTime")
		}
	}
	if end != "" {
		b, e = parse(end)
		if e != nil {
			return a, b, errors.New("invalid EndTime")
		}
	}
	if !a.IsZero() && !b.IsZero() && a.After(b) {
		return a, b, errors.New("StartTime is after EndTime")
	}
	return a, b, nil
}

func parseItem(n *Node) (Item, error) {
	var i Item
	if n.Name != "Item" {
		return i, errors.New("invalid Catalog item")
	}
	if e := fields(n, "DeviceID", "Name", "Manufacturer", "Model", "CivilCode", "Address", "Parental", "ParentID", "RegisterWay", "Secrecy", "Status", "Info?"); e != nil {
		return i, e
	}
	i = Item{DeviceID: n.Value("DeviceID"), Name: n.Value("Name"), Manufacturer: n.Value("Manufacturer"), Model: n.Value("Model"), CivilCode: n.Value("CivilCode"), Address: n.Value("Address"), ParentID: n.Value("ParentID"), Status: n.Value("Status")}
	for _, c := range n.Children {
		if c.Name != "Info" && scalar(c) != nil {
			return i, errors.New("invalid Catalog scalar")
		}
	}
	var e error
	i.Parental, e = nonnegative(n.Value("Parental"))
	if e != nil {
		return i, e
	}
	i.RegisterWay, e = nonnegative(n.Value("RegisterWay"))
	if e != nil {
		return i, e
	}
	i.Secrecy, e = nonnegative(n.Value("Secrecy"))
	if e != nil {
		return i, e
	}
	if info := n.Child("Info"); info != nil {
		if e = fields(info, "GrassrootsCode", "PointType?"); e != nil {
			return i, e
		}
		for _, c := range info.Children {
			if scalar(c) != nil {
				return i, errors.New("invalid Info scalar")
			}
		}
		i.GrassrootsCode = info.Value("GrassrootsCode")
		if c := info.Child("PointType"); c != nil {
			i.PointType, e = nonnegative(c.Text)
			if e != nil {
				return i, e
			}
		}
	}
	return i, i.Validate()
}
func (i Item) Validate() error {
	if !ValidID(i.DeviceID) || !ValidID(i.ParentID) || i.DeviceID == i.ParentID {
		return errors.New("invalid Catalog identity relation")
	}
	for _, s := range []string{i.Name, i.Manufacturer, i.Model, i.Address} {
		if !ValidText(s) {
			return errors.New("invalid Catalog text")
		}
	}
	if !digits.MatchString(i.CivilCode) || (len(i.CivilCode) != 2 && len(i.CivilCode) != 4 && len(i.CivilCode) != 6 && len(i.CivilCode) != 8) {
		return errors.New("invalid CivilCode")
	}
	if i.Parental < 0 || i.Parental > 1 || i.RegisterWay != 1 || i.Secrecy != 0 || (i.Status != "ON" && i.Status != "OFF") {
		return errors.New("unsupported M0 Catalog attributes")
	}
	typ := i.DeviceID[10:13]
	if typ == "131" || typ == "132" {
		if i.PointType != 9 || len(i.GrassrootsCode) != 6 || !digits.MatchString(i.GrassrootsCode) {
			return errors.New("M0 cameras require PointType 9 and GrassrootsCode")
		}
	}
	return nil
}

func field(b *strings.Builder, name, value string) {
	b.WriteString("<" + name + ">")
	xml.EscapeText(b, []byte(value))
	b.WriteString("</" + name + ">\r\n")
}
func XML(m Message) ([]byte, error) {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"GB18030\"?>\r\n<" + m.Root + ">\r\n")
	field(&b, "CmdType", m.CmdType)
	field(&b, "SN", m.SN)
	field(&b, "DeviceID", m.DeviceID)
	switch {
	case m.Root == "Query":
		if m.StartTime != "" {
			field(&b, "StartTime", m.StartTime)
		}
		if m.EndTime != "" {
			field(&b, "EndTime", m.EndTime)
		}
	case m.CmdType == "Keepalive":
		field(&b, "Status", m.Status)
		if len(m.Faults) == 0 {
			b.WriteString("<Info></Info>\r\n")
		} else {
			b.WriteString("<Info>\r\n")
			for _, id := range m.Faults {
				field(&b, "DeviceID", id)
			}
			b.WriteString("</Info>\r\n")
		}
	case m.CmdType == "DeviceInfo":
		if m.DeviceName != "" {
			field(&b, "DeviceName", m.DeviceName)
		}
		field(&b, "Result", m.Result)
		for _, p := range [][2]string{{"Manufacturer", m.Manufacturer}, {"Model", m.Model}, {"Firmware", m.Firmware}} {
			if p[1] != "" {
				field(&b, p[0], p[1])
			}
		}
		if m.Channel != nil {
			field(&b, "Channel", strconv.Itoa(*m.Channel))
		}
	case m.CmdType == "Catalog":
		field(&b, "SumNum", strconv.Itoa(m.SumNum))
		if len(m.Items) > 0 {
			b.WriteString(fmt.Sprintf("<DeviceList Num=\"%d\">\r\n", len(m.Items)))
			for _, i := range m.Items {
				b.WriteString("<Item>\r\n")
				for _, p := range [][2]string{{"DeviceID", i.DeviceID}, {"Name", i.Name}, {"Manufacturer", i.Manufacturer}, {"Model", i.Model}, {"CivilCode", i.CivilCode}, {"Address", i.Address}, {"Parental", strconv.Itoa(i.Parental)}, {"ParentID", i.ParentID}, {"RegisterWay", strconv.Itoa(i.RegisterWay)}, {"Secrecy", strconv.Itoa(i.Secrecy)}, {"Status", i.Status}} {
					field(&b, p[0], p[1])
				}
				if i.GrassrootsCode != "" {
					b.WriteString("<Info>\r\n")
					field(&b, "GrassrootsCode", i.GrassrootsCode)
					if i.PointType != 0 {
						field(&b, "PointType", strconv.Itoa(i.PointType))
					}
					b.WriteString("</Info>\r\n")
				}
				b.WriteString("</Item>\r\n")
			}
			b.WriteString("</DeviceList>\r\n")
		}
	}
	b.WriteString("</" + m.Root + ">\r\n")
	raw, e := Encode(b.String())
	if e != nil {
		return nil, e
	}
	if _, e = ParseMessage(raw); e != nil {
		return nil, e
	}
	return raw, nil
}

// Batches works on an immutable snapshot. The sender must await each SIP
// acknowledgement before sending the next batch. No private batch field exists.
func Batches(sn, target string, items []Item, limit int) ([][]byte, error) {
	if limit < 1 || limit > 10000 {
		return nil, errors.New("batch limit must be 1..10000")
	}
	if len(items) > 20000 {
		return nil, errors.New("Catalog exceeds isolated fixture budget")
	}
	var out [][]byte
	if len(items) == 0 {
		b, e := XML(Message{Root: "Response", CmdType: "Catalog", SN: sn, DeviceID: target})
		return [][]byte{b}, e
	}
	for start := 0; start < len(items); start += limit {
		end := min(start+limit, len(items))
		b, e := XML(Message{Root: "Response", CmdType: "Catalog", SN: sn, DeviceID: target, SumNum: len(items), Items: items[start:end]})
		if e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	return out, nil
}

type Catalog struct {
	Target, SN  string
	Total       int
	Items       []Item
	seen        map[string]bool
	initialized bool
}

func (c *Catalog) Add(m Message) (bool, error) {
	if m.Root != "Response" || m.CmdType != "Catalog" || m.SN != c.SN || m.DeviceID != c.Target {
		return false, errors.New("Catalog correlation mismatch")
	}
	if m.SumNum > 20000 {
		return false, errors.New("Catalog aggregation budget exceeded")
	}
	if !c.initialized {
		c.Total = m.SumNum
		c.seen = map[string]bool{}
		c.initialized = true
	} else if c.Total != m.SumNum {
		return false, errors.New("Catalog total changed")
	}
	for _, i := range m.Items {
		if c.seen[i.DeviceID] {
			return false, errors.New("duplicate Catalog batch or identity")
		}
	}
	if len(c.Items)+len(m.Items) > c.Total {
		return false, errors.New("Catalog exceeds advertised total")
	}
	for _, i := range m.Items {
		c.seen[i.DeviceID] = true
		c.Items = append(c.Items, i)
	}
	return len(c.Items) == c.Total, nil
}
