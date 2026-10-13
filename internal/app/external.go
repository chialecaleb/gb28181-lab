package app

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// LoadExternal reads an optional protected host configuration. It is deliberately
// separate from dotenv and never consumes TEST_* or returns credentials to UI.
func LoadExternal(dataDir string) (*Binding, error) {
	f, e := os.Open(filepath.Join(dataDir, "binding.external.json"))
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	if e != nil {
		return nil, errors.New("external binding file unavailable")
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 8193))
	d.DisallowUnknownFields()
	var b Binding
	if d.Decode(&b) != nil || d.Decode(new(any)) != io.EOF {
		return nil, errors.New("invalid external binding file")
	}
	return &b, nil
}
