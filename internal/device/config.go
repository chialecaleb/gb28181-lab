package device

import (
	"errors"
	"github.com/chialecaleb/gb28181-lab/internal/gb2022"
	"strings"
	"time"
)

type Channel struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Manufacturer string    `json:"manufacturer"`
	Model        string    `json:"model"`
	CivilCode    string    `json:"civilCode"`
	Address      string    `json:"address"`
	AddedAt      time.Time `json:"addedAt"`
}
type Definition struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Manufacturer string    `json:"manufacturer"`
	Model        string    `json:"model"`
	Firmware     string    `json:"firmware"`
	CivilCode    string    `json:"civilCode"`
	Address      string    `json:"address"`
	AddedAt      time.Time `json:"addedAt"`
	Channels     []Channel `json:"channels"`
}

func Default() Definition {
	return Definition{ID: "34020000001180000001", Name: "实验设备 01", Manufacturer: "GB28181 Lab", Model: "M0-NVR", Firmware: "0.1.0", CivilCode: "340200", Address: "合成实验空间", Channels: []Channel{{ID: "34020000001310000001", Name: "实验通道 01", Manufacturer: "GB28181 Lab", Model: "M0-Camera", CivilCode: "340200", Address: "合成实验点位"}}}
}
func (d Definition) Validate() error {
	if !gb2022.ValidID(d.ID) || d.ID[10:13] != "118" {
		return errors.New("M0 device requires a 20-digit NVR identity (type 118)")
	}
	if len(d.Channels) > 32 {
		return errors.New("M0 supports at most 32 configured channels")
	}
	for _, s := range []string{d.Name, d.Manufacturer, d.Model, d.Firmware, d.Address} {
		if !gb2022.ValidText(s) || s == "" {
			return errors.New("device descriptions must be nonempty single-line text up to 256 characters")
		}
	}
	if e := d.item("34020000002000000001").Validate(); e != nil {
		return e
	}
	seen := map[string]bool{d.ID: true}
	for _, c := range d.Channels {
		if !gb2022.ValidID(c.ID) || (c.ID[10:13] != "131" && c.ID[10:13] != "132") || seen[c.ID] {
			return errors.New("invalid or duplicate channel identity")
		}
		seen[c.ID] = true
		if e := c.item(d.ID).Validate(); e != nil {
			return e
		}
		if strings.TrimSpace(c.Name) == "" {
			return errors.New("channel name is required")
		}
	}
	return nil
}
func (d Definition) item(platform string) gb2022.Item {
	p := 0
	if len(d.Channels) > 0 {
		p = 1
	}
	return gb2022.Item{DeviceID: d.ID, Name: d.Name, Manufacturer: d.Manufacturer, Model: d.Model, CivilCode: d.CivilCode, Address: d.Address, Parental: p, ParentID: platform, RegisterWay: 1, Status: "ON"}
}
func (c Channel) item(parent string) gb2022.Item {
	return gb2022.Item{DeviceID: c.ID, Name: c.Name, Manufacturer: c.Manufacturer, Model: c.Model, CivilCode: c.CivilCode, Address: c.Address, ParentID: parent, RegisterWay: 1, Status: "ON", GrassrootsCode: "000000", PointType: 9}
}
func (d Definition) Catalog(platform, target, start, end string, now time.Time) ([]gb2022.Item, error) {
	a, b, e := gb2022.TimeRange(start, end)
	if e != nil {
		return nil, e
	}
	if b.IsZero() {
		b = now
	}
	in := func(t time.Time) bool { return (a.IsZero() || !t.Before(a)) && !t.After(b) }
	items := []gb2022.Item{}
	if target == d.ID {
		if in(d.AddedAt) {
			items = append(items, d.item(platform))
		}
		for _, c := range d.Channels {
			if in(c.AddedAt) {
				items = append(items, c.item(d.ID))
			}
		}
		return items, nil
	}
	for _, c := range d.Channels {
		if c.ID == target {
			if in(c.AddedAt) {
				items = append(items, c.item(d.ID))
			}
			return items, nil
		}
	}
	return nil, errors.New("unknown Catalog target")
}
func (d Definition) Info(sn, target string) (gb2022.Message, error) {
	if target == d.ID {
		n := len(d.Channels)
		return gb2022.Message{Root: "Response", CmdType: "DeviceInfo", SN: sn, DeviceID: target, DeviceName: d.Name, Result: "OK", Manufacturer: d.Manufacturer, Model: d.Model, Firmware: d.Firmware, Channel: &n}, nil
	}
	for _, c := range d.Channels {
		if c.ID == target {
			n := 1
			return gb2022.Message{Root: "Response", CmdType: "DeviceInfo", SN: sn, DeviceID: target, DeviceName: c.Name, Result: "OK", Manufacturer: c.Manufacturer, Model: c.Model, Firmware: d.Firmware, Channel: &n}, nil
		}
	}
	return gb2022.Message{}, errors.New("unknown DeviceInfo target")
}

// Stamp keeps creation times server-owned and stable across edits.
func (d *Definition) Stamp(old Definition, now time.Time) {
	d.AddedAt = now
	if old.ID == d.ID && !old.AddedAt.IsZero() {
		d.AddedAt = old.AddedAt
	}
	for i := range d.Channels {
		d.Channels[i].AddedAt = now
		for _, c := range old.Channels {
			if c.ID == d.Channels[i].ID && !c.AddedAt.IsZero() {
				d.Channels[i].AddedAt = c.AddedAt
				break
			}
		}
	}
}
func (d Definition) Clone() Definition { d.Channels = append([]Channel{}, d.Channels...); return d }
